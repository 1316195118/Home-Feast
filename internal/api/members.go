package api

import (
	"errors"
	"net/http"
	"regexp"

	"familymenu/internal/store"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var palette = []string{"#d9a066", "#cf8b9b", "#7fa98d", "#8d9bc4", "#c59a6d", "#a98bb5"}

type memberInput struct {
	Name   string   `json:"name"`
	Avatar string   `json:"avatar"`
	Color  string   `json:"color"`
	Avoid  []string `json:"avoid"`
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, s.st.ListMembers())
}

func (s *Server) createMember(w http.ResponseWriter, r *http.Request) {
	var in memberInput
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	m, err := normalizeMember(in, len(s.st.ListMembers()))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.st.AddMember(m)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存成员失败")
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, created)
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name   *string   `json:"name"`
		Avatar *string   `json:"avatar"`
		Color  *string   `json:"color"`
		Avoid  *[]string `json:"avoid"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	p := store.MemberPatch{}
	if in.Name != nil {
		v := trimmed(*in.Name, 20)
		if v == "" {
			jsonErr(w, http.StatusBadRequest, "名字不能为空")
			return
		}
		p.Name = &v
	}
	if in.Avatar != nil {
		v := cleanAvatar(*in.Avatar)
		p.Avatar = &v
	}
	if in.Color != nil {
		v := cleanColor(*in.Color, 0)
		p.Color = &v
	}
	if in.Avoid != nil {
		v := cleanAvoid(*in.Avoid)
		p.Avoid = &v
	}
	updated, err := s.st.UpdateMember(r.PathValue("id"), p)
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, updated)
}

// setMemberPin 设置 / 修改 / 清除成员密码。已设过密码时必须带对 oldPin。
func (s *Server) setMemberPin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Pin    string `json:"pin"`
		OldPin string `json:"oldPin"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.SetMemberPin(r.PathValue("id"), in.Pin, in.OldPin); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "hasPin": in.Pin != ""})
}

// verifyMemberPin 校验成员密码。
// 注意：失败也返回 200 + {ok:false}，不能返回 401 —— 前端 request() 把 401
// 当成「访问口令错误」会触发 fm:locked 把整页锁掉，那是另一套全局口令机制。
func (s *Server) verifyMemberPin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Pin string `json:"pin"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// PIN 只有 4~16 位，且很多家庭用 4 位数字：不设闸的话实测 56 次/秒、3 分钟能穷举完。
	key := "pin:" + clientIP(r)
	s.limit.wait(key)
	ok := s.st.VerifyMemberPin(r.PathValue("id"), in.Pin)
	if ok {
		s.limit.Reset(key)
	} else {
		s.limit.Fail(key)
	}
	jsonOK(w, map[string]bool{"ok": ok})
}

func (s *Server) deleteMember(w http.ResponseWriter, r *http.Request) {
	if len(s.st.ListMembers()) <= 1 {
		jsonErr(w, http.StatusBadRequest, "至少保留一位家庭成员")
		return
	}
	if err := s.st.DeleteMember(r.PathValue("id")); err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func normalizeMember(in memberInput, index int) (store.Member, error) {
	name := trimmed(in.Name, 20)
	if name == "" {
		return store.Member{}, errors.New("名字不能为空")
	}
	return store.Member{
		Name:   name,
		Avatar: cleanAvatar(in.Avatar),
		Color:  cleanColor(in.Color, index),
		Avoid:  cleanAvoid(in.Avoid),
	}, nil
}

// 头像只放行：文字、数字、下划线、短横线、空格、emoji（So/Sk），
// 以及组合 emoji 必需的 ZWJ 与变体选择符。引号/尖括号/分号一律回退默认表情，
// 这样即使以后前端改成把头像拼进属性或 HTML，也没有注入面。
var avatarPattern = regexp.MustCompile(`^[\p{L}\p{N}_\-\s\p{So}\p{Sk}\x{200d}\x{fe0f}]{1,8}$`)

func cleanAvatar(v string) string {
	v = trimmed(v, 8)
	if v == "" || !avatarPattern.MatchString(v) {
		return "🙂"
	}
	return v
}

func cleanColor(v string, index int) string {
	v = trimmed(v, 16)
	if colorPattern.MatchString(v) {
		return v
	}
	return palette[index%len(palette)]
}

// cleanAvoid 清洗忌口标签：去空白、去重、限长限数量，避免手改请求塞进一堆垃圾。
func cleanAvoid(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, v := range list {
		v = trimmed(v, 16)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) >= 16 {
			break
		}
	}
	return out
}
