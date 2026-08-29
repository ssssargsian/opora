package document

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")
var ErrVersionConflict = errors.New("document version conflict")

type Document struct {
	ID, OrganizationID, StudentID                 uuid.UUID
	Title                                         string
	DocumentType                                  *string
	ConfidentialityLevel                          string
	CurrentVersionID                              uuid.UUID
	VersionNumber                                 int32
	OriginalFilename, MIMEType, SHA256, ChangedBy string
	Size                                          int64
	CreatedAt, UpdatedAt                          time.Time
}
type Version struct {
	ID, DocumentID                                           uuid.UUID
	VersionNumber                                            int32
	ObjectKey, OriginalFilename, MIMEType, SHA256, ChangedBy string
	Size                                                     int64
	CreatedBy                                                uuid.UUID
	CreatedAt                                                time.Time
}
type NewFile struct {
	ID                                            uuid.UUID
	ObjectKey, OriginalFilename, MIMEType, SHA256 string
	Size                                          int64
}
type AuditContext struct{ IPAddress, UserAgent string }

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const documentSelect = `SELECT d.id,d.organization_id,d.student_id,d.title,d.document_type,d.confidentiality_level,
	d.current_version_id,v.version_number,v.original_filename,v.mime_type,v.size,v.sha256,u.display_name,d.created_at,d.updated_at
	FROM documents d JOIN document_versions v ON v.id=d.current_version_id AND v.organization_id=d.organization_id
	JOIN users u ON u.id=v.created_by WHERE d.organization_id=$1 AND d.id=$2`

