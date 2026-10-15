package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Felix-yaw/expense-tracker-api/internal/models"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestExpenseCRUD(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	user, err := s.CreateUser(ctx, "a@b.com", "A", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := s.CreateExpense(ctx, user.ID, models.Expense{
		Title: "Coffee", Amount: 4.5, Category: "food", Date: time.Now(),
	})
	if err != nil {
		t.Fatalf("create expense: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected a generated id")
	}

	list, total, err := s.ListExpenses(ctx, user.ID, ExpenseFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d", total, len(list))
	}

	got, err := s.GetExpense(ctx, user.ID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.Title = "Tea"
	if _, err := s.UpdateExpense(ctx, user.ID, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	again, _ := s.GetExpense(ctx, user.ID, created.ID)
	if again.Title != "Tea" {
		t.Fatalf("title = %q, want Tea", again.Title)
	}

	if err := s.DeleteExpense(ctx, user.ID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetExpense(ctx, user.ID, created.ID); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSummaryAndBudget(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	user, _ := s.CreateUser(ctx, "c@d.com", "C", "hash")
	_, _ = s.CreateExpense(ctx, user.ID, models.Expense{Title: "Lunch", Amount: 20, Category: "food", Date: time.Now()})
	_, _ = s.CreateExpense(ctx, user.ID, models.Expense{Title: "Bus", Amount: 5, Category: "transport", Date: time.Now()})

	summary, err := s.Summary(ctx, user.ID, nil, nil)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Total != 25 || summary.Count != 2 {
		t.Fatalf("total=%v count=%d", summary.Total, summary.Count)
	}
	if len(summary.ByCategory) != 2 {
		t.Fatalf("categories=%d, want 2", len(summary.ByCategory))
	}
	if summary.ByCategory[0].Category != "food" {
		t.Fatalf("top category = %q, want food", summary.ByCategory[0].Category)
	}

	if err := s.SaveBudget(ctx, user.ID, models.Budget{
		MonthlyLimit:   1000,
		CategoryLimits: map[string]float64{"food": 300},
	}); err != nil {
		t.Fatalf("save budget: %v", err)
	}
	budget, err := s.GetBudget(ctx, user.ID)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if budget.MonthlyLimit != 1000 || budget.CategoryLimits["food"] != 300 {
		t.Fatalf("budget = %+v", budget)
	}
}

func TestExpensesAreScopedToUser(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	alice, _ := s.CreateUser(ctx, "alice@x.com", "Alice", "h")
	bob, _ := s.CreateUser(ctx, "bob@x.com", "Bob", "h")

	aliceExpense, _ := s.CreateExpense(ctx, alice.ID, models.Expense{Title: "Alice", Amount: 1, Category: "other", Date: time.Now()})

	if _, err := s.GetExpense(ctx, bob.ID, aliceExpense.ID); err != ErrNotFound {
		t.Fatalf("bob should not see alice's expense, got %v", err)
	}
	list, total, _ := s.ListExpenses(ctx, bob.ID, ExpenseFilter{})
	if total != 0 || len(list) != 0 {
		t.Fatalf("bob list should be empty, got total=%d", total)
	}
}
