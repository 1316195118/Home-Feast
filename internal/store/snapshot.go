package store

import "sort"

// BootstrapData 是一次读锁内取出的首屏数据。
//
// 接口层不要再逐项调用 GetSettings / ListDishes / ListMembers / ListPlans：
// 多次读取既可能拼出前后不一致的快照，也会让一次页面刷新重复排队拿锁。
type BootstrapData struct {
	Settings  Settings
	Meals     []MealSlot
	Dishes    []Dish
	Members   []Member
	Plans     []Plan
	Plan      Plan
	PlanCount int
}

// PlanningData 是编辑分工、点菜前校验需要的只读快照。
type PlanningData struct {
	Meals   []MealSlot
	Dishes  []Dish
	Members []Member
}

// DutyData 只包含分工编辑需要的餐次与成员，避免一次分工保存复制整个菜谱。
type DutyData struct {
	Meals   []MealSlot
	Members []Member
}

// SettingsData 让设置页只取一次当前设置与成员，不把两次读取拼成可能错位的状态。
type SettingsData struct {
	Settings Settings
	Members  []Member
}

// MediaData 供备份统计图片引用，菜品与历史计划必须来自同一个时点。
type MediaData struct {
	Dishes []Dish
	Plans  []Plan
}

// Bootstrap 只做一次加载过期餐次（如有必要），然后在一个 RLock 内生成完整快照。
// 返回的所有切片、映射和指针都是独立副本，HTTP 层可以在锁外安全地序列化。
func (s *Store) Bootstrap(date string) BootstrapData {
	s.expireToday(date)

	s.mu.RLock()
	defer s.mu.RUnlock()

	settings := cloneSettings(s.settings)
	out := BootstrapData{
		Settings:  settings,
		Meals:     effectiveMeals(settings.Meals),
		Dishes:    cloneDishes(s.dishes),
		Members:   s.cloneMembersLocked(),
		PlanCount: len(s.plans),
	}
	out.Plans = make([]Plan, 0, len(s.plans))
	for _, p := range s.plans {
		p = clonePlan(p)
		p.NormalizeDuty()
		out.Plans = append(out.Plans, p)
	}
	sort.Slice(out.Plans, func(i, j int) bool { return out.Plans[i].Date > out.Plans[j].Date })
	if p, ok := s.plans[date]; ok {
		out.Plan = clonePlan(p)
	} else {
		out.Plan = Plan{Date: date, Orders: []Order{}}
	}
	out.Plan.NormalizeDuty()
	if out.Plan.Orders == nil {
		out.Plan.Orders = []Order{}
	}
	return out
}

// Planning 为分工和点菜接口提供一个一致的家庭成员 / 菜单 / 餐次快照。
func (s *Store) Planning() PlanningData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return PlanningData{
		Meals:   effectiveMeals(s.settings.Meals),
		Dishes:  cloneDishes(s.dishes),
		Members: s.cloneMembersLocked(),
	}
}

func (s *Store) DutySnapshot() DutyData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return DutyData{
		Meals:   effectiveMeals(s.settings.Meals),
		Members: s.cloneMembersLocked(),
	}
}

func (s *Store) SettingsSnapshot() SettingsData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SettingsData{
		Settings: cloneSettings(s.settings),
		Members:  s.cloneMembersLocked(),
	}
}

func (s *Store) MediaSnapshot() MediaData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := MediaData{
		Dishes: cloneDishes(s.dishes),
		Plans:  make([]Plan, 0, len(s.plans)),
	}
	for _, p := range s.plans {
		out.Plans = append(out.Plans, clonePlan(p))
	}
	return out
}

// cloneMembersLocked 必须在持有读锁时调用；HasPin 是 API 计算字段，不写回存储。
func (s *Store) cloneMembersLocked() []Member {
	out := make([]Member, len(s.members))
	for i, m := range s.members {
		out[i] = cloneMember(m)
		_, out[i].HasPin = s.pins[m.ID]
	}
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string(nil), in...)
}

func cloneDish(d Dish) Dish {
	d.Tags = cloneStrings(d.Tags)
	d.Ingredients = cloneStrings(d.Ingredients)
	d.Steps = cloneStrings(d.Steps)
	return d
}

func cloneDishes(in []Dish) []Dish {
	out := make([]Dish, len(in))
	for i, d := range in {
		out[i] = cloneDish(d)
	}
	return out
}

func cloneMember(m Member) Member {
	m.Avoid = cloneStrings(m.Avoid)
	return m
}

func clonePlan(p Plan) Plan {
	p.ChefIDs = cloneStrings(p.ChefIDs)
	p.DishwasherIDs = cloneStrings(p.DishwasherIDs)
	p.Orders = append([]Order(nil), p.Orders...)
	p.PKShown = cloneStrings(p.PKShown)
	if p.MealDuties != nil {
		meals := make(map[string]MealDuty, len(p.MealDuties))
		for key, duty := range p.MealDuties {
			meals[key] = MealDuty{
				ChefIDs:       cloneStrings(duty.ChefIDs),
				DishwasherIDs: cloneStrings(duty.DishwasherIDs),
			}
		}
		p.MealDuties = meals
	}
	if p.Dice != nil {
		dice := make(map[string]int, len(p.Dice))
		for id, value := range p.Dice {
			dice[id] = value
		}
		p.Dice = dice
	}
	return p
}

func cloneSettings(in Settings) Settings {
	out := in
	out.HiddenCuisines = cloneStrings(in.HiddenCuisines)
	out.CuisineOrder = cloneStrings(in.CuisineOrder)
	out.Meals = append([]MealSlot(nil), in.Meals...)
	out.Pantry = cloneStrings(in.Pantry)
	if in.PantryAdded != nil {
		out.PantryAdded = make(map[string]string, len(in.PantryAdded))
		for key, value := range in.PantryAdded {
			out.PantryAdded[key] = value
		}
	}
	if in.Roster.Days != nil {
		out.Roster.Days = make(map[string]RosterDay, len(in.Roster.Days))
		for key, value := range in.Roster.Days {
			out.Roster.Days[key] = value
		}
	}
	if in.AutoCheckUpdate != nil {
		value := *in.AutoCheckUpdate
		out.AutoCheckUpdate = &value
	}
	return out
}
