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

// memLinks guarda enlaces entre incidencias.
type memLinks struct {
	links []*domain.IssueLink
}

func (m *memLinks) Save(_ context.Context, l *domain.IssueLink) error {
	l.ID = uuid.New()
	m.links = append(m.links, l)
	return nil
}

func (m *memLinks) ListByIssue(_ context.Context, issueID uuid.UUID) ([]*domain.IssueLink, error) {
	var out []*domain.IssueLink
	for _, l := range m.links {
		if l.SourceIssueID == issueID || l.TargetIssueID == issueID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (m *memLinks) Delete(_ context.Context, issueID, linkID uuid.UUID) error {
	for i, l := range m.links {
		if l.ID == linkID && l.SourceIssueID == issueID {
			m.links = append(m.links[:i], m.links[i+1:]...)
			return nil
		}
	}
	return errors.New("link not found")
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

func TestIssueServiceActivitiesAndLinks(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	links := &memLinks{}
	svc := NewIssueService(repo, newFakeUserDirectory(), links, contributorRoleResolver())
	source, _ := svc.Create(ctx, port.CreateInput{Summary: "Origen", Description: "Desc", Priority: domain.PriorityLow, ReporterID: uuid.New()})
	target, _ := svc.Create(ctx, port.CreateInput{Summary: "Destino", Description: "Desc", Priority: domain.PriorityLow, ReporterID: uuid.New()})
	actor := uuid.New()

	link, err := svc.CreateIssueLink(ctx, source.ID, target.ID, actor, "agent", domain.IssueLinkBlocks)
	if err != nil {
		t.Fatalf("CreateIssueLink: %v", err)
	}
	listed, err := svc.ListIssueLinks(ctx, source.ID, actor, "agent")
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListIssueLinks = %d, %v; want 1", len(listed), err)
	}
	if err := svc.DeleteIssueLink(ctx, source.ID, link.ID, actor, "agent"); err != nil {
		t.Fatalf("DeleteIssueLink: %v", err)
	}
	if err := svc.DeleteIssueLink(ctx, source.ID, link.ID, actor, "agent"); err == nil {
		t.Fatal("deleting a link twice must fail")
	}

	if _, err := svc.CreateIssueLink(ctx, source.ID, source.ID, actor, "agent", "unknown-kind"); err == nil {
		t.Fatal("an invalid link kind must be rejected")
	}
	denied := NewIssueService(repo, newFakeUserDirectory(), links, fakeProjectRoleResolver{})
	if _, err := denied.CreateIssueLink(ctx, source.ID, target.ID, actor, "requester", domain.IssueLinkRelates); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non member links issues: got %v", err)
	}
	if _, err := svc.ListActivities(ctx, source.ID, actor, "agent"); err != nil {
		t.Fatalf("ListActivities: %v", err)
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
