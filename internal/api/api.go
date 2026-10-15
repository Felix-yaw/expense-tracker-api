// Package api wires HTTP routes to the store and auth services.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Felix-yaw/expense-tracker-api/internal/auth"
	"github.com/Felix-yaw/expense-tracker-api/internal/config"
	"github.com/Felix-yaw/expense-tracker-api/internal/httpx"
	"github.com/Felix-yaw/expense-tracker-api/internal/models"
	"github.com/Felix-yaw/expense-tracker-api/internal/store"
)

// Server holds handler dependencies and the router.
type Server struct {
	store *store.Store
	auth  *auth.Service
	cfg   config.Config
	log   *slog.Logger
	mux   *http.ServeMux
}

// New constructs a Server and registers its routes.
func New(st *store.Store, a *auth.Service, cfg config.Config, log *slog.Logger) *Server {
	s := &Server{store: st, auth: a, cfg: cfg, log: log, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Routes returns the fully-wrapped HTTP handler.
func (s *Server) Routes() http.Handler {
	return httpx.Recover(s.log, httpx.Logging(s.log, httpx.CORS(s.cfg.CORSOrigins, s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/v1/categories", s.handleCategories)

	s.mux.Handle("GET /api/v1/me", s.authMiddleware(s.handleMe))
	s.mux.Handle("GET /api/v1/expenses", s.authMiddleware(s.handleListExpenses))
	s.mux.Handle("POST /api/v1/expenses", s.authMiddleware(s.handleCreateExpense))
	s.mux.Handle("GET /api/v1/expenses/summary", s.authMiddleware(s.handleSummary))
	s.mux.Handle("GET /api/v1/expenses/{id}", s.authMiddleware(s.handleGetExpense))
	s.mux.Handle("PATCH /api/v1/expenses/{id}", s.authMiddleware(s.handleUpdateExpense))
	s.mux.Handle("DELETE /api/v1/expenses/{id}", s.authMiddleware(s.handleDeleteExpense))
	s.mux.Handle("GET /api/v1/budget", s.authMiddleware(s.handleGetBudget))
	s.mux.Handle("PUT /api/v1/budget", s.authMiddleware(s.handlePutBudget))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCategories(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"categories": models.Categories})
}

// ───────────────────────────── Middleware ───────────────────────────────

type contextKey int

const userIDContextKey contextKey = iota

func (s *Server) authMiddleware(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			httpx.WriteError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		id, err := s.auth.Parse(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userIDContextKey, id)))
	})
}

func userID(r *http.Request) int64 {
	id, _ := r.Context().Value(userIDContextKey).(int64)
	return id
}

// ───────────────────────────── Helpers ─────────────────────────────────

func (s *Server) serverError(w http.ResponseWriter, err error) {
	s.log.Error("internal error", "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func atoiDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func parseTimeParam(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		u := t.UTC()
		return &u, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return &t, nil
	}
	return nil, errors.New("date must be RFC3339 or YYYY-MM-DD")
}
