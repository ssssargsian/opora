// Package auth owns authentication and server-side session primitives.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidSession deliberately does not reveal why a session is invalid.
var ErrInvalidSession = errors.New("invalid session")

const sessionTokenBytes = 32

// Session represents a revocable server-side browser session.
type Session struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	ExpiresAt      time.Time
	RevokedAt      *time.Time
}

// Validate rejects malformed, expired, and revoked sessions.
func (s Session) Validate(now time.Time) error {
	if s.ID == uuid.Nil || s.UserID == uuid.Nil || s.OrganizationID == uuid.Nil {
		return ErrInvalidSession
	}
	if s.RevokedAt != nil || !now.Before(s.ExpiresAt) {
		return ErrInvalidSession
	}
	return nil
}

// NewSessionToken returns a browser token and the only representation persisted by the server.
func NewSessionToken() (plaintext string, hash [sha256.Size]byte, err error) {
	value := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(value); err != nil {
		return "", hash, err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(value)
	hash = HashSessionToken(plaintext)
	return plaintext, hash, nil
}

// HashSessionToken hashes an opaque token for constant-size database lookup.
func HashSessionToken(plaintext string) [sha256.Size]byte {
	return sha256.Sum256([]byte(plaintext))
}
