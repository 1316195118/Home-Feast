package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestUpdateShoppingMergesConcurrentMarks(t *testing.T) {
	st, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	members := st.ListMembers()
	if len(members) == 0 {
		t.Fatal("测试库应包含成员")
	}
	const date = "2026-11-23"

	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			member := members[index%len(members)]
			status := "claimed"
			if index%3 == 0 {
				status = "bought"
			}
			_, err := st.UpdateShopping(date, member.ID, []ShoppingOp{{
				Key: fmt.Sprintf("%s::食材%02d", date, index), Status: status,
			}})
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("并发采购状态写入失败: %v", err)
	}

	marks := st.SettingsSnapshot().Settings.ShoppingMarks
	if len(marks) != 24 {
		t.Fatalf("并发写入不应互相覆盖，期望 24 项，实际 %d", len(marks))
	}
	for i := 0; i < 24; i++ {
		key := fmt.Sprintf("%s::食材%02d", date, i)
		mark, ok := marks[key]
		if !ok || mark.Date != date || mark.Status == "" || mark.UserID == "" {
			t.Fatalf("采购状态缺失或不完整: %q=%+v", key, mark)
		}
	}
}

func TestUpdateShoppingValidationAndClear(t *testing.T) {
	st, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	member := st.ListMembers()[0]

	if _, err := st.UpdateShopping("not-a-date", member.ID, []ShoppingOp{{Key: "番茄", Status: "bought"}}); err == nil {
		t.Fatal("非法日期应被拒绝")
	}
	if _, err := st.UpdateShopping("2026-11-23", "missing-member", []ShoppingOp{{Key: "番茄", Status: "bought"}}); err == nil {
		t.Fatal("不存在的成员应被拒绝")
	}
	if _, err := st.UpdateShopping("2026-11-23", member.ID, nil); err == nil {
		t.Fatal("空变更应被拒绝")
	}

	if _, err := st.UpdateShopping("2026-11-23", member.ID, []ShoppingOp{
		{Key: "番茄", Status: "claimed"},
		{Key: "  ", Status: "bought"},
	}); err != nil {
		t.Fatal(err)
	}
	settings, err := st.UpdateShopping("2026-11-23", member.ID, []ShoppingOp{{Key: "番茄"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := settings.ShoppingMarks["番茄"]; ok {
		t.Fatal("空 status 应清除对应采购标记")
	}
}
