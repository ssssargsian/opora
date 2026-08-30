package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/testdatabase"
)

type recordingMailer struct{ message InvitationMessage }

func (m *recordingMailer) SendInvitation(_ context.Context, message InvitationMessage) error {
	m.message = message
	return nil
}

func TestCreateUserIsTenantScopedAndCanAcceptInvitation(t *testing.T) {
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
	mailer := &recordingMailer{}
	service := NewService(NewRepository(pool), WithInvitationMailer(mailer))
	created, err := service.Create(ctx, actor, CreateInput{
		LastName: "Петрова", FirstName: "Анна", Email: "anna@test.local", RoleKey: "specialist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "invited" || mailer.message.Token == "" {
		t.Fatalf("unexpected created account: %#v", created)
	}
	if created.InvitationCreatedAt == nil || created.InvitationAcceptedAt != nil {
		t.Fatalf("unexpected invitation status timestamps: %#v", created)
	}
	var storedHash []byte
	if err = pool.QueryRow(ctx, `SELECT token_hash FROM user_invitations WHERE organization_id=$1 AND user_id=$2`, orgA, created.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if string(storedHash) == mailer.message.Token {
		t.Fatal("invitation token was stored in plaintext")
	}
	firstToken := mailer.message.Token
	if _, err = service.ResendInvitation(ctx, actor, created.ID); err != nil {
		t.Fatalf("ResendInvitation() error=%v", err)
	}
	if mailer.message.Token == firstToken {
		t.Fatal("resend reused invitation token")
	}
	if _, err = service.AcceptInvitation(ctx, AcceptInvitationInput{Token: firstToken, Password: "unused-secure-password"}); !errors.Is(err, ErrUsedInvitation) {
		t.Fatalf("old invitation after resend error=%v", err)
	}
	const password = "new-secure-password-123"
	accepted, err := service.AcceptInvitation(ctx, AcceptInvitationInput{Token: mailer.message.Token, Password: password})
	if err != nil || accepted.Email != created.Email {
		t.Fatalf("AcceptInvitation() = %#v, %v", accepted, err)
	}
	account, err := auth.NewService(auth.NewRepository(pool), time.Hour).Login(ctx, created.Email, password)
	if err != nil {
		t.Fatalf("new account cannot login: %v", err)
	}
	if account.Principal.Actor.OrganizationID != orgA {
		t.Fatalf("wrong tenant: %s", account.Principal.Actor.OrganizationID)
	}
	if _, err = service.AcceptInvitation(ctx, AcceptInvitationInput{Token: mailer.message.Token, Password: password}); !errors.Is(err, ErrUsedInvitation) {
		t.Fatalf("accepted invitation replay error=%v", err)
	}
	profile, err := service.UpdateProfile(ctx, actorForUser(account.Principal.Actor), ProfileInput{LastName: "Петрова", FirstName: "Анна", Email: "anna.updated@test.local"}, RequestAudit{})
	if err != nil || profile.Email != "anna.updated@test.local" {
		t.Fatalf("UpdateProfile()=%#v,%v", profile, err)
	}
	if _, err = service.UpdateProfile(ctx, actorForUser(account.Principal.Actor), ProfileInput{LastName: "Петрова", FirstName: "Анна", Email: "manager@test.local"}, RequestAudit{}); !errors.Is(err, ErrEmailConflict) {
		t.Fatalf("duplicate profile email error=%v", err)
	}
	if err = service.ChangePassword(ctx, actorForUser(account.Principal.Actor), account.Principal.Session.ID, PasswordInput{CurrentPassword: "wrong", NewPassword: "replacement-password-123"}, RequestAudit{}); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong current password error=%v", err)
	}
	if err = service.ChangePassword(ctx, actorForUser(account.Principal.Actor), account.Principal.Session.ID, PasswordInput{CurrentPassword: password, NewPassword: "replacement-password-123"}, RequestAudit{}); err != nil {
		t.Fatalf("ChangePassword() error=%v", err)
	}
	if _, err = auth.NewService(auth.NewRepository(pool), time.Hour).Login(ctx, profile.Email, "replacement-password-123"); err != nil {
		t.Fatalf("new password cannot login: %v", err)
	}
	expired, err := service.Create(ctx, actor, CreateInput{LastName: "Сидоров", FirstName: "Семён", Email: "expired@test.local", RoleKey: "specialist"})
	if err != nil {
		t.Fatal(err)
	}
	expiredToken := mailer.message.Token
	if _, err = pool.Exec(ctx, `UPDATE user_invitations SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 minute' WHERE organization_id=$1 AND user_id=$2`, orgA, expired.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AcceptInvitation(ctx, AcceptInvitationInput{Token: expiredToken, Password: password}); !errors.Is(err, ErrExpiredInvitation) {
		t.Fatalf("expired invitation error=%v", err)
	}
	update := CreateInput{LastName: "Сидорова", FirstName: "Семёна", Email: "expired.updated@test.local", RoleKey: "manager"}
	withoutPermission := actor
	withoutPermission.Permissions = map[access.Permission]struct{}{}
	if _, err = service.Update(ctx, withoutPermission, expired.ID, update); !errors.Is(err, access.ErrPermissionDenied) {
		t.Fatalf("update without users.manage error=%v", err)
	}
	otherTenantActor := actor
	otherTenantActor.OrganizationID = orgB
	if _, err = service.Update(ctx, otherTenantActor, expired.ID, update); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant update error=%v", err)
	}
	updatedUser, err := service.Update(ctx, actor, expired.ID, update)
	if err != nil || updatedUser.Email != update.Email || updatedUser.RoleKey != update.RoleKey || updatedUser.Status != "invited" {
		t.Fatalf("Update()=%#v,%v", updatedUser, err)
	}
	var allStudents bool
	if err = pool.QueryRow(ctx, `SELECT all_students FROM memberships WHERE organization_id=$1 AND user_id=$2`, orgA, expired.ID).Scan(&allStudents); err != nil || !allStudents {
		t.Fatalf("updated role scope all_students=%v,error=%v", allStudents, err)
	}
	otherUsers, err := NewRepository(pool).List(ctx, orgB)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherUsers) != 0 {
		t.Fatalf("organization B sees organization A users: %#v", otherUsers)
	}
}

func actorForUser(actor access.Actor) access.Actor {
	actor.Active = true
	return actor
}
