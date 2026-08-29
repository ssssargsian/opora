// Package user owns organization users, memberships, and role selection.
package user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
)

var (
	ErrConflict     = errors.New("user already belongs to organization")
	ErrInvalidInput = errors.New("invalid user input")
	ErrRoleNotFound = errors.New("role not found")
)

type Role struct {
	ID       uuid.UUID `json:"id"`
	Key      string    `json:"key"`
	Name     string    `json:"name"`
	IsSystem bool      `json:"isSystem"`
}

type User struct {
	ID              uuid.UUID `json:"id"`
	RoleID          uuid.UUID `json:"roleId"`
	DisplayName     string    `json:"displayName"`
	Email           string    `json:"email"`
	RoleKey         string    `json:"roleKey"`
	RoleName        string    `json:"roleName"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	InitialPassword string    `json:"initialPassword,omitempty"`
}

type CreateInput struct {
	LastName   string  `json:"lastName"`
	FirstName  string  `json:"firstName"`
	MiddleName *string `json:"middleName"`
	Email      string  `json:"email"`
	RoleKey    string  `json:"roleKey"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Roles(ctx context.Context, organizationID uuid.UUID) ([]Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,role_key,name,is_system FROM roles
		WHERE organization_id=$1 ORDER BY CASE role_key WHEN 'organization_admin' THEN 0 WHEN 'psychologist' THEN 1
		WHEN 'specialist' THEN 2 WHEN 'viewer' THEN 3 ELSE 4 END,name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Role, 0)
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Key, &role.Name, &role.IsSystem); err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, rows.Err()
}

func (r *Repository) List(ctx context.Context, organizationID uuid.UUID) ([]User, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id,u.display_name,u.email,r.id,r.role_key,r.name,
		CASE WHEN NOT u.is_active OR NOT m.is_active THEN 'blocked'
		     WHEN u.password_hash IS NULL THEN 'invited' ELSE 'active' END,m.created_at
		FROM memberships m JOIN users u ON u.id=m.user_id JOIN roles r ON r.id=m.role_id
		WHERE m.organization_id=$1 ORDER BY u.display_name,u.id`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]User, 0)
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Email, &item.RoleID, &item.RoleKey, &item.RoleName, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) Create(ctx context.Context, organizationID, actorID uuid.UUID, input CreateInput, passwordHash string) (User, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var roleID uuid.UUID
	var allStudents bool
	err = tx.QueryRow(ctx, `SELECT id,default_all_students FROM roles
		WHERE organization_id=$1 AND role_key=$2`, organizationID, input.RoleKey).Scan(&roleID, &allStudents)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrRoleNotFound
	}
	if err != nil {
		return User{}, err
	}

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, input.Email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		userID, err = uuid.NewV7()
		if err != nil {
			return User{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO users(id,email,display_name,password_hash) VALUES($1,$2,$3,$4)`,
			userID, input.Email, joinName(input.LastName, input.FirstName, input.MiddleName), passwordHash); err != nil {
			return User{}, err
		}
	} else if err != nil {
		return User{}, err
	} else {
		// Account linking across organizations needs an explicit identity-verification flow.
		return User{}, ErrConflict
	}

	membershipID, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO memberships(id,organization_id,user_id,role_id,all_students)
		VALUES($1,$2,$3,$4,$5)`, membershipID, organizationID, userID, roleID, allStudents)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}

	eventID, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	metadata, _ := json.Marshal(map[string]any{"roleKey": input.RoleKey, "status": "active"})
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,metadata)
		VALUES($1,$2,$3,'user.invite','user',$4,$5)`, eventID, organizationID, actorID, userID, metadata); err != nil {
		return User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return r.Get(ctx, organizationID, userID)
}

func (r *Repository) Get(ctx context.Context, organizationID, userID uuid.UUID) (User, error) {
	var item User
	err := r.pool.QueryRow(ctx, `SELECT u.id,u.display_name,u.email,r.id,r.role_key,r.name,
		CASE WHEN NOT u.is_active OR NOT m.is_active THEN 'blocked'
		     WHEN u.password_hash IS NULL THEN 'invited' ELSE 'active' END,m.created_at
		FROM memberships m JOIN users u ON u.id=m.user_id JOIN roles r ON r.id=m.role_id
		WHERE m.organization_id=$1 AND m.user_id=$2`, organizationID, userID).Scan(
		&item.ID, &item.DisplayName, &item.Email, &item.RoleID, &item.RoleKey, &item.RoleName, &item.Status, &item.CreatedAt)
	return item, err
}

type Service struct {
	repo          *Repository
	authorization access.AuthorizationService
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Roles(ctx context.Context, actor access.Actor) ([]Role, error) {
	if err := s.requireAny(ctx, actor, access.UsersView, access.UsersCreate, access.UsersInvite, access.UsersManage); err != nil {
		return nil, err
	}
	return s.repo.Roles(ctx, actor.OrganizationID)
}

func (s *Service) List(ctx context.Context, actor access.Actor) ([]User, error) {
	if err := s.requireAny(ctx, actor, access.UsersView, access.UsersManage); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, actor.OrganizationID)
}

func (s *Service) Create(ctx context.Context, actor access.Actor, input CreateInput) (User, error) {
	if err := s.requireAny(ctx, actor, access.UsersCreate, access.UsersInvite, access.UsersManage); err != nil {
		return User{}, err
	}
	input.LastName = strings.TrimSpace(input.LastName)
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.RoleKey = strings.TrimSpace(input.RoleKey)
	if input.MiddleName != nil && len([]rune(strings.TrimSpace(*input.MiddleName))) > 100 {
		return User{}, ErrInvalidInput
	}
	input.MiddleName = cleanOptional(input.MiddleName, 100)
	if !validName(input.LastName) || !validName(input.FirstName) || input.RoleKey == "" {
		return User{}, ErrInvalidInput
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 320 {
		return User{}, ErrInvalidInput
	}
	initialPassword, err := generateInitialPassword()
	if err != nil {
		return User{}, err
	}
	passwordHash, err := auth.HashPassword(initialPassword)
	if err != nil {
		return User{}, err
	}
	created, err := s.repo.Create(ctx, actor.OrganizationID, actor.UserID, input, passwordHash)
	if err != nil {
		return User{}, err
	}
	created.InitialPassword = initialPassword
	return created, nil
}

func (s *Service) requireAny(ctx context.Context, actor access.Actor, permissions ...access.Permission) error {
	resource := access.Resource{OrganizationID: actor.OrganizationID}
	for _, permission := range permissions {
		if s.authorization.Can(ctx, actor, permission, resource) == nil {
			return nil
		}
	}
	return access.ErrPermissionDenied
}

func generateInitialPassword() (string, error) {
	random := make([]byte, 15)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "Op-" + base64.RawURLEncoding.EncodeToString(random), nil
}

func validName(value string) bool { return value != "" && len([]rune(value)) <= 100 }

func cleanOptional(value *string, max int) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(*value)
	if clean == "" || len([]rune(clean)) > max {
		return nil
	}
	return &clean
}

func joinName(lastName, firstName string, middleName *string) string {
	parts := []string{lastName, firstName}
	if middleName != nil {
		parts = append(parts, *middleName)
	}
	return strings.Join(parts, " ")
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}
