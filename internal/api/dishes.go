package api

import (
	"errors"
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
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
}

func (s *Server) listDishes(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, s.st.ListDishes())
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
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, created)
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
		cuisine = "homestyle"
	}
	return store.Dish{
		Name:        name,
		Cuisine:     cuisine,
		Desc:        trimmed(in.Desc, 200),
		Tags:        cleanTags(in.Tags),
		Image:       safeImage(in.Image),
		Spicy:       clamp(in.Spicy, 0, 3),
		Minutes:     clamp(in.Minutes, 0, 600),
		Ingredients: cleanList(in.Ingredients, 60, 20),
		Steps:       cleanList(in.Steps, 200, 20),
	}, nil
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
	for _, t := range tags {
		v := trimmed(t, 12)
		if v == "" {
			continue
		}
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
