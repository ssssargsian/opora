package access

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorizationTenantIsolation(t *testing.T) {
	actor := actorWith(StudentsView, StudentView)
	resource := Resource{OrganizationID: uuid.New(), StudentID: pointer(uuid.New())}
	if err := (AuthorizationService{}).Can(context.Background(), actor, StudentsView, resource); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Can() error = %v, want permission denied", err)
	}
}

func TestAuthorizationViewDoesNotAllowEdit(t *testing.T) {
	actor := actorWith(DocumentsView, StudentView)
	studentID := firstStudent(actor)
	resource := Resource{OrganizationID: actor.OrganizationID, StudentID: &studentID}
	if err := (AuthorizationService{}).Can(context.Background(), actor, DocumentsEdit, resource); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Can() error = %v, want permission denied", err)
	}
}

func TestAuthorizationRequiresStudentAssignment(t *testing.T) {
	actor := actorWith(StudentsView, StudentView)
	unassignedStudent := uuid.New()
	resource := Resource{OrganizationID: actor.OrganizationID, StudentID: &unassignedStudent}
	if err := (AuthorizationService{}).Can(context.Background(), actor, StudentsView, resource); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Can() error = %v, want permission denied", err)
	}
}

func TestAuthorizationAllowsAdditiveStudentGrant(t *testing.T) {
	actor := actorWith(DocumentsDownload, StudentDownload)
	studentID := firstStudent(actor)
	resource := Resource{OrganizationID: actor.OrganizationID, StudentID: &studentID}
	if err := (AuthorizationService{}).Can(context.Background(), actor, DocumentsDownload, resource); err != nil {
		t.Fatalf("Can() error = %v, want nil", err)
	}
}

func TestAuthorizationRejectsInactiveActor(t *testing.T) {
	actor := actorWith(StudentsView, StudentView)
	actor.Active = false
	studentID := firstStudent(actor)
	resource := Resource{OrganizationID: actor.OrganizationID, StudentID: &studentID}
	if err := (AuthorizationService{}).Can(context.Background(), actor, StudentsView, resource); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Can() error = %v, want permission denied", err)
	}
}

func TestAuthorizationRequiresStudentContextForDocument(t *testing.T) {
	actor := actorWith(DocumentsView, StudentView)
	resource := Resource{OrganizationID: actor.OrganizationID}
	if err := (AuthorizationService{}).Can(context.Background(), actor, DocumentsView, resource); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Can() error = %v, want permission denied", err)
	}
}

func TestRestrictedActorCannotDeleteDocumentWithoutExplicitGrantModel(t *testing.T) {
	actor := actorWith(DocumentsDelete, StudentEdit)
	studentID := firstStudent(actor)
	resource := Resource{OrganizationID: actor.OrganizationID, StudentID: &studentID}
	if err := (AuthorizationService{}).Can(context.Background(), actor, DocumentsDelete, resource); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Can() error = %v, want permission denied", err)
	}
}

func actorWith(permission Permission, grant StudentGrant) Actor {
	studentID := uuid.New()
	return Actor{
		UserID: uuid.New(), OrganizationID: uuid.New(), Active: true,
		Permissions:   map[Permission]struct{}{permission: {}},
		StudentGrants: map[uuid.UUID]map[StudentGrant]struct{}{studentID: {grant: {}}},
	}
}

func firstStudent(actor Actor) uuid.UUID {
	for id := range actor.StudentGrants {
		return id
	}
	return uuid.Nil
}

func pointer(value uuid.UUID) *uuid.UUID { return &value }
