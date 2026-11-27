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
	// 压缩后体积不约束像素数：先读头部尺寸，拒绝会让缩略图解码 OOM 的图。
	if tooLarge, err := imageTooLarge(data); err != nil {
		jsonErr(w, http.StatusBadRequest, "无法识别的图片格式")
		return
	} else if tooLarge {
		jsonErr(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("图片尺寸过大，单边请不超过 %d 像素且总像素不超过 %d 万", maxThumbEdge, maxThumbPixels/10000))
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
	// 手动导出、恢复和定时备份共享同一组数据/图片文件。串行化后不会出现
	// 导出到一半时图片被恢复任务覆盖，得到内部不一致的 ZIP。
	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	tmp, err := os.CreateTemp("", "fm-export-*.zip")
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "无法创建备份文件")
		return
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if err := s.writeBackupZip(tmp); err != nil {
		jsonErr(w, http.StatusInternalServerError, "生成备份失败："+err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "生成备份失败："+err.Error())
		return
	}
	f, err := os.Open(tmpPath)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "读取备份失败")
		return
	}
	defer f.Close()

	name := fmt.Sprintf("family-menu-%s.zip", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	// 备份可达数百 MB，慢速链路上 60s 全局 WriteTimeout 会掐断连接，
	// 用户拿到不完整 zip 却以为备份成功。与 SSE 一样单独豁免写超时。
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		// 不支持该接口时按原行为继续，不影响功能。
		_ = err
	}
	http.ServeContent(w, r, name, time.Now(), f)
}

// usedImages 收集当前数据真正引用到的图片文件名（不含路径）。
func (s *Server) usedImages() map[string]bool {
	media := s.st.MediaSnapshot()
	used := make(map[string]bool, len(media.Dishes))
	add := func(image string) {
		if image == "" {
			return
		}
		name := filepath.Base(image)
		if name != "" && name != "." && name != "/" {
			used[name] = true
		}
	}
	for _, d := range media.Dishes {
		add(d.Image)
	}
	for _, p := range media.Plans {
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
			if zf.UncompressedSize64 > 8<<20 {
				jsonErr(w, http.StatusBadRequest, fmt.Sprintf("%s 单张图片超过 8MB", filepath.Base(name)))
				return
			}
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

	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	// 恢复前的回滚点（只针对 JSON；图片不可逆但也不影响数据自洽）。
	// 快照本身也走原子写：崩溃在写快照中途时不会留下半个回滚点。
	stamp := time.Now().Format("20060102-150405")
	type snapshot struct{ dest, keep string; data []byte }
	var snaps []snapshot
	for _, it := range jsons {
		dest := s.st.Path(it.name)
		cur, readErr := os.ReadFile(dest)
		if readErr != nil {
			continue
		}
		keep := dest + ".pre-restore-" + stamp
		if werr := atomicFile(keep, cur); werr != nil {
			jsonErr(w, http.StatusInternalServerError, "无法创建恢复前的快照，已中止")
			return
		}
		snaps = append(snaps, snapshot{dest: dest, keep: keep, data: cur})
	}
	// 回滚同样在单个写锁内完成替换 + 重载，避免回滚又和别的写请求交错。
	rollback := func() {
		prev := make(map[string][]byte, len(snaps))
		for _, sv := range snaps {
			prev[filepath.Base(sv.dest)] = sv.data
		}
		_ = s.st.RestoreFiles(prev)
	}

	// 一次写锁内替换全部 JSON 并重载：常规写接口无法插入，
	// 已返回成功的写不会被随后的 Reload 抹掉。
	payload := make(map[string][]byte, len(jsons))
	for _, it := range jsons {
		payload[it.name] = it.data
	}
	if err := s.st.RestoreFiles(payload); err != nil {
		rollback()
		jsonErr(w, http.StatusInternalServerError, "写入数据失败，已回滚到恢复前的数据")
		return
	}

	// 图片：JSON 已就绪，这里尽力而为，失败只影响个别图片
	imgDir := filepath.Join(s.st.Dir(), "uploads")
	_ = os.MkdirAll(imgDir, 0o755)
	restoredImg := 0
	failedImg := 0
	for _, it := range images {
		if err := extractZipFileAtomic(imgDir, it.base, it.zf); err != nil {
			failedImg++
		} else {
			restoredImg++
		}
	}
	// 恢复备份可能原地覆盖同名原图，丢掉旧的内存缩略图，下一次请求按新图重建。
	s.clearGeneratedThumbs()
	jsonOK(w, map[string]any{
		"ok": true, "restored": len(jsons), "images": restoredImg,
		"imagesFailed": failedImg, "snapshot": stamp,
	})
}

func extractZipFileAtomic(root, name string, zf *zip.File) error {
	dest := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	n, err := io.Copy(tmp, io.LimitReader(rc, 8<<20+1))
	if err != nil {
		return err
	}
	if n > 8<<20 || n != int64(zf.UncompressedSize64) {
		return errors.New("解压后的图片大小不一致")
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dest)
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
	// 唯一临时名：常规写路径也用 dest+".tmp"，固定名字会互相覆盖；
	// 这里用 CreateTemp 并与目标同目录，rename 才是同一文件系统内的原子替换。
	f, err := os.CreateTemp(filepath.Dir(dest), ".fm-restore-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
