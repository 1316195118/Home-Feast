package api

import (
	"net/http"
	"time"

	"familymenu/internal/store"
)

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	jsonRevalidate(w, r, s.st.ListPlans())
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	jsonOK(w, s.st.GetPlan(date))
}

// updatePlan 只更新当天「谁掌勺 / 谁洗碗 / 备注」，已点的菜不受影响。
func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	var in struct {
		DutyMeal      string          `json:"dutyMeal"` // 非空表示只改这一餐
		ChefIDs       *[]string       `json:"chefIds"`
		DishwasherIDs *[]string       `json:"dishwasherIds"`
		ChefID        *string         `json:"chefId"`       // 兼容旧前端
		DishwasherID  *string         `json:"dishwasherId"` // 兼容旧前端
		Note          *string         `json:"note"`
		Dice          *map[string]int `json:"dice"`
		DicePhase     *string         `json:"dicePhase"`
		PKShown       *[]string       `json:"pkShown"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	planning := s.st.DutySnapshot()
	members := planning.Members
	patch := store.PlanPatch{}

	// 按餐分工：餐次必须存在于已配置的用餐时段里
	if in.DutyMeal != "" {
		ok := false
		for _, m := range planning.Meals {
			if m.Key == in.DutyMeal {
				ok = true
				break
			}
		}
		if !ok {
			jsonErr(w, http.StatusBadRequest, "餐次不在已配置的用餐时段里，请刷新页面重试")
			return
		}
		patch.DutyMeal = in.DutyMeal
	}

	// 多人分工：去重、限数量、必须都是家里成员
	cleanDuty := func(ids []string, label string) ([]string, bool) {
		seen := make(map[string]bool, len(ids))
		out := make([]string, 0, len(ids))
		for _, raw := range ids {
			v := trimmed(raw, 64)
			if v == "" || seen[v] {
				continue
			}
			if !memberExists(members, v) {
				return nil, false
			}
			seen[v] = true
			out = append(out, v)
			if len(out) >= 12 {
				break
			}
		}
		return out, true
	}

	if in.ChefIDs != nil {
		v, ok := cleanDuty(*in.ChefIDs, "掌勺")
		if !ok {
			jsonErr(w, http.StatusBadRequest, "请选择家里的成员掌勺")
			return
		}
		patch.ChefIDs = &v
	} else if in.ChefID != nil {
		v := trimmed(*in.ChefID, 64)
		if v != "" && !memberExists(members, v) {
			jsonErr(w, http.StatusBadRequest, "请选择家里的成员掌勺")
			return
		}
		patch.ChefID = &v
	}
	if in.DishwasherIDs != nil {
		v, ok := cleanDuty(*in.DishwasherIDs, "洗碗")
		if !ok {
			jsonErr(w, http.StatusBadRequest, "请选择家里的成员洗碗")
			return
		}
		patch.DishwasherIDs = &v
	} else if in.DishwasherID != nil {
		v := trimmed(*in.DishwasherID, 64)
		if v != "" && !memberExists(members, v) {
			jsonErr(w, http.StatusBadRequest, "请选择家里的成员洗碗")
			return
		}
		patch.DishwasherID = &v
	}
	if in.Note != nil {
		v := trimmed(*in.Note, 200)
		patch.Note = &v
	}
	// 骰子 PK 的点数、阶段、已看动画名单：一律校验后再落盘，
	// 防止手改请求写入不存在的成员或非 1..6 的点数。
	if in.Dice != nil {
		if len(*in.Dice) > 32 {
			jsonErr(w, http.StatusBadRequest, "骰子结果人数过多")
			return
		}
		clean := make(map[string]int, len(*in.Dice))
		for id, v := range *in.Dice {
			id = trimmed(id, 64)
			if !memberExists(members, id) || v < 1 || v > 6 {
				jsonErr(w, http.StatusBadRequest, "骰子点数不合法")
				return
			}
			clean[id] = v
		}
		patch.Dice = &clean
	}
	if in.DicePhase != nil {
		v := trimmed(*in.DicePhase, 16)
		if v != "" && v != "rolling" && v != "tied" && v != "done" {
			jsonErr(w, http.StatusBadRequest, "骰子阶段不合法")
			return
		}
		patch.DicePhase = &v
	}
	if in.PKShown != nil {
		seen := make(map[string]bool, len(*in.PKShown))
		out := make([]string, 0, len(*in.PKShown))
		for _, id := range *in.PKShown {
			id = trimmed(id, 64)
			if id == "" || seen[id] || !memberExists(members, id) {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
		if len(out) > 32 {
			out = out[:32]
		}
		patch.PKShown = &out
	}
	plan, err := s.st.SavePlan(date, patch)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	jsonOK(w, plan)
}

func (s *Server) addOrder(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	var in struct {
		DishID string `json:"dishId"`
		UserID string `json:"userId"`
		Note   string `json:"note"`
		Meal   string `json:"meal"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// 餐别以用户在设置里配置的为准，名字和时间都能改，所以不能写死早/中/晚。
	lookup := s.st.OrderLookup(in.DishID, in.UserID)
	meals := lookup.Meals
	allowed := make(map[string]bool, len(meals))
	for _, m := range meals {
		allowed[m.Key] = true
	}
	meal := trimmed(in.Meal, 32)
	if meal == "" {
		meal = store.CurrentMeal(time.Now(), meals)
	}
	if !allowed[meal] {
		jsonErr(w, http.StatusBadRequest, "餐别不在已配置的用餐时段里，请刷新页面重试")
		return
	}
	if !lookup.DishFound {
		jsonErr(w, http.StatusNotFound, "菜品不存在，可能已被删除")
		return
	}
	if !lookup.MemberFound {
		jsonErr(w, http.StatusBadRequest, "请先选择点菜的人")
		return
	}
	order := store.Order{
		DishID:      lookup.Dish.ID,
		DishName:    lookup.Dish.Name,
		DishCuisine: lookup.Dish.Cuisine,
		DishImage:   lookup.Dish.Image,
		UserID:      lookup.Member.ID,
		UserName:    lookup.Member.Name,
		UserAvatar:  lookup.Member.Avatar,
		Note:        trimmed(in.Note, 100),
		Meal:        meal,
	}
	plan, err := s.st.AddOrder(date, order)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "点菜失败")
		return
	}
	jsonStatus(w, http.StatusCreated, plan)
}

