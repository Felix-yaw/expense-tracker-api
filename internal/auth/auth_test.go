package auth

import (
	"testing"
	"time"
)

func TestIssueAndParse(t *testing.T) {
	s := New("test-secret", time.Hour)

	token, err := s.Issue(42)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	id, err := s.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if id != 42 {
		t.Fatalf("got id %d, want 42", id)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	s := New("test-secret", time.Hour)
	if _, err := s.Parse("not-a-token"); err == nil {
		t.Fatal("expected an error for an invalid token")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	token, err := New("secret-a", time.Hour).Issue(1)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := New("secret-b", time.Hour).Parse(token); err == nil {
		t.Fatal("expected an error when verifying with a different secret")
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword(hash, "password123") {
		t.Fatal("correct password should match")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("incorrect password should not match")
	}
}
