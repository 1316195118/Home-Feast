package api

import "net/http"

// changelog 返回 CHANGELOG.md 的结构化副本，供设置页「版本更新」展示全部历史记录。
// 内容在发布时由 scripts/release.py 生成，随二进制走，不依赖运行期文件。
func (s *Server) changelog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(changelogJSON))
}
