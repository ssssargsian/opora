package organization

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/platform/testdatabase"
)

func TestOrganizationUpdatePermissionAndTenantScope(t *testing.T) {
	pool := testdatabase.Start(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()
	user, role, membership := uuid.New(), uuid.New(), uuid.New()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations(id,name) VALUES($1,'School A'),($2,'School B')`, []any{orgA, orgB}},
		{`INSERT INTO users(id,email,display_name) VALUES($1,'admin@school.test','Admin')`, []any{user}},
		{`INSERT INTO roles(id,organization_id,role_key,name) VALUES($1,$2,'admin','Admin')`, []any{role, orgA}},
		{`INSERT INTO memberships(id,organization_id,user_id,role_id) VALUES($1,$2,$3,$4)`, []any{membership, orgA, user, role}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(NewRepository(pool))
	actor := access.Actor{UserID: user, OrganizationID: orgA, Active: true, Permissions: map[access.Permission]struct{}{}}
	if _, err := service.Update(ctx, actor, "Renamed", AuditContext{}); !errors.Is(err, access.ErrPermissionDenied) {
		t.Fatalf("update without permission error=%v", err)
	}
	actor.Permissions[access.OrganizationUpdate] = struct{}{}
	updated, err := service.Update(ctx, actor, "Новая школа", AuditContext{})
	if err != nil || updated.Name != "Новая школа" {
		t.Fatalf("Update()=%#v,%v", updated, err)
	}
	var otherName string
	if err = pool.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1`, orgB).Scan(&otherName); err != nil || otherName != "School B" {
		t.Fatalf("other tenant changed: %q,%v", otherName, err)
	}
}