// addOrders 是一次事务的批量点菜入口。它只复制请求涉及的菜品和成员，
// 跳过已经不存在的菜，并且整批只写盘一次，避免逐道落盘阻塞全家其它请求。
func (s *Server) addOrders(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	var in struct {
		UserID string `json:"userId"`
		Items  []struct {
			DishID string `json:"dishId"`
			Note   string `json:"note"`
			Meal   string `json:"meal"`
		} `json:"items"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(in.Items) == 0 {
		jsonErr(w, http.StatusBadRequest, "请至少选择一道菜")
		return
	}
	if len(in.Items) > 80 {
		jsonErr(w, http.StatusBadRequest, "一次最多批量点 80 道菜")
		return
	}
	userID := trimmed(in.UserID, 64)
	dishIDs := make([]string, 0, len(in.Items))
	seenDish := make(map[string]bool, len(in.Items))
	for _, item := range in.Items {
		id := trimmed(item.DishID, 64)
		if id != "" && !seenDish[id] {
			seenDish[id] = true
			dishIDs = append(dishIDs, id)
		}
	}
	lookup := s.st.OrderBatchLookup(dishIDs, userID)
	if !lookup.MemberFound {
		jsonErr(w, http.StatusBadRequest, "请先选择点菜的人")
		return
	}
	allowed := make(map[string]bool, len(lookup.Meals))
	for _, meal := range lookup.Meals {
		allowed[meal.Key] = true
	}
	orders := make([]store.Order, 0, len(in.Items))
	skipped := 0
	seenOrder := make(map[string]bool, len(in.Items))
	for _, item := range in.Items {
		dishID := trimmed(item.DishID, 64)
		dish, found := lookup.Dishes[dishID]
		if !found || seenOrder[dishID] {
			skipped++
			continue
		}
		meal := trimmed(item.Meal, 32)
		if meal == "" {
			meal = store.CurrentMeal(time.Now(), lookup.Meals)
		}
		if !allowed[meal] {
			jsonErr(w, http.StatusBadRequest, "餐别不在已配置的用餐时段里，请刷新页面重试")
			return
		}
		orders = append(orders, store.Order{
			DishID:      dish.ID,
			DishName:    dish.Name,
			DishCuisine: dish.Cuisine,
			DishImage:   dish.Image,
			UserID:      lookup.Member.ID,
			UserName:    lookup.Member.Name,
			UserAvatar:  lookup.Member.Avatar,
			Note:        trimmed(item.Note, 100),
			Meal:        meal,
		})
		seenOrder[dishID] = true
	}
	if len(orders) == 0 {
		jsonOK(w, map[string]any{"plan": s.st.GetPlan(date), "added": 0, "skipped": skipped})
		return
	}
	plan, added, err := s.st.AddOrders(date, orders)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "批量点菜失败")
		return
	}
	jsonOK(w, map[string]any{
		"plan": plan, "added": added, "skipped": skipped + len(orders) - added,
	})
}

// setOrdersDone 在同一事务内批量设置上桌状态，替代「全部上桌」逐道请求。
func (s *Server) setOrdersDone(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	var in struct {
		OrderIDs []string `json:"orderIds"`
		Done     *bool    `json:"done"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(in.OrderIDs) == 0 || len(in.OrderIDs) > 200 {
		jsonErr(w, http.StatusBadRequest, "请选择 1–200 道要更新的菜")
		return
	}
	ids := make([]string, 0, len(in.OrderIDs))
	seen := make(map[string]bool, len(in.OrderIDs))
	for _, raw := range in.OrderIDs {
		id := trimmed(raw, 80)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	done := true
	if in.Done != nil {
		done = *in.Done
	}
	plan, updated, err := s.st.SetOrdersDone(date, ids, done)
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, map[string]any{"plan": plan, "updated": updated})
}

func (s *Server) toggleOrder(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	plan, err := s.st.ToggleOrder(date, r.PathValue("orderId"))
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, plan)
}

func (s *Server) removeOrder(w http.ResponseWriter, r *http.Request) {
	date, ok := checkedDate(w, r)
	if !ok {
		return
	}
	plan, err := s.st.RemoveOrder(date, r.PathValue("orderId"))
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, plan)
}

func checkedDate(w http.ResponseWriter, r *http.Request) (string, bool) {
	date := r.PathValue("date")
	if !validDate(date) {
		jsonErr(w, http.StatusBadRequest, "日期格式应为 YYYY-MM-DD")
		return "", false
	}
	return date, true
}

func memberExists(list []store.Member, id string) bool {
	for _, m := range list {
		if m.ID == id {
			return true
		}
	}
	return false
}
