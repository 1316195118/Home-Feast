package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func gzipTestHandler(body string, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	})
}

func TestWithGzipCompressesJSON(t *testing.T) {
	body := strings.Repeat(`{"name":"宫保鸡丁"},`, 200)
	h := withGzip(gzipTestHandler(body, "application/json; charset=utf-8"))

	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("期望 Content-Encoding=gzip，实际 %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("压缩后必须删掉 Content-Length，实际仍有 %q", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("缺少 Vary: Accept-Encoding")
	}

	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("响应不是合法 gzip: %v", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	if string(plain) != body {
		t.Fatalf("解压内容与原文不一致")
	}
	if rec.Body.Len() >= len(body) {
		t.Fatalf("压缩后体积没有变小: %d >= %d", rec.Body.Len(), len(body))
	}
}

func TestWithGzipSkipsWhenNotAccepted(t *testing.T) {
	h := withGzip(gzipTestHandler(`{"ok":true}`, "application/json"))
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("未声明 Accept-Encoding 却压缩了: %q", got)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("内容被改动: %q", rec.Body.String())
	}
}

func TestWithGzipSkipsSSE(t *testing.T) {
	h := withGzip(gzipTestHandler("data: ping\n\n", "text/event-stream"))
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("SSE 不应被压缩: %q", got)
	}
}

func TestWithGzipSkipsImages(t *testing.T) {
	h := withGzip(gzipTestHandler("fake-webp", "image/webp"))
	req := httptest.NewRequest(http.MethodGet, "/uploads/img.webp", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("图片不应被压缩: %q", got)
	}
}

func TestWithGzipSkipsRangeRequest(t *testing.T) {
	h := withGzip(gzipTestHandler(strings.Repeat("x", 4096), "text/plain"))
	req := httptest.NewRequest(http.MethodGet, "/assets/a.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-1023")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Range 请求不应压缩: %q", got)
	}
}
