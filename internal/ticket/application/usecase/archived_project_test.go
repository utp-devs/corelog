package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

// archivedAccessLookup serves one fixed project context to the real authorizer.
type archivedAccessLookup struct {
	access authdomain.Access
}

func (l archivedAccessLookup) FindRole(context.Context, uuid.UUID, uuid.UUID) (*authdomain.ProjectRole, error) {
	return l.access.ProjectRole, nil
}

func (l archivedAccessLookup) FindAccess(context.Context, uuid.UUID, uuid.UUID) (authdomain.Access, error) {
	return l.access, nil
}

// archiveGate is the project role resolver and authorizer of the ticket service, backed by the real
// authorization use case so the archive rules are exercised end to end.
type archiveGate struct {
	authorizer *authusecase.Authorizer
	role       *string
}

func (g archiveGate) FindRole(context.Context, uuid.UUID, uuid.UUID) (*string, error) {
	return g.role, nil
}

func (g archiveGate) ListProjectIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (g archiveGate) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return g.authorizer.Authorize(ctx, authport.Action(action), actorID, globalRole, projectID)
}

// fakeCollaboration stores comments and activities in memory.
type fakeCollaboration struct {
	comments map[uuid.UUID]*domain.Comment
}

func (f *fakeCollaboration) FindComment(_ context.Context, id uuid.UUID) (*domain.Comment, error) {
	return f.comments[id], nil
}

func (f *fakeCollaboration) SaveComment(_ context.Context, c *domain.Comment) error {
	if f.comments == nil {
		f.comments = map[uuid.UUID]*domain.Comment{}
	}
	c.ID = uuid.New()
	f.comments[c.ID] = c
	return nil
}

func (f *fakeCollaboration) FindComments(context.Context, uuid.UUID) ([]*domain.Comment, error) {
	return nil, nil
}

func (f *fakeCollaboration) UpdateComment(context.Context, *domain.Comment) error { return nil }

func (f *fakeCollaboration) DeleteComment(context.Context, *domain.Comment) error { return nil }

func (f *fakeCollaboration) SaveActivity(context.Context, *domain.Activity) error { return nil }

func (f *fakeCollaboration) FindActivities(context.Context, uuid.UUID) ([]*domain.Activity, error) {
	return nil, nil
}

func (f *fakeCollaboration) FindMentionIDs(context.Context, uuid.UUID, []string) ([]uuid.UUID, error) {
	return nil, nil
}

// TestTicketServiceArchivedProjectIsReadOnly covers an issue create and a comment on an archived
// project (409 project.archived), reads that keep working, and success once the project is restored.
func TestTicketServiceArchivedProjectIsReadOnly(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	lead := authdomain.ProjectRoleLead
	leadRole := "lead"
	actor := uuid.New()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	collab := &fakeCollaboration{}

	active := archiveGate{authorizer: authusecase.NewAuthorizer(archivedAccessLookup{access: authdomain.Access{ProjectRole: &lead, OrgRole: authdomain.OrgRoleNone, Visibility: authdomain.VisibilityOpen}}), role: &leadRole}
	seed, err := NewTicketService(repo, dir, active, collab).Create(ctx, port.CreateInput{
		Title: "Before archive", Description: "Desc", Priority: domain.PriorityMedium, RequesterID: actor, ProjectID: &projectID,
	})
	if err != nil {
		t.Fatalf("create on active project: %v", err)
	}

	archivedAccess := authdomain.Access{ProjectRole: &lead, OrgRole: authdomain.OrgRoleNone, Visibility: authdomain.VisibilityOpen, Archived: true}
	archived := archiveGate{authorizer: authusecase.NewAuthorizer(archivedAccessLookup{access: archivedAccess}), role: &leadRole}
	svc := NewTicketService(repo, dir, archived, collab)

	_, err = svc.Create(ctx, port.CreateInput{Title: "New", Description: "Desc", Priority: domain.PriorityMedium, RequesterID: actor, ProjectID: &projectID})
	assertArchived(t, "create issue", err)

	_, err = svc.CreateComment(ctx, port.CreateCommentInput{IssueID: seed.ID, AuthorID: actor, Body: "Hola", GlobalRole: "user"})
	assertArchived(t, "create comment", err)

	if _, err := svc.Get(ctx, seed.ID, actor, "user"); err != nil {
		t.Fatalf("read issue on archived project: %v", err)
	}

	restored := archiveGate{authorizer: authusecase.NewAuthorizer(archivedAccessLookup{access: authdomain.Access{ProjectRole: &lead, OrgRole: authdomain.OrgRoleNone, Visibility: authdomain.VisibilityOpen}}), role: &leadRole}
	after := NewTicketService(repo, dir, restored, collab)
	if _, err := after.Create(ctx, port.CreateInput{Title: "After restore", Description: "Desc", Priority: domain.PriorityMedium, RequesterID: actor, ProjectID: &projectID}); err != nil {
		t.Fatalf("create after restore: %v", err)
	}
	if _, err := after.CreateComment(ctx, port.CreateCommentInput{IssueID: seed.ID, AuthorID: actor, Body: "Hola", GlobalRole: "user"}); err != nil {
		t.Fatalf("comment after restore: %v", err)
	}
}

// assertArchived checks the 409 project.archived contract of the archive rules.
func assertArchived(t *testing.T, step string, err error) {
	t.Helper()
	if apperror.CodeOf(err) != "project.archived" || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("%s on archived project: want 409 project.archived, got %v", step, err)
	}
}
