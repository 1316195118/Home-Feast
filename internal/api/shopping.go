package api

import (
	"net/http"
	"time"

	"familymenu/internal/store"
)

func (s *Server) updateShoppingMarks(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date   string             `json:"date"`
		UserID string             `json:"userId"`
		Items  []store.ShoppingOp `json:"items"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}
	saved, err := s.st.UpdateShopping(in.Date, in.UserID, in.Items)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, saved)
}
