package api

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var jsonNames = []string{"dishes.json", "members.json", "plans.json", "settings.json"}

// upload 接收一张自定义菜品照片，按文件头判定真实格式后写入数据目录。
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		jsonErr(w, http.StatusBadRequest, "图片太大或表单不合法")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "请选择一张图片")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 6<<20+1))
	if err != nil || len(data) > 6<<20 {
		jsonErr(w, http.StatusRequestEntityTooLarge, "图片请控制在 6MB 以内")
		return
	}
	ext := sniffImage(data)
	if ext == "" {
		jsonErr(w, http.StatusBadRequest, "只支持 PNG / JPEG / WebP / GIF 图片")
		return
	}
	base := randomName()
	name := base + ext
	uploadDir := filepath.Join(s.st.Dir(), "uploads")
	if err = os.WriteFile(filepath.Join(uploadDir, name), data, 0o644); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存图片失败")
		return
	}
	// 顺手生成列表用的缩略图；失败不影响上传，前端会回落到原图
	_ = writeThumb(data, uploadDir, base)

	jsonStatus(w, http.StatusCreated, map[string]string{"url": "/uploads/" + name})
}

func sniffImage(b []byte) string {
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return ".png"
	case len(b) >= 3 && bytes.Equal(b[:3], []byte{0xff, 0xd8, 0xff}):
		return ".jpg"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return ".webp"
	case len(b) >= 4 && string(b[:4]) == "GIF8":
		return ".gif"
	}
	return ""
}

func randomName() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("img%d", time.Now().UnixNano())
	}
	return "img" + hex.EncodeToString(buf)
}

