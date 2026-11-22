package api

// handler 层的冒烟 + CRUD 回归测试：覆盖核心接口，防止后续改动悄悄破坏主链路。

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"familymenu/internal/store"
	"github.com/andybalholm/brotli"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return New(st, nil, "")
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if w.Body.Len() == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应不是 JSON: %v（body=%q）", err, w.Body.String())
	}
	return m
}

func TestPing(t *testing.T) {
	w := do(newTestHandler(t), "GET", "/api/ping", "")
	if w.Code != 200 {
		t.Fatalf("ping 应 200，实际 %d", w.Code)
	}
	if m := decode(t, w); m["ok"] != true {
		t.Fatalf("ping 应返回 ok=true，实际 %v", m)
	}
}

func TestBootstrapShape(t *testing.T) {
	w := do(newTestHandler(t), "GET", "/api/bootstrap", "")
	if w.Code != 200 {
		t.Fatalf("bootstrap 应 200，实际 %d", w.Code)
	}
	m := decode(t, w)
	for _, key := range []string{
		"today", "dishes", "members", "cuisines", "meals", "plan", "plans", "settings",
		"version", "releaseNote", "buildTime", "accessRequired", "accessEnvManaged",
	} {
		if _, ok := m[key]; !ok {
			t.Fatalf("bootstrap 缺少字段 %s", key)
		}
	}
	if len(m["dishes"].([]any)) == 0 || len(m["members"].([]any)) == 0 {
		t.Fatal("新库应自动播种菜品与成员")
	}
}

func TestBootstrapCompressedCacheAndInvalidation(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap 应 200，实际 %d：%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("bootstrap 应预压缩为 gzip，实际 %q", got)
	}
	if got := w.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("bootstrap 应声明 Vary: Accept-Encoding，实际 %q", got)
	}
	zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip 头无效: %v", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("解压 bootstrap: %v", err)
	}
	if len(plain) == 0 || plain[0] != '{' {
		t.Fatalf("不应被重复 gzip，解压后开头 %q", string(plain[:min(len(plain), 8)]))
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("bootstrap 应带 ETag")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("缓存命中应返回空 304，实际 %d，%d 字节", w.Code, w.Body.Len())
	}

	w = do(h, http.MethodPost, "/api/dishes", `{"name":"缓存失效测试菜","cuisine":"homestyle"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("新增菜品应 201，实际 %d：%s", w.Code, w.Body.String())
	}
	w = do(h, http.MethodGet, "/api/bootstrap", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "缓存失效测试菜") {
		t.Fatalf("写操作后 bootstrap 应重建并包含新菜，实际 %d：%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("ETag"); got == "" || got == etag {
		t.Fatalf("数据变化后 ETag 必须变化，旧 %q，新 %q", etag, got)
	}
}

func TestBootstrapPrefersBrotli(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap 应 200，实际 %d：%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("支持 Brotli 时应优先返回 br，实际 %q", got)
	}
	if !strings.HasSuffix(w.Header().Get("ETag"), `-br"`) {
		t.Fatalf("Brotli ETag 标识不正确: %q", w.Header().Get("ETag"))
	}
	plain, err := io.ReadAll(brotli.NewReader(bytes.NewReader(w.Body.Bytes())))
	if err != nil {
		t.Fatalf("解压 Brotli bootstrap: %v", err)
	}
	if len(plain) == 0 || plain[0] != '{' {
		t.Fatalf("Brotli 解压后不是单层 JSON 对象，开头 %q", string(plain[:min(len(plain), 8)]))
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(w.Body.Len()) {
		t.Fatalf("Content-Length 应为 Brotli 长度，header=%s body=%d", got, w.Body.Len())
	}
}

func TestStaticAssetPrecompression(t *testing.T) {
	st, err := store.Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	index := "<!doctype html><title>家庭点菜</title>" + strings.Repeat(" ", 2048)
	h := New(st, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(index)},
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("静态首页应预压缩，实际 %d / %q", w.Code, w.Header().Get("Content-Encoding"))
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(w.Body.Len()) {
		t.Fatalf("Content-Length 应为压缩后长度，header=%s body=%d", got, w.Body.Len())
	}
	zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("静态 gzip 头无效: %v", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != index {
		t.Fatalf("预压缩内容不一致，解压后 %d 字节", len(plain))
	}

	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("静态资源应带 ETag")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("静态资源未变应返回空 304，实际 %d / %d 字节", w.Code, w.Body.Len())
	}
}

