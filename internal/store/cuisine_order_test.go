package store

import "testing"

func TestOrderedCuisines(t *testing.T) {
	// 空顺序 = 内置顺序
	got := OrderedCuisines(nil)
	if len(got) != len(Cuisines) || got[0].Key != Cuisines[0].Key {
		t.Fatalf("空顺序应返回内置顺序")
	}
	// 指定部分顺序：指定的排前面并按给定次序，其余保持内置相对顺序
	order := []string{"snack", "sichuan"}
	got = OrderedCuisines(order)
	if got[0].Key != "snack" || got[1].Key != "sichuan" {
		t.Fatalf("前两位应为 snack/sichuan，实际 %s/%s", got[0].Key, got[1].Key)
	}
	if len(got) != len(Cuisines) {
		t.Fatalf("分类数量不能变: %d != %d", len(got), len(Cuisines))
	}
	// 未列出的保持内置顺序：xianjiang 在内置里排第 2，现在应排第 3
	if got[2].Key != "xinjiang" {
		t.Fatalf("第三位应为 xinjiang，实际 %s", got[2].Key)
	}
	// 未知 key 被忽略
	got = OrderedCuisines([]string{"not_exist", "hunan"})
	if got[0].Key != "hunan" {
		t.Fatalf("未知 key 应忽略，首位应为 hunan，实际 %s", got[0].Key)
	}
}
