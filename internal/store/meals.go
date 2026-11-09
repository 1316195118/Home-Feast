package store

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultMeals 是旧数据（settings.json 没有 meals 字段）和用户点「恢复默认」时使用的三餐。
// 与前端 utils.js 里的 DEFAULT_MEALS 保持一致；运行时一律以后端下发的配置为准。
func DefaultMeals() []MealSlot {
	return []MealSlot{
		{Key: "breakfast", Label: "早餐", Icon: "🌅", End: "10:00"},
		{Key: "lunch", Label: "午餐", Icon: "☀️", End: "15:00"},
		{Key: "dinner", Label: "晚餐", Icon: "🌙", End: "23:00"},
	}
}

// MealEndMinutes 解析 "HH:MM"，返回从 0 点起的分钟数（0..1440），非法返回 -1。
func MealEndMinutes(end string) int {
	p := strings.SplitN(strings.TrimSpace(end), ":", 2)
	if len(p) != 2 || p[0] == "" || p[1] == "" {
		return -1
	}
	h, err := strconv.Atoi(p[0])
	if err != nil {
		return -1
	}
	m, err := strconv.Atoi(p[1])
	if err != nil {
		return -1
	}
	if h < 0 || h > 24 || m < 0 || m > 59 || (h == 24 && m != 0) {
		return -1
	}
	return h*60 + m
}

// effectiveMeals 得到真正可用的配置：为空或全部非法时回退默认三餐，
// 并按结束时刻升序排列（当前餐推导依赖这个顺序）。
func effectiveMeals(in []MealSlot) []MealSlot {
	out := make([]MealSlot, 0, len(in))
	for _, m := range in {
		if strings.TrimSpace(m.Key) == "" || MealEndMinutes(m.End) < 0 {
			continue
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		out = append(out, DefaultMeals()...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return MealEndMinutes(out[i].End) < MealEndMinutes(out[j].End)
	})
	return out
}

// Meals 返回当前生效的餐别配置（带读锁，供 API 层使用）。
func (s *Store) Meals() []MealSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return effectiveMeals(s.settings.Meals)
}

// NormalizeMeals 校验并规范化用户在设置页提交的配置，防止脏数据写进 settings.json。
func NormalizeMeals(in []MealSlot) ([]MealSlot, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("至少保留一餐")
	}
	if len(in) > 8 {
		return nil, fmt.Errorf("最多设置 8 餐")
	}
	out := make([]MealSlot, 0, len(in))
	seen := map[string]bool{}
	for i, m := range in {
		key := strings.TrimSpace(m.Key)
		if !validMealKey(key) {
			return nil, fmt.Errorf("第 %d 餐的标识只能用小写字母、数字、下划线或短横线", i+1)
		}
		if seen[key] {
			return nil, fmt.Errorf("餐别标识重复：%s", key)
		}
		seen[key] = true
		label := strings.TrimSpace(m.Label)
		if label == "" || len([]rune(label)) > 12 {
			return nil, fmt.Errorf("第 %d 餐的名字需为 1-12 个字", i+1)
		}
		icon := strings.TrimSpace(m.Icon)
		if icon == "" {
			icon = "🍽️"
		}
		if len([]rune(icon)) > 8 {
			return nil, fmt.Errorf("第 %d 餐的图标过长", i+1)
		}
		end := strings.TrimSpace(m.End)
		if MealEndMinutes(end) < 0 {
			return nil, fmt.Errorf("第 %d 餐的时间格式应为 HH:MM", i+1)
		}
		out = append(out, MealSlot{Key: key, Label: label, Icon: icon, End: end})
	}
	return effectiveMeals(out), nil
}

func validMealKey(k string) bool {
	if k == "" || len(k) > 24 {
		return false
	}
	for _, r := range k {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// CurrentMeal 按当前时刻返回应归入的餐别 key：取第一个「结束时刻还没到」的餐；
// 若所有餐都已过（深夜），归到最后一餐——此时该餐虽已过饭点，但新点的菜会被保留。
func CurrentMeal(now time.Time, meals []MealSlot) string {
	list := effectiveMeals(meals)
	cur := now.Hour()*60 + now.Minute()
	for _, m := range list {
		if cur < MealEndMinutes(m.End) {
			return m.Key
		}
	}
	if len(list) > 0 {
		return list[len(list)-1].Key
	}
	return ""
}

// orderedAfterMealEnd 判断订单是否是在「该餐结束之后」才点的。
// 过了饭点才加的菜（深夜加菜、手动补记）不参与自动收起，否则会「点完立刻消失」。
// 创建时间缺失或跨天的旧数据按原规则处理（返回 false）。
func orderedAfterMealEnd(o Order, date string, endMinutes int, loc *time.Location) bool {
	t, err := time.Parse(time.RFC3339, o.CreatedAt)
	if err != nil {
		return false
	}
	t = t.In(loc)
	if t.Format("2006-01-02") != date {
		return false
	}
	return t.Hour()*60+t.Minute() >= endMinutes
}

// dropExpired 移除今天已过饭点餐别的订单，返回是否有变化（调用者需持写锁）。
func dropExpired(p *Plan, now time.Time, meals []MealSlot) bool {
	if p.Date != now.Format("2006-01-02") || len(p.Orders) == 0 {
		return false
	}
	endByKey := map[string]int{}
	for _, m := range effectiveMeals(meals) {
		endByKey[m.Key] = MealEndMinutes(m.End)
	}
	cur := now.Hour()*60 + now.Minute()
	kept := make([]Order, 0, len(p.Orders))
	changed := false
	for _, o := range p.Orders {
		if end, ok := endByKey[o.Meal]; ok && cur >= end &&
			!orderedAfterMealEnd(o, p.Date, end, now.Location()) {
			changed = true
			continue
		}
		kept = append(kept, o)
	}
	if changed {
		p.Orders = kept
		p.UpdatedAt = now.Format(time.RFC3339)
	}
	return changed
}
