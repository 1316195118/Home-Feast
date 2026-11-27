package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"log"
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

const defaultIndexTitle = "家里吃点啥｜今天吃什么，全家一起定"
const defaultIndexDescription = "全家一起挑菜、点菜、分工和看记录。237 道家常菜，数据只留在自己的设备。"
const defaultIndexHTMLDescription = "全家一起挑菜、点菜、分工和看记录的家庭点菜系统，237 道家常菜，数据保存在自己的 NAS 或本机。"

type indexPageMeta struct {
	title       string
	description string
	canonical   string
	ogType      string
	noindex     bool
}

func indexMetaFor(requested string, status int) indexPageMeta {
	switch requested {
	case "privacy":
		return indexPageMeta{
			title:       "隐私政策｜家里吃点啥",
			description: "了解家里吃点啥会保存哪些家庭数据、数据存放在哪里、如何使用、如何导出删除，以及当前第三方服务情况。",
			canonical:   "/privacy",
			ogType:      "article",
		}
	case "terms":
		return indexPageMeta{
			title:       "服务条款｜家里吃点啥",
			description: "查看家里吃点啥的使用规则、家庭内容责任、知识产权、免责声明、服务终止和争议处理方式。",
			canonical:   "/terms",
			ogType:      "article",
		}
	case "", "index.html":
		return indexPageMeta{
			title:       defaultIndexTitle,
			description: defaultIndexDescription,
			canonical:   "/",
			ogType:      "website",
		}
	default:
		return indexPageMeta{
			title:       "页面未找到｜家里吃点啥",
			description: "这个页面可能被撤掉或地址输错了。回到今天、去菜谱挑菜，或查看家人与用餐记录。",
			canonical:   "/",
			ogType:      "website",
			noindex:     status == http.StatusNotFound,
		}
	}
}

func renderIndexHTML(data []byte, requested string, status int, baseURL string) []byte {
	base := html.EscapeString(strings.TrimRight(baseURL, "/"))
	meta := indexMetaFor(requested, status)
	out := bytes.ReplaceAll(data, []byte("__SITE_URL__"), []byte(base))
	out = bytes.ReplaceAll(out, []byte(defaultIndexTitle), []byte(html.EscapeString(meta.title)))
	out = bytes.ReplaceAll(out, []byte(defaultIndexHTMLDescription), []byte(html.EscapeString(meta.description)))
	out = bytes.ReplaceAll(out, []byte(defaultIndexDescription), []byte(html.EscapeString(meta.description)))
	if meta.canonical != "/" {
		canonical := base + html.EscapeString(meta.canonical)
		out = bytes.Replace(out, []byte(`href="`+base+`/"`), []byte(`href="`+canonical+`"`), 1)
		out = bytes.Replace(out, []byte(`content="`+base+`/"`), []byte(`content="`+canonical+`"`), 1)
	}
	if meta.ogType != "website" {
		out = bytes.Replace(out, []byte(`property="og:type" content="website"`), []byte(`property="og:type" content="`+html.EscapeString(meta.ogType)+`"`), 1)
	}
	if meta.noindex {
		out = bytes.Replace(out, []byte("</head>"), []byte(`<meta name="robots" content="noindex, nofollow" />`+"\n  </head>"), 1)
	}
	return out
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
	requested := name
	if name == "" {
		name = "index.html"
	}
	asset, ok := s.loadStatic(name)
	status := http.StatusOK
	if !ok {
		// 缺失的脚本、样式、图片必须老老实实返回 404，不能伪装成 HTML。
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		if requested != "privacy" && requested != "terms" {
			status = http.StatusNotFound
		}
		// 前端用 history 路由时，刷新子路径也要拿到入口文件。
		name = "index.html"
		asset, ok = s.loadStatic(name)
		if !ok {
			http.Error(w, "前端资源尚未构建，请先执行 build.bat / build.sh", http.StatusInternalServerError)
			return
		}
	}
	// 社交平台读取的是原始 HTML，不执行 Vue。服务端按当前访问域名和公开页
	// 直接写入 canonical / OG / Twitter，因此同一镜像换域名部署也无需重构建。
	if name == "index.html" && bytes.Contains(asset.data, []byte("__SITE_URL__")) {
		baseURL := publicBaseURL(r)
		// baseURL + 路由 + 状态决定唯一渲染结果；同一镜像换域名时 key 自然不同。
		key := baseURL + "\x00" + requested + "\x00" + strconv.Itoa(status)
		if hit, ok := s.indexCache.Load(key); ok {
			asset = hit.(*staticAsset)
		} else {
			data := renderIndexHTML(asset.data, requested, status, baseURL)
			rendered := &staticAsset{data: data, contentType: asset.contentType}
			sum := sha256.Sum256(data)
			rendered.etag = `"` + hex.EncodeToString(sum[:12]) + `"`
			if len(data) >= 1024 && compressible(rendered.contentType) {
				if br, err := brotliBody(data); err == nil {
					rendered.brotli = br
					brotliSum := sha256.Sum256(br)
					rendered.brotliETag = `"` + hex.EncodeToString(brotliSum[:12]) + `-br"`
				}
				if gz, err := gzipBody(data); err == nil {
					rendered.gzip = gz
					gzipSum := sha256.Sum256(gz)
					rendered.gzipETag = `"` + hex.EncodeToString(gzipSum[:12]) + `-gzip"`
				}
			}
			actual, _ := s.indexCache.LoadOrStore(key, rendered)
			asset = actual.(*staticAsset)
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
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func (s *Server) serveRobots(w http.ResponseWriter, r *http.Request) {
	body := "User-agent: *\nAllow: /\n" +
		"Disallow: /api/\n" +
		"Disallow: /uploads/\n" +
		"Disallow: /thumb-bundle\n" +
		"Disallow: /config/\n\n" +
		"Sitemap: " + publicBaseURL(r) + "/sitemap.xml\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body))
	}
}

func (s *Server) serveSitemap(w http.ResponseWriter, r *http.Request) {
	base := html.EscapeString(publicBaseURL(r))
	lastmod := time.Now().UTC().Format("2006-01-02")
	body := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
		`<url><loc>` + base + `</loc><lastmod>` + lastmod + `</lastmod><changefreq>daily</changefreq><priority>1.0</priority></url>` +
		`<url><loc>` + base + `/privacy</loc><lastmod>` + lastmod + `</lastmod><changefreq>yearly</changefreq><priority>0.3</priority></url>` +
		`<url><loc>` + base + `/terms</loc><lastmod>` + lastmod + `</lastmod><changefreq>yearly</changefreq><priority>0.3</priority></url>` +
		`</urlset>` + "\n"
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body))
	}
}

