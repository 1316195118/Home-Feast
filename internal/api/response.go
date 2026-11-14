package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// jsonOK 返回成功 JSON；接口响应一律禁止缓存，避免家庭数据被代理留住。
func jsonOK(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func jsonErr(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// decodeBody 限制请求体大小并解析 JSON，拒绝尾部多余内容。
func decodeBody(w http.ResponseWriter, r *http.Request, dest any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("请求内容不是合法 JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("请求只能包含一个 JSON 对象")
	}
	return nil
}

// trimmed 去掉首尾空白，并按字符数（不是字节数）限制长度。
func trimmed(value string, max int) string {
	v := strings.TrimSpace(stripControl(value))
	if utf8.RuneCountInString(v) > max {
		v = string([]rune(v)[:max])
	}
	return v
}

// stripControl 去掉 NUL 等控制字符。它们能存进 JSON，但会让前端显示成空白、
// 导出的 Excel 也可能出错；换行与制表符是合理输入，保留。
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// validDate 只接受 YYYY-MM-DD，且必须是真实存在的日期。
func validDate(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func today() string { return time.Now().Format("2006-01-02") }
