// Package student owns tenant-scoped student cards.
package student

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

var (
	ErrNotFound     = errors.New("student not found")
	ErrInvalidInput = errors.New("invalid student input")
)

type Student struct {
	ID            uuid.UUID  `json:"id"`
	LastName      string     `json:"lastName"`
	FirstName     string     `json:"firstName"`
	MiddleName    *string    `json:"middleName"`
	BirthDate     *time.Time `json:"birthDate"`
	ClassName     *string    `json:"className"`
	DocumentCount int64      `json:"documentCount"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (s Student) FullName() string {
	parts := []string{s.LastName, s.FirstName}
	if s.MiddleName != nil {
		parts = append(parts, *s.MiddleName)
	}
	return strings.Join(parts, " ")
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) List(ctx context.Context, actor access.Actor) ([]Student, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id,s.last_name,s.first_name,s.middle_name,s.birth_date,s.class_name,
		       count(d.id),s.created_at,GREATEST(s.updated_at,COALESCE(max(d.updated_at),s.updated_at))
		FROM students s LEFT JOIN documents d ON d.organization_id=s.organization_id AND d.student_id=s.id
		WHERE s.organization_id=$1 AND ($2 OR EXISTS (
			SELECT 1 FROM student_access_grants g WHERE g.organization_id=s.organization_id
			AND g.student_id=s.id AND g.user_id=$3 AND g.grant_code='view'))
		GROUP BY s.id ORDER BY s.last_name,s.first_name,s.id`, actor.OrganizationID, actor.AllStudents, actor.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Student, 0)
	for rows.Next() {
		var s Student
		if err := rows.Scan(&s.ID, &s.LastName, &s.FirstName, &s.MiddleName, &s.BirthDate, &s.ClassName, &s.DocumentCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *Repository) Get(ctx context.Context, organizationID, id uuid.UUID) (Student, error) {
	var s Student
	err := r.pool.QueryRow(ctx, `SELECT s.id,s.last_name,s.first_name,s.middle_name,s.birth_date,s.class_name,
		count(d.id),s.created_at,GREATEST(s.updated_at,COALESCE(max(d.updated_at),s.updated_at))
		FROM students s LEFT JOIN documents d ON d.organization_id=s.organization_id AND d.student_id=s.id
		WHERE s.organization_id=$1 AND s.id=$2 GROUP BY s.id`, organizationID, id).Scan(
		&s.ID, &s.LastName, &s.FirstName, &s.MiddleName, &s.BirthDate, &s.ClassName, &s.DocumentCount, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Student{}, ErrNotFound
	}
	return s, err
}

func (r *Repository) Exists(ctx context.Context, organizationID, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM students WHERE organization_id=$1 AND id=$2)`, organizationID, id).Scan(&exists)
	return exists, err
}

func (r *Repository) Create(ctx context.Context, actor access.Actor, input CreateInput) (Student, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Student{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Student{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO students(id,organization_id,last_name,first_name,middle_name,birth_date,class_name)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, id, actor.OrganizationID, input.LastName, input.FirstName, input.MiddleName, input.BirthDate, input.ClassName); err != nil {
		return Student{}, err
	}
	grants := make([]access.StudentGrant, 0, 4)
	if _, ok := actor.Permissions[access.StudentsView]; ok {
		grants = append(grants, access.StudentView)
	}
	for permission, grant := range map[access.Permission]access.StudentGrant{
		access.DocumentsUpload: access.StudentUpload, access.DocumentsDownload: access.StudentDownload, access.DocumentsEdit: access.StudentEdit,
	} {
		if _, ok := actor.Permissions[permission]; ok {
			grants = append(grants, grant)
		}
	}
	for _, grant := range grants {
		if _, err = tx.Exec(ctx, `INSERT INTO student_access_grants(organization_id,student_id,user_id,grant_code)
			VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor.OrganizationID, id, actor.UserID, grant); err != nil {
			return Student{}, err
		}
	}
	auditID, err := uuid.NewV7()
	if err != nil {
		return Student{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(id,organization_id,actor_user_id,action,resource_type,resource_id)
		VALUES($1,$2,$3,'student.create','student',$4)`, auditID, actor.OrganizationID, actor.UserID, id); err != nil {
		return Student{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Student{}, err
	}
	return r.Get(ctx, actor.OrganizationID, id)
}

type CreateInput struct {
	LastName, FirstName   string
	MiddleName, ClassName *string
	BirthDate             *time.Time
}

type AuditContext struct {
	IPAddress string
	UserAgent string
}

func (r *Repository) Update(ctx context.Context, actor access.Actor, id uuid.UUID, input CreateInput, audit AuditContext) (Student, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Student{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `UPDATE students SET last_name=$1,first_name=$2,middle_name=$3,birth_date=$4,class_name=$5,updated_at=now()
		WHERE organization_id=$6 AND id=$7`, input.LastName, input.FirstName, input.MiddleName, input.BirthDate, input.ClassName, actor.OrganizationID, id)
	if err != nil {
		return Student{}, err
	}
	if result.RowsAffected() != 1 {
		return Student{}, ErrNotFound
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return Student{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,ip_address,user_agent)
		VALUES($1,$2,$3,'student.update','student',$4,$5,$6)`, eventID, actor.OrganizationID, actor.UserID, id,
		nullIfEmpty(audit.IPAddress), nullIfEmpty(audit.UserAgent)); err != nil {
		return Student{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Student{}, err
	}
	return r.Get(ctx, actor.OrganizationID, id)
}

type Service struct {
	repo          *Repository
	authorization access.AuthorizationService
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }
func (s *Service) List(ctx context.Context, actor access.Actor) ([]Student, error) {
	if err := s.authorization.Can(ctx, actor, access.StudentsList, access.Resource{OrganizationID: actor.OrganizationID}); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, actor)
}
func (s *Service) Get(ctx context.Context, actor access.Actor, id uuid.UUID) (Student, error) {
	if err := s.authorization.Can(ctx, actor, access.StudentsView, access.Resource{OrganizationID: actor.OrganizationID, StudentID: &id}); err != nil {
		return Student{}, err
	}
	return s.repo.Get(ctx, actor.OrganizationID, id)
}
func (s *Service) Create(ctx context.Context, actor access.Actor, input CreateInput) (Student, error) {
	if err := s.authorization.Can(ctx, actor, access.StudentsCreate, access.Resource{OrganizationID: actor.OrganizationID}); err != nil {
		return Student{}, err
	}
	if err := normalizeInput(&input); err != nil {
		return Student{}, ErrInvalidInput
	}
	return s.repo.Create(ctx, actor, input)
}

