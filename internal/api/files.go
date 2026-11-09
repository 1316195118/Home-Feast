package api

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
	name := randomName() + ext
	if err = os.WriteFile(filepath.Join(s.st.Dir(), "uploads", name), data, 0o644); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存图片失败")
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]string{"url": "/uploads/" + name})
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

// exportBackup 把四份 JSON 与上传的照片打成一个 zip，供 NAS 定期备份。
func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="family-menu-%s.zip"`, time.Now().Format("20060102")))
	zw := zip.NewWriter(w)
	for _, name := range jsonNames {
		b, err := os.ReadFile(filepath.Join(s.st.Dir(), name))
		if err != nil {
			continue
		}
		entry, err := zw.Create(name)
		if err == nil {
			_, _ = entry.Write(b)
		}
	}
	root := s.st.Dir()
	_ = filepath.WalkDir(filepath.Join(root, "uploads"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		entry, err := zw.Create(filepath.ToSlash(rel))
		if err == nil {
			_, _ = entry.Write(b)
		}
		return nil
	})
	_ = zw.Close()
}

// importBackup 只恢复白名单里的文件，忽略压缩包中的其它路径，防止目录穿越。
func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "请选择备份 zip 文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 40<<20))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "读取备份失败")
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "这不是有效的 zip 备份")
		return
	}
	var total int64
	restored := 0
	for _, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		if !backupAllowed(name) || zf.UncompressedSize64 > 8<<20 {
			continue
		}
		total += int64(zf.UncompressedSize64)
		if total > 64<<20 {
			jsonErr(w, http.StatusBadRequest, "备份内容过大")
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
		dest := filepath.Join(s.st.Dir(), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			continue
		}
		if err := atomicFile(dest, b); err == nil {
			restored++
		}
	}
	if restored == 0 {
		jsonErr(w, http.StatusBadRequest, "备份里没有可恢复的数据文件")
		return
	}
	if err := s.st.Reload(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "恢复后读取数据失败: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "restored": restored})
}

func backupAllowed(name string) bool {
	for _, n := range jsonNames {
		if name == n {
			return true
		}
	}
	if !strings.HasPrefix(name, "uploads/") {
		return false
	}
	base := strings.TrimPrefix(name, "uploads/")
	if base == "" || strings.Contains(base, "/") || strings.Contains(base, "..") {
		return false
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
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
