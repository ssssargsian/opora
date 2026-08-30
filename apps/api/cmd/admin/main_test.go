package main

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()
	got, err := normalizeEmail(" Admin@Example.COM ")
	if err != nil || got != "admin@example.com" {
		t.Fatalf("normalizeEmail() = %q, %v", got, err)
	}
	if _, err := normalizeEmail("not-an-email"); err == nil {
		t.Fatal("normalizeEmail() accepted an invalid email")
	}
}

func TestGeneratedPasswordMeetsPasswordPolicy(t *testing.T) {
	t.Parallel()
	password, err := generatePassword()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(password, "Op-") || len(password) < 24 {
		t.Fatalf("unexpected generated password format: length=%d", len(password))
	}
}