// exportBackup 把四份 JSON 与「被引用到的」照片打成一个 zip。
//
// 历史上这里会把整个 uploads 目录打进去，包含大量改图留下的孤儿文件，
// 备份动辄 170MB+，反而超过导入上限、恢复不回来。现在只收菜品与订单实际引用的图片。
func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="family-menu-%s.zip"`, time.Now().Format("20060102")))
	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, name := range jsonNames {
		b, err := os.ReadFile(s.st.Path(name))
		if err != nil {
			continue
		}
		entry, err := zw.Create(name)
		if err == nil {
			_, _ = entry.Write(b)
		}
	}

	used := s.usedImages()
	root := s.st.Dir()
	_ = filepath.WalkDir(filepath.Join(root, "uploads"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !used[d.Name()] {
			return nil // 没人引用的旧图不打包
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		entry, err := zw.Create(filepath.ToSlash(rel))
		if err == nil {
			_, _ = io.Copy(entry, f)
		}
		// 缩略图一起打包：否则恢复后列表只能回落原图（128KB vs 23KB），流量翻几倍。
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		thumbPath := filepath.Join(filepath.Dir(p), "thumb", base+".jpg")
		if tf, err := os.Open(thumbPath); err == nil {
			defer tf.Close()
			name := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "thumb", base+".jpg"))
			if te, err := zw.Create(name); err == nil {
				_, _ = io.Copy(te, tf)
			}
		}
		return nil
	})
}

// usedImages 收集当前数据真正引用到的图片文件名（不含路径）。
func (s *Server) usedImages() map[string]bool {
	used := make(map[string]bool, len(s.st.ListDishes()))
	add := func(image string) {
		if image == "" {
			return
		}
		name := filepath.Base(image)
		if name != "" && name != "." && name != "/" {
			used[name] = true
		}
	}
	for _, d := range s.st.ListDishes() {
		add(d.Image)
	}
	for _, p := range s.st.ListPlans() {
		for _, o := range p.Orders {
			add(o.DishImage)
		}
	}
	return used
}

// importBackup 恢复备份：四份 JSON + uploads 里的照片。
//
// 保护措施：
//  1. 先整包校验 JSON 合法性与图片路径安全，任一不通过则整体拒绝，当前数据不动；
//  2. 覆盖 JSON 前另存 .pre-restore-<时间戳> 快照，写失败或重载失败时自动回滚；
//  3. 大文件走临时文件 + 流式拷贝，避免整个 zip 读进内存。
//
// 图片只做「新增或覆盖」，删不掉多余文件；即便中途失败，残留的图片也只是孤儿文件，
// JSON 回滚后数据仍然自洽。
func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	const maxBackup = 256 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBackup+8<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "请选择备份 zip 文件")
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "fm-restore-*.zip")
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "无法创建临时文件")
		return
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()
	n, err := io.Copy(tmp, io.LimitReader(file, maxBackup+1))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "读取备份失败")
		return
	}
	if n > maxBackup {
		jsonErr(w, http.StatusBadRequest, "备份文件超过 256MB")
		return
	}

	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "这不是有效的 zip 备份")
		return
	}
	defer zr.Close()

	type jsonItem struct {
		name string
		data []byte
	}
	type imgItem struct {
		base string
		zf   *zip.File
	}
	var jsons []jsonItem
	var images []imgItem
	var total int64
	for _, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		if isJSONEntry(name) {
			if zf.UncompressedSize64 > 8<<20 {
				jsonErr(w, http.StatusBadRequest, fmt.Sprintf("%s 体积异常", filepath.Base(name)))
				return
			}
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			b, err := io.ReadAll(io.LimitReader(rc, 8<<20))
			_ = rc.Close()
			if err != nil {
				continue
			}
			if verr := validateBackupJSON(filepath.Base(name), b); verr != nil {
				jsonErr(w, http.StatusBadRequest,
					fmt.Sprintf("%s 无法恢复：%v。当前数据未改动", filepath.Base(name), verr))
				return
			}
			jsons = append(jsons, jsonItem{name: filepath.Base(name), data: b})
			continue
		}
		if base, ok := uploadEntryName(name); ok {
			total += int64(zf.UncompressedSize64)
			if total > maxBackup {
				jsonErr(w, http.StatusBadRequest, "备份里的图片总量过大")
				return
			}
			images = append(images, imgItem{base: base, zf: zf})
		}
	}
	if len(jsons) == 0 {
		jsonErr(w, http.StatusBadRequest, "备份里没有可恢复的数据文件")
		return
	}

	// 恢复前的回滚点（只针对 JSON；图片不可逆但也不影响数据自洽）
	stamp := time.Now().Format("20060102-150405")
	type snapshot struct{ dest, keep string }
	var snaps []snapshot
	for _, it := range jsons {
		dest := s.st.Path(it.name)
		cur, readErr := os.ReadFile(dest)
		if readErr != nil {
			continue
		}
		keep := dest + ".pre-restore-" + stamp
		if werr := os.WriteFile(keep, cur, 0o644); werr != nil {
			jsonErr(w, http.StatusInternalServerError, "无法创建恢复前的快照，已中止")
			return
		}
		snaps = append(snaps, snapshot{dest: dest, keep: keep})
	}
	rollback := func() {
		for _, sv := range snaps {
			if b, e := os.ReadFile(sv.keep); e == nil {
				_ = atomicFile(sv.dest, b)
			}
		}
		_ = s.st.Reload()
	}

	for _, it := range jsons {
		dest := s.st.Path(it.name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			rollback()
			jsonErr(w, http.StatusInternalServerError, "创建目录失败，已回滚")
			return
		}
		if err := atomicFile(dest, it.data); err != nil {
			rollback()
			jsonErr(w, http.StatusInternalServerError, "写入数据失败，已回滚到恢复前的数据")
			return
		}
	}
	if err := s.st.Reload(); err != nil {
		rollback()
		jsonErr(w, http.StatusInternalServerError, "恢复后读取数据失败，已回滚到恢复前的数据")
		return
	}

	// 图片：JSON 已就绪，这里尽力而为，失败只影响个别图片
	imgDir := filepath.Join(s.st.Dir(), "uploads")
	_ = os.MkdirAll(imgDir, 0o755)
	restoredImg := 0
	for _, it := range images {
		rc, err := it.zf.Open()
		if err != nil {
			continue
		}
		f, err := os.Create(filepath.Join(imgDir, it.base))
		if err != nil {
			_ = rc.Close()
			continue
		}
		_, cerr := io.Copy(f, io.LimitReader(rc, 8<<20))
		_ = rc.Close()
		if cerr == nil {
			_ = f.Sync()
		}
		_ = f.Close()
		if cerr == nil {
			restoredImg++
		}
	}
	jsonOK(w, map[string]any{
		"ok": true, "restored": len(jsons), "images": restoredImg, "snapshot": stamp,
	})
}

// validateBackupJSON 在写入之前判断备份里的数据文件是否可用：
// 语法合法、顶层结构对得上、成员不能为空（没有成员全家都点不了菜）。
func validateBackupJSON(name string, b []byte) error {
	if !json.Valid(b) {
		return errors.New("不是合法的 JSON")
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return errors.New("内容为空")
	}
	switch name {
	case "dishes.json", "members.json", "plans.json":
		if trimmed[0] != '[' {
			return errors.New("应是一个数组")
		}
		if name == "members.json" {
			var list []json.RawMessage
			if err := json.Unmarshal(trimmed, &list); err == nil && len(list) == 0 {
				return errors.New("备份里没有家庭成员，恢复后将无法点菜")
			}
		}
	case "settings.json":
		if trimmed[0] != '{' {
			return errors.New("应是一个对象")
		}
	}
	return nil
}

// uploadEntryName 只接受 uploads/ 下的扁平文件名，字符限定为字母数字与 ._-，
// 这是防 zip slip 的关键：备份里写 ../../etc/passwd 之类会被直接丢弃。
func uploadEntryName(name string) (string, bool) {
	const prefix = "uploads/"
	const thumbPrefix = "uploads/thumb/"
	sub := ""
	rest := ""
	switch {
	case strings.HasPrefix(name, thumbPrefix):
		sub, rest = "thumb/", name[len(thumbPrefix):]
	case strings.HasPrefix(name, prefix):
		rest = name[len(prefix):]
	default:
		return "", false
	}
	if rest == "" || len(rest) > 96 || strings.Contains(rest, "..") {
		return "", false
	}
	for _, r := range rest {
		ok := r == '.' || r == '_' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return "", false
		}
	}
	return sub + rest, true
}

// isJSONEntry 判断是不是四份数据文件之一（允许带 config/ 前缀）。
func isJSONEntry(name string) bool {
	for _, n := range jsonNames {
		if name == n || name == "config/"+n {
			return true
		}
	}
	return false
}

func atomicFile(dest string, b []byte) error {
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
