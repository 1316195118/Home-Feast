package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"familymenu/internal/store"
)

type dishInput struct {
	Name        string   `json:"name"`
	Cuisine     string   `json:"cuisine"`
	Desc        string   `json:"desc"`
	Tags        []string `json:"tags"`
	Image       string   `json:"image"`
	Spicy       int      `json:"spicy"`
	Minutes     int      `json:"minutes"`
	Servings    int      `json:"servings"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	Rating      int      `json:"rating"`
}

func (s *Server) listDishes(w http.ResponseWriter, r *http.Request) {
	jsonRevalidate(w, r, s.st.ListDishes())
}

func (s *Server) createDish(w http.ResponseWriter, r *http.Request) {
	var in dishInput
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	d, err := normalizeDish(in)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.st.AddDish(d)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存菜品失败")
		return
	}
	jsonStatus(w, http.StatusCreated, created)
}

func (s *Server) updateDish(w http.ResponseWriter, r *http.Request) {
	var p store.DishPatch
	if err := decodeBody(w, r, &p); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.Name != nil {
		v := trimmed(*p.Name, 40)
		if v == "" {
			jsonErr(w, http.StatusBadRequest, "菜名不能为空")
			return
		}
		p.Name = &v
	}
	if p.Cuisine != nil && !knownCuisine(*p.Cuisine) {
		jsonErr(w, http.StatusBadRequest, "未知的菜系")
		return
	}
	if p.Desc != nil {
		v := trimmed(*p.Desc, 200)
		p.Desc = &v
	}
	if p.Image != nil {
		v := safeImage(*p.Image)
		p.Image = &v
	}
	if p.Tags != nil {
		v := cleanTags(*p.Tags)
		p.Tags = &v
	}
	if p.Ingredients != nil {
		v := cleanList(*p.Ingredients, 60, 20)
		if err := validateIngredientAmounts(v); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		p.Ingredients = &v
	}
	if p.Steps != nil {
		v := cleanList(*p.Steps, 200, 20)
		p.Steps = &v
	}
	if p.Spicy != nil {
		v := clamp(*p.Spicy, 0, 3)
		p.Spicy = &v
	}
	if p.Minutes != nil {
		v := clamp(*p.Minutes, 0, 600)
		p.Minutes = &v
	}
	if p.Servings != nil {
		v := clamp(*p.Servings, 1, 20)
		if *p.Servings == 0 {
			v = 2
		}
		p.Servings = &v
	}
	updated, err := s.st.UpdateDish(r.PathValue("id"), p)
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, updated)
}

func (s *Server) deleteDish(w http.ResponseWriter, r *http.Request) {
	if err := s.st.DeleteDish(r.PathValue("id")); err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func normalizeDish(in dishInput) (store.Dish, error) {
	name := trimmed(in.Name, 40)
	if name == "" {
		return store.Dish{}, errors.New("菜名不能为空")
	}
	cuisine := trimmed(in.Cuisine, 32)
	if !knownCuisine(cuisine) {
		return store.Dish{}, errors.New("请选择菜系或地区标签")
	}
	ingredients := cleanList(in.Ingredients, 60, 20)
	if err := validateIngredientAmounts(ingredients); err != nil {
		return store.Dish{}, err
	}
	servings := clamp(in.Servings, 1, 20)
	if in.Servings == 0 {
		// 旧版编辑器不提交份数；新菜默认按两人份处理。
		servings = 2
	}
	return store.Dish{
		Name:        name,
		Cuisine:     cuisine,
		Desc:        trimmed(in.Desc, 200),
		Tags:        cleanTags(in.Tags),
		Image:       safeImage(in.Image),
		Spicy:       clamp(in.Spicy, 0, 3),
		Minutes:     clamp(in.Minutes, 0, 600),
		Servings:    servings,
		Ingredients: ingredients,
		Steps:       cleanList(in.Steps, 200, 20),
		Rating:      clamp(in.Rating, 0, 3),
	}, nil
}

func validateIngredientAmounts(items []string) error {
	for _, item := range items {
		line := strings.TrimSpace(item)
		if line == "" {
			continue
		}
		for _, vague := range []string{"适量", "少许", "少量", "按口味", "随意"} {
			if strings.Contains(line, vague) {
				return fmt.Errorf("配料「%s」的用量要写具体，不能用「%s」", line, vague)
			}
		}
		if !strings.ContainsAny(line, "0123456789一二三四五六七八九十半两") {
			return fmt.Errorf("配料「%s」缺少具体用量，例如 200g、2 个或 30ml", line)
		}
	}
	return nil
}

func knownCuisine(key string) bool {
	for _, c := range store.Cuisines {
		if c.Key == key {
			return true
		}
	}
	return false
}

// safeImage 只接受本项目上传目录下的图片，避免外链请求泄露家庭访问信息。
func safeImage(v string) string {
	v = trimmed(v, 200)
	if strings.HasPrefix(v, "/uploads/") && !strings.Contains(v, "..") && !strings.Contains(v, "\\") {
		return v
	}
	return ""
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, t := range tags {
		v := trimmed(t, 12)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) == 6 {
			break
		}
	}
	return out
}

// cleanList 清洗「配料 / 做法步骤」这类自由输入：逐条去空白、跳过空行、限长限条数。
// 始终返回非 nil 切片，保证写进 JSON 是 [] 而不是 null。
func cleanList(items []string, maxLen, maxCount int) []string {
	out := make([]string, 0, len(items))
	for _, v := range items {
		v = trimmed(v, maxLen)
		if v == "" {
			continue
		}
		out = append(out, v)
		if len(out) == maxCount {
			break
		}
	}
	return out
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