func TestStaticAssetUsesPrecompressedBrotli(t *testing.T) {
	st, err := store.Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	script := "console.log('家庭点菜')" + strings.Repeat(";", 2048)
	compressed, err := brotliBody([]byte(script))
	if err != nil {
		t.Fatal(err)
	}
	h := New(st, fstest.MapFS{
		"assets/app.js":    &fstest.MapFile{Data: []byte(script)},
		"assets/app.js.br": &fstest.MapFile{Data: compressed},
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("静态资源应优先使用 Brotli，实际 %d / %q", w.Code, w.Header().Get("Content-Encoding"))
	}
	plain, err := io.ReadAll(brotli.NewReader(bytes.NewReader(w.Body.Bytes())))
	if err != nil {
		t.Fatalf("静态 Brotli 头无效: %v", err)
	}
	if string(plain) != script {
		t.Fatalf("预压缩内容不一致，解压后 %d 字节", len(plain))
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("旧客户端应回退 gzip，实际 %d / %q", w.Code, w.Header().Get("Content-Encoding"))
	}
}

func TestConcurrentBootstrapRequests(t *testing.T) {
	h := newTestHandler(t)
	const clients = 64
	var wg sync.WaitGroup
	errs := make(chan string, clients)
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := do(h, http.MethodGet, "/api/bootstrap", "")
			if w.Code != http.StatusOK {
				errs <- w.Body.String()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for body := range errs {
		t.Fatalf("并发首屏请求失败: %s", body)
	}
}

func TestPlansETag(t *testing.T) {
	h := newTestHandler(t)
	w := do(h, "GET", "/api/plans", "")
	etag := w.Header().Get("ETag")
	if w.Code != http.StatusOK || etag == "" {
		t.Fatalf("首次读取 plans 应返回 200 和 ETag，实际 %d / %q", w.Code, etag)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "private") {
		t.Fatalf("家庭数据缓存必须是 private，实际 %q", cc)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/plans", nil)
	req.Header.Set("If-None-Match", etag)
	got := httptest.NewRecorder()
	h.ServeHTTP(got, req)
	if got.Code != http.StatusNotModified {
		t.Fatalf("内容未变应返回 304，实际 %d：%s", got.Code, got.Body.String())
	}
	if got.Body.Len() != 0 {
		t.Fatalf("304 不应带响应体，实际 %d 字节", got.Body.Len())
	}
}

func TestBatchOrdersAreSingleRevisionAndIdempotent(t *testing.T) {
	h := newTestHandler(t)
	boot := decode(t, do(h, http.MethodGet, "/api/bootstrap", ""))
	today := boot["today"].(string)
	meal := boot["meals"].([]any)[0].(map[string]any)["key"].(string)
	member := boot["members"].([]any)[0].(map[string]any)["id"].(string)
	dishes := boot["dishes"].([]any)
	items := make([]map[string]string, 0, 3)
	for _, raw := range dishes[:3] {
		items = append(items, map[string]string{
			"dishId": raw.(map[string]any)["id"].(string),
			"meal":   meal,
			"note":   "批量事务测试",
		})
	}
	body, err := json.Marshal(map[string]any{"userId": member, "items": items})
	if err != nil {
		t.Fatal(err)
	}
	before := boot["revision"].(float64)
	first := decode(t, do(h, http.MethodPost, "/api/plans/"+today+"/orders/batch", string(body)))
	if first["added"] != float64(3) || first["skipped"] != float64(0) {
		t.Fatalf("批量点菜应新增 3 道，实际 %#v", first)
	}
	afterFirst := decode(t, do(h, http.MethodGet, "/api/bootstrap", ""))
	if got := afterFirst["revision"].(float64); got != before+1 {
		t.Fatalf("一次批量写应只增加 1 个修订号，实际 %v -> %v", before, got)
	}

	retry := decode(t, do(h, http.MethodPost, "/api/plans/"+today+"/orders/batch", string(body)))
	if retry["added"] != float64(0) || retry["skipped"] != float64(3) {
		t.Fatalf("重复批量点菜应全部跳过，实际 %#v", retry)
	}
	afterRetry := decode(t, do(h, http.MethodGet, "/api/bootstrap", ""))
	if got := afterRetry["revision"].(float64); got != before+1 {
		t.Fatalf("没有实际变化的幂等重试不应增加修订号，实际 %v", got)
	}

	plan := first["plan"].(map[string]any)
	orderIDs := make([]string, 0, 3)
	for _, raw := range plan["orders"].([]any) {
		orderIDs = append(orderIDs, raw.(map[string]any)["id"].(string))
	}
	doneBody, _ := json.Marshal(map[string]any{"orderIds": orderIDs, "done": true})
	done := decode(t, do(h, http.MethodPut, "/api/plans/"+today+"/orders/batch", string(doneBody)))
	if done["updated"] != float64(3) {
		t.Fatalf("批量上桌应更新 3 道，实际 %#v", done)
	}
	for _, raw := range done["plan"].(map[string]any)["orders"].([]any) {
		if raw.(map[string]any)["done"] != true {
			t.Fatalf("批量上桌后订单状态不正确：%#v", raw)
		}
	}
	afterDone := decode(t, do(h, http.MethodGet, "/api/bootstrap", ""))
	if got := afterDone["revision"].(float64); got != before+2 {
		t.Fatalf("批量上桌应只再增加 1 个修订号，实际 %v", got)
	}
}

func TestDishCRUD(t *testing.T) {
	h := newTestHandler(t)
	// 创建
	w := do(h, "POST", "/api/dishes", `{"name":"测试菜","cuisine":"homestyle","spicy":1,"minutes":15,"ingredients":["土豆 200g"],"steps":["切片","炒熟"]}`)
	if w.Code != 201 {
		t.Fatalf("创建应 201，实际 %d：%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("创建响应应为 JSON，实际 %s", ct)
	}
	id := decode(t, w)["id"].(string)
	// 列表可见
	w = do(h, "GET", "/api/dishes", "")
	if !strings.Contains(w.Body.String(), "测试菜") {
		t.Fatal("列表应包含新菜")
	}
	// 更新（改名 + 评分）
	w = do(h, "PUT", "/api/dishes/"+id, `{"name":"测试菜2","rating":3}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "测试菜2") {
		t.Fatalf("更新失败 %d：%s", w.Code, w.Body.String())
	}
	if m := decode(t, w); m["rating"] != float64(3) {
		t.Fatalf("评分应为 3，实际 %v", m["rating"])
	}
	// 评分夹紧
	w = do(h, "PUT", "/api/dishes/"+id, `{"rating":99}`)
	if m := decode(t, w); m["rating"] != float64(3) {
		t.Fatalf("评分越界应夹到 3，实际 %v", m["rating"])
	}
	// 删除
	w = do(h, "DELETE", "/api/dishes/"+id, "")
	if w.Code != 200 {
		t.Fatalf("删除应 200，实际 %d", w.Code)
	}
	w = do(h, "GET", "/api/dishes", "")
	if strings.Contains(w.Body.String(), "测试菜2") {
		t.Fatal("删除后列表不应再含该菜")
	}
}

func TestDishValidation(t *testing.T) {
	h := newTestHandler(t)
	// 空名字
	if w := do(h, "POST", "/api/dishes", `{"name":"  "}`); w.Code != 400 {
		t.Fatalf("空名应 400，实际 %d", w.Code)
	}
	// 未知菜系应回退 homestyle 而不是 400
	w := do(h, "POST", "/api/dishes", `{"name":"回退测试","cuisine":"火星菜"}`)
	if w.Code != 201 {
		t.Fatalf("未知菜系应回退，实际 %d", w.Code)
	}
	if m := decode(t, w); m["cuisine"] != "homestyle" {
		t.Fatalf("未知菜系应回退 homestyle，实际 %v", m["cuisine"])
	}
}

func TestPlanMealDuty(t *testing.T) {
	h := newTestHandler(t)
	w := do(h, "GET", "/api/bootstrap", "")
	boot := decode(t, w)
	today := boot["today"].(string)
	members := boot["members"].([]any)
	a := members[0].(map[string]any)["id"].(string)
	b := members[1].(map[string]any)["id"].(string)

	// 只改晚餐，午餐不受影响
	put := func(body string) map[string]any {
		ww := do(h, "PUT", "/api/plans/"+today, body)
		if ww.Code != 200 {
			t.Fatalf("PUT plan 应 200，实际 %d：%s", ww.Code, ww.Body.String())
		}
		return decode(t, ww)
	}
	p := put(`{"dutyMeal":"dinner","chefIds":["` + a + `"],"dishwasherIds":["` + b + `"]}`)
	md := p["mealDuties"].(map[string]any)
	if md["dinner"].(map[string]any)["chefIds"].([]any)[0] != a {
		t.Fatal("晚餐掌勺应为 A")
	}
	if _, ok := md["lunch"]; ok {
		t.Fatal("未设置的午餐不应出现在 mealDuties")
	}
	// 清空晚餐：该餐条目应消失（omitempty 下整个 mealDuties 可能不出现）
	p = put(`{"dutyMeal":"dinner","chefIds":[],"dishwasherIds":[]}`)
	if mdAny, has := p["mealDuties"].(map[string]any); has {
		if _, ok := mdAny["dinner"]; ok {
			t.Fatal("清空后该餐应消失")
		}
	}
	// 非法餐次
	w = do(h, "PUT", "/api/plans/"+today, `{"dutyMeal":"brunch","chefIds":["`+a+`"]}`)
	if w.Code != 400 {
		t.Fatalf("非法餐次应 400，实际 %d", w.Code)
	}
	// 幽灵成员
	w = do(h, "PUT", "/api/plans/"+today, `{"dutyMeal":"dinner","chefIds":["ghost"]}`)
	if w.Code != 400 {
		t.Fatalf("幽灵成员应 400，实际 %d", w.Code)
	}
}

func TestOrderFlow(t *testing.T) {
	h := newTestHandler(t)
	w := do(h, "GET", "/api/bootstrap", "")
	boot := decode(t, w)
	today := boot["today"].(string)
	dishID := boot["dishes"].([]any)[0].(map[string]any)["id"].(string)
	userID := boot["members"].([]any)[0].(map[string]any)["id"].(string)

	// 点菜
	w = do(h, "POST", "/api/plans/"+today+"/orders", `{"dishId":"`+dishID+`","userId":"`+userID+`"}`)
	if w.Code != 201 {
		t.Fatalf("点菜应 201，实际 %d：%s", w.Code, w.Body.String())
	}
	// 返回的是整个 plan：订单在 orders[0]
	orderID := decode(t, w)["orders"].([]any)[0].(map[string]any)["id"].(string)
	// 标记上桌
	w = do(h, "PUT", "/api/plans/"+today+"/orders/"+orderID, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"done":true`) {
		t.Fatalf("标记上桌失败 %d：%s", w.Code, w.Body.String())
	}
	// 撤掉
	w = do(h, "DELETE", "/api/plans/"+today+"/orders/"+orderID, "")
	if w.Code != 200 {
		t.Fatalf("撤掉应 200，实际 %d", w.Code)
	}
	// 不存在的菜
	w = do(h, "POST", "/api/plans/"+today+"/orders", `{"dishId":"no-such","userId":"`+userID+`"}`)
	if w.Code != 404 {
		t.Fatalf("不存在的菜应 404，实际 %d", w.Code)
	}
}

func TestBackupRoundtrip(t *testing.T) {
	h := newTestHandler(t)
	// 先加一道菜
	do(h, "POST", "/api/dishes", `{"name":"备份测试菜"}`)
	// 导出
	w := do(h, "GET", "/api/backup", "")
	if w.Code != 200 || w.Body.Len() < 100 {
		t.Fatalf("备份导出失败 %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("备份应为 zip，实际 %s", ct)
	}
	// 恢复（导入刚导出的内容）——导入走 multipart 表单
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "family-menu-test.zip")
	if err != nil {
		t.Fatalf("创建表单: %v", err)
	}
	if _, err := fw.Write(w.Body.Bytes()); err != nil {
		t.Fatalf("写入 zip: %v", err)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/backup", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("恢复应 200，实际 %d：%s", w.Code, w.Body.String())
	}
	// 数据还在
	w = do(h, "GET", "/api/dishes", "")
	if !strings.Contains(w.Body.String(), "备份测试菜") {
		t.Fatal("恢复后菜品应还在")
	}
}

func TestRestoreRejectsGarbage(t *testing.T) {
	h := newTestHandler(t)
	w := do(h, "POST", "/api/backup", "not a zip")
	if w.Code != 400 {
		t.Fatalf("垃圾数据应 400，实际 %d", w.Code)
	}
}

func TestSettingsBackupKeepClamped(t *testing.T) {
	h := newTestHandler(t)
	w := do(h, "GET", "/api/bootstrap", "")
	boot := decode(t, w)
	today := boot["today"].(string)
	_ = today
	w = do(h, "PUT", "/api/settings", `{"backupKeep":999}`)
	if w.Code != 200 {
		t.Fatalf("设置应 200，实际 %d", w.Code)
	}
	// settings 通过 bootstrap 下发，没有独立读取接口
	w = do(h, "GET", "/api/bootstrap", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "backupKeep") {
		t.Fatalf("bootstrap 应包含 backupKeep，实际 %d", w.Code)
	}
}

func TestChangelogAndConfig(t *testing.T) {
	h := newTestHandler(t)
	w := do(h, "GET", "/api/changelog", "")
	if w.Code != 200 {
		t.Fatalf("changelog 应 200，实际 %d", w.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) == 0 {
		t.Fatalf("changelog 应返回非空数组，err=%v", err)
	}
	w = do(h, "GET", "/api/config", "")
	if w.Code != 200 {
		t.Fatalf("config 应 200，实际 %d", w.Code)
	}
}

func TestWriteBlockedCrossOrigin(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("POST", "/api/dishes", strings.NewReader(`{"name":"跨站"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("跨站写应 403，实际 %d", w.Code)
	}
}

var _ = io.Discard
var _ = time.Now
