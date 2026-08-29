package user

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/testdatabase"
)

func TestCreateUserIsTenantScopedAndCanLogin(t *testing.T) {
	pool := testdatabase.Start(t)
	ctx := context.Background()
	orgA, _ := uuid.NewV7()
	orgB, _ := uuid.NewV7()
	actorID, _ := uuid.NewV7()
	actorRoleID, _ := uuid.NewV7()
	specialistRoleID, _ := uuid.NewV7()
	membershipID, _ := uuid.NewV7()
	_, err := pool.Exec(ctx, `INSERT INTO organizations(id,name) VALUES($1,'School A'),($2,'School B')`, orgA, orgB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO roles(id,organization_id,role_key,name,default_all_students) VALUES
			($1,$2,'manager','Manager',true),($3,$2,'specialist','Специалист',false)
	`, actorRoleID, orgA, specialistRoleID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_code) VALUES($1,'users.create'),($1,'users.view')`, actorRoleID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,display_name,password_hash) VALUES($1,'manager@test.local','Manager','$argon2id$placeholder')`, actorID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO memberships(id,organization_id,user_id,role_id,all_students) VALUES($1,$2,$3,$4,true)`, membershipID, orgA, actorID, actorRoleID)
	if err != nil {
		t.Fatal(err)
	}
	actor := access.Actor{UserID: actorID, OrganizationID: orgA, Active: true, Permissions: map[access.Permission]struct{}{
		access.UsersManage: {},
	}}
	service := NewService(NewRepository(pool))
	created, err := service.Create(ctx, actor, CreateInput{
		LastName: "Петрова", FirstName: "Анна", Email: "anna@test.local", RoleKey: "specialist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.InitialPassword == "" || created.Status != "active" {
		t.Fatalf("unexpected created account: %#v", created)
	}
	account, err := auth.NewService(auth.NewRepository(pool), time.Hour).Login(ctx, created.Email, created.InitialPassword)
	if err != nil {
		t.Fatalf("new account cannot login: %v", err)
	}
	if account.Principal.Actor.OrganizationID != orgA {
		t.Fatalf("wrong tenant: %s", account.Principal.Actor.OrganizationID)
	}
	otherUsers, err := NewRepository(pool).List(ctx, orgB)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherUsers) != 0 {
		t.Fatalf("organization B sees organization A users: %#v", otherUsers)
	}
}