func (r *Repository) Get(ctx context.Context, org, id uuid.UUID) (Document, error) {
	var d Document
	err := r.pool.QueryRow(ctx, documentSelect, org, id).Scan(&d.ID, &d.OrganizationID, &d.StudentID, &d.Title, &d.DocumentType, &d.ConfidentialityLevel, &d.CurrentVersionID, &d.VersionNumber, &d.OriginalFilename, &d.MIMEType, &d.Size, &d.SHA256, &d.ChangedBy, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	return d, err
}
func (r *Repository) List(ctx context.Context, org, studentID uuid.UUID) ([]Document, error) {
	rows, err := r.pool.Query(ctx, `SELECT d.id,d.organization_id,d.student_id,d.title,d.document_type,d.confidentiality_level,
			d.current_version_id,v.version_number,v.original_filename,v.mime_type,v.size,v.sha256,u.display_name,d.created_at,d.updated_at
			FROM documents d JOIN document_versions v ON v.id=d.current_version_id AND v.organization_id=d.organization_id
			JOIN users u ON u.id=v.created_by WHERE d.organization_id=$1 AND d.student_id=$2 ORDER BY d.updated_at DESC,d.id`, org, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Document, 0)
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.StudentID, &d.Title, &d.DocumentType, &d.ConfidentialityLevel, &d.CurrentVersionID, &d.VersionNumber, &d.OriginalFilename, &d.MIMEType, &d.Size, &d.SHA256, &d.ChangedBy, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (r *Repository) Versions(ctx context.Context, org, documentID uuid.UUID) ([]Version, error) {
	rows, err := r.pool.Query(ctx, `SELECT v.id,v.document_id,v.version_number,v.object_key,v.original_filename,v.mime_type,v.size,v.sha256,v.created_by,u.display_name,v.created_at
		FROM document_versions v JOIN users u ON u.id=v.created_by WHERE v.organization_id=$1 AND v.document_id=$2 ORDER BY v.version_number DESC`, org, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Version, 0)
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.DocumentID, &v.VersionNumber, &v.ObjectKey, &v.OriginalFilename, &v.MIMEType, &v.Size, &v.SHA256, &v.CreatedBy, &v.ChangedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (r *Repository) Version(ctx context.Context, org, documentID, versionID uuid.UUID) (Version, error) {
	var v Version
	err := r.pool.QueryRow(ctx, `SELECT v.id,v.document_id,v.version_number,v.object_key,v.original_filename,v.mime_type,v.size,v.sha256,v.created_by,u.display_name,v.created_at
		FROM document_versions v JOIN users u ON u.id=v.created_by WHERE v.organization_id=$1 AND v.document_id=$2 AND v.id=$3`, org, documentID, versionID).Scan(&v.ID, &v.DocumentID, &v.VersionNumber, &v.ObjectKey, &v.OriginalFilename, &v.MIMEType, &v.Size, &v.SHA256, &v.CreatedBy, &v.ChangedBy, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	return v, err
}
func (r *Repository) Create(ctx context.Context, org, studentID, actorID, documentID uuid.UUID, title string, documentType *string, confidentiality string, file NewFile, audit AuditContext) (Document, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Document{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO documents(id,organization_id,student_id,title,document_type,confidentiality_level,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, documentID, org, studentID, title, documentType, confidentiality, actorID); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO document_versions(id,organization_id,document_id,version_number,object_key,original_filename,mime_type,size,sha256,created_by)
		VALUES($1,$2,$3,1,$4,$5,$6,$7,$8,$9)`, file.ID, org, documentID, file.ObjectKey, file.OriginalFilename, file.MIMEType, file.Size, file.SHA256, actorID); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE documents SET current_version_id=$1,updated_at=now() WHERE organization_id=$2 AND id=$3`, file.ID, org, documentID); err != nil {
		return Document{}, err
	}
	if err = insertAudit(ctx, tx, org, actorID, "document.upload", documentID, audit, map[string]any{"versionNumber": 1, "mimeType": file.MIMEType, "size": file.Size}); err != nil {
		return Document{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Document{}, err
	}
	return r.Get(ctx, org, documentID)
}
func (r *Repository) AddVersion(ctx context.Context, org, documentID, actorID, expectedCurrent uuid.UUID, file NewFile, audit AuditContext) (Document, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Document{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current uuid.UUID
	var next int32
	err = tx.QueryRow(ctx, `SELECT current_version_id FROM documents WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, documentID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	if current != expectedCurrent {
		return Document{}, ErrVersionConflict
	}
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version_number),0)+1 FROM document_versions WHERE organization_id=$1 AND document_id=$2`, org, documentID).Scan(&next); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO document_versions(id,organization_id,document_id,version_number,object_key,original_filename,mime_type,size,sha256,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, file.ID, org, documentID, next, file.ObjectKey, file.OriginalFilename, file.MIMEType, file.Size, file.SHA256, actorID); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE document_versions SET source_version_id=$1 WHERE organization_id=$2 AND document_id=$3 AND id=$4`, expectedCurrent, org, documentID, file.ID); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE documents SET current_version_id=$1,updated_at=now() WHERE organization_id=$2 AND id=$3`, file.ID, org, documentID); err != nil {
		return Document{}, err
	}
	if err = insertAudit(ctx, tx, org, actorID, "document.edit", documentID, audit, map[string]any{"versionNumber": next, "mimeType": file.MIMEType, "size": file.Size}); err != nil {
		return Document{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Document{}, err
	}
	return r.Get(ctx, org, documentID)
}

func (r *Repository) HasSourceHash(ctx context.Context, org, documentID, sourceVersionID uuid.UUID, sha256 string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM document_versions
		WHERE organization_id=$1 AND document_id=$2 AND source_version_id=$3 AND sha256=$4)`, org, documentID, sourceVersionID, sha256).Scan(&exists)
	return exists, err
}
func (r *Repository) Audit(ctx context.Context, org, actorID, resourceID uuid.UUID, action string, audit AuditContext, metadata map[string]any) error {
	return insertAudit(ctx, r.pool, org, actorID, action, resourceID, audit, metadata)
}
func insertAudit(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, org, actorID uuid.UUID, action string, resourceID uuid.UUID, audit AuditContext, metadata map[string]any) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var ip any
	if audit.IPAddress != "" {
		ip = audit.IPAddress
	}
	var ua any
	if audit.UserAgent != "" {
		ua = audit.UserAgent
	}
	_, err = db.Exec(ctx, `INSERT INTO audit_events(id,organization_id,actor_user_id,action,resource_type,resource_id,ip_address,user_agent,metadata)
		VALUES($1,$2,$3,$4,'document',$5,$6,$7,$8)`, id, org, actorID, action, resourceID, ip, ua, raw)
	return err
}
