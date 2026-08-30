// Package access provides centralized, tenant-aware authorization decisions.
package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrPermissionDenied is returned when an actor cannot perform an action.
var ErrPermissionDenied = errors.New("permission denied")

// Permission identifies one business capability independent of role names.
type Permission string

// Supported organization permissions.
const (
	StudentsList   Permission = "students.list"
	StudentsView   Permission = "students.view"
	StudentsCreate Permission = "students.create"
	StudentsUpdate Permission = "students.update"

	DocumentsList     Permission = "documents.list"
	DocumentsView     Permission = "documents.view"
	DocumentsDownload Permission = "documents.download"
	DocumentsUpload   Permission = "documents.upload"
	DocumentsEdit     Permission = "documents.edit"
	DocumentsDelete   Permission = "documents.delete"

	AccessView   Permission = "access.view"
	AccessManage Permission = "access.manage"
	AuditView    Permission = "audit.view"
	UsersView    Permission = "users.view"
	UsersCreate  Permission = "users.create"
	UsersInvite  Permission = "users.invite"
	UsersManage  Permission = "users.manage"

	OrganizationUpdate Permission = "organization.update"
)

// StudentGrant narrows an actor's capabilities to an assigned student.
type StudentGrant string

// Supported additive student grants.
const (
	StudentView     StudentGrant = "view"
	StudentUpload   StudentGrant = "upload"
	StudentEdit     StudentGrant = "edit"
	StudentDownload StudentGrant = "download"
)

// Actor contains the effective permissions and scope of one active membership.
type Actor struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Active         bool
	Permissions    map[Permission]struct{}
	AllStudents    bool
	StudentGrants  map[uuid.UUID]map[StudentGrant]struct{}
}

// Resource identifies the tenant and optional student owning a protected resource.
type Resource struct {
	OrganizationID uuid.UUID
	StudentID      *uuid.UUID
}

// AuthorizationService evaluates effective permissions without role-name branching.
type AuthorizationService struct{}

// Can returns nil only when the actor may perform action on resource.
func (AuthorizationService) Can(ctx context.Context, actor Actor, action Permission, resource Resource) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !actor.Active || actor.OrganizationID == uuid.Nil || resource.OrganizationID != actor.OrganizationID {
		return ErrPermissionDenied
	}
	if _, allowed := actor.Permissions[action]; !allowed {
		return ErrPermissionDenied
	}
	if resource.StudentID == nil {
		if isOrganizationScoped(action) {
			return nil
		}
		return ErrPermissionDenied
	}
	if actor.AllStudents {
		return nil
	}

	required, studentScoped := requiredStudentGrant(action)
	if !studentScoped {
		return ErrPermissionDenied
	}
	grants := actor.StudentGrants[*resource.StudentID]
	if _, allowed := grants[required]; !allowed {
		return ErrPermissionDenied
	}
	return nil
}

func isOrganizationScoped(permission Permission) bool {
	switch permission {
	case StudentsList, StudentsCreate, AccessView, AccessManage, AuditView, UsersView, UsersCreate, UsersInvite, UsersManage, OrganizationUpdate:
		return true
	case StudentsView, StudentsUpdate, DocumentsList, DocumentsView, DocumentsDownload, DocumentsUpload, DocumentsEdit, DocumentsDelete:
		return false
	default:
		return false
	}
}

func requiredStudentGrant(permission Permission) (StudentGrant, bool) {
	switch permission {
	case StudentsView, DocumentsList, DocumentsView:
		return StudentView, true
	case DocumentsUpload:
		return StudentUpload, true
	case StudentsUpdate, DocumentsEdit:
		return StudentEdit, true
	case DocumentsDownload:
		return StudentDownload, true
	case StudentsList, StudentsCreate, DocumentsDelete, AccessView, AccessManage, AuditView, UsersView, UsersCreate, UsersInvite, UsersManage, OrganizationUpdate:
		return "", false
	default:
		return "", false
	}
}
