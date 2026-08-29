package document

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/access"
	"opora.local/api/internal/platform/testdatabase"
	"opora.local/api/internal/student"
)

type memoryStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemoryStorage() *memoryStorage { return &memoryStorage{objects: make(map[string][]byte)} }
func (s *memoryStorage) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(r)
	if err == nil {
		s.mu.Lock()
		s.objects[key] = data
		s.mu.Unlock()
	}
	return err
}
func (s *memoryStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("missing object")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (s *memoryStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}

type cleanScanner struct{ calls int }

func (s *cleanScanner) Scan(context.Context, []byte) error { s.calls++; return nil }

func TestDocumentVerticalSliceAndAuthorization(t *testing.T) {
	pool := testdatabase.Start(t)
	ctx := context.Background()
	orgA, userA, studentA := seedTenant(t, pool, "a@example.test")
	orgB, _, studentB := seedTenant(t, pool, "b@example.test")
	storage := newMemoryStorage()
	scanner := &cleanScanner{}
	service := NewService(NewRepository(pool), storage, scanner, student.NewRepository(pool), 1024*1024)
	actor := fullActor(orgA, userA)

	doc, err := service.Upload(ctx, actor, studentA, UploadInput{Title: "Заключение", OriginalFilename: "report.docx", Bytes: testDOCX(t, "version one")})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if doc.VersionNumber != 1 || doc.StudentID != studentA {
		t.Fatalf("uploaded document = %+v", doc)
	}
	versions, err := service.Versions(ctx, actor, doc.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions after upload = %d, err=%v", len(versions), err)
	}
	if len(storage.objects) != 1 || scanner.calls != 1 {
		t.Fatalf("storage=%d scans=%d", len(storage.objects), scanner.calls)
	}

	if _, err = service.Upload(ctx, actor, studentA, UploadInput{OriginalFilename: "malware.exe", Bytes: []byte("MZ")}); !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("unsupported upload error=%v", err)
	}
	if _, err = service.Upload(ctx, actor, studentA, UploadInput{OriginalFilename: "large.pdf", Bytes: bytes.Repeat([]byte("x"), 1024*1024+1)}); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("oversized upload error=%v", err)
	}
	if _, err = service.Upload(ctx, actor, studentB, UploadInput{OriginalFilename: "file.pdf", Bytes: []byte("%PDF-1.7")}); !errors.Is(err, ErrStudentNotFound) {
		t.Fatalf("other tenant student error=%v", err)
	}
	if _, err = service.Get(ctx, access.Actor{UserID: userA, OrganizationID: orgB, Active: true, AllStudents: true, Permissions: actor.Permissions, StudentGrants: map[uuid.UUID]map[access.StudentGrant]struct{}{}}, doc.ID, AuditContext{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other tenant document error=%v", err)
	}

	viewOnly := actor
	viewOnly.Permissions = map[access.Permission]struct{}{access.DocumentsView: {}, access.DocumentsList: {}}
	if _, _, _, err = service.OpenCurrent(ctx, viewOnly, doc.ID, AuditContext{}); !errors.Is(err, access.ErrPermissionDenied) {
		t.Fatalf("download without permission error=%v", err)
	}
	if _, _, err = service.EditorDocument(ctx, viewOnly, doc.ID, AuditContext{}); !errors.Is(err, access.ErrPermissionDenied) {
		t.Fatalf("edit without permission error=%v", err)
	}

	edited, err := service.SaveEditedVersion(ctx, orgA, userA, doc.ID, doc.CurrentVersionID, testDOCX(t, "version two"), AuditContext{})
	if err != nil {
		t.Fatalf("SaveEditedVersion() error=%v", err)
	}
	if edited.VersionNumber != 2 || edited.CurrentVersionID == doc.CurrentVersionID {
		t.Fatalf("edited document=%+v", edited)
	}
	versions, err = service.Versions(ctx, actor, doc.ID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions after edit=%d err=%v", len(versions), err)
	}
	if versions[1].ID != doc.CurrentVersionID {
		t.Fatal("old version was not preserved")
	}
	if _, err = service.SaveEditedVersion(ctx, orgA, userA, doc.ID, doc.CurrentVersionID, testDOCX(t, "version two"), AuditContext{}); err != nil {
		t.Fatalf("idempotent callback error=%v", err)
	}
}

func seedTenant(t *testing.T, pool *pgxpool.Pool, email string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	org := uuid.New()
	user := uuid.New()
	role := uuid.New()
	membership := uuid.New()
	studentID := uuid.New()
	ctx := context.Background()
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO organizations(id,name)VALUES($1,$2)`, []any{org, "School"}},
		{`INSERT INTO users(id,email,display_name)VALUES($1,$2,'Tester')`, []any{user, email}},
		{`INSERT INTO roles(id,organization_id,role_key,name)VALUES($1,$2,'tester','Tester')`, []any{role, org}},
		{`INSERT INTO memberships(id,organization_id,user_id,role_id,all_students)VALUES($1,$2,$3,$4,true)`, []any{membership, org, user, role}},
		{`INSERT INTO students(id,organization_id,last_name,first_name)VALUES($1,$2,'Иванов','Иван')`, []any{studentID, org}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}
	return org, user, studentID
}
func fullActor(org, user uuid.UUID) access.Actor {
	return access.Actor{UserID: user, OrganizationID: org, Active: true, AllStudents: true, Permissions: map[access.Permission]struct{}{access.DocumentsList: {}, access.DocumentsView: {}, access.DocumentsUpload: {}, access.DocumentsDownload: {}, access.DocumentsEdit: {}}, StudentGrants: map[uuid.UUID]map[access.StudentGrant]struct{}{}}
}
func testDOCX(t *testing.T, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range map[string]string{"[Content_Types].xml": "<Types/>", "word/document.xml": "<document>" + text + "</document>"} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write([]byte(content))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
