package api

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

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
	data, err := fs.ReadFile(s.web, name)
	if err != nil {
		// 前端用 history 路由时，刷新子路径也要拿到入口文件。
		name = "index.html"
		data, err = fs.ReadFile(s.web, name)
		if err != nil {
			http.Error(w, "前端资源尚未构建，请先执行 build.bat / build.sh", http.StatusInternalServerError)
			return
		}
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

// serveUpload 提供用户自己上传的菜品照片，路径只允许单层文件名。
func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(s.st.Dir(), "uploads", name))
}
