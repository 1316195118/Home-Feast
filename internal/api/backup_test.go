package api

import "testing"

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
	}
	for _, name := range bad {
		if got, good := uploadEntryName(name); good {
			t.Fatalf("%s 不该被接受，却返回 %q", name, got)
		}
	}
}