func (s *Service) Update(ctx context.Context, actor access.Actor, id uuid.UUID, input CreateInput, audit AuditContext) (Student, error) {
	if err := s.authorization.Can(ctx, actor, access.StudentsUpdate, access.Resource{OrganizationID: actor.OrganizationID, StudentID: &id}); err != nil {
		return Student{}, err
	}
	if err := normalizeInput(&input); err != nil {
		return Student{}, err
	}
	return s.repo.Update(ctx, actor, id, input, audit)
}

func normalizeInput(input *CreateInput) error {
	input.LastName = strings.TrimSpace(input.LastName)
	input.FirstName = strings.TrimSpace(input.FirstName)
	if (input.MiddleName != nil && len([]rune(strings.TrimSpace(*input.MiddleName))) > 100) ||
		(input.ClassName != nil && len([]rune(strings.TrimSpace(*input.ClassName))) > 32) {
		return ErrInvalidInput
	}
	input.MiddleName = cleanOptional(input.MiddleName, 100)
	input.ClassName = cleanOptional(input.ClassName, 32)
	if input.LastName == "" || input.FirstName == "" || len([]rune(input.LastName)) > 100 || len([]rune(input.FirstName)) > 100 {
		return ErrInvalidInput
	}
	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func cleanOptional(value *string, max int) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(*value)
	if clean == "" {
		return nil
	}
	if len([]rune(clean)) > max {
		return nil
	}
	return &clean
}
