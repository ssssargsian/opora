package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestExpiredSessionIsInvalid(t *testing.T) {
	now := time.Now()
	session := validSession(now)
	session.ExpiresAt = now
	if err := session.Validate(now); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Validate() error = %v, want invalid session", err)
	}
}

func TestRevokedSessionIsInvalid(t *testing.T) {
	now := time.Now()
	session := validSession(now)
	session.RevokedAt = &now
	if err := session.Validate(now); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Validate() error = %v, want invalid session", err)
	}
}

func TestSessionTokenIsRandomAndStoredAsHash(t *testing.T) {
	first, firstHash, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	second, secondHash, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("session tokens must be unique")
	}
	if first == string(firstHash[:]) {
		t.Fatal("plaintext token must not equal its stored hash")
	}
}

func validSession(now time.Time) Session {
	return Session{
		ID: uuid.New(), UserID: uuid.New(), OrganizationID: uuid.New(), ExpiresAt: now.Add(time.Hour),
	}
}
