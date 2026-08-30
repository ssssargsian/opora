package user

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
)

var (
	ErrEmailConflict = errors.New("email already exists")
	ErrWrongPassword = errors.New("wrong current password")
)

type ProfileInput struct {
	LastName   string  `json:"lastName"`
	FirstName  string  `json:"firstName"`
	MiddleName *string `json:"middleName"`
	Email      string  `json:"email"`
}

type Profile struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"displayName"`
	LastName    string    `json:"lastName"`
	FirstName   string    `json:"firstName"`
	MiddleName  *string   `json:"middleName"`
	Email       string    `json:"email"`
}

type PasswordInput struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type RequestAudit struct {
	IPAddress string
	UserAgent string
}

func (r *Repository) UpdateProfile(ctx context.Context, actor access.Actor, input ProfileInput, audit RequestAudit) (Profile, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Profile{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	displayName := joinName(input.LastName, input.FirstName, input.MiddleName)
	result, err := tx.Exec(ctx, `UPDATE users SET email=$1,last_name=$2,first_name=$3,middle_name=$4,display_name=$5,updated_at=now()
		WHERE id=$6 AND EXISTS(SELECT 1 FROM memberships WHERE organization_id=$7 AND user_id=$6)`,
		input.Email, input.LastName, input.FirstName, input.MiddleName, displayName, actor.UserID, actor.OrganizationID)
	if isUniqueViolation(err) {
		return Profile{}, ErrEmailConflict
	}
	if err != nil {
		return Profile{}, err
	}
	if result.RowsAffected() != 1 {
		return Profile{}, access.ErrPermissionDenied
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return Profile{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,ip_address,user_agent)
		VALUES($1,$2,$3,'user.profile_update','user',$3,$4,$5)`, eventID, actor.OrganizationID, actor.UserID,
		nullable(audit.IPAddress), nullable(audit.UserAgent)); err != nil {
		return Profile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Profile{}, err
	}
	return Profile{ID: actor.UserID, DisplayName: displayName, LastName: input.LastName, FirstName: input.FirstName,
		MiddleName: input.MiddleName, Email: input.Email}, nil
}

func (r *Repository) PasswordHash(ctx context.Context, actor access.Actor) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(u.password_hash,'') FROM users u
		JOIN memberships m ON m.user_id=u.id WHERE m.organization_id=$1 AND u.id=$2 AND u.is_active AND m.is_active`,
		actor.OrganizationID, actor.UserID).Scan(&hash)
	return hash, err
}

func (r *Repository) ChangePassword(ctx context.Context, actor access.Actor, currentSessionID uuid.UUID, passwordHash string, audit RequestAudit) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2
		AND EXISTS(SELECT 1 FROM memberships WHERE organization_id=$3 AND user_id=$2)`, passwordHash, actor.UserID, actor.OrganizationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return access.ErrPermissionDenied
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,now())
		WHERE organization_id=$1 AND user_id=$2 AND id<>$3 AND revoked_at IS NULL`, actor.OrganizationID, actor.UserID, currentSessionID); err != nil {
		return err
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,ip_address,user_agent)
		VALUES($1,$2,$3,'user.password_change','user',$3,$4,$5)`, eventID, actor.OrganizationID, actor.UserID,
		nullable(audit.IPAddress), nullable(audit.UserAgent)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) UpdateProfile(ctx context.Context, actor access.Actor, input ProfileInput, audit RequestAudit) (Profile, error) {
	input.LastName = strings.TrimSpace(input.LastName)
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.MiddleName = cleanOptional(input.MiddleName, 100)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	address, err := mail.ParseAddress(input.Email)
	if !validName(input.LastName) || !validName(input.FirstName) || err != nil || address.Address != input.Email || len(input.Email) > 320 {
		return Profile{}, ErrInvalidInput
	}
	return s.repo.UpdateProfile(ctx, actor, input, audit)
}

func (s *Service) ChangePassword(ctx context.Context, actor access.Actor, currentSessionID uuid.UUID, input PasswordInput, audit RequestAudit) error {
	if len(input.CurrentPassword) == 0 || len(input.NewPassword) < 12 || len(input.NewPassword) > 1024 {
		return ErrInvalidInput
	}
	currentHash, err := s.repo.PasswordHash(ctx, actor)
	if err != nil {
		return err
	}
	if !auth.VerifyPassword(input.CurrentPassword, currentHash) {
		return ErrWrongPassword
	}
	newHash, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	return s.repo.ChangePassword(ctx, actor, currentSessionID, newHash, audit)
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
