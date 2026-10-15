package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Felix-yaw/expense-tracker-api/internal/api"
	"github.com/Felix-yaw/expense-tracker-api/internal/auth"
	"github.com/Felix-yaw/expense-tracker-api/internal/config"
	"github.com/Felix-yaw/expense-tracker-api/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.Config{
		JWTSecret:   "test-secret",
		JWTTTL:      time.Hour,
		CORSOrigins: []string{"*"},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := api.New(st, auth.New(cfg.JWTSecret, cfg.JWTTTL), cfg, logger).Routes()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func request(t *testing.T, client *http.Client, method, url, token string, body any) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestEndToEnd(t *testing.T) {
	srv := newTestServer(t)
	client := srv.Client()
	base := srv.URL

	if status, _ := request(t, client, http.MethodGet, base+"/healthz", "", nil); status != http.StatusOK {
		t.Fatalf("health status = %d", status)
	}

	// Register.
	status, body := request(t, client, http.MethodPost, base+"/api/v1/auth/register", "",
		map[string]string{"email": "me@example.com", "name": "Me", "password": "password123"})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d: %s", status, body)
	}
	var reg struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &reg); err != nil || reg.Token == "" {
		t.Fatalf("no token in response: %s", body)
	}

	// Duplicate registration is a conflict.
	if status, _ := request(t, client, http.MethodPost, base+"/api/v1/auth/register", "",
		map[string]string{"email": "me@example.com", "name": "Me", "password": "password123"}); status != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409", status)
	}

	// Protected route without a token.
	if status, _ := request(t, client, http.MethodGet, base+"/api/v1/expenses", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", status)
	}

	// Create an expense.
	status, body = request(t, client, http.MethodPost, base+"/api/v1/expenses", reg.Token,
		map[string]any{"title": "Coffee", "amount": 4.5, "category": "food", "date": "2026-01-15", "note": "morning"})
	if status != http.StatusCreated {
		t.Fatalf("create expense status = %d: %s", status, body)
	}

	// Invalid category is rejected.
	if status, _ := request(t, client, http.MethodPost, base+"/api/v1/expenses", reg.Token,
		map[string]any{"title": "X", "amount": 1, "category": "nope"}); status != http.StatusBadRequest {
		t.Fatalf("invalid category status = %d, want 400", status)
	}

	// List.
	status, body = request(t, client, http.MethodGet, base+"/api/v1/expenses", reg.Token, nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	var list struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(body, &list)
	if list.Total != 1 {
		t.Fatalf("list total = %d, want 1", list.Total)
	}

	// Summary.
	status, body = request(t, client, http.MethodGet, base+"/api/v1/expenses/summary", reg.Token, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"total":4.5`)) {
		t.Fatalf("summary status = %d body = %s", status, body)
	}

	// Budget put + get.
	if status, _ := request(t, client, http.MethodPut, base+"/api/v1/budget", reg.Token,
		map[string]any{"monthlyLimit": 2000, "categoryLimits": map[string]float64{"food": 400}}); status != http.StatusOK {
		t.Fatalf("budget put status = %d", status)
	}
	status, body = request(t, client, http.MethodGet, base+"/api/v1/budget", reg.Token, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte("2000")) {
		t.Fatalf("budget get status = %d body = %s", status, body)
	}

	// Login returns a token.
	status, _ = request(t, client, http.MethodPost, base+"/api/v1/auth/login", "",
		map[string]string{"email": "me@example.com", "password": "password123"})
	if status != http.StatusOK {
		t.Fatalf("login status = %d", status)
	}

	// Wrong password is rejected.
	if status, _ := request(t, client, http.MethodPost, base+"/api/v1/auth/login", "",
		map[string]string{"email": "me@example.com", "password": "wrong"}); status != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want 401", status)
	}
}
