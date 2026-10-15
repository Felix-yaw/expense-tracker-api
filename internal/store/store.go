// Package store is the SQLite persistence layer.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Felix-yaw/expense-tracker-api/internal/models"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the SQLite database at path.
func Open(path string) (*Store, error) {
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite serialises writes; a single connection avoids "database is locked".
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		email         TEXT NOT NULL UNIQUE,
		name          TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS expenses (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		title      TEXT NOT NULL,
		amount     REAL NOT NULL,
		category   TEXT NOT NULL,
		date       TEXT NOT NULL,
		note       TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_expenses_user_date ON expenses(user_id, date DESC)`,
	`CREATE TABLE IF NOT EXISTS budgets (
		user_id         INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		monthly_limit   REAL NOT NULL DEFAULT 0,
		category_limits TEXT NOT NULL DEFAULT '{}'
	)`,
}

func (s *Store) migrate(ctx context.Context) error {
	for _, stmt := range migrations {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// ─────────────────────────────── Users ───────────────────────────────

// CreateUser inserts a user and returns it.
func (s *Store) CreateUser(ctx context.Context, email, name, passwordHash string) (models.User, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(email, name, password_hash, created_at) VALUES(?, ?, ?, ?)`,
		email, name, passwordHash, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return models.User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.User{}, err
	}
	return models.User{ID: id, Email: email, Name: name, CreatedAt: now}, nil
}

// UserByEmail returns a user and their password hash.
func (s *Store) UserByEmail(ctx context.Context, email string) (models.User, string, error) {
	var u models.User
	var hash, created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, password_hash, created_at FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &hash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, "", ErrNotFound
	}
	if err != nil {
		return models.User{}, "", err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return u, hash, nil
}

// UserByID returns a user by id.
func (s *Store) UserByID(ctx context.Context, id int64) (models.User, error) {
	var u models.User
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return u, nil
}

// ────────────────────────────── Expenses ─────────────────────────────

// ExpenseFilter narrows a list query.
type ExpenseFilter struct {
	Category string
	Query    string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// CreateExpense inserts an expense for a user.
func (s *Store) CreateExpense(ctx context.Context, userID int64, e models.Expense) (models.Expense, error) {
	e.CreatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO expenses(user_id, title, amount, category, date, note, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		userID, e.Title, e.Amount, e.Category,
		e.Date.UTC().Format(time.RFC3339Nano), e.Note, e.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return models.Expense{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Expense{}, err
	}
	e.ID = id
	return e, nil
}

func (s *Store) buildWhere(userID int64, f ExpenseFilter) (string, []any) {
	where := []string{"user_id = ?"}
	args := []any{userID}
	if f.Category != "" {
		where = append(where, "category = ?")
		args = append(args, f.Category)
	}
	if f.Query != "" {
		where = append(where, "(title LIKE ? OR note LIKE ?)")
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}
	if f.From != nil {
		where = append(where, "date >= ?")
		args = append(args, f.From.UTC().Format(time.RFC3339Nano))
	}
	if f.To != nil {
		where = append(where, "date <= ?")
		args = append(args, f.To.UTC().Format(time.RFC3339Nano))
	}
	return strings.Join(where, " AND "), args
}

// ListExpenses returns a page of expenses plus the total count.
func (s *Store) ListExpenses(ctx context.Context, userID int64, f ExpenseFilter) ([]models.Expense, int, error) {
	whereSQL, args := s.buildWhere(userID, f)

	var total int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM expenses WHERE "+whereSQL, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := "SELECT id, title, amount, category, date, note, created_at FROM expenses WHERE " +
		whereSQL + " ORDER BY date DESC, id DESC LIMIT ? OFFSET ?"
	rows, err := s.db.QueryContext(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]models.Expense, 0)
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, e)
	}
	return list, total, rows.Err()
}

// GetExpense returns one expense owned by the user.
func (s *Store) GetExpense(ctx context.Context, userID, id int64) (models.Expense, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, amount, category, date, note, created_at
		 FROM expenses WHERE id = ? AND user_id = ?`, id, userID)
	e, err := scanExpense(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Expense{}, ErrNotFound
	}
	return e, err
}

// UpdateExpense replaces the mutable fields of an expense.
func (s *Store) UpdateExpense(ctx context.Context, userID int64, e models.Expense) (models.Expense, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE expenses SET title = ?, amount = ?, category = ?, date = ?, note = ?
		 WHERE id = ? AND user_id = ?`,
		e.Title, e.Amount, e.Category, e.Date.UTC().Format(time.RFC3339Nano), e.Note, e.ID, userID,
	)
	if err != nil {
		return models.Expense{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return models.Expense{}, ErrNotFound
	}
	return s.GetExpense(ctx, userID, e.ID)
}

// DeleteExpense removes an expense.
func (s *Store) DeleteExpense(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM expenses WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Summary aggregates spending totals for a user over an optional period.
func (s *Store) Summary(ctx context.Context, userID int64, from, to *time.Time) (models.Summary, error) {
	whereSQL, args := s.buildWhere(userID, ExpenseFilter{From: from, To: to})

	summary := models.Summary{ByCategory: []models.CategoryTotal{}}
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM expenses WHERE "+whereSQL, args...,
	).Scan(&summary.Total, &summary.Count)
	if err != nil {
		return models.Summary{}, err
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT category, COALESCE(SUM(amount), 0), COUNT(*) FROM expenses WHERE "+
			whereSQL+" GROUP BY category ORDER BY SUM(amount) DESC", args...)
	if err != nil {
		return models.Summary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var ct models.CategoryTotal
		if err := rows.Scan(&ct.Category, &ct.Amount, &ct.Count); err != nil {
			return models.Summary{}, err
		}
		summary.ByCategory = append(summary.ByCategory, ct)
	}
	return summary, rows.Err()
}

// ─────────────────────────────── Budget ──────────────────────────────

// GetBudget returns a user's budget (zero value when unset).
func (s *Store) GetBudget(ctx context.Context, userID int64) (models.Budget, error) {
	var limit float64
	var limitsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT monthly_limit, category_limits FROM budgets WHERE user_id = ?`, userID,
	).Scan(&limit, &limitsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Budget{CategoryLimits: map[string]float64{}}, nil
	}
	if err != nil {
		return models.Budget{}, err
	}
	limits := map[string]float64{}
	if limitsJSON != "" {
		_ = json.Unmarshal([]byte(limitsJSON), &limits)
	}
	return models.Budget{MonthlyLimit: limit, CategoryLimits: limits}, nil
}

// SaveBudget inserts or updates a user's budget.
func (s *Store) SaveBudget(ctx context.Context, userID int64, b models.Budget) error {
	if b.CategoryLimits == nil {
		b.CategoryLimits = map[string]float64{}
	}
	data, err := json.Marshal(b.CategoryLimits)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO budgets(user_id, monthly_limit, category_limits) VALUES(?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   monthly_limit = excluded.monthly_limit,
		   category_limits = excluded.category_limits`,
		userID, b.MonthlyLimit, string(data),
	)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanExpense(sc rowScanner) (models.Expense, error) {
	var e models.Expense
	var date, created string
	if err := sc.Scan(&e.ID, &e.Title, &e.Amount, &e.Category, &date, &e.Note, &created); err != nil {
		return models.Expense{}, err
	}
	e.Date, _ = time.Parse(time.RFC3339Nano, date)
	e.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return e, nil
}
