package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/access"
)

var ErrCredentials = errors.New("invalid credentials")

type LoginAccount struct {
	UserID, OrganizationID                             uuid.UUID
	Email, DisplayName, OrganizationName, PasswordHash string
	MembershipActive, UserActive, AllStudents          bool
}

type Principal struct {
	Session                              Session
	Email, DisplayName, OrganizationName string
	Actor                                access.Actor
	CSRFHash                             [sha256.Size]byte
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) FindLoginAccount(ctx context.Context, email string) (LoginAccount, error) {
	var a LoginAccount
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, m.organization_id, u.email, u.display_name, o.name, COALESCE(u.password_hash, ''),
		       m.is_active, u.is_active, m.all_students
		FROM users u
		JOIN memberships m ON m.user_id = u.id
		JOIN organizations o ON o.id = m.organization_id
		WHERE u.email = lower($1)
		ORDER BY m.created_at
		LIMIT 1`, email).Scan(&a.UserID, &a.OrganizationID, &a.Email, &a.DisplayName, &a.OrganizationName,
		&a.PasswordHash, &a.MembershipActive, &a.UserActive, &a.AllStudents)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginAccount{}, ErrCredentials
	}
	return a, err
}

func (r *Repository) CreateSession(ctx context.Context, account LoginAccount, tokenHash, csrfHash [sha256.Size]byte, expiresAt time.Time) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO sessions
		(id, organization_id, user_id, token_hash, csrf_token_hash, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, id, account.OrganizationID, account.UserID, tokenHash[:], csrfHash[:], expiresAt)
	return id, err
}

func (r *Repository) PrincipalByToken(ctx context.Context, token string, now time.Time) (Principal, error) {
	hash := HashSessionToken(token)
	var p Principal
	var tokenHash, csrfHash []byte
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, s.user_id, s.organization_id, s.expires_at, s.revoked_at,
		       s.token_hash, s.csrf_token_hash, u.email, u.display_name, o.name, m.is_active, m.all_students
		FROM sessions s
		JOIN users u ON u.id = s.user_id AND u.is_active
		JOIN memberships m ON m.organization_id = s.organization_id AND m.user_id = s.user_id
		JOIN organizations o ON o.id = s.organization_id
		WHERE s.token_hash = $1`, hash[:]).Scan(
		&p.Session.ID, &p.Session.UserID, &p.Session.OrganizationID, &p.Session.ExpiresAt, &p.Session.RevokedAt,
		&tokenHash, &csrfHash, &p.Email, &p.DisplayName, &p.OrganizationName, &p.Actor.Active, &p.Actor.AllStudents)
	if err != nil {
		return Principal{}, ErrInvalidSession
	}
	if len(csrfHash) != sha256.Size || len(tokenHash) != sha256.Size || p.Session.Validate(now) != nil {
		return Principal{}, ErrInvalidSession
	}
	copy(p.CSRFHash[:], csrfHash)
	p.Actor.UserID = p.Session.UserID
	p.Actor.OrganizationID = p.Session.OrganizationID
	p.Actor.Permissions = make(map[access.Permission]struct{})
	p.Actor.StudentGrants = make(map[uuid.UUID]map[access.StudentGrant]struct{})
	rows, err := r.pool.Query(ctx, `SELECT rp.permission_code
		FROM memberships m JOIN role_permissions rp ON rp.role_id=m.role_id
		WHERE m.organization_id=$1 AND m.user_id=$2`, p.Actor.OrganizationID, p.Actor.UserID)
	if err != nil {
		return Principal{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return Principal{}, err
		}
		p.Actor.Permissions[access.Permission(code)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return Principal{}, err
	}
	grants, err := r.pool.Query(ctx, `SELECT student_id, grant_code FROM student_access_grants
		WHERE organization_id=$1 AND user_id=$2`, p.Actor.OrganizationID, p.Actor.UserID)
	if err != nil {
		return Principal{}, err
	}
	defer grants.Close()
	for grants.Next() {
		var studentID uuid.UUID
		var code string
		if err := grants.Scan(&studentID, &code); err != nil {
			return Principal{}, err
		}
		if p.Actor.StudentGrants[studentID] == nil {
			p.Actor.StudentGrants[studentID] = make(map[access.StudentGrant]struct{})
		}
		p.Actor.StudentGrants[studentID][access.StudentGrant(code)] = struct{}{}
	}
	return p, grants.Err()
}

func (r *Repository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at, now()) WHERE id=$1`, id)
	return err
}

func (r *Repository) RevokeByToken(ctx context.Context, token string) error {
	hash := HashSessionToken(token)
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at, now()) WHERE token_hash=$1`, hash[:])
	return err
}

// Actor returns the current effective authorization state without requiring a browser session.
// It is used only for independently signed server callbacks such as ONLYOFFICE saves.
func (r *Repository) Actor(ctx context.Context, organizationID, userID uuid.UUID) (access.Actor, error) {
	actor := access.Actor{OrganizationID: organizationID, UserID: userID, Permissions: make(map[access.Permission]struct{}), StudentGrants: make(map[uuid.UUID]map[access.StudentGrant]struct{})}
	if err := r.pool.QueryRow(ctx, `SELECT (m.is_active AND u.is_active),m.all_students FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.organization_id=$1 AND m.user_id=$2`, organizationID, userID).Scan(&actor.Active, &actor.AllStudents); err != nil {
		return access.Actor{}, err
	}
	rows, err := r.pool.Query(ctx, `SELECT rp.permission_code FROM memberships m JOIN role_permissions rp ON rp.role_id=m.role_id
		WHERE m.organization_id=$1 AND m.user_id=$2`, organizationID, userID)
	if err != nil {
		return access.Actor{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return access.Actor{}, err
		}
		actor.Permissions[access.Permission(code)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return access.Actor{}, err
	}
	grants, err := r.pool.Query(ctx, `SELECT student_id,grant_code FROM student_access_grants WHERE organization_id=$1 AND user_id=$2`, organizationID, userID)
	if err != nil {
		return access.Actor{}, err
	}
	defer grants.Close()
	for grants.Next() {
		var studentID uuid.UUID
		var code string
		if err := grants.Scan(&studentID, &code); err != nil {
			return access.Actor{}, err
		}
		if actor.StudentGrants[studentID] == nil {
			actor.StudentGrants[studentID] = make(map[access.StudentGrant]struct{})
		}
		actor.StudentGrants[studentID][access.StudentGrant(code)] = struct{}{}
	}
	return actor, grants.Err()
}
