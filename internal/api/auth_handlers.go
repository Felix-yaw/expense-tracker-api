package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Felix-yaw/expense-tracker-api/internal/auth"
	"github.com/Felix-yaw/expense-tracker-api/internal/httpx"
	"github.com/Felix-yaw/expense-tracker-api/internal/models"
	"github.com/Felix-yaw/expense-tracker-api/internal/store"
)

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !httpx.ReadJSON(w, r, &req) {
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)
	switch {
	case !strings.Contains(email, "@") || len(email) < 3:
		httpx.WriteError(w, http.StatusBadRequest, "a valid email is required")
		return
	case name == "":
		httpx.WriteError(w, http.StatusBadRequest, "name is required")
		return
	case len(req.Password) < 8:
		httpx.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if _, _, err := s.store.UserByEmail(r.Context(), email); err == nil {
		httpx.WriteError(w, http.StatusConflict, "email already registered")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.serverError(w, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), email, name, hash)
	if err != nil {
		s.serverError(w, err)
		return
	}
	token, err := s.auth.Issue(user.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.ReadJSON(w, r, &req) {
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, hash, err := s.store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	if !auth.CheckPassword(hash, req.Password) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := s.auth.Issue(user.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.store.UserByID(r.Context(), userID(r))
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}
