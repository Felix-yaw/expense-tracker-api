package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Felix-yaw/expense-tracker-api/internal/httpx"
	"github.com/Felix-yaw/expense-tracker-api/internal/models"
	"github.com/Felix-yaw/expense-tracker-api/internal/store"
)

type createExpenseRequest struct {
	Title    string  `json:"title"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Date     string  `json:"date"`
	Note     string  `json:"note"`
}

type updateExpenseRequest struct {
	Title    *string  `json:"title"`
	Amount   *float64 `json:"amount"`
	Category *string  `json:"category"`
	Date     *string  `json:"date"`
	Note     *string  `json:"note"`
}

type expenseListResponse struct {
	Items  []models.Expense `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func (s *Server) handleListExpenses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := store.ExpenseFilter{
		Category: strings.TrimSpace(q.Get("category")),
		Query:    strings.TrimSpace(q.Get("q")),
		Limit:    atoiDefault(q.Get("limit"), 100),
		Offset:   atoiDefault(q.Get("offset"), 0),
	}
	if filter.Category != "" && !models.ValidCategory(filter.Category) {
		httpx.WriteError(w, http.StatusBadRequest, "unknown category")
		return
	}
	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.From, filter.To = from, to

	items, total, err := s.store.ListExpenses(r.Context(), userID(r), filter)
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, expenseListResponse{
		Items:  items,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func (s *Server) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	var req createExpenseRequest
	if !httpx.ReadJSON(w, r, &req) {
		return
	}
	expense, err := buildExpense(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.store.CreateExpense(r.Context(), userID(r), expense)
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetExpense(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	expense, err := s.store.GetExpense(r.Context(), userID(r), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "expense not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, expense)
}

func (s *Server) handleUpdateExpense(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	current, err := s.store.GetExpense(r.Context(), userID(r), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "expense not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	var req updateExpenseRequest
	if !httpx.ReadJSON(w, r, &req) {
		return
	}
	if req.Title != nil {
		current.Title = strings.TrimSpace(*req.Title)
	}
	if req.Amount != nil {
		current.Amount = *req.Amount
	}
	if req.Category != nil {
		current.Category = *req.Category
	}
	if req.Note != nil {
		current.Note = strings.TrimSpace(*req.Note)
	}
	if req.Date != nil {
		date, err := parseDate(*req.Date)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		current.Date = date
	}

	if current.Title == "" {
		httpx.WriteError(w, http.StatusBadRequest, "title is required")
		return
	}
	if current.Amount <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}
	if !models.ValidCategory(current.Category) {
		httpx.WriteError(w, http.StatusBadRequest, "unknown category")
		return
	}

	updated, err := s.store.UpdateExpense(r.Context(), userID(r), current)
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.store.DeleteExpense(r.Context(), userID(r), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "expense not found")
			return
		}
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := s.store.Summary(r.Context(), userID(r), from, to)
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, summary)
}

func buildExpense(req createExpenseRequest) (models.Expense, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return models.Expense{}, errors.New("title is required")
	}
	if req.Amount <= 0 {
		return models.Expense{}, errors.New("amount must be greater than zero")
	}
	if !models.ValidCategory(req.Category) {
		return models.Expense{}, errors.New("unknown category")
	}
	date, err := parseDate(req.Date)
	if err != nil {
		return models.Expense{}, err
	}
	return models.Expense{
		Title:    title,
		Amount:   req.Amount,
		Category: req.Category,
		Date:     date,
		Note:     strings.TrimSpace(req.Note),
	}, nil
}

func parseDate(value string) (time.Time, error) {
	if value == "" {
		return time.Now().UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("date must be RFC3339 or YYYY-MM-DD")
}
