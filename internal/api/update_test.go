package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resetUpdateStateForTest(t *testing.T) {
	t.Helper()
	updateMu.Lock()
	previous := updateState
	updateState = updateStatus{}
	updateMu.Unlock()
	t.Cleanup(func() {
		updateMu.Lock()
		updateState = previous
		updateMu.Unlock()
	})
}

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int // -1 / 0 / 1 只看符号
	}{
		{"1.0.2.4", "1.0.2.3", 1},
		{"1.0.2.4", "1.0.2.4", 0},
		{"1.0.2.3", "1.0.2.4", -1},
		{"1.0.3.0", "1.0.2.9", 1}, // 末位进位
		{"1.1.0.0", "1.0.9.9", 1},
		{"2.0.0.0", "1.9.9.9", 1},
		{"", "1.0.0.0", -1},
	}
	for _, c := range cases {
		got := compareVersion(c.a, c.b)
		if (got > 0) != (c.want > 0) || (got < 0) != (c.want < 0) {
			t.Errorf("compareVersion(%q,%q) = %d, want sign %d", c.a, c.b, got, c.want)
		}
	}
}

func TestManifestParse(t *testing.T) {
	raw := `{
		"version":"1.0.2.5",
		"publishedAt":"2026-11-20T12:00:00+08:00",
		"note":"测试更新",
		"files":{
			"amd64":{"name":"menu-linux-amd64","sha256":"abc123","size":1234},
			"arm64":{"name":"menu-linux-arm64","sha256":"def456","size":2345}
		}
	}`
	var m releaseManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != "1.0.2.5" || m.Files["amd64"].SHA256 != "abc123" {
		t.Fatalf("解析错误: %+v", m)
	}
	if compareVersion(m.Version, "1.0.2.4") <= 0 {
		t.Fatal("应判断为有更新")
	}
}

func TestUpdateSourcesUseFullBranchRef(t *testing.T) {
	t.Setenv("MENU_UPDATE_BASE", "")
	sources := updateSources()
	if len(sources) < 2 {
		t.Fatalf("更新源数量不足: %v", sources)
	}
	for _, source := range sources {
		if !strings.Contains(source, "/refs/heads/main/dist/") {
			t.Fatalf("更新源应固定到完整分支引用，避免 Raw 短地址缓存旧 manifest: %s", source)
		}
	}
}

func TestUpdaterStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	st := updaterState{Phase: "applying", From: "1.0.2.4", To: "1.0.2.5", At: "2026-11-20T12:00:00Z"}
	if err := writeUpdaterState(dir, st); err != nil {
		t.Fatal(err)
	}
	_, _, _, path := updatePaths(dir)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got updaterState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Phase != "applying" || got.To != "1.0.2.5" {
		t.Fatalf("状态不一致: %+v", got)
	}
	// 确认写在 .update 下，不污染业务数据根目录
	if filepath.Dir(path) != filepath.Join(dir, updateDir) {
		t.Fatalf("路径不对: %s", path)
	}
}

func TestGoArchKey(t *testing.T) {
	// 只在常见架构上断言，保证映射不退化
	k := goArchKey()
	if k != "amd64" && k != "arm64" && k != "armv7" {
		t.Fatalf("未知架构 key: %s", k)
	}
}

func TestApplyUpdateIsAcceptedBeforeDownload(t *testing.T) {
	// 更新可能持续数分钟，接口必须先受理并立即返回，不能把下载挂在 HTTP 响应里。
	resetUpdateStateForTest(t)
	t.Setenv("MENU_UPDATE_BASE", "http://127.0.0.1:1/dist/")
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/update/apply", strings.NewReader(`{"force":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("更新任务应返回 202，实际 %d：%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("更新受理响应应为 JSON，实际 %s", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["accepted"] != true {
		t.Fatalf("更新受理响应不完整: %v / %v", body, err)
	}

	// 后台任务应自行失败收尾，不能把 Downloading 永久卡住。
	deadline := time.Now().Add(3 * time.Second)
	for {
		updateMu.Lock()
		downloading := updateState.Downloading
		lastErr := updateState.LastError
		updateMu.Unlock()
		if !downloading {
			if lastErr == "" {
				t.Fatal("不可达更新源应留下错误信息")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("后台更新失败后未释放 Downloading 状态")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDownloadReleaseReportsRealProgress(t *testing.T) {
	resetUpdateStateForTest(t)
	payload := bytes.Repeat([]byte("family-menu-update-"), 32*1024)
	var packed bytes.Buffer
	gz := gzip.NewWriter(&packed)
	if _, err := gz.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	binaryHash := sha256.Sum256(payload)
	packedHash := sha256.Sum256(packed.Bytes())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/menu-linux.gz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(packed.Len()))
		_, _ = w.Write(packed.Bytes())
	}))
	defer srv.Close()
	t.Setenv("MENU_UPDATE_BASE", srv.URL+"/")

	staging := filepath.Join(t.TempDir(), "menu.new")
	var last int
	var sawBytes bool
	err := downloadRelease(context.Background(), releaseFile{
		Name:     "menu-linux",
		SHA256:   fmt.Sprintf("%x", binaryHash[:]),
		Size:     int64(len(payload)),
		Gz:       "menu-linux.gz",
		GzSHA256: fmt.Sprintf("%x", packedHash[:]),
		GzSize:   int64(packed.Len()),
	}, staging, "", func(progress int, message string) {
		if progress < last {
			t.Fatalf("下载进度不能倒退：%d → %d", last, progress)
		}
		last = progress
		if strings.Contains(message, "正在下载") && strings.Contains(message, "MB") {
			sawBytes = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawBytes || last < 80 {
		t.Fatalf("应持续报告下载字节与进度，最后进度 %d，字节消息 %v", last, sawBytes)
	}
	got, err := os.ReadFile(staging)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("解压后的更新文件内容不一致")
	}
}
