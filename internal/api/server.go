// Package api 提供家庭点菜系统的 HTTP 接口与内嵌前端的静态服务。
package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"familymenu/internal/store"
)

// Version / ReleaseNote / BuildTime 由 scripts/release.py 在编译前写进
// version_gen.go（四段式版本号，末位满 10 进位），这里只留开发态默认值。
var (
	Version     = "dev"
	ReleaseNote = "开发版（未经过 release 脚本）"
	BuildTime   = ""
)

// Server 持有一个 JSON 存储实例和构建时嵌入的前端文件系统。
type Server struct {
	st      *store.Store
	web     fs.FS
	envCode string       // MENU_ACCESS_CODE 环境变量；非空时优先于网页里设置的口令
	events  *eventBroker // SSE 广播器，写操作成功后通知所有在线设备
	limit   *failLimiter // 口令 / PIN 的失败退避

	bootstrapMu    sync.RWMutex
	bootstrapCache *bootstrapCacheEntry
	staticCache    sync.Map
}

type bootstrapCacheEntry struct {
	revision uint64
	date     string
	raw      []byte
	gzip     []byte
	rawETag  string
	gzipETag string
}

type closeHandler struct {
	http.Handler
	close func()
}

func (h *closeHandler) Close() {
	if h.close != nil {
		h.close()
	}
}

// New 组装全部路由。网页里也能设置访问口令，见 updateSettings。
func New(st *store.Store, web fs.FS, envCode string) http.Handler {
	s := &Server{st: st, web: web, envCode: envCode, events: newEventBroker(), limit: newFailLimiter()}

	api := http.NewServeMux()
	api.HandleFunc("GET /api/ping", s.ping)
	api.HandleFunc("GET /api/config", s.config)
	api.HandleFunc("GET /api/changelog", s.changelog)
	api.HandleFunc("GET /api/bootstrap", s.bootstrap)
	api.HandleFunc("GET /api/events", s.streamEvents)

	api.HandleFunc("GET /api/dishes", s.listDishes)
	api.HandleFunc("POST /api/dishes", s.createDish)
	api.HandleFunc("PUT /api/dishes/{id}", s.updateDish)
	api.HandleFunc("DELETE /api/dishes/{id}", s.deleteDish)

	api.HandleFunc("GET /api/members", s.listMembers)
	api.HandleFunc("POST /api/members", s.createMember)
	api.HandleFunc("PUT /api/members/{id}", s.updateMember)
	api.HandleFunc("DELETE /api/members/{id}", s.deleteMember)
	api.HandleFunc("POST /api/members/{id}/pin", s.setMemberPin)
	api.HandleFunc("POST /api/members/{id}/verify", s.verifyMemberPin)

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
	api.HandleFunc("GET /api/export/excel", s.exportExcel)
	api.HandleFunc("GET /api/backups", s.listBackups)
	api.HandleFunc("POST /api/backups/run", s.runBackupNow)

	// 在线自更新：检查 / 进度 / 应用（详见 update.go）
	api.HandleFunc("GET /api/update/check", s.checkUpdate)
	api.HandleFunc("GET /api/update/status", s.updateStatus)
	api.HandleFunc("POST /api/update/apply", s.applyUpdate)
	api.HandleFunc("POST /api/update/proxy-test", s.testProxy)

	// 应用内定时备份：随进程启动，不需要宿主机 cron
	s.startAutoBackup()

	root := http.NewServeMux()
	root.Handle("/api/", s.protect(api))
	root.HandleFunc("/uploads/", s.serveUpload)
	root.HandleFunc("/", s.serveFrontend)
	// 最外层压缩：JSON/JS/CSS/HTML 实测能省 65%~80% 传输量（见 gzip.go）
	return &closeHandler{Handler: withGzip(root), close: s.events.close}
}

