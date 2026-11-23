package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"familymenu/internal/store"
)

func newStaticTestHandler(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	web := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte(`<html><head><title>家里吃点啥｜今天吃什么，全家一起定</title>` +
				`<meta name="description" content="全家一起挑菜、点菜、分工和看记录的家庭点菜系统，237 道家常菜，数据保存在自己的 NAS 或本机。" />` +
				`<meta property="og:type" content="website"><link rel="canonical" href="__SITE_URL__/">` +
				`<meta property="og:url" content="__SITE_URL__/"><meta property="og:image" content="__SITE_URL__/og-cover.png"></head></html>`),
		},
	}
	return New(st, web, "")
}

func TestPublicRoutesSecurityAndSitemap(t *testing.T) {
	h := newStaticTestHandler(t)

	for _, path := range []string{"/privacy", "/terms"} {
		req := httptest.NewRequest(http.MethodGet, "http://menu.local"+path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应 200，实际 %d", path, w.Code)
		}
		if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("缺少 nosniff，实际 %q", got)
		}
		if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
			t.Fatalf("缺少基础 CSP，实际 %q", got)
		}
		if strings.Contains(w.Body.String(), "__SITE_URL__") {
			t.Fatalf("%s 的站点占位符未替换", path)
		}
		if !strings.Contains(w.Body.String(), "http://menu.local/og-cover.png") {
			t.Fatalf("%s 的 OG 地址不是当前站点绝对地址：%s", path, w.Body.String())
		}
		wantTitle := "隐私政策｜家里吃点啥"
		wantDescription := "了解家里吃点啥会保存哪些家庭数据"
		wantCanonical := `href="http://menu.local/privacy"`
		wantType := `content="article"`
		if path == "/terms" {
			wantTitle = "服务条款｜家里吃点啥"
			wantDescription = "查看家里吃点啥的使用规则"
			wantCanonical = `href="http://menu.local/terms"`
		}
		if !strings.Contains(w.Body.String(), wantTitle) ||
			!strings.Contains(w.Body.String(), wantDescription) ||
			!strings.Contains(w.Body.String(), wantCanonical) ||
			!strings.Contains(w.Body.String(), wantType) {
			t.Fatalf("%s 的公开页元数据未在服务端注入：%s", path, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "http://menu.local/not-a-page", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("未知页面应返回 404，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "__SITE_URL__") && !strings.Contains(w.Body.String(), "menu.local") {
		t.Fatal("未知页面仍应返回 SPA 入口，交给前端渲染 404")
	}
	if !strings.Contains(w.Body.String(), "页面未找到｜家里吃点啥") ||
		!strings.Contains(w.Body.String(), `name="robots" content="noindex, nofollow"`) {
		t.Fatalf("未知页面应使用 404 元数据并禁止索引：%s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "http://menu.local/missing.js", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "<html") {
		t.Fatalf("缺失静态资源必须 404 且不能伪装成 HTML，code=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "http://menu.local/sitemap.xml", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sitemap 应 200，实际 %d", w.Code)
	}
	for _, path := range []string{"/privacy", "/terms"} {
		if !strings.Contains(w.Body.String(), "http://menu.local"+path) {
			t.Fatalf("sitemap 缺少 %s：%s", path, w.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, "http://menu.local/robots.txt", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Sitemap: http://menu.local/sitemap.xml") {
		t.Fatalf("robots 内容异常：code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHSTSOnlyOnTLS(t *testing.T) {
	h := newStaticTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "https://menu.local/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("HTTPS 请求应返回 HSTS")
	}

	req = httptest.NewRequest(http.MethodGet, "http://menu.local/", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HTTP 局域网访问不应强制 HSTS，实际 %q", got)
	}
}
