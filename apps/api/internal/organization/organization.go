// Package organization owns tenant settings.
package organization

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/access"
)

var ErrInvalidInput = errors.New("invalid organization input")

type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AuditContext struct {
	IPAddress string
	UserAgent string
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Update(ctx context.Context, actor access.Actor, name string, audit AuditContext) (Organization, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Organization{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var result Organization
	err = tx.QueryRow(ctx, `UPDATE organizations SET name=$1,updated_at=now() WHERE id=$2 RETURNING id,name,updated_at`,
		name, actor.OrganizationID).Scan(&result.ID, &result.Name, &result.UpdatedAt)
	if err != nil {
		return Organization{}, err
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return Organization{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,ip_address,user_agent)
		VALUES($1,$2,$3,'organization.update','organization',$2,$4,$5)`, eventID, actor.OrganizationID, actor.UserID,
		nullable(audit.IPAddress), nullable(audit.UserAgent)); err != nil {
		return Organization{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Organization{}, err
	}
	return result, nil
}

type Service struct {
	repo          *Repository
	authorization access.AuthorizationService
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Update(ctx context.Context, actor access.Actor, name string, audit AuditContext) (Organization, error) {
	if err := s.authorization.Can(ctx, actor, access.OrganizationUpdate, access.Resource{OrganizationID: actor.OrganizationID}); err != nil {
		return Organization{}, err
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 255 {
		return Organization{}, ErrInvalidInput
	}
	return s.repo.Update(ctx, actor, name, audit)
}

func nullable(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
