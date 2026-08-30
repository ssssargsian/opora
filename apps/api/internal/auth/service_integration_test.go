package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/platform/testdatabase"
)

func TestPersistentSessionExpiryAndLogout(t *testing.T) {
	pool := testdatabase.Start(t)
	password := "test-password-long-enough-1"
	seedAccount(t, pool, password)
	repository := NewRepository(pool)
	service := NewService(repository, 7*24*time.Hour)

	login, err := service.Login(context.Background(), "admin@example.test", password)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	firstRequest, err := service.Authenticate(context.Background(), login.SessionToken)
	if err != nil {
		t.Fatalf("first Authenticate() error = %v", err)
	}
	secondRequest, err := service.Authenticate(context.Background(), login.SessionToken)
	if err != nil || secondRequest.Session.ID != firstRequest.Session.ID {
		t.Fatalf("persistent Authenticate() session=%v error=%v", secondRequest.Session.ID, err)
	}
	var storedHash []byte
	if err := pool.QueryRow(context.Background(), `SELECT token_hash FROM sessions WHERE id=$1`, login.Principal.Session.ID).Scan(&storedHash); err != nil {
		t.Fatalf("read stored session hash: %v", err)
	}
	if string(storedHash) == login.SessionToken {
		t.Fatal("database stored the plaintext session token")
	}
	if err := service.Logout(context.Background(), firstRequest); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), login.SessionToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("revoked session error = %v", err)
	}

	expired, err := service.Login(context.Background(), "admin@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE sessions SET expires_at=created_at + interval '1 microsecond' WHERE id=$1`, expired.Principal.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), expired.SessionToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expired session error = %v", err)
	}
}

func seedAccount(t *testing.T, pool *pgxpool.Pool, password string) {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	org, user, role, membership := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO organizations(id,name)VALUES($1,'School')`, []any{org}},
		{`INSERT INTO users(id,email,password_hash,display_name)VALUES($1,'admin@example.test',$2,'Admin')`, []any{user, hash}},
		{`INSERT INTO roles(id,organization_id,role_key,name)VALUES($1,$2,'admin','Admin')`, []any{role, org}},
		{`INSERT INTO memberships(id,organization_id,user_id,role_id,all_students)VALUES($1,$2,$3,$4,true)`, []any{membership, org, user, role}},
		{`INSERT INTO role_permissions(role_id,permission_code) SELECT $1,code FROM permissions`, []any{role}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed account: %v", err)
		}
	}
}
