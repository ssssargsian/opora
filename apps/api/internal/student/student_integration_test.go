package student

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/platform/testdatabase"
)

func TestSpecialistWithPermissionCreatesVisibleStudent(t *testing.T) {
	pool := testdatabase.Start(t)
	ctx := context.Background()
	organizationID, _ := uuid.NewV7()
	userID, _ := uuid.NewV7()
	roleID, _ := uuid.NewV7()
	membershipID, _ := uuid.NewV7()
	if _, err := pool.Exec(ctx, `INSERT INTO organizations(id,name) VALUES($1,'School')`, organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roles(id,organization_id,role_key,name) VALUES($1,$2,'case_worker','Специалист')`, roleID, organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,display_name) VALUES($1,'specialist@test.local','Специалист')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memberships(id,organization_id,user_id,role_id) VALUES($1,$2,$3,$4)`, membershipID, organizationID, userID, roleID); err != nil {
		t.Fatal(err)
	}
	actor := access.Actor{UserID: userID, OrganizationID: organizationID, Active: true, Permissions: map[access.Permission]struct{}{
		access.StudentsCreate: {}, access.StudentsList: {}, access.StudentsView: {}, access.DocumentsUpload: {},
	}, StudentGrants: map[uuid.UUID]map[access.StudentGrant]struct{}{}}
	service := NewService(NewRepository(pool))
	created, err := service.Create(ctx, actor, CreateInput{LastName: "Иванов", FirstName: "Иван"})
	if err != nil {
		t.Fatal(err)
	}
	actor.StudentGrants[created.ID] = map[access.StudentGrant]struct{}{access.StudentView: {}, access.StudentUpload: {}}
	items, err := service.List(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("creator cannot see created student: %#v", items)
	}
	var grants []string
	if err := pool.QueryRow(ctx, `SELECT array_agg(grant_code ORDER BY grant_code) FROM student_access_grants
		WHERE organization_id=$1 AND student_id=$2 AND user_id=$3`, organizationID, created.ID, userID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 || grants[0] != "upload" || grants[1] != "view" {
		t.Fatalf("unexpected creator grants: %#v", grants)
	}
}
