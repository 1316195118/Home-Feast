package store

import (
	"path/filepath"
	"testing"
)

// 分工改成多人数组后，最容易回归的是「清空」：旧单值字段若不清，NormalizeDuty 会把
// 上一次的人补回数组，表现为点了「退出」却还在名单里。
func TestPlanDutyArrays(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir, filepath.Join(dir, "config"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}

	ids := []string{"a", "b"}
	p, err := st.SavePlan("2026-01-01", PlanPatch{ChefIDs: &ids})
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if len(p.ChefIDs) != 2 {
		t.Fatalf("应写入 2 人，实际 %v", p.ChefIDs)
	}
	if p.ChefID != "a" {
		t.Fatalf("兼容单值应同步为首位 a，实际 %q", p.ChefID)
	}

	// 清空：不能被旧单值补回来
	empty := []string{}
	p, err = st.SavePlan("2026-01-01", PlanPatch{ChefIDs: &empty})
	if err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if len(p.ChefIDs) != 0 || p.ChefID != "" {
		t.Fatalf("清空后应完全为空，实际 chefIds=%v chefId=%q", p.ChefIDs, p.ChefID)
	}
	if got := st.GetPlan("2026-01-01"); len(got.ChefIDs) != 0 {
		t.Fatalf("重新读取后仍应为空，实际 %v", got.ChefIDs)
	}

	// 只有旧单值的存量数据，读取时应自动补出数组
	wash := []string{"c"}
	if _, err := st.SavePlan("2026-01-02", PlanPatch{DishwasherIDs: &wash}); err != nil {
		t.Fatalf("写入洗碗失败: %v", err)
	}
	got := st.GetPlan("2026-01-02")
	if len(got.DishwasherIDs) != 1 || got.DishwasherIDs[0] != "c" {
		t.Fatalf("洗碗名单应含 c，实际 %v", got.DishwasherIDs)
	}
	if got.DishwasherID != "c" {
		t.Fatalf("兼容单值应为 c，实际 %q", got.DishwasherID)
	}

	// 掌勺与洗碗互不影响
	if len(got.ChefIDs) != 0 {
		t.Fatalf("只设洗碗时掌勺应为空，实际 %v", got.ChefIDs)
	}
}
