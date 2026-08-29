// Package studentaccess manages additive grants for one student.
package studentaccess

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/access"
)

var (
	ErrNotFound     = errors.New("student or membership not found")
	ErrInvalidGrant = errors.New("invalid student grant")
)

type Assignment struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	RoleKey     string    `json:"roleKey"`
	RoleName    string    `json:"roleName"`
	Grants      []string  `json:"grants"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) List(ctx context.Context, organizationID, studentID uuid.UUID) ([]Assignment, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id,u.display_name,u.email,r.role_key,r.name,
		array_agg(g.grant_code ORDER BY g.grant_code)
		FROM student_access_grants g
		JOIN memberships m ON m.organization_id=g.organization_id AND m.user_id=g.user_id AND m.is_active
		JOIN users u ON u.id=m.user_id AND u.is_active
		JOIN roles r ON r.id=m.role_id
		WHERE g.organization_id=$1 AND g.student_id=$2
		GROUP BY u.id,u.display_name,u.email,r.role_key,r.name
		ORDER BY u.display_name,u.id`, organizationID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Assignment, 0)
	for rows.Next() {
		var item Assignment
		if err := rows.Scan(&item.UserID, &item.DisplayName, &item.Email, &item.RoleKey, &item.RoleName, &item.Grants); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) MembershipExists(ctx context.Context, organizationID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships m JOIN users u ON u.id=m.user_id
		WHERE m.organization_id=$1 AND m.user_id=$2 AND m.is_active AND u.is_active)`, organizationID, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) Set(ctx context.Context, organizationID, studentID, userID, actorID uuid.UUID, grants []string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM student_access_grants
		WHERE organization_id=$1 AND student_id=$2 AND user_id=$3`, organizationID, studentID, userID); err != nil {
		return err
	}
	for _, grant := range grants {
		if _, err = tx.Exec(ctx, `INSERT INTO student_access_grants(organization_id,student_id,user_id,grant_code)
			VALUES($1,$2,$3,$4)`, organizationID, studentID, userID, grant); err != nil {
			return err
		}
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	action := "permission.grant"
	if len(grants) == 0 {
		action = "permission.revoke"
	}
	metadata, _ := json.Marshal(map[string]any{"userId": userID, "grants": grants})
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,metadata)
		VALUES($1,$2,$3,$4,'student',$5,$6)`, eventID, organizationID, actorID, action, studentID, metadata); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type StudentChecker interface {
	Exists(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type Service struct {
	repo          *Repository
	students      StudentChecker
	authorization access.AuthorizationService
}

func NewService(repo *Repository, students StudentChecker) *Service {
	return &Service{repo: repo, students: students}
}

func (s *Service) List(ctx context.Context, actor access.Actor, studentID uuid.UUID) ([]Assignment, error) {
	if err := s.authorization.Can(ctx, actor, access.AccessView, access.Resource{OrganizationID: actor.OrganizationID}); err != nil {
		return nil, err
	}
	if err := s.requireStudent(ctx, actor.OrganizationID, studentID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, actor.OrganizationID, studentID)
}

func (s *Service) Set(ctx context.Context, actor access.Actor, studentID, userID uuid.UUID, grants []string) error {
	if err := s.authorization.Can(ctx, actor, access.AccessManage, access.Resource{OrganizationID: actor.OrganizationID}); err != nil {
		return err
	}
	if err := s.requireStudent(ctx, actor.OrganizationID, studentID); err != nil {
		return err
	}
	member, err := s.repo.MembershipExists(ctx, actor.OrganizationID, userID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNotFound
	}
	normalized, err := normalizeGrants(grants)
	if err != nil {
		return err
	}
	return s.repo.Set(ctx, actor.OrganizationID, studentID, userID, actor.UserID, normalized)
}

func (s *Service) Revoke(ctx context.Context, actor access.Actor, studentID, userID uuid.UUID) error {
	return s.Set(ctx, actor, studentID, userID, nil)
}

func (s *Service) requireStudent(ctx context.Context, organizationID, studentID uuid.UUID) error {
	exists, err := s.students.Exists(ctx, organizationID, studentID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func normalizeGrants(values []string) ([]string, error) {
	allowed := map[string]struct{}{"view": {}, "upload": {}, "download": {}, "edit": {}}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return nil, ErrInvalidGrant
		}
		set[value] = struct{}{}
	}
	if len(set) > 0 {
		set["view"] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}
