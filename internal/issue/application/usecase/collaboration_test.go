package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
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
