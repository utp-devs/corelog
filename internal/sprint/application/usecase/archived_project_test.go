package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// archivedAccessLookup serves one fixed project context to the real authorization use case.
type archivedAccessLookup struct {
	access authdomain.Access
}

func (l archivedAccessLookup) FindRole(context.Context, uuid.UUID, uuid.UUID) (*authdomain.ProjectRole, error) {
	return l.access.ProjectRole, nil
}

func (l archivedAccessLookup) FindAccess(context.Context, uuid.UUID, uuid.UUID) (authdomain.Access, error) {
	return l.access, nil
}

// realAuthorizer adapts the authorization use case to the sprint authorizer port.
type realAuthorizer struct {
	authorizer *authusecase.Authorizer
}

func (r realAuthorizer) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return r.authorizer.Authorize(ctx, authport.Action(action), actorID, globalRole, projectID)
}

func leadOnProject(archived bool) realAuthorizer {
	lead := authdomain.ProjectRoleLead
	return realAuthorizer{authorizer: authusecase.NewAuthorizer(archivedAccessLookup{access: authdomain.Access{
		ProjectRole: &lead,
		OrgRole:     authdomain.OrgRoleNone,
		Visibility:  authdomain.VisibilityOpen,
		Archived:    archived,
	}})}
}

// TestSprintCreateOnArchivedProject checks that a sprint create on an archived project fails with
// 409 project.archived and stores nothing, while listing keeps working and creation works after restore.
func TestSprintCreateOnArchivedProject(t *testing.T) {
	ctx := context.Background()
	actor, project := uuid.New(), uuid.New()
	repo := newMemoryRepo()

	archived := NewSprintService(repo, leadOnProject(true))
	_, err := archived.Create(ctx, actor, "user", project, "Sprint 1", "Goal", nil, nil)
	if apperror.CodeOf(err) != "project.archived" || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("create sprint on archived project: want 409 project.archived, got %v", err)
	}
	if list, err := archived.List(ctx, actor, "user", project); err != nil || len(list) != 0 {
		t.Fatalf("list sprints on archived project = %d, %v; want 0, nil", len(list), err)
	}

	restored := NewSprintService(repo, leadOnProject(false))
	if _, err := restored.Create(ctx, actor, "user", project, "Sprint 1", "Goal", nil, nil); err != nil {
		t.Fatalf("create sprint after restore: %v", err)
	}
}
