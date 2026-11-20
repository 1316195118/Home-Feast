package store

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
}

// 启动体检应当修掉：重复 ID、未知菜系、越界数值、悬空的分工引用
func TestRepairOnOpen(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	writeFile(t, filepath.Join(dir, "dishes.json"), `[
		{"id":"d1","name":"甲菜","cuisine":"sichuan","spicy":9,"minutes":9999},
		{"id":"d1","name":"重复ID","cuisine":"sichuan"},
		{"id":"","name":"空ID","cuisine":"sichuan"},
		{"id":"d2","name":"乙菜","cuisine":"不存在的菜系"}
	]`)
	writeFile(t, filepath.Join(dir, "members.json"), `[
		{"id":"u1","name":"张三","avatar":"🙂","color":"#d9a066"},
		{"id":"u1","name":"重复成员"}
	]`)
	writeFile(t, filepath.Join(dir, "plans.json"), `[
		{"date":"2026-01-01","chefIds":["u1","ghost"],"dishwasherIds":["ghost"]},
		{"date":"bad-date","chefIds":["u1"]}
	]`)

	st, err := Open(dir, cfg)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	dishes := st.ListDishes()
	if len(dishes) != 2 {
		t.Fatalf("应只剩 2 道有效菜品，实际 %d: %+v", len(dishes), dishes)
	}
	for _, d := range dishes {
		if d.Spicy < 0 || d.Spicy > 3 {
			t.Fatalf("辣度未夹紧: %s=%d", d.Name, d.Spicy)
		}
		if d.Minutes < 0 || d.Minutes > 600 {
			t.Fatalf("耗时未夹紧: %s=%d", d.Name, d.Minutes)
		}
		if d.Cuisine == "不存在的菜系" {
			t.Fatalf("未知菜系未回退: %s", d.Name)
		}
	}

	if got := len(st.ListMembers()); got != 1 {
		t.Fatalf("重复成员应去重，实际 %d 个", got)
	}

	p := st.GetPlan("2026-01-01")
	if len(p.ChefIDs) != 1 || p.ChefIDs[0] != "u1" {
		t.Fatalf("悬空掌勺引用未清理: %v", p.ChefIDs)
	}
	if len(p.DishwasherIDs) != 0 {
		t.Fatalf("全是悬空引用的洗碗名单应清空: %v", p.DishwasherIDs)
	}
	if got := st.GetPlan("bad-date"); len(got.Orders) != 0 || len(got.ChefIDs) != 0 {
		t.Fatalf("非法日期的安排应被丢弃，实际 %+v", got)
	}
}

// 删除成员要把他从分工与骰子里摘掉，历史订单保留
func TestDeleteMemberCleansDuty(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	st, err := Open(dir, cfg)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	st.SaveSettings(Settings{FamilyName: "家"})
	m1, err := st.AddMember(Member{Name: "甲", Avatar: "🙂", Color: "#d9a066"})
	if err != nil {
		t.Fatalf("加成员失败: %v", err)
	}
	m2, err := st.AddMember(Member{Name: "乙", Avatar: "🙂", Color: "#cf8b9b"})
	if err != nil {
		t.Fatalf("加成员失败: %v", err)
	}
	ids := []string{m1.ID, m2.ID}
	if _, err := st.SavePlan("2026-02-02", PlanPatch{ChefIDs: &ids}); err != nil {
		t.Fatalf("写分工失败: %v", err)
	}
	dice := map[string]int{m1.ID: 5, m2.ID: 3}
	if _, err := st.SavePlan("2026-02-02", PlanPatch{Dice: &dice}); err != nil {
		t.Fatalf("写骰子失败: %v", err)
	}

	if err := st.DeleteMember(m1.ID); err != nil {
		t.Fatalf("删成员失败: %v", err)
	}

	p := st.GetPlan("2026-02-02")
	if len(p.ChefIDs) != 1 || p.ChefIDs[0] != m2.ID {
		t.Fatalf("被删成员应只剩另一个人: %v", p.ChefIDs)
	}
	if _, has := p.Dice[m1.ID]; has {
		t.Fatalf("骰子记录里仍有被删成员")
	}
}

// 文件损坏时应当留档并用空数据启动，而不是拒绝启动
func TestCorruptFileDegrades(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	writeFile(t, filepath.Join(dir, "members.json"), `[{"id":"u1","name":"张三","avatar":"🙂","color":"#d9a066"}]`)
	writeFile(t, filepath.Join(dir, "dishes.json"), `{"坏掉的内容":`)

	st, err := Open(dir, cfg)
	if err != nil {
		t.Fatalf("损坏文件不应阻止启动: %v", err)
	}
	// 菜品会回退到内置种子
	if len(st.ListDishes()) == 0 {
		t.Fatalf("损坏后应以内置菜品启动")
	}
	// 成员文件是好的，应当保留
	if len(st.ListMembers()) != 1 {
		t.Fatalf("正常文件不应受影响，实际 %d 个成员", len(st.ListMembers()))
	}
	// 损坏文件应被留档
	matches, _ := filepath.Glob(filepath.Join(dir, "dishes.json.corrupt-*"))
	if len(matches) != 1 {
		t.Fatalf("应留下 1 份损坏留档，实际 %v", matches)
	}
}

func TestSameDataAndConfigDirKeepsSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, settingsFile)
	writeFile(t, path, `{"familyName":"同一目录","backupKeep":7}`)

	st, err := Open(dir, dir)
	if err != nil {
		t.Fatalf("同目录启动失败: %v", err)
	}
	if got := st.GetSettings().FamilyName; got != "同一目录" {
		t.Fatalf("同目录设置被误删或未读取，实际 %q", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("原设置文件不应被迁移逻辑删除: %v", err)
	}
}

func TestLegacySettingsMigration(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	legacy := filepath.Join(dir, settingsFile)
	writeFile(t, legacy, `{"familyName":"旧目录","backupKeep":7}`)

	st, err := Open(dir, cfg)
	if err != nil {
		t.Fatalf("迁移启动失败: %v", err)
	}
	if got := st.GetSettings().FamilyName; got != "旧目录" {
		t.Fatalf("迁移后设置错误，实际 %q", got)
	}
	if _, err := os.Stat(filepath.Join(cfg, settingsFile)); err != nil {
		t.Fatalf("新设置文件未生成: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("迁移成功后旧设置应移除，实际 err=%v", err)
	}
}
