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

// realAuthorizer adapts the authorization use case to the board authorizer port.
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

// TestBoardMoveCardOnArchivedProject checks that a board move on an archived project fails with
// 409 project.archived before any issue moves, and that the same move succeeds after restore.
func TestBoardMoveCardOnArchivedProject(t *testing.T) {
	ctx := context.Background()
	actor, project := uuid.New(), uuid.New()

	f := newKanbanFixture()
	mover := &fakeMover{}
	archived := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, leadOnProject(true), mover)
	err := archived.MoveCard(ctx, project, actor, "user", f.cardInProgress, f.openColumn)
	if apperror.CodeOf(err) != "project.archived" || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("move card on archived project: want 409 project.archived, got %v", err)
	}
	if mover.calls != 0 {
		t.Fatalf("archived move reached the issue mover %d times", mover.calls)
	}

	if _, err := archived.Get(ctx, project, actor, "user"); err != nil {
		t.Fatalf("read board on archived project: %v", err)
	}

	restored := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, leadOnProject(false), mover)
	if err := restored.MoveCard(ctx, project, actor, "user", f.cardInProgress, f.openColumn); err != nil {
		t.Fatalf("move card after restore: %v", err)
	}
	if mover.calls != 1 {
		t.Fatalf("move after restore: mover calls = %d, want 1", mover.calls)
	}
}
