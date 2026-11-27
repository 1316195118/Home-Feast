package api

import (
	"archive/zip"
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"familymenu/internal/store"
)

func TestSameOriginDoesNotTrustForwardedHost(t *testing.T) {
	t.Setenv("MENU_SITE_URL", "")
	r := httptest.NewRequest("POST", "http://10.10.10.251:8090/api/dishes", nil)
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("X-Forwarded-Host", "evil.example")
	if sameOrigin(r) {
		t.Fatal("伪造 X-Forwarded-Host 不应通过同源检查")
	}

	r.Header.Set("Origin", "http://10.10.10.251:8090")
	if !sameOrigin(r) {
		t.Fatal("真实的同源请求应通过")
	}
	r.Header.Set("Origin", "file:///tmp/evil")
	if sameOrigin(r) {
		t.Fatal("非 HTTP(S) Origin 不应通过")
	}
}

func TestSameOriginNormalizesDefaultPort(t *testing.T) {
	t.Setenv("MENU_SITE_URL", "")
	// 浏览器把 Origin 规范化为不带默认端口，Host 头可能仍带 :443 / :80。
	for _, tc := range []struct{ scheme, origin, host string }{
		{"https", "https://menu.example.com", "menu.example.com:443"},
		{"http", "http://192.168.1.5", "192.168.1.5:80"},
		{"http", "http://127.0.0.1:8090", "127.0.0.1:8090"}, // 非默认端口必须保留
	} {
		r := httptest.NewRequest("POST", tc.scheme+"://"+tc.host+"/api/dishes", nil)
		r.Header.Set("Origin", tc.origin)
		if !sameOrigin(r) {
			t.Errorf("默认端口归一化后应同源：origin=%s host=%s", tc.origin, tc.host)
		}
	}
	// 端口不同且非默认时仍须拒绝
	r := httptest.NewRequest("POST", "http://127.0.0.1:8090/api/dishes", nil)
	r.Header.Set("Origin", "http://127.0.0.1:8091")
	if sameOrigin(r) {
		t.Error("不同端口不应通过同源检查")
	}
}

func TestSameOriginUsesConfiguredPublicSite(t *testing.T) {
	t.Setenv("MENU_SITE_URL", "https://menu.example.com")
	r := httptest.NewRequest("POST", "http://127.0.0.1:8090/api/dishes", nil)
	r.Header.Set("Origin", "https://menu.example.com")
	if !sameOrigin(r) {
		t.Fatal("反向代理改写 Host 时，配置的公开站点应作为同源基准")
	}
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("X-Forwarded-Host", "evil.example")
	if sameOrigin(r) {
		t.Fatal("配置公开站点后仍不得信任客户端提供的代理头")
	}
}

// 备份恢复前必须先判断内容能不能用，避免「写进去再回滚」这种惊险操作。
func TestValidateBackupJSON(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"dishes.json", `[{"id":"d1","name":"菜"}]`, false},
		{"members.json", `[{"id":"u1","name":"张三"}]`, false},
		{"plans.json", `[]`, false},
		{"settings.json", `{"familyName":"家"}`, false},

		// 结构不对：语法合法但顶层类型错了
		{"dishes.json", `{"id":"d1"}`, true},
		{"members.json", `"张三"`, true},
		{"settings.json", `[]`, true},

		// 语法错误
		{"dishes.json", `[{`, true},
		{"settings.json", `坏内容`, true},

		// 空内容
		{"dishes.json", `   `, true},

		// 成员为空：恢复后全家都点不了菜，必须拦住
		{"members.json", `[]`, true},
	}
	for _, c := range cases {
		err := validateBackupJSON(c.name, []byte(c.body))
		if c.wantErr && err == nil {
			t.Fatalf("%s %s 应该被拒绝", c.name, c.body)
		}
		if !c.wantErr && err != nil {
			t.Fatalf("%s %s 不该被拒绝: %v", c.name, c.body, err)
		}
	}
}

// 图片路径必须挡在 uploads/ 之内，压缩包里的穿越路径直接丢弃。
func TestUploadEntryName(t *testing.T) {
	ok := []string{
		"uploads/img0123456789abcdef01234567.webp",
		"uploads/a.png",
		"uploads/thumb/img0123456789abcdef01234567.jpg",
	}
	for _, name := range ok {
		if _, good := uploadEntryName(name); !good {
			t.Fatalf("%s 应被接受", name)
		}
	}
	bad := []string{
		"uploads/../../etc/passwd",
		"uploads/../secret.webp",
		"uploads/sub/dir/a.webp",
		"uploads/",
		"dishes.json",
		"/uploads/a.webp",
		"uploads/a.webp/../../x",
		"uploads/thumb/../../etc/passwd",
		"uploads/thumb/sub/a.jpg",
	}
	for _, name := range bad {
		if got, good := uploadEntryName(name); good {
			t.Fatalf("%s 不该被接受，却返回 %q", name, got)
		}
	}
}

func TestAutoBackupIncludesOnlyReferencedImages(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir, filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDish(store.Dish{
		Name: "备份图片测试", Cuisine: "homestyle",
		Image: "/uploads/img-used.webp",
	}); err != nil {
		t.Fatal(err)
	}
	uploadDir := filepath.Join(dir, "uploads")
	thumbDir := filepath.Join(uploadDir, "thumb")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"img-used.webp", "img-orphan.webp"} {
		if err := os.WriteFile(filepath.Join(uploadDir, name), []byte("image"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(thumbDir, "img-used.jpg"), []byte("thumb"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	s := &Server{st: st}
	if err := s.writeBackupZip(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	if !found["uploads/img-used.webp"] || !found["uploads/thumb/img-used.jpg"] {
		t.Fatalf("自动备份缺少被引用图片或缩略图: %v", found)
	}
	if found["uploads/img-orphan.webp"] {
		t.Fatalf("自动备份不应包含孤儿图片: %v", found)
	}
}

func TestExtractBackupImageUsesAtomicDestination(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entry, err := zw.Create("uploads/img-atomic.webp")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("complete-image-bytes")
	if _, err := entry.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "img-atomic.webp"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractZipFileAtomic(root, "img-atomic.webp", zr.File[0]); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "img-atomic.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("恢复后的图片内容不完整: %q", got)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "img-atomic.webp" {
		t.Fatalf("恢复图片后残留临时文件: %v", entries)
	}
}

func TestRunBackupConcurrentCreatesValidUniqueArchives(t *testing.T) {
	dir := t.TempDir()
	backupDir := t.TempDir()
	t.Setenv("MENU_BACKUP_DIR", backupDir)
	st, err := store.Open(dir, filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	settings := st.GetSettings()
	settings.BackupKeep = 20
	if _, err := st.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	s := &Server{st: st}

	const workers = 12
	names := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			names[index], _, errs[index] = s.runBackup()
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, workers)
	for i, name := range names {
		if errs[i] != nil {
			t.Fatalf("第 %d 个并发备份失败: %v", i, errs[i])
		}
		if name == "" || seen[name] {
			t.Fatalf("备份文件名重复或为空: %q", name)
		}
		seen[name] = true
		zr, err := zip.OpenReader(filepath.Join(backupDir, name))
		if err != nil {
			t.Fatalf("备份 %s 损坏: %v", name, err)
		}
		_ = zr.Close()
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".zip" {
			t.Fatalf("备份目录残留临时文件: %s", entry.Name())
		}
	}
}
