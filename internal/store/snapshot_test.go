package store

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBootstrapSnapshotIsIndependent(t *testing.T) {
	st, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dishes := st.ListDishes()
	if len(dishes) == 0 {
		t.Fatal("测试库应包含内置菜品")
	}
	date := "2026-11-21"
	order, err := st.AddOrder(date, Order{DishID: dishes[0].ID, DishName: dishes[0].Name})
	if err != nil {
		t.Fatal(err)
	}
	dice := map[string]int{"u-test": 6}
	if _, err := st.SavePlan(date, PlanPatch{Dice: &dice}); err != nil {
		t.Fatal(err)
	}

	first := st.Bootstrap(date)
	if len(first.Plans) != 1 || len(first.Plan.Orders) != 1 {
		t.Fatalf("首屏快照不完整: plans=%d orders=%d", len(first.Plans), len(first.Plan.Orders))
	}

	// HTTP 序列化发生在锁外；调用方即使误改返回值，也不能污染 Store 内部状态。
	first.Plan.Orders[0].Done = true
	first.Plan.Dice["u-test"] = 1
	first.Plans[0].Orders[0].Note = "被外部修改"

	second := st.Bootstrap(date)
	if second.Plan.Orders[0].ID != order.Orders[0].ID || second.Plan.Orders[0].Done {
		t.Fatalf("返回订单没有深拷贝: %+v", second.Plan.Orders)
	}
	if second.Plan.Dice["u-test"] != 6 {
		t.Fatalf("返回骰子 map 没有深拷贝: %v", second.Plan.Dice)
	}
	if second.Plans[0].Orders[0].Note != "" {
		t.Fatalf("历史计划没有深拷贝: %+v", second.Plans[0].Orders)
	}
}

func TestOrderLookupAndRevision(t *testing.T) {
	st, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dish := st.ListDishes()[0]
	member := st.ListMembers()[0]
	before := st.Revision()
	lookup := st.OrderLookup(dish.ID, member.ID)
	if !lookup.DishFound || lookup.Dish.ID != dish.ID || lookup.Dish.Name != dish.Name {
		t.Fatalf("目标菜品查找失败: %+v", lookup)
	}
	if !lookup.MemberFound || lookup.Member.ID != member.ID || lookup.Member.Name != member.Name {
		t.Fatalf("目标成员查找失败: %+v", lookup)
	}
	if len(lookup.Meals) == 0 {
		t.Fatal("点菜校验应返回有效餐次")
	}
	if missing := st.OrderLookup("missing-dish", "missing-member"); missing.DishFound || missing.MemberFound {
		t.Fatalf("不存在的目标不应被误判为找到: %+v", missing)
	}

	if _, err := st.UpdateDish(dish.ID, DishPatch{Favorite: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	if after := st.Revision(); after <= before {
		t.Fatalf("写操作后修订号应递增，before=%d after=%d", before, after)
	}
	if snapshot := st.Bootstrap("2026-11-21"); snapshot.Revision != st.Revision() {
		t.Fatalf("快照修订号应与数据一致，snapshot=%d store=%d", snapshot.Revision, st.Revision())
	}
}

func boolPtr(v bool) *bool { return &v }

func TestConcurrentReadPlanWhileWriting(t *testing.T) {
	st, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dishes := st.ListDishes()
	date := "2026-11-21"
	plan, err := st.AddOrder(date, Order{DishID: dishes[0].ID, DishName: dishes[0].Name})
	if err != nil {
		t.Fatal(err)
	}
	orderID := plan.Orders[0].ID

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				_ = st.Bootstrap(date)
				_ = st.Planning()
				_ = st.ListPlans()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 12; j++ {
			if _, err := st.ToggleOrder(date, orderID); err != nil {
				errs <- err
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("并发读写超时，疑似持锁时重入")
	}
	close(errs)
	for err := range errs {
		t.Fatalf("并发切换订单失败: %v", err)
	}
}

func TestLastMemberDeletionIsAtomic(t *testing.T) {
	st, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for len(st.ListMembers()) > 2 {
		if err := st.DeleteMember(st.ListMembers()[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	members := st.ListMembers()
	if len(members) != 2 {
		t.Fatalf("准备阶段应剩 2 位成员，实际 %d", len(members))
	}

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, member := range members {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			results <- st.DeleteMember(id)
		}(member.ID)
	}
	wg.Wait()
	close(results)

	success, blocked := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrLastMember):
			blocked++
		default:
			t.Fatalf("意外错误: %v", err)
		}
	}
	if success != 1 || blocked != 1 || len(st.ListMembers()) != 1 {
		t.Fatalf("并发删除应只成功一次并保留一人: success=%d blocked=%d remain=%d",
			success, blocked, len(st.ListMembers()))
	}
}
