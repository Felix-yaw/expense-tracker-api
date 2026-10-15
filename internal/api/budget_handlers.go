package api

import (
	"net/http"

	"github.com/Felix-yaw/expense-tracker-api/internal/httpx"
	"github.com/Felix-yaw/expense-tracker-api/internal/models"
)

type budgetRequest struct {
	MonthlyLimit   float64            `json:"monthlyLimit"`
	CategoryLimits map[string]float64 `json:"categoryLimits"`
}

func (s *Server) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	budget, err := s.store.GetBudget(r.Context(), userID(r))
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, budget)
}

func (s *Server) handlePutBudget(w http.ResponseWriter, r *http.Request) {
	var req budgetRequest
	if !httpx.ReadJSON(w, r, &req) {
		return
	}
	if req.MonthlyLimit < 0 {
		httpx.WriteError(w, http.StatusBadRequest, "monthlyLimit cannot be negative")
		return
	}
	for category := range req.CategoryLimits {
		if !models.ValidCategory(category) {
			httpx.WriteError(w, http.StatusBadRequest, "unknown category: "+category)
			return
		}
	}

	budget := models.Budget{
		MonthlyLimit:   req.MonthlyLimit,
		CategoryLimits: req.CategoryLimits,
	}
	if err := s.store.SaveBudget(r.Context(), userID(r), budget); err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, budget)
}
