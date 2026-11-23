package store

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// ShoppingOp 是一次原子采购状态变更。Status 为空表示清除。
type ShoppingOp struct {
	Key    string `json:"key"`
	Status string `json:"status"`
}

// UpdateShopping 在单个写锁内合并一批采购状态，避免多人同时勾选时互相覆盖。
// 状态只保留最近 60 天，避免长期使用后 settings.json 无限增长。
func (s *Store) UpdateShopping(date, userID string, ops []ShoppingOp) (Settings, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Settings{}, errors.New("采购日期格式不正确")
	}
	if len(ops) == 0 || len(ops) > 200 {
		return Settings{}, errors.New("采购变更数量不正确")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userName := ""
	for _, member := range s.members {
		if member.ID == userID {
			userName = member.Name
			break
		}
	}
	if userName == "" {
		return Settings{}, errors.New("家庭成员不存在")
	}

	next := cloneSettings(s.settings)
	if next.ShoppingMarks == nil {
		next.ShoppingMarks = make(map[string]ShoppingMark)
	}
	for _, op := range ops {
		key := strings.TrimSpace(op.Key)
		// 前端会给每日/整周清单加作用域前缀；普通食材名仍只有几个字，
		// 80 个字符足够容纳日期、周期标记和最长食材名。
		if key == "" || len([]rune(key)) > 80 {
			continue
		}
		status := strings.TrimSpace(op.Status)
		if status == "" {
			delete(next.ShoppingMarks, key)
			continue
		}
		if status != "claimed" && status != "bought" {
			continue
		}
		next.ShoppingMarks[key] = ShoppingMark{
			Date: date, Status: status, UserID: userID, UserName: userName, UpdatedAt: now(),
		}
	}

	// 只删旧记录；若手工改过未来日期也保留，不用当前时钟误伤。
	cutoff := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
	for key, mark := range next.ShoppingMarks {
		if mark.Date != "" && mark.Date < cutoff {
			delete(next.ShoppingMarks, key)
		}
	}

	// 极端情况下保留最近 600 项，按更新时间截断，防止配置无限膨胀。
	if len(next.ShoppingMarks) > 600 {
		type entry struct {
			key  string
			mark ShoppingMark
		}
		list := make([]entry, 0, len(next.ShoppingMarks))
		for key, mark := range next.ShoppingMarks {
			list = append(list, entry{key: key, mark: mark})
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].mark.UpdatedAt > list[j].mark.UpdatedAt
		})
		clean := make(map[string]ShoppingMark, 600)
		for _, item := range list[:600] {
			clean[item.key] = item.mark
		}
		next.ShoppingMarks = clean
	}

	next.UpdatedAt = now()
	if err := s.writeJSON(settingsFile, next); err != nil {
		return Settings{}, err
	}
	s.settings = next
	s.markChangedLocked()
	return cloneSettings(next), nil
}
