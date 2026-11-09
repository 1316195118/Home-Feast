// Package api 提供家庭点菜系统的 HTTP 接口与内嵌前端的静态服务。
package api

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"familymenu/internal/store"
)

// Version 会显示在界面设置页与 /api/ping 中。
const Version = "0.1.0"

// Server 持有一个 JSON 存储实例和构建时嵌入的前端文件系统。
type Server struct {
	st      *store.Store
	web     fs.FS
	envCode string // MENU_ACCESS_CODE 环境变量；非空时优先于网页里设置的口令
}

// New 组装全部路由。网页里也能设置访问口令，见 updateSettings。
func New(st *store.Store, web fs.FS, envCode string) http.Handler {
	s := &Server{st: st, web: web, envCode: envCode}

	api := http.NewServeMux()
	api.HandleFunc("GET /api/ping", s.ping)
	api.HandleFunc("GET /api/config", s.config)
	api.HandleFunc("GET /api/bootstrap", s.bootstrap)

	api.HandleFunc("GET /api/dishes", s.listDishes)
	api.HandleFunc("POST /api/dishes", s.createDish)
	api.HandleFunc("PUT /api/dishes/{id}", s.updateDish)
	api.HandleFunc("DELETE /api/dishes/{id}", s.deleteDish)

	api.HandleFunc("GET /api/members", s.listMembers)
	api.HandleFunc("POST /api/members", s.createMember)
	api.HandleFunc("PUT /api/members/{id}", s.updateMember)
	api.HandleFunc("DELETE /api/members/{id}", s.deleteMember)

	api.HandleFunc("GET /api/plans", s.listPlans)
	api.HandleFunc("GET /api/plans/{date}", s.getPlan)
	api.HandleFunc("PUT /api/plans/{date}", s.updatePlan)
	api.HandleFunc("POST /api/plans/{date}/orders", s.addOrder)
	api.HandleFunc("PUT /api/plans/{date}/orders/{orderId}", s.toggleOrder)
	api.HandleFunc("DELETE /api/plans/{date}/orders/{orderId}", s.removeOrder)

	api.HandleFunc("PUT /api/settings", s.updateSettings)
	api.HandleFunc("POST /api/upload", s.upload)
	api.HandleFunc("GET /api/backup", s.exportBackup)
	api.HandleFunc("POST /api/backup", s.importBackup)

	root := http.NewServeMux()
	root.Handle("/api/", s.protect(api))
	root.HandleFunc("/uploads/", s.serveUpload)
	root.HandleFunc("/", s.serveFrontend)
	return root
}

// protect 做两件小事：写接口校验同源，口令开启时校验自定义请求头。
// 家庭场景下默认是开放的；口令只是防止外人随手改菜单，不是加密方案。
func (s *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			jsonErr(w, http.StatusForbidden, "跨站请求已拒绝")
			return
		}
		// 每次请求都读当前口令，网页里改完立刻生效，不用重启容器。
		if code := s.effectiveCode(); code != "" && r.URL.Path != "/api/ping" && r.URL.Path != "/api/config" {
			given := r.Header.Get("X-Access-Code")
			if subtle.ConstantTimeCompare([]byte(given), []byte(code)) != 1 {
				jsonErr(w, http.StatusUnauthorized, "需要访问口令")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// effectiveCode 环境变量优先，其次才是网页里保存到 access.json 的口令。
func (s *Server) effectiveCode() string {
	if s.envCode != "" {
		return s.envCode
	}
	return s.st.AccessCode()
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return strings.EqualFold(u.Host, host)
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"ok": true, "version": Version,
		"time": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"version":          Version,
		"accessRequired":   s.effectiveCode() != "",
		"accessEnvManaged": s.envCode != "", // 环境变量控制时，网页里不能改
	})
}

// bootstrap 让前端一次拿到首屏需要的全部数据，省掉一串并发请求。
func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	date := today()
	jsonOK(w, map[string]any{
		"version":   Version,
		"today":     date,
		"settings":  s.st.GetSettings(),
		"meals":     s.st.Meals(), // 生效的用餐时段配置（含旧数据兜底默认三餐）
		"cuisines":  store.Cuisines,
		"dishes":    s.st.ListDishes(),
		"members":   s.st.ListMembers(),
		"plan":      s.st.GetPlan(date),
		"planCount": len(s.st.ListPlans()),
	})
}
