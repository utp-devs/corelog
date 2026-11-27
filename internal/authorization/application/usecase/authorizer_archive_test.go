package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// archivedAccessOf returns a project context for the given explicit role, archived or not.
func archivedAccessOf(role *domain.ProjectRole, archived bool) *fakeAccess {
	return &fakeAccess{access: domain.Access{
		OrgRole:     domain.OrgRoleNone,
		Visibility:  domain.VisibilityOpen,
		ProjectRole: role,
		Archived:    archived,
	}}
}

func rolePtr(r domain.ProjectRole) *domain.ProjectRole {
	return &r
}

func isProjectArchived(err error) bool {
	return apperror.CodeOf(err) == domain.CodeProjectArchived && apperror.KindOf(err) == apperror.KindConflict
}

// TestArchivedBlocksEveryWriteAction runs every action of the port against an archived project as a
// lead, who holds every write permission. Reads and the archive-allowed actions must pass; every
// other action must fail with the archived conflict.
func TestArchivedBlocksEveryWriteAction(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(rolePtr(domain.ProjectRoleLead), true))
	for _, action := range port.Actions {
		err := authorizer.Authorize(ctx, action, uuid.New(), "user", uuid.New())
		if domain.ArchivedActionAllowed(string(action)) {
			if err != nil {
				t.Errorf("%s on archived project: expected allowed, got %v", action, err)
			}
			continue
		}
		if !isProjectArchived(err) {
			t.Errorf("%s on archived project: expected project.archived conflict, got %v", action, err)
		}
	}
}

// TestArchivedAllowsReadsAndLifecycleForLead checks the explicit allow-list by action name: reads,
// restore (edit_project through restore_project), archive and delete.
func TestArchivedAllowsReadsAndLifecycleForLead(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(rolePtr(domain.ProjectRoleLead), true))
	for _, action := range []port.Action{port.ActionViewProject, port.ActionRestoreProject, port.ActionArchiveProject, port.ActionDeleteProject} {
		if err := authorizer.Authorize(ctx, action, uuid.New(), "user", uuid.New()); err != nil {
			t.Errorf("lead on archived project, %s: expected allowed, got %v", action, err)
		}
	}
	if err := authorizer.Authorize(ctx, port.ActionEditProject, uuid.New(), "user", uuid.New()); !isProjectArchived(err) {
		t.Errorf("lead on archived project, edit_project: expected conflict, got %v", err)
	}
}

// TestArchivedCanAndRequireAgreeWithAuthorize checks the permission-level entry points: Can reports
// false for a blocked write, Require returns the conflict, and reads keep working.
func TestArchivedCanAndRequireAgreeWithAuthorize(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(rolePtr(domain.ProjectRoleLead), true))

	ok, err := authorizer.Can(ctx, uuid.New(), "user", uuid.New(), domain.PermissionEditProject)
	if err != nil || ok {
		t.Errorf("Can(edit_project) on archived = %v, %v; want false, nil", ok, err)
	}
	if err := authorizer.Require(ctx, uuid.New(), "user", uuid.New(), domain.PermissionEditProject); !isProjectArchived(err) {
		t.Errorf("Require(edit_project) on archived = %v; want project.archived", err)
	}
	ok, err = authorizer.Can(ctx, uuid.New(), "user", uuid.New(), domain.PermissionBrowseProject)
	if err != nil || !ok {
		t.Errorf("Can(browse_project) on archived = %v, %v; want true, nil", ok, err)
	}
}

// TestArchivedDoesNotLeakToNonMembers checks that a caller without the permission receives forbidden,
// not the archive conflict, so the archive state is only revealed to callers who could write.
func TestArchivedDoesNotLeakToNonMembers(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(nil, true))
	err := authorizer.Authorize(ctx, port.ActionCreateIssue, uuid.New(), "user", uuid.New())
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("non-member create_issue on archived project: expected forbidden, got %v", err)
	}
	err = authorizer.Require(ctx, uuid.New(), "user", uuid.New(), domain.PermissionEditProject)
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("non-member Require(edit_project) on archived project: expected forbidden, got %v", err)
	}
}

// TestArchivedPlatformAdminStaysReadOnly checks that the platform admin viewer role keeps reading an
// archived project and is still denied writes by the permission rules.
func TestArchivedPlatformAdminStaysReadOnly(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(nil, true))
	if err := authorizer.Authorize(ctx, port.ActionViewProject, uuid.New(), "admin", uuid.New()); err != nil {
		t.Errorf("platform admin view on archived project: %v", err)
	}
	if err := authorizer.Authorize(ctx, port.ActionCreateIssue, uuid.New(), "admin", uuid.New()); apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("platform admin create_issue on archived project: expected forbidden, got %v", err)
	}
}

// TestArchivedEffectiveListsOnlyReadsAndLifecycle checks the effective permission list of an archived
// project: reads and delete, never the other writes.
func TestArchivedEffectiveListsOnlyReadsAndLifecycle(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(rolePtr(domain.ProjectRoleLead), true))
	_, granted, err := authorizer.Effective(ctx, uuid.New(), "user", uuid.New())
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if len(granted) == 0 {
		t.Fatal("Effective returned no permissions for the lead of an archived project")
	}
	for _, p := range granted {
		if !domain.ArchivedAllowed("", p) {
			t.Errorf("Effective lists %s on an archived project", p)
		}
	}
}

// TestActiveProjectUnaffected checks that the same lead keeps every action on an active project.
func TestActiveProjectUnaffected(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(archivedAccessOf(rolePtr(domain.ProjectRoleLead), false))
	for _, action := range port.Actions {
		if err := authorizer.Authorize(ctx, action, uuid.New(), "user", uuid.New()); err != nil {
			t.Errorf("lead on active project, %s: %v", action, err)
		}
	}
}
