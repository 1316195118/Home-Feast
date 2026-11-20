package api

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"familymenu/internal/store"
)

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
