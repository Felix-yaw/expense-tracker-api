// Package models defines the API domain types.
package models

import "time"

// User is an account owner.
type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// Expense is a single recorded expense.
type Expense struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Amount    float64   `json:"amount"`
	Category  string    `json:"category"`
	Date      time.Time `json:"date"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

// Budget holds a monthly limit and optional per-category limits.
type Budget struct {
	MonthlyLimit   float64            `json:"monthlyLimit"`
	CategoryLimits map[string]float64 `json:"categoryLimits"`
}

// CategoryTotal is one row of a spending summary.
type CategoryTotal struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Count    int     `json:"count"`
}

// Summary aggregates spending over a period.
type Summary struct {
	Total      float64         `json:"total"`
	Count      int             `json:"count"`
	ByCategory []CategoryTotal `json:"byCategory"`
}

// Categories is the fixed set of spending categories, mirroring the app.
var Categories = []string{
	"food",
	"transport",
	"shopping",
	"bills",
	"health",
	"entertainment",
	"education",
	"other",
}

// ValidCategory reports whether c is a known category.
func ValidCategory(c string) bool {
	for _, k := range Categories {
		if k == c {
			return true
		}
	}
	return false
}
