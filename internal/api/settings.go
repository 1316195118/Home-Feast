package api

import (
	"net/http"
	"strings"

	"familymenu/internal/store"
)

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FamilyName     *string           `json:"familyName"`
		HiddenCuisines *[]string         `json:"hiddenCuisines"`
		CuisineOrder   *[]string         `json:"cuisineOrder"` // 菜系展示顺序
		Meals          *[]store.MealSlot `json:"meals"`
		Roster         *store.Roster     `json:"roster"`  // 每周掌勺 / 洗碗排班
		Pantry         *[]string         `json:"pantry"`  // 冰箱现有食材
		AutoBackup     *bool             `json:"autoBackup"`
		BackupKeep     *int              `json:"backupKeep"`
		AccessCode     *string           `json:"accessCode"` // 空串 = 关闭口令
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	v := s.st.GetSettings()
	if in.FamilyName != nil {
		name := trimmed(*in.FamilyName, 24)
		if name == "" {
			jsonErr(w, http.StatusBadRequest, "家的名字不能为空")
			return
		}
		v.FamilyName = name
	}
	if in.HiddenCuisines != nil {
		// 去重并只保留确实存在的菜系 key，避免前端手滑写进脏数据。
		known := map[string]bool{}
		for _, c := range store.Cuisines {
			known[c.Key] = true
		}
		seen := map[string]bool{}
		clean := make([]string, 0, len(*in.HiddenCuisines))
		for _, k := range *in.HiddenCuisines {
			if known[k] && !seen[k] {
				seen[k] = true
				clean = append(clean, k)
			}
		}
		v.HiddenCuisines = clean
	}
	if in.CuisineOrder != nil {
		// 只接受已知 key、去重，并把没提到的分类补到末尾，
		// 这样顺序表永远覆盖全部分类，前端不必再兜底。
		known := map[string]bool{}
		for _, c := range store.Cuisines {
			known[c.Key] = true
		}
		seen := map[string]bool{}
		order := make([]string, 0, len(store.Cuisines))
		for _, k := range *in.CuisineOrder {
			k = trimmed(k, 32)
			if !known[k] || seen[k] {
				continue
			}
			seen[k] = true
			order = append(order, k)
		}
		for _, c := range store.Cuisines {
			if !seen[c.Key] {
				order = append(order, c.Key)
			}
		}
		v.CuisineOrder = order
	}
	if in.Meals != nil {
		// 用户自定义餐别：名字、图标、结束时刻都在这里校验，脏数据不落盘。
		normalized, err := store.NormalizeMeals(*in.Meals)
		if err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		v.Meals = normalized
	}
	if in.Roster != nil {
		// 排班里只保留合法星期与真实存在的成员，删掉不存在的 ID
		v.Roster = store.NormalizeRoster(*in.Roster, s.st.ListMembers())
	}
	if in.AutoBackup != nil {
		v.AutoBackup = *in.AutoBackup
	}
	if in.BackupKeep != nil {
		k := *in.BackupKeep
		if k < 1 {
			k = 1
		}
		if k > 60 {
			k = 60
		}
		v.BackupKeep = k
	}
	if in.Pantry != nil {
		// 冰箱食材：走统一的 trimmed，顺带剥离控制字符（只做 TrimSpace 会漏 NUL）
		seen := map[string]bool{}
		clean := make([]string, 0, len(*in.Pantry))
		for _, raw := range *in.Pantry {
			item := trimmed(raw, 12)
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			clean = append(clean, item)
			if len(clean) >= 40 {
				break
			}
		}
		v.Pantry = clean
	}
	// 访问口令：先校验（失败不产生任何改动），设置保存成功后再写独立文件。
	var newCode *string
	if in.AccessCode != nil {
		if s.envCode != "" {
			jsonErr(w, http.StatusBadRequest, "访问口令由服务端环境变量 MENU_ACCESS_CODE 控制，请在 docker-compose.yml 里修改")
			return
		}
		// 口令允许符号与内部空格，但控制字符必须去掉：它无法通过 HTTP 头传输，
		// 存进去等于设了一个谁都输不对的口令。
		c := stripControl(strings.TrimSpace(*in.AccessCode))
		if len([]rune(c)) > 64 {
			jsonErr(w, http.StatusBadRequest, "访问口令最多 64 个字符")
			return
		}
		newCode = &c
	}
	saved, err := s.st.SaveSettings(v)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存设置失败")
		return
	}
	if newCode != nil {
		if err := s.st.SetAccessCode(*newCode); err != nil {
			jsonErr(w, http.StatusInternalServerError, "保存访问口令失败")
			return
		}
	}
	jsonOK(w, saved)
}
