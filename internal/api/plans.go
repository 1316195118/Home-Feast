package api

import (
	"errors"
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
	members := s.st.ListMembers()
	patch := store.PlanPatch{}

	// 按餐分工：餐次必须存在于已配置的用餐时段里
	if in.DutyMeal != "" {
		ok := false
		for _, m := range s.st.Meals() {
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
	meals := s.st.Meals()
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
	dish, err := findDish(s.st.ListDishes(), in.DishID)
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	member, err := findMember(s.st.ListMembers(), in.UserID)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	order := store.Order{
		DishID:      dish.ID,
		DishName:    dish.Name,
		DishCuisine: dish.Cuisine,
		DishImage:   dish.Image,
		UserID:      member.ID,
		UserName:    member.Name,
		UserAvatar:  member.Avatar,
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

func findDish(list []store.Dish, id string) (store.Dish, error) {
	for _, d := range list {
		if d.ID == id {
			return d, nil
		}
	}
	return store.Dish{}, errors.New("菜品不存在，可能已被删除")
}

func findMember(list []store.Member, id string) (store.Member, error) {
	for _, m := range list {
		if m.ID == id {
			return m, nil
		}
	}
	return store.Member{}, errors.New("请先选择点菜的人")
}

func memberExists(list []store.Member, id string) bool {
	for _, m := range list {
		if m.ID == id {
			return true
		}
	}
	return false
}
