package studentaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/platform/testdatabase"
	"opora.local/api/internal/student"
)

func TestManageAccessIsTenantScoped(t *testing.T) {
	pool := testdatabase.Start(t)
	ctx := context.Background()
	orgA, _ := uuid.NewV7()
	orgB, _ := uuid.NewV7()
	roleA, _ := uuid.NewV7()
	roleB, _ := uuid.NewV7()
	actorID, _ := uuid.NewV7()
	targetID, _ := uuid.NewV7()
	foreignID, _ := uuid.NewV7()
	studentA, _ := uuid.NewV7()
	studentB, _ := uuid.NewV7()
	membershipA, _ := uuid.NewV7()
	membershipTarget, _ := uuid.NewV7()
	membershipForeign, _ := uuid.NewV7()
	_, err := pool.Exec(ctx, `INSERT INTO organizations(id,name) VALUES($1,'School A'),($2,'School B')`, orgA, orgB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO roles(id,organization_id,role_key,name) VALUES($1,$2,'manager','Manager'),($3,$4,'manager','Manager')`, roleA, orgA, roleB, orgB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users(id,email,display_name) VALUES
			($1,'actor@test.local','Actor'),($2,'target@test.local','Target'),($3,'foreign@test.local','Foreign')
	`, actorID, targetID, foreignID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO memberships(id,organization_id,user_id,role_id) VALUES
			($1,$2,$3,$4),($5,$2,$6,$4),($7,$8,$9,$10)
	`, membershipA, orgA, actorID, roleA, membershipTarget, targetID, membershipForeign, orgB, foreignID, roleB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO students(id,organization_id,last_name,first_name) VALUES($1,$2,'Иванов','Иван'),($3,$4,'Петров','Пётр')`, studentA, orgA, studentB, orgB)
	if err != nil {
		t.Fatal(err)
	}
	actor := access.Actor{UserID: actorID, OrganizationID: orgA, Active: true, Permissions: map[access.Permission]struct{}{
		access.AccessView: {}, access.AccessManage: {},
	}}
	service := NewService(NewRepository(pool), student.NewRepository(pool))
	if err := service.Set(ctx, actor, studentA, targetID, []string{"view", "download"}); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(ctx, actor, studentA)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].UserID != targetID || len(items[0].Grants) != 2 {
		t.Fatalf("unexpected grants: %#v", items)
	}
	if err := service.Set(ctx, actor, studentB, targetID, []string{"view"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign student accepted: %v", err)
	}
	if err := service.Set(ctx, actor, studentA, foreignID, []string{"view"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign membership accepted: %v", err)
	}
}
