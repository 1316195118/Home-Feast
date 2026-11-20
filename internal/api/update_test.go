package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
