package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type staticAsset struct {
	data        []byte
	gzip        []byte
	contentType string
	etag        string
	gzipETag    string
}

func (s *Server) loadStatic(name string) (*staticAsset, bool) {
	if cached, ok := s.staticCache.Load(name); ok {
		return cached.(*staticAsset), true
	}
	if s.web == nil {
		return nil, false
	}
	data, err := fs.ReadFile(s.web, name)
	if err != nil {
		return nil, false
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	asset := &staticAsset{data: data, contentType: contentType}
	sum := sha256.Sum256(data)
	asset.etag = `"` + hex.EncodeToString(sum[:12]) + `"`
	if len(data) >= 1024 && compressible(contentType) {
		if gz, gzipErr := gzipBody(data); gzipErr == nil {
			asset.gzip = gz
			gzipSum := sha256.Sum256(gz)
			asset.gzipETag = `"` + hex.EncodeToString(gzipSum[:12]) + `-gzip"`
		}
	}
	actual, _ := s.staticCache.LoadOrStore(name, asset)
	return actual.(*staticAsset), true
}

// serveFrontend 从构建时嵌入的文件系统里提供前端资源，未知路径回落到单页入口。
func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "只支持 GET / HEAD", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || strings.HasSuffix(name, "/") {
		name = "index.html"
	}
	asset, ok := s.loadStatic(name)
	if !ok {
		// 前端用 history 路由时，刷新子路径也要拿到入口文件。
		name = "index.html"
		asset, ok = s.loadStatic(name)
		if !ok {
			http.Error(w, "前端资源尚未构建，请先执行 build.bat / build.sh", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", asset.contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	body := asset.data
	etag := asset.etag
	useGzip := len(asset.gzip) > 0 && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	if useGzip {
		body = asset.gzip
		etag = asset.gzipETag
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// serveUpload 提供用户自己上传的菜品照片，路径只允许单层文件名。
func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/uploads/")
	// thumb/ 是列表用的 400px 缩略图，文件名与原图一致，可长缓存
	sub := ""
	if strings.HasPrefix(name, "thumb/") {
		sub = "thumb/"
		name = strings.TrimPrefix(name, "thumb/")
	}
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	if sub == "" {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=604800")
	}
	http.ServeFile(w, r, filepath.Join(s.st.Dir(), "uploads", sub, name))
}
