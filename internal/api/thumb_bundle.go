package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxThumbBundleItems = 320
	maxThumbBundleBytes = 32 << 20
)

type thumbBundleRequest struct {
	URLs []string `json:"urls"`
}

type thumbBundleEntry struct {
	URL    string `json:"url"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
	Type   string `json:"type"`
}

// serveThumbBundle 把一页需要的缩略图合成一次响应，绕开 HTTP/1.1 的同域连接上限。
// 只读上传缩略图 / 内嵌菜系图，不接受任意文件路径；单个响应最多 32 MiB。
func (s *Server) serveThumbBundle(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		jsonErr(w, http.StatusForbidden, "跨站请求已拒绝")
		return
	}
	var input thumbBundleRequest
	if err := decodeBody(w, r, &input); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(input.URLs) == 0 {
		jsonErr(w, http.StatusBadRequest, "至少需要一张缩略图")
		return
	}
	if len(input.URLs) > maxThumbBundleItems {
		jsonErr(w, http.StatusBadRequest, "一次最多读取 320 张缩略图")
		return
	}

	entries := make([]thumbBundleEntry, 0, len(input.URLs))
	var payload bytes.Buffer
	seen := make(map[string]bool, len(input.URLs))
	for _, rawURL := range input.URLs {
		if seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		data, contentType, ok := s.readThumbBundleAsset(rawURL)
		if !ok || len(data) == 0 {
			continue
		}
		if payload.Len()+len(data) > maxThumbBundleBytes {
			break
		}
		entries = append(entries, thumbBundleEntry{
			URL: rawURL, Offset: payload.Len(), Size: len(data), Type: contentType,
		})
		_, _ = payload.Write(data)
	}
	if len(entries) == 0 {
		jsonErr(w, http.StatusNotFound, "没有可读取的缩略图")
		return
	}

	header, err := json.Marshal(entries)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "生成缩略图索引失败")
		return
	}
	total := 4 + len(header) + payload.Len()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(total))
	w.WriteHeader(http.StatusOK)
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(header)))
	_, _ = w.Write(size[:])
	_, _ = w.Write(header)
	_, _ = w.Write(payload.Bytes())
}

func (s *Server) readThumbBundleAsset(rawURL string) ([]byte, string, bool) {
	if strings.HasPrefix(rawURL, "/uploads/thumb/") {
		name := strings.TrimPrefix(rawURL, "/uploads/thumb/")
		if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
			return nil, "", false
		}
		target := filepath.Join(s.st.Dir(), "uploads", "thumb", name)
		if data, err := os.ReadFile(target); err == nil {
			return data, "image/jpeg", true
		}
		if !strings.HasSuffix(strings.ToLower(name), ".jpg") {
			return nil, "", false
		}
		thumb, err := s.generatedUploadThumb(name[:len(name)-len(".jpg")])
		if err != nil || thumb == nil {
			return nil, "", false
		}
		return thumb.data, "image/jpeg", true
	}

	if !strings.HasPrefix(rawURL, "/dish-photo/thumb/") && !strings.HasPrefix(rawURL, "/dishes/") {
		return nil, "", false
	}
	name := strings.TrimPrefix(rawURL, "/")
	if strings.Contains(name, "..") || strings.Contains(name, `\`) {
		return nil, "", false
	}
	asset, ok := s.loadStatic(name)
	if !ok {
		return nil, "", false
	}
	contentType := asset.contentType
	if !strings.HasPrefix(contentType, "image/") {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}
	return asset.data, contentType, true
}
