package api

// handler 层的冒烟 + CRUD 回归测试：覆盖核心接口，防止后续改动悄悄破坏主链路。

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"familymenu/internal/store"
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
	for _, key := range []string{"today", "dishes", "members", "cuisines", "meals", "plan", "settings"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("bootstrap 缺少字段 %s", key)
		}
	}
	if len(m["dishes"].([]any)) == 0 || len(m["members"].([]any)) == 0 {
		t.Fatal("新库应自动播种菜品与成员")
	}
}

func TestDishCRUD(t *testing.T) {
	h := newTestHandler(t)
	// 创建
	w := do(h, "POST", "/api/dishes", `{"name":"测试菜","cuisine":"homestyle","spicy":1,"minutes":15,"ingredients":["土豆 200g"],"steps":["切片","炒熟"]}`)
	if w.Code != 201 {
		t.Fatalf("创建应 201，实际 %d：%s", w.Code, w.Body.String())
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
