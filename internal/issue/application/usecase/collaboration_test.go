package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// memCollab guarda comentarios, actividad y menciones en memoria.
type memCollab struct {
	comments   []*domain.Comment
	activities []*domain.Activity
	mentionIDs []uuid.UUID
}

func (m *memCollab) FindComment(_ context.Context, id uuid.UUID) (*domain.Comment, error) {
	for _, c := range m.comments {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, errors.New("comment not found")
}

func (m *memCollab) SaveComment(_ context.Context, c *domain.Comment) error {
	c.ID = uuid.New()
	m.comments = append(m.comments, c)
	return nil
}

func (m *memCollab) FindComments(_ context.Context, issueID uuid.UUID) ([]*domain.Comment, error) {
	var out []*domain.Comment
	for _, c := range m.comments {
		if c.IssueID == issueID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memCollab) UpdateComment(context.Context, *domain.Comment) error { return nil }
func (m *memCollab) DeleteComment(context.Context, *domain.Comment) error { return nil }

func (m *memCollab) SaveActivity(_ context.Context, a *domain.Activity) error {
	m.activities = append(m.activities, a)
	return nil
}

func (m *memCollab) FindActivities(context.Context, uuid.UUID) ([]*domain.Activity, error) {
	return m.activities, nil
}

func (m *memCollab) FindMentionIDs(_ context.Context, _ uuid.UUID, names []string) ([]uuid.UUID, error) {
	if len(names) == 0 {
		return nil, nil
	}
	return m.mentionIDs, nil
}

func TestIssueServiceCommentsAndMentions(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	collab := &memCollab{mentionIDs: []uuid.UUID{uuid.New()}}
	svc := NewIssueService(repo, newFakeUserDirectory(), collab, contributorRoleResolver())
	issue, _ := svc.Create(ctx, port.CreateInput{Summary: "Comentarios", Description: "Desc", Priority: domain.PriorityLow, ReporterID: uuid.New()})
	author := uuid.New()

	if _, err := svc.CreateComment(ctx, port.CreateCommentInput{IssueID: issue.ID, AuthorID: author, GlobalRole: "agent", Body: "  "}); !errors.Is(err, domain.ErrInvalidComment) {
		t.Fatalf("empty comment: got %v", err)
	}
	created, err := svc.CreateComment(ctx, port.CreateCommentInput{IssueID: issue.ID, AuthorID: author, GlobalRole: "agent", Body: "Revisar con @ana y @luis"})
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if len(created.Mentions) != 1 {
		t.Fatalf("mentions = %v, want the resolved ids", created.Mentions)
	}

	listed, err := svc.ListComments(ctx, issue.ID, author, "agent")
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListComments = %d, %v; want 1", len(listed), err)
	}
	outsiders := NewIssueService(repo, newFakeUserDirectory(), collab, fakeProjectRoleResolver{})
	if _, err := outsiders.ListComments(ctx, issue.ID, uuid.New(), "requester"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non member lists comments: got %v", err)
	}

}

func TestIssueServiceUserNamesResolvesOnlyRequestedPeople(t *testing.T) {
	ctx := context.Background()
	dir := newFakeUserDirectory()
	ana, luis := uuid.New(), uuid.New()
	dir.names[ana] = "Ana Torres"
	dir.names[luis] = "Luis Paredes"
	svc := NewIssueService(newFakeIssueRepo(), dir, contributorRoleResolver())

	names, err := svc.UserNames(ctx, []uuid.UUID{ana})
	if err != nil || names[ana] != "Ana Torres" || len(names) != 1 {
		t.Fatalf("UserNames = %v, %v; want only Ana", names, err)
	}
	empty, err := svc.UserNames(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("UserNames without ids = %v, %v", empty, err)
	}
}
