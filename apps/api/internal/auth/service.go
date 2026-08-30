package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

type Service struct {
	repo              *Repository
	ttl               time.Duration
	now               func() time.Time
	dummyPasswordHash string
}

type LoginResult struct {
	Principal               Principal
	SessionToken, CSRFToken string
}

func NewService(repo *Repository, ttl time.Duration) *Service {
	dummy, _ := HashPassword("invalid-password-placeholder-1")
	return &Service{repo: repo, ttl: ttl, now: time.Now, dummyPasswordHash: dummy}
}

func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	account, err := s.repo.FindLoginAccount(ctx, email)
	if err != nil {
		_ = VerifyPassword(password, s.dummyPasswordHash)
		if errors.Is(err, ErrCredentials) {
			return LoginResult{}, ErrCredentials
		}
		return LoginResult{}, err
	}
	if !account.UserActive || !account.MembershipActive || !VerifyPassword(password, account.PasswordHash) {
		return LoginResult{}, ErrCredentials
	}
	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	csrf, csrfHash, err := newSecret()
	if err != nil {
		return LoginResult{}, err
	}
	expires := s.now().Add(s.ttl)
	id, err := s.repo.CreateSession(ctx, account, tokenHash, csrfHash, expires)
	if err != nil {
		return LoginResult{}, err
	}
	p, err := s.repo.PrincipalByToken(ctx, token, s.now())
	if err != nil {
		return LoginResult{}, err
	}
	p.Session.ID, p.Session.ExpiresAt = id, expires
	return LoginResult{Principal: p, SessionToken: token, CSRFToken: csrf}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrInvalidSession
	}
	return s.repo.PrincipalByToken(ctx, token, s.now())
}

func (s *Service) Logout(ctx context.Context, principal Principal) error {
	return s.repo.RevokeSession(ctx, principal.Session.ID)
}

func (s *Service) ValidateCSRF(principal Principal, token string) error {
	if token == "" {
		return ErrInvalidSession
	}
	hash := sha256.Sum256([]byte(token))
	if !equalHash(hash, principal.CSRFHash) {
		return ErrInvalidSession
	}
	return nil
}

func newSecret() (string, [sha256.Size]byte, error) {
	var hash [sha256.Size]byte
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", hash, err
	}
	plain := base64.RawURLEncoding.EncodeToString(value)
	return plain, sha256.Sum256([]byte(plain)), nil
}

func equalHash(a, b [sha256.Size]byte) bool { return subtle.ConstantTimeCompare(a[:], b[:]) == 1 }
