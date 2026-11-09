package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"familymenu/internal/store"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var palette = []string{"#d9a066", "#cf8b9b", "#7fa98d", "#8d9bc4", "#c59a6d", "#a98bb5"}

type memberInput struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Color  string `json:"color"`
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
		Name   *string `json:"name"`
		Avatar *string `json:"avatar"`
		Color  *string `json:"color"`
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
	updated, err := s.st.UpdateMember(r.PathValue("id"), p)
	if err != nil {
		jsonErr(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, updated)
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
	}, nil
}

func cleanAvatar(v string) string {
	v = trimmed(v, 8)
	if v == "" || strings.ContainsAny(v, "<>{}") {
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