// protect 做两件小事：写接口校验同源，口令开启时校验自定义请求头。
// 家庭场景下默认是开放的；口令只是防止外人随手改菜单，不是加密方案。
func (s *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		isWrite := r.Method != http.MethodGet && r.Method != http.MethodHead
		if isWrite && !sameOrigin(r) {
			jsonErr(w, http.StatusForbidden, "跨站请求已拒绝")
			return
		}
		// 每次请求都读当前口令，网页里改完立刻生效，不用重启容器。
		// EventSource 无法带自定义请求头，事件流与 ping/config 一样跳过校验；
		// 它只推版本号，不含任何业务数据。
		if code := s.effectiveCode(); code != "" && !publicAPIPath(r.URL.Path) {
			key := "code:" + clientIP(r)
			s.limit.wait(key) // 连续输错后逐步变慢，压制口令爆破
			given := r.Header.Get("X-Access-Code")
			if subtle.ConstantTimeCompare([]byte(given), []byte(code)) != 1 {
				s.limit.Fail(key)
				jsonErr(w, http.StatusUnauthorized, "需要访问口令")
				return
			}
			s.limit.Reset(key)
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		// 写操作成功后广播一次；前端自己也会 refresh，回声会被前端去重
		if isWrite && sw.status >= 200 && sw.status < 300 {
			s.events.publish(r.Header.Get("X-Client-ID"))
		}
	})
}

func publicAPIPath(path string) bool {
	return path == "/api/ping" || path == "/api/config" || path == "/api/events" ||
		path == "/api/update/check" || path == "/api/update/status"
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
		"ok": true, "version": Version, "releaseNote": ReleaseNote, "buildTime": BuildTime,
		"time": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"version":          Version,
		"releaseNote":      ReleaseNote,
		"buildTime":        BuildTime,
		"accessRequired":   s.effectiveCode() != "",
		"accessEnvManaged": s.envCode != "", // 环境变量控制时，网页里不能改
	})
}

// bootstrap 在一次存储读锁内生成完整首屏快照（含历史计划），
// 再按存储修订号缓存序列化与 gzip 结果。多人同时刷新时不再重复排序、复制和压缩
// 同一份 237 道菜的数据；任意写操作成功后修订号变化，下一次请求自动重建。
func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	date := today()
	entry := s.cachedBootstrap(date, s.st.Revision())
	if entry == nil {
		s.bootstrapMu.Lock()
		entry = s.bootstrapCache
		if entry == nil || entry.date != date || entry.revision != s.st.Revision() {
			built, err := s.buildBootstrapCache(date)
			if err != nil {
				s.bootstrapMu.Unlock()
				jsonErr(w, http.StatusInternalServerError, "生成数据失败")
				return
			}
			entry = built
			s.bootstrapCache = built
		}
		s.bootstrapMu.Unlock()
	}
	s.serveBootstrap(w, r, entry)
}

func (s *Server) cachedBootstrap(date string, revision uint64) *bootstrapCacheEntry {
	s.bootstrapMu.RLock()
	defer s.bootstrapMu.RUnlock()
	if entry := s.bootstrapCache; entry != nil && entry.date == date && entry.revision == revision {
		return entry
	}
	return nil
}

func (s *Server) buildBootstrapCache(date string) (*bootstrapCacheEntry, error) {
	snapshot := s.st.Bootstrap(date)
	familyID := strings.TrimSpace(snapshot.Settings.FamilyID)
	if familyID == "" {
		familyID = "default"
	}
	payload := map[string]any{
		"version":   Version,
		"revision":  snapshot.Revision,
		"today":     date,
		"familyId":  familyID,
		"settings":  snapshot.Settings,
		"meals":     snapshot.Meals, // 生效的用餐时段配置（含旧数据兜底默认三餐）
		"cuisines":  store.OrderedCuisines(snapshot.Settings.CuisineOrder),
		"dishes":    snapshot.Dishes,
		"members":   snapshot.Members,
		"plan":      snapshot.Plan,
		"plans":     snapshot.Plans,
		"planCount": snapshot.PlanCount,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	gzipped, err := gzipBody(body)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	gzipSum := sha256.Sum256(gzipped)
	return &bootstrapCacheEntry{
		revision: snapshot.Revision,
		date:     date,
		raw:      body,
		gzip:     gzipped,
		rawETag:  `"` + hex.EncodeToString(sum[:12]) + `"`,
		gzipETag: `"` + hex.EncodeToString(gzipSum[:12]) + `-gzip"`,
	}, nil
}

func gzipBody(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(body); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) serveBootstrap(w http.ResponseWriter, r *http.Request, entry *bootstrapCacheEntry) {
	useGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	body, etag := entry.raw, entry.rawETag
	if useGzip {
		body, etag = entry.gzip, entry.gzipETag
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Vary", "Accept-Encoding")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if useGzip {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
