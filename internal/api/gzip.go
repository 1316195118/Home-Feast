package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// gzip 中间件：按 Content-Type 决定是否压缩，避免把已经压过的图片/zip 再包一层。
//
// 几个刻意的取舍：
//   - /api/events 是 SSE，必须逐条 flush，压缩会破坏实时性 → 直接跳过
//   - 带 Range 的请求（视频/断点续传语义）不压缩 → 跳过
//   - Content-Length 在压缩后必须删掉，否则浏览器按原长度读会截断
//   - 204/304 这类无实体响应不压缩，避免出现「有 Content-Encoding 但无内容」
type gzipResponseWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	gzipped bool
	decided bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if !w.decided {
		w.decided = true
		// 只有 200 且内容类型可压缩时才真正压
		if code == http.StatusOK && compressible(w.Header().Get("Content-Type")) {
			w.gzipped = true
			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Add("Vary", "Accept-Encoding")
			w.gz.Reset(w.ResponseWriter)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.decided {
		// handler 没显式 WriteHeader：此时 Content-Type 多半已由 Write 前的 Set 写好
		w.WriteHeader(http.StatusOK)
	}
	if w.gzipped {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Flush 透传，保证仍有需要即时输出的场景不被缓冲住。
func (w *gzipResponseWriter) Flush() {
	if w.gzipped {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) Close() error {
	if w.gzipped {
		return w.gz.Close()
	}
	return nil
}

var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// compressible 判断内容类型是否值得压缩。图片、音视频、zip 本身已压缩，跳过。
func compressible(ct string) bool {
	if ct == "" {
		return false
	}
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch {
	case strings.HasPrefix(ct, "text/"):
		return true
	case ct == "application/json",
		ct == "application/javascript",
		ct == "application/xml",
		ct == "application/manifest+json",
		ct == "image/svg+xml":
		return true
	}
	return false
}

// withGzip 是最外层中间件。体积很小的响应压缩收益不抵开销，
// 交给 gzip 自带的阈值判断不划算，这里用 Content-Length 先挡一道。
func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			r.Header.Get("Range") != "" ||
			r.URL.Path == "/api/events" {
			next.ServeHTTP(w, r)
			return
		}
		// 小文件（< 1KB）别压：gzip 头就有 20 字节，还多一次 CPU
		if n, err := strconv.Atoi(r.Header.Get("Content-Length")); err == nil && n > 0 && n < 1024 {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		gw := &gzipResponseWriter{ResponseWriter: w, gz: gz}
		defer func() {
			_ = gw.Close()
			gz.Reset(io.Discard)
			gzipPool.Put(gz)
		}()
		next.ServeHTTP(gw, r)
	})
}