func publicBaseURL(r *http.Request) string {
	if configured := strings.TrimSpace(os.Getenv("MENU_SITE_URL")); configured != "" {
		return strings.TrimRight(configured, "/")
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
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
	// modTime 存的是 UnixNano，ServeContent 需要 time.Time；用 (0, nano) 还原，
	// 若误当秒传会变成公元五千万年，If-Modified-Since 永远不匹配。
	http.ServeContent(w, r, name, time.Unix(0, thumb.modTime), bytes.NewReader(thumb.data))
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

	// 关键：无论正常返回还是 panic，都必须写回结果、移除 loading 条目并关闭 done。
	// 否则解码一旦 panic，所有后续请求都会在 <-load.done 永久挂住，而 net/http
	// 的 recover 只救得了当前 goroutine，其余等待者会被一起拖死。
	var thumb *generatedThumb
	var readErr error
	defer func() {
		if v := recover(); v != nil {
			log.Printf("生成缩略图 %s 时 panic: %v", base, v)
			thumb = nil
			readErr = fmt.Errorf("生成缩略图失败: %v", v)
		}
		s.thumbMu.Lock()
		if thumb != nil && generation == s.thumbGeneration {
			s.thumbCache[base] = thumb
		}
		load.thumb, load.err = thumb, readErr
		if s.thumbLoads[base] == load {
			delete(s.thumbLoads, base)
		}
		close(load.done)
		s.thumbMu.Unlock()
	}()

	data, err := os.ReadFile(source)
	if err != nil {
		readErr = err
		return nil, err
	}
	rendered, err := renderThumbJPEG(data)
	if err != nil {
		readErr = err
		return nil, err
	}
	if len(rendered) == 0 {
		readErr = errors.New("原图尺寸无效")
		return nil, readErr
	}
	sum := sha256.Sum256(rendered)
	thumb = &generatedThumb{
		source: source, modTime: info.ModTime().UnixNano(), size: info.Size(),
		data: rendered, etag: `"` + hex.EncodeToString(sum[:12]) + `-generated"`,
	}
	return thumb, nil
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
