package document

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
)

const (
	MIMEDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MIMEPDF  = "application/pdf"
)

var (
	ErrUnsupportedFile = errors.New("unsupported file")
	ErrFileTooLarge    = errors.New("file too large")
	ErrStudentNotFound = errors.New("student not found")
)

type StudentChecker interface {
	Exists(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type Service struct {
	repo          *Repository
	storage       Storage
	scanner       Scanner
	students      StudentChecker
	authorization access.AuthorizationService
	maxBytes      int64
}

type UploadInput struct {
	Title                string
	DocumentType         *string
	ConfidentialityLevel string
	OriginalFilename     string
	Bytes                []byte
	Audit                AuditContext
}

func NewService(repo *Repository, storage Storage, scanner Scanner, students StudentChecker, maxBytes int64) *Service {
	return &Service{repo: repo, storage: storage, scanner: scanner, students: students, maxBytes: maxBytes}
}

func (s *Service) List(ctx context.Context, actor access.Actor, studentID uuid.UUID) ([]Document, error) {
	if err := s.authorizeStudent(ctx, actor, access.DocumentsList, studentID, "standard"); err != nil {
		return nil, err
	}
	exists, err := s.students.Exists(ctx, actor.OrganizationID, studentID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrStudentNotFound
	}
	documents, err := s.repo.List(ctx, actor.OrganizationID, studentID)
	if err != nil {
		return nil, err
	}
	filtered := documents[:0]
	for _, document := range documents {
		if s.authorizeStudent(ctx, actor, access.DocumentsView, studentID, document.ConfidentialityLevel) == nil {
			filtered = append(filtered, document)
		}
	}
	return filtered, nil
}

func (s *Service) Get(ctx context.Context, actor access.Actor, documentID uuid.UUID, audit AuditContext) (Document, error) {
	document, err := s.repo.Get(ctx, actor.OrganizationID, documentID)
	if err != nil {
		return Document{}, err
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsView, document.StudentID, document.ConfidentialityLevel); err != nil {
		return Document{}, err
	}
	if err := s.repo.Audit(ctx, actor.OrganizationID, actor.UserID, document.ID, "document.view", audit,
		map[string]any{"versionNumber": document.VersionNumber}); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Service) Versions(ctx context.Context, actor access.Actor, documentID uuid.UUID) ([]Version, error) {
	document, err := s.repo.Get(ctx, actor.OrganizationID, documentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsView, document.StudentID, document.ConfidentialityLevel); err != nil {
		return nil, err
	}
	return s.repo.Versions(ctx, actor.OrganizationID, documentID)
}

func (s *Service) Upload(ctx context.Context, actor access.Actor, studentID uuid.UUID, input UploadInput) (Document, error) {
	confidentiality := input.ConfidentialityLevel
	if confidentiality == "" {
		confidentiality = "standard"
	}
	if confidentiality != "standard" && confidentiality != "restricted" {
		return Document{}, ErrUnsupportedFile
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsUpload, studentID, confidentiality); err != nil {
		return Document{}, err
	}
	exists, err := s.students.Exists(ctx, actor.OrganizationID, studentID)
	if err != nil {
		return Document{}, err
	}
	if !exists {
		return Document{}, ErrStudentNotFound
	}
	file, err := s.inspectAndScan(ctx, input.OriginalFilename, input.Bytes)
	if err != nil {
		return Document{}, err
	}
	documentID, err := uuid.NewV7()
	if err != nil {
		return Document{}, err
	}
	versionID, err := uuid.NewV7()
	if err != nil {
		return Document{}, err
	}
	objectKey := objectKey(actor.OrganizationID, studentID, documentID, versionID)
	if err := s.storage.Put(ctx, objectKey, bytes.NewReader(input.Bytes), int64(len(input.Bytes)), file.mime); err != nil {
		return Document{}, err
	}
	created := false
	defer func() {
		if !created {
			_ = s.storage.Delete(context.WithoutCancel(ctx), objectKey)
		}
	}()
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = safeDisplayFilename(input.OriginalFilename)
	}
	if len(title) == 0 || len(title) > 255 {
		return Document{}, ErrUnsupportedFile
	}
	document, err := s.repo.Create(ctx, actor.OrganizationID, studentID, actor.UserID, documentID, title,
		cleanOptional(input.DocumentType, 100), confidentiality,
		NewFile{ID: versionID, ObjectKey: objectKey, OriginalFilename: safeDisplayFilename(input.OriginalFilename), MIMEType: file.mime, Size: int64(len(input.Bytes)), SHA256: file.sha256}, input.Audit)
	if err != nil {
		return Document{}, err
	}
	created = true
	return document, nil
}

func (s *Service) OpenCurrent(ctx context.Context, actor access.Actor, documentID uuid.UUID, audit AuditContext) (Document, Version, io.ReadCloser, error) {
	return s.openVersion(ctx, actor, documentID, uuid.Nil, audit)
}

func (s *Service) OpenVersion(ctx context.Context, actor access.Actor, documentID, versionID uuid.UUID, audit AuditContext) (Document, Version, io.ReadCloser, error) {
	return s.openVersion(ctx, actor, documentID, versionID, audit)
}

func (s *Service) OpenPDFPreview(ctx context.Context, actor access.Actor, documentID uuid.UUID, audit AuditContext) (Document, Version, io.ReadCloser, error) {
	document, err := s.repo.Get(ctx, actor.OrganizationID, documentID)
	if err != nil {
		return Document{}, Version{}, nil, err
	}
	if document.MIMEType != MIMEPDF {
		return Document{}, Version{}, nil, ErrUnsupportedFile
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsView, document.StudentID, document.ConfidentialityLevel); err != nil {
		return Document{}, Version{}, nil, err
	}
	version, err := s.repo.Version(ctx, actor.OrganizationID, document.ID, document.CurrentVersionID)
	if err != nil {
		return Document{}, Version{}, nil, err
	}
	reader, err := s.storage.Get(ctx, version.ObjectKey)
	if err != nil {
		return Document{}, Version{}, nil, err
	}
	if err := s.repo.Audit(ctx, actor.OrganizationID, actor.UserID, document.ID, "document.view", audit,
		map[string]any{"versionNumber": version.VersionNumber, "mimeType": version.MIMEType, "size": version.Size}); err != nil {
		_ = reader.Close()
		return Document{}, Version{}, nil, err
	}
	return document, version, reader, nil
}

func (s *Service) openVersion(ctx context.Context, actor access.Actor, documentID, versionID uuid.UUID, audit AuditContext) (Document, Version, io.ReadCloser, error) {
	document, err := s.repo.Get(ctx, actor.OrganizationID, documentID)
	if err != nil {
		return Document{}, Version{}, nil, err
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsDownload, document.StudentID, document.ConfidentialityLevel); err != nil {
		return Document{}, Version{}, nil, err
	}
	if versionID == uuid.Nil {
		versionID = document.CurrentVersionID
	}
	version, err := s.repo.Version(ctx, actor.OrganizationID, documentID, versionID)
	if err != nil {
		return Document{}, Version{}, nil, err
	}
	reader, err := s.storage.Get(ctx, version.ObjectKey)
	if err != nil {
		return Document{}, Version{}, nil, err
	}
	if err := s.repo.Audit(ctx, actor.OrganizationID, actor.UserID, document.ID, "document.download", audit,
		map[string]any{"versionNumber": version.VersionNumber, "mimeType": version.MIMEType, "size": version.Size}); err != nil {
		_ = reader.Close()
		return Document{}, Version{}, nil, err
	}
	return document, version, reader, nil
}

func (s *Service) AuthorizeEdit(ctx context.Context, actor access.Actor, documentID uuid.UUID) (Document, error) {
	document, err := s.repo.Get(ctx, actor.OrganizationID, documentID)
	if err != nil {
		return Document{}, err
	}
	if document.MIMEType != MIMEDOCX {
		return Document{}, ErrUnsupportedFile
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsView, document.StudentID, document.ConfidentialityLevel); err != nil {
		return Document{}, err
	}
	if err := s.authorizeStudent(ctx, actor, access.DocumentsEdit, document.StudentID, document.ConfidentialityLevel); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Service) EditorDocument(ctx context.Context, actor access.Actor, documentID uuid.UUID, audit AuditContext) (Document, Version, error) {
	document, err := s.AuthorizeEdit(ctx, actor, documentID)
	if err != nil {
		return Document{}, Version{}, err
	}
	version, err := s.repo.Version(ctx, actor.OrganizationID, document.ID, document.CurrentVersionID)
	if err != nil {
		return Document{}, Version{}, err
	}
	if err := s.repo.Audit(ctx, actor.OrganizationID, actor.UserID, document.ID, "document.view", audit, map[string]any{"versionNumber": version.VersionNumber}); err != nil {
		return Document{}, Version{}, err
	}
	return document, version, nil
}

func (s *Service) OpenInternalVersion(ctx context.Context, organizationID, documentID, versionID uuid.UUID) (Version, io.ReadCloser, error) {
	version, err := s.repo.Version(ctx, organizationID, documentID, versionID)
	if err != nil {
		return Version{}, nil, err
	}
	reader, err := s.storage.Get(ctx, version.ObjectKey)
	return version, reader, err
}

func (s *Service) SaveEditedVersion(ctx context.Context, organizationID, actorID, documentID, expectedVersionID uuid.UUID, data []byte, audit AuditContext) (Document, error) {
	document, err := s.repo.Get(ctx, organizationID, documentID)
	if err != nil {
		return Document{}, err
	}
	file, err := s.inspectAndScan(ctx, "document.docx", data)
	if err != nil {
		return Document{}, err
	}
	if file.mime != MIMEDOCX {
		return Document{}, ErrUnsupportedFile
	}
	if document.CurrentVersionID != expectedVersionID {
		alreadySaved, lookupErr := s.repo.HasSourceHash(ctx, organizationID, documentID, expectedVersionID, file.sha256)
		if lookupErr != nil {
			return Document{}, lookupErr
		}
		if alreadySaved {
			return document, nil
		}
		return Document{}, ErrVersionConflict
	}
	versionID, err := uuid.NewV7()
	if err != nil {
		return Document{}, err
	}
	key := objectKey(organizationID, document.StudentID, documentID, versionID)
	if err := s.storage.Put(ctx, key, bytes.NewReader(data), int64(len(data)), file.mime); err != nil {
		return Document{}, err
	}
	created := false
	defer func() {
		if !created {
			_ = s.storage.Delete(context.WithoutCancel(ctx), key)
		}
	}()
	result, err := s.repo.AddVersion(ctx, organizationID, documentID, actorID, expectedVersionID,
		NewFile{ID: versionID, ObjectKey: key, OriginalFilename: document.OriginalFilename, MIMEType: file.mime, Size: int64(len(data)), SHA256: file.sha256}, audit)
	if errors.Is(err, ErrVersionConflict) {
		alreadySaved, lookupErr := s.repo.HasSourceHash(ctx, organizationID, documentID, expectedVersionID, file.sha256)
		if lookupErr == nil && alreadySaved {
			return s.repo.Get(ctx, organizationID, documentID)
		}
	}
	if err != nil {
		return Document{}, err
	}
	created = true
	return result, nil
}

func (s *Service) authorizeStudent(ctx context.Context, actor access.Actor, permission access.Permission, studentID uuid.UUID, confidentiality string) error {
	checkActor := actor
	if confidentiality == "restricted" {
		checkActor.AllStudents = false
	}
	return s.authorization.Can(ctx, checkActor, permission, access.Resource{OrganizationID: actor.OrganizationID, StudentID: &studentID})
}

type inspectedFile struct{ mime, sha256 string }

func (s *Service) inspectAndScan(ctx context.Context, filename string, data []byte) (inspectedFile, error) {
	if int64(len(data)) > s.maxBytes {
		return inspectedFile{}, ErrFileTooLarge
	}
	if len(data) == 0 {
		return inspectedFile{}, ErrUnsupportedFile
	}
	extension := strings.ToLower(filepath.Ext(filename))
	var mime string
	switch extension {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return inspectedFile{}, ErrUnsupportedFile
		}
		mime = MIMEPDF
	case ".docx":
		if !isDOCX(data) {
			return inspectedFile{}, ErrUnsupportedFile
		}
		mime = MIMEDOCX
	default:
		return inspectedFile{}, ErrUnsupportedFile
	}
	if err := s.scanner.Scan(ctx, data); err != nil {
		return inspectedFile{}, err
	}
	hash := sha256.Sum256(data)
	return inspectedFile{mime: mime, sha256: hex.EncodeToString(hash[:])}, nil
}

func isDOCX(data []byte) bool {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	hasContentTypes, hasDocument := false, false
	for _, file := range reader.File {
		switch file.Name {
		case "[Content_Types].xml":
			hasContentTypes = true
		case "word/document.xml":
			hasDocument = true
		}
	}
	return hasContentTypes && hasDocument
}

func objectKey(organizationID, studentID, documentID, versionID uuid.UUID) string {
	return fmt.Sprintf("organizations/%s/students/%s/documents/%s/versions/%s", organizationID, studentID, documentID, versionID)
}

func safeDisplayFilename(value string) string {
	value = strings.TrimSpace(filepath.Base(strings.ReplaceAll(value, "\\", "/")))
	value = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
	if len(value) > 255 {
		value = value[:255]
	}
	if value == "" || value == "." {
		return "document"
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
	if len(clean) > max {
		clean = clean[:max]
	}
	return &clean
}
