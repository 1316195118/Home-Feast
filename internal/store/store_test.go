package store

import (
	"testing"
	"time"
)

// 餐别时段与过期清理的回归测试。用固定时区构造时间，避免依赖运行机器的时区。
var cst = time.FixedZone("CST", 8*3600)

const testDate = "2026-11-09"

func at(hour, minute int) string {
	return time.Date(2026, 11, 9, hour, minute, 0, 0, cst).Format(time.RFC3339)
}

func nowAt(hour, minute int) time.Time {
	return time.Date(2026, 11, 9, hour, minute, 0, 0, cst)
}

func order(id, meal, createdAt string) Order {
	return Order{ID: id, Meal: meal, CreatedAt: createdAt}
}

func ids(p Plan) []string {
	out := make([]string, 0, len(p.Orders))
	for _, o := range p.Orders {
		out = append(out, o.ID)
	}
	return out
}

func TestMealEndMinutes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"00:00", 0}, {"09:30", 570}, {"10:00", 600}, {"23:59", 1439}, {"24:00", 1440},
		{"", -1}, {"10", -1}, {"10:60", -1}, {"25:00", -1}, {"24:01", -1}, {"aa:bb", -1},
	}
	for _, c := range cases {
		if got := MealEndMinutes(c.in); got != c.want {
			t.Errorf("MealEndMinutes(%q)=%d，期望 %d", c.in, got, c.want)
		}
	}
}

func TestDefaultMealsFallback(t *testing.T) {
	for _, in := range [][]MealSlot{nil, {}, {{Key: "bad", End: "oops"}}} {
		got := effectiveMeals(in)
		if len(got) != 3 || got[0].Key != "breakfast" || got[2].Key != "dinner" {
			t.Errorf("空/非法配置应回退默认三餐，实际 %+v", got)
		}
	}
}

func TestCurrentMeal(t *testing.T) {
	cases := []struct {
		hour int
		want string
	}{
		{6, "breakfast"}, {9, "breakfast"},
		{10, "lunch"}, {14, "lunch"},
		{15, "dinner"}, {22, "dinner"}, {23, "dinner"},
	}
	for _, c := range cases {
		if got := CurrentMeal(nowAt(c.hour, 0), nil); got != c.want {
			t.Errorf("%d 点：CurrentMeal=%q，期望 %q", c.hour, got, c.want)
		}
	}
}

// 用户自定义名字和时间：结束时刻精确到分钟，且不再叫早/中/晚。
func customMeals() []MealSlot {
	return []MealSlot{
		{Key: "morning-tea", Label: "早茶", Icon: "🫖", End: "09:30"},
		{Key: "noon", Label: "午饭", Icon: "🍚", End: "14:00"},
		{Key: "supper", Label: "夜宵", Icon: "🌃", End: "22:00"},
	}
}

func TestCurrentMeal_CustomMeals(t *testing.T) {
	cases := []struct {
		h, m int
		want string
	}{
		{8, 0, "morning-tea"}, {9, 29, "morning-tea"},
		{9, 30, "noon"}, {13, 59, "noon"},
		{14, 0, "supper"}, {21, 59, "supper"},
		{22, 0, "supper"}, {23, 30, "supper"}, // 全都过点后归最后一餐
	}
	for _, c := range cases {
		if got := CurrentMeal(nowAt(c.h, c.m), customMeals()); got != c.want {
			t.Errorf("%02d:%02d：CurrentMeal=%q，期望 %q", c.h, c.m, got, c.want)
		}
	}
}

