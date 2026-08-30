package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
)

var (
	ErrInvalidInvitation = errors.New("invalid invitation")
	ErrExpiredInvitation = errors.New("expired invitation")
	ErrUsedInvitation    = errors.New("used invitation")
)

type InvitationMessage struct {
	Email            string
	DisplayName      string
	OrganizationName string
	Token            string
	ExpiresAt        time.Time
}

type InvitationMailer interface {
	SendInvitation(context.Context, InvitationMessage) error
}

type DisabledInvitationMailer struct{}

func (DisabledInvitationMailer) SendInvitation(context.Context, InvitationMessage) error {
	return ErrInvitationDelivery
}

type AcceptInvitationInput struct {
	Token    string
	Password string
}

type AcceptedInvitation struct {
	Email string `json:"email"`
}

func newInvitationToken() (string, [sha256.Size]byte, error) {
	var hash [sha256.Size]byte
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", hash, err
	}
	plain := base64.RawURLEncoding.EncodeToString(random)
	return plain, sha256.Sum256([]byte(plain)), nil
}

func (r *Repository) OrganizationName(ctx context.Context, organizationID uuid.UUID) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1`, organizationID).Scan(&name)
	return name, err
}

func (r *Repository) ResendInvitation(ctx context.Context, organizationID, actorID, userID, invitationID uuid.UUID, tokenHash [32]byte, expiresAt time.Time) (User, string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var organizationName string
	var passwordHash *string
	if err = tx.QueryRow(ctx, `SELECT o.name,u.password_hash FROM memberships m
		JOIN users u ON u.id=m.user_id JOIN organizations o ON o.id=m.organization_id
		WHERE m.organization_id=$1 AND m.user_id=$2 FOR UPDATE OF u,m`, organizationID, userID).Scan(&organizationName, &passwordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", ErrInvalidInvitation
		}
		return User{}, "", err
	}
	if passwordHash != nil {
		return User{}, "", ErrUsedInvitation
	}
	if _, err = tx.Exec(ctx, `UPDATE user_invitations SET revoked_at=now()
		WHERE organization_id=$1 AND user_id=$2 AND accepted_at IS NULL AND revoked_at IS NULL`, organizationID, userID); err != nil {
		return User{}, "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_invitations
		(id,organization_id,user_id,token_hash,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6)`,
		invitationID, organizationID, userID, tokenHash[:], expiresAt, actorID); err != nil {
		return User{}, "", err
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return User{}, "", err
	}
	metadata, _ := json.Marshal(map[string]any{"expiresAt": expiresAt})
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,metadata)
		VALUES($1,$2,$3,'user.invitation_resent','user',$4,$5)`, eventID, organizationID, actorID, userID, metadata); err != nil {
		return User{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, "", err
	}
	item, err := r.Get(ctx, organizationID, userID)
	return item, organizationName, err
}

func (r *Repository) AcceptInvitation(ctx context.Context, tokenHash [32]byte, passwordHash string, now time.Time) (AcceptedInvitation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AcceptedInvitation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var invitationID, organizationID, userID uuid.UUID
	var email string
	var expiresAt time.Time
	var acceptedAt, revokedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT i.id,i.organization_id,i.user_id,i.expires_at,i.accepted_at,i.revoked_at,u.email
		FROM user_invitations i JOIN users u ON u.id=i.user_id
		WHERE i.token_hash=$1 FOR UPDATE OF i,u`, tokenHash[:]).Scan(
		&invitationID, &organizationID, &userID, &expiresAt, &acceptedAt, &revokedAt, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptedInvitation{}, ErrInvalidInvitation
	}
	if err != nil {
		return AcceptedInvitation{}, err
	}
	if acceptedAt != nil || revokedAt != nil {
		return AcceptedInvitation{}, ErrUsedInvitation
	}
	if !now.Before(expiresAt) {
		return AcceptedInvitation{}, ErrExpiredInvitation
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1,is_active=true,updated_at=now() WHERE id=$2`, passwordHash, userID); err != nil {
		return AcceptedInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE memberships SET is_active=true,updated_at=now() WHERE organization_id=$1 AND user_id=$2`, organizationID, userID); err != nil {
		return AcceptedInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_invitations SET accepted_at=$1 WHERE id=$2`, now, invitationID); err != nil {
		return AcceptedInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_invitations SET revoked_at=$1 WHERE organization_id=$2 AND user_id=$3
		AND id<>$4 AND accepted_at IS NULL AND revoked_at IS NULL`, now, organizationID, userID, invitationID); err != nil {
		return AcceptedInvitation{}, err
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return AcceptedInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id)
		VALUES($1,$2,$3,'user.invitation_accepted','user',$3)`, eventID, organizationID, userID); err != nil {
		return AcceptedInvitation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AcceptedInvitation{}, err
	}
	return AcceptedInvitation{Email: email}, nil
}

func (s *Service) ResendInvitation(ctx context.Context, actor access.Actor, userID uuid.UUID) (User, error) {
	if err := s.requireAny(ctx, actor, access.UsersInvite, access.UsersManage, access.UsersCreate); err != nil {
		return User{}, err
	}
	invitationID, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	token, hash, err := newInvitationToken()
	if err != nil {
		return User{}, err
	}
	expiresAt := s.now().Add(s.invitationTTL)
	item, organizationName, err := s.repo.ResendInvitation(ctx, actor.OrganizationID, actor.UserID, userID, invitationID, hash, expiresAt)
	if err != nil {
		return User{}, err
	}
	if err := s.mailer.SendInvitation(ctx, InvitationMessage{Email: item.Email, DisplayName: item.DisplayName,
		OrganizationName: organizationName, Token: token, ExpiresAt: expiresAt}); err != nil {
		return item, err
	}
	return item, nil
}

func (s *Service) AcceptInvitation(ctx context.Context, input AcceptInvitationInput) (AcceptedInvitation, error) {
	if len(input.Token) < 32 || len(input.Password) < 12 || len(input.Password) > 1024 {
		return AcceptedInvitation{}, ErrInvalidInput
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return AcceptedInvitation{}, err
	}
	tokenHash := sha256.Sum256([]byte(input.Token))
	return s.repo.AcceptInvitation(ctx, tokenHash, passwordHash, s.now())
}
