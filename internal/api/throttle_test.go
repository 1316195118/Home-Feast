package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFailLimiterEscalates(t *testing.T) {
	l := newFailLimiter()
	key := "pin:1.2.3.4"

	// 失败 7 次：仍不限速，正常用户输错几次不该被罚
	for i := 0; i < 7; i++ {
		l.Fail(key)
	}
	if d := l.Penalty(key); d != 0 {
		t.Fatalf("7 次失败不该限速，实际 %v", d)
	}

	// 第 8 次起：0.5s
	l.Fail(key)
	if d := l.Penalty(key); d != 500*time.Millisecond {
		t.Fatalf("8 次失败应为 500ms，实际 %v", d)
	}

	// 第 20 次起：2s
	for i := 0; i < 12; i++ {
		l.Fail(key)
	}
	if d := l.Penalty(key); d != 2*time.Second {
		t.Fatalf("20 次失败应为 2s，实际 %v", d)
	}
}

func TestFailLimiterResetOnSuccess(t *testing.T) {
	l := newFailLimiter()
	key := "pin:x"
	for i := 0; i < 10; i++ {
		l.Fail(key)
	}
	l.Reset(key)
	if d := l.Penalty(key); d != 0 {
		t.Fatalf("成功后应清零，实际还有 %v", d)
	}
}

func TestFailLimiterExpires(t *testing.T) {
	l := newFailLimiter()
	l.ttl = 50 * time.Millisecond
	key := "pin:y"
	for i := 0; i < 10; i++ {
		l.Fail(key)
	}
	if d := l.Penalty(key); d == 0 {
		t.Fatal("刚失败后应当限速")
	}
	time.Sleep(80 * time.Millisecond)
	if d := l.Penalty(key); d != 0 {
		t.Fatalf("超过 TTL 应自动解除，实际 %v", d)
	}
}

func TestFailLimiterIsolatesKeys(t *testing.T) {
	l := newFailLimiter()
	for i := 0; i < 10; i++ {
		l.Fail("pin:a")
	}
	if d := l.Penalty("pin:b"); d != 0 {
		t.Fatalf("不同来源不应互相影响，实际 %v", d)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/members/x/verify", nil)
	r.RemoteAddr = "10.10.10.251:54321"
	if got := clientIP(r); got != "10.10.10.251" {
		t.Fatalf("期望 10.10.10.251，实际 %q", got)
	}
	// 没有端口时原样返回，不能 panic
	r.RemoteAddr = "weird"
	if got := clientIP(r); got != "weird" {
		t.Fatalf("异常 RemoteAddr 应原样返回，实际 %q", got)
	}
}

func TestTrimmedStripsControl(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc\x00def", "abcdef"},
		{"正常\x01\x02菜名", "正常菜名"},
		{"含\n换行\t制表", "含\n换行\t制表"}, // 换行制表符保留
		{"  \x00 前后空格  ", "前后空格"},
	}
	for _, c := range cases {
		if got := trimmed(c.in, 40); got != c.want {
			t.Fatalf("trimmed(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	// 截断仍按字符数，且不受控制字符影响
	if got := trimmed(strings.Repeat("宫", 50)+"\x00", 40); len([]rune(got)) != 40 {
		t.Fatalf("截断后应正好 40 个字符，实际 %d", len([]rune(got)))
	}
}

func TestVersionDefaults(t *testing.T) {
	if Version == "" {
		t.Fatal("Version 不应为空")
	}
}