func TestNormalizeMeals(t *testing.T) {
	if _, err := NormalizeMeals(nil); err == nil {
		t.Error("空配置应报错")
	}
	if _, err := NormalizeMeals([]MealSlot{{Key: "a", Label: "一", End: "10:00"}}); err != nil {
		t.Errorf("合法单餐应通过：%v", err)
	}
	if _, err := NormalizeMeals([]MealSlot{
		{Key: "a", Label: "一", End: "10:00"}, {Key: "a", Label: "二", End: "15:00"},
	}); err == nil {
		t.Error("重复 key 应报错")
	}
	if _, err := NormalizeMeals([]MealSlot{{Key: "a", Label: "一", End: "99:99"}}); err == nil {
		t.Error("非法时间应报错")
	}
	if _, err := NormalizeMeals([]MealSlot{{Key: "A B", Label: "一", End: "10:00"}}); err == nil {
		t.Error("非法 key 应报错")
	}
	// 乱序提交会按结束时刻排序，保证当前餐推导正确
	got, err := NormalizeMeals([]MealSlot{
		{Key: "late", Label: "晚", End: "20:00"}, {Key: "early", Label: "早", End: "08:00"},
	})
	if err != nil || got[0].Key != "early" {
		t.Errorf("应按时间排序，actual=%+v err=%v", got, err)
	}
}

func TestDropExpired_NormalMeals(t *testing.T) {
	p := Plan{Date: testDate, Orders: []Order{
		order("breakfast", "breakfast", at(8, 30)),
		order("lunch", "lunch", at(12, 30)),
		order("dinner", "dinner", at(17, 0)),
	}}
	if !dropExpired(&p, nowAt(18, 0), nil) {
		t.Fatal("早/午餐已过饭点，应发生清理")
	}
	got := ids(p)
	if len(got) != 1 || got[0] != "dinner" {
		t.Fatalf("应只保留晚餐，实际 %v", got)
	}
}

func TestDropExpired_LateNightOrderKept(t *testing.T) {
	// 23:00 后点的菜属于「过了饭点才加」，不能被立刻清掉；
	// 饭点前点的晚餐仍然按时收起。
	p := Plan{Date: testDate, Orders: []Order{
		order("old-dinner", "dinner", at(18, 0)),
		order("late-snack", "dinner", at(23, 10)),
	}}
	if !dropExpired(&p, nowAt(23, 30), nil) {
		t.Fatal("旧晚餐应被清掉")
	}
	got := ids(p)
	if len(got) != 1 || got[0] != "late-snack" {
		t.Fatalf("深夜加菜应保留，实际 %v", got)
	}
}

func TestDropExpired_CustomMeals(t *testing.T) {
	// 早茶 09:30 收起：9:00 点的会被清，10:00 才点的（补记）保留。
	p := Plan{Date: testDate, Orders: []Order{
		order("tea-old", "morning-tea", at(9, 0)),
		order("tea-backfill", "morning-tea", at(10, 0)),
		order("noon", "noon", at(12, 0)),
	}}
	if !dropExpired(&p, nowAt(9, 31), customMeals()) {
		t.Fatal("早茶已过 09:30，应发生清理")
	}
	got := ids(p)
	if len(got) != 2 || got[0] != "tea-backfill" || got[1] != "noon" {
		t.Fatalf("自定义时段清理结果不对，实际 %v", got)
	}
}

func TestDropExpired_ManualBackfillKept(t *testing.T) {
	// 用户手动切到已过饭点的餐别补记一道菜，同样不该被立刻清掉。
	p := Plan{Date: testDate, Orders: []Order{
		order("manual", "breakfast", at(18, 20)),
	}}
	if dropExpired(&p, nowAt(18, 30), nil) {
		t.Fatalf("手动补记被误清，剩余 %v", ids(p))
	}
}

func TestDropExpired_UnclassifiedAndHistoryUntouched(t *testing.T) {
	// 旧数据（meal 为空）或已被删掉的餐别，不参与清理
	p := Plan{Date: testDate, Orders: []Order{
		order("legacy", "", at(8, 0)),
		order("removed-meal", "deleted-slot", at(8, 0)),
	}}
	if dropExpired(&p, nowAt(23, 0), nil) {
		t.Fatalf("未归类/已删除餐别的订单被误清，剩余 %v", ids(p))
	}
	// 历史日期永不清理
	hist := Plan{Date: "2026-11-08", Orders: []Order{
		order("past", "breakfast", "2026-11-08T08:00:00+08:00"),
	}}
	if dropExpired(&hist, nowAt(23, 0), nil) {
		t.Fatal("历史记录不应被清理")
	}
}
