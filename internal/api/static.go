package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

type staticAsset struct {
	data        []byte
	gzip        []byte
	brotli      []byte
	contentType string
	etag        string
	gzipETag    string
	brotliETag  string
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
		// 构建脚本会预生成 .br / .gz；直接嵌入，避免线上第一次请求承担压缩成本。
		if precompressed, readErr := fs.ReadFile(s.web, name+".br"); readErr == nil && len(precompressed) > 0 {
			asset.brotli = precompressed
		} else if br, brotliErr := brotliBody(data); brotliErr == nil {
			asset.brotli = br
		}
		if len(asset.brotli) > 0 {
			brotliSum := sha256.Sum256(asset.brotli)
			asset.brotliETag = `"` + hex.EncodeToString(brotliSum[:12]) + `-br"`
		}
		if precompressed, readErr := fs.ReadFile(s.web, name+".gz"); readErr == nil && len(precompressed) > 0 {
			asset.gzip = precompressed
		} else if gz, gzipErr := gzipBody(data); gzipErr == nil {
			asset.gzip = gz
		}
		if len(asset.gzip) > 0 {
			gzipSum := sha256.Sum256(asset.gzip)
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
	acceptEncoding := r.Header.Get("Accept-Encoding")
	if len(asset.brotli) > 0 && acceptsEncoding(acceptEncoding, "br") {
		body = asset.brotli
		etag = asset.brotliETag
		w.Header().Set("Content-Encoding", "br")
		w.Header().Set("Vary", "Accept-Encoding")
	} else if len(asset.gzip) > 0 && acceptsEncoding(acceptEncoding, "gzip") {
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

func brotliBody(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := brotli.NewWriter(&buf)
	if _, err := zw.Write(body); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	// 上传图沿用固定文件名，恢复备份或重新上传时内容可能原地变化。
	// no-cache 允许浏览器长期保存，但每次使用前用 Last-Modified/ETag 校验，
	// 既不会重复下载未变化图片，也不会再把旧图固定缓存一整天。
	w.Header().Set("Cache-Control", "public, no-cache")
	target := filepath.Join(s.st.Dir(), "uploads", sub, name)
	if sub == "thumb/" {
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			http.ServeFile(w, r, target)
			return
		}
		// 历史图片、恢复备份或旧版本数据可能只有原图没有缩略图。
		// 这里直接从原图生成并缓存在内存里，避免前端先吃 404 再下载大图。
		if s.serveGeneratedThumb(w, r, name) {
			return
		}
	}
	http.ServeFile(w, r, target)
}

var thumbSourceExts = [...]string{".webp", ".jpg", ".jpeg", ".png", ".gif"}

// serveGeneratedThumb 返回 true 表示已经生成了响应；false 表示没有对应原图。
func (s *Server) serveGeneratedThumb(w http.ResponseWriter, r *http.Request, name string) bool {
	const ext = ".jpg"
	if !strings.HasSuffix(strings.ToLower(name), ext) {
		return false
	}
	base := name[:len(name)-len(ext)]
	if base == "" || strings.ContainsAny(base, `/\`) || strings.Contains(base, "..") {
		return false
	}
	thumb, err := s.generatedUploadThumb(base)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, "缩略图生成失败", http.StatusInternalServerError)
		return true
	}
	if err != nil {
		return false
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, no-cache")
	w.Header().Set("ETag", thumb.etag)
	if r.Header.Get("If-None-Match") == thumb.etag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	http.ServeContent(w, r, name, time.Unix(thumb.modTime, 0), bytes.NewReader(thumb.data))
	return true
}

// generatedUploadThumb 对同一个缺失缩略图只生成一次。不同图片仍可并行解码，
// 不会因为一把全局大锁把整页图片串行拖慢。
func (s *Server) generatedUploadThumb(base string) (*generatedThumb, error) {
	source, info, err := s.findThumbSource(base)
	if err != nil {
		return nil, err
	}

	s.thumbMu.Lock()
	if cached := s.thumbCache[base]; cached != nil &&
		cached.source == source && cached.modTime == info.ModTime().UnixNano() && cached.size == info.Size() {
		s.thumbMu.Unlock()
		return cached, nil
	}
	if load := s.thumbLoads[base]; load != nil {
		s.thumbMu.Unlock()
		<-load.done
		return load.thumb, load.err
	}
	generation := s.thumbGeneration
	load := &generatedThumbLoad{done: make(chan struct{})}
	s.thumbLoads[base] = load
	s.thumbMu.Unlock()

	data, readErr := os.ReadFile(source)
	var rendered []byte
	if readErr == nil {
		rendered, readErr = renderThumbJPEG(data)
		if readErr == nil && len(rendered) == 0 {
			readErr = errors.New("原图尺寸无效")
		}
	}

	var thumb *generatedThumb
	if readErr == nil {
		sum := sha256.Sum256(rendered)
		thumb = &generatedThumb{
			source: source, modTime: info.ModTime().UnixNano(), size: info.Size(),
			data: rendered, etag: `"` + hex.EncodeToString(sum[:12]) + `-generated"`,
		}
	}

	s.thumbMu.Lock()
	if thumb != nil && generation == s.thumbGeneration {
		s.thumbCache[base] = thumb
	}
	load.thumb, load.err = thumb, readErr
	delete(s.thumbLoads, base)
	close(load.done)
	s.thumbMu.Unlock()
	return thumb, readErr
}

func (s *Server) findThumbSource(base string) (string, os.FileInfo, error) {
	dir := filepath.Join(s.st.Dir(), "uploads")
	for _, ext := range thumbSourceExts {
		target := filepath.Join(dir, base+ext)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			return target, info, nil
		}
	}
	return "", nil, os.ErrNotExist
}

func (s *Server) clearGeneratedThumbs() {
	s.thumbMu.Lock()
	s.thumbGeneration++
	clear(s.thumbCache)
	s.thumbMu.Unlock()
}
