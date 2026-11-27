package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// candidatesRepo returns a fixed list of key candidates and ignores everything else about the query.
type candidatesRepo struct {
	*fakeIssueRepo
	candidates []*domain.Issue
}

func (r candidatesRepo) FindKeyCandidates(context.Context, string, int, uuid.UUID, uuid.UUID, bool) ([]*domain.Issue, error) {
	return r.candidates, nil
}

// viewAuthorizer allows view_project only on the listed projects.
type viewAuthorizer struct{ visible map[uuid.UUID]bool }

func (a viewAuthorizer) Authorize(_ context.Context, _ string, _ uuid.UUID, _ string, projectID uuid.UUID) error {
	if a.visible[projectID] {
		return nil
	}
	return apperror.New(apperror.KindNotFound, "project not found")
}

func TestIssueService_ResolveKey(t *testing.T) {
	ctx := context.Background()
	projectA, projectB := uuid.New(), uuid.New()
	issueA := &domain.Issue{ID: uuid.New(), ProjectID: projectA, ProjectKey: "CORE", KeyNumber: 7}
	issueB := &domain.Issue{ID: uuid.New(), ProjectID: projectB, ProjectKey: "CORE", KeyNumber: 7}

	build := func(visible []uuid.UUID, candidates ...*domain.Issue) *IssueService {
		allowed := map[uuid.UUID]bool{}
		for _, id := range visible {
			allowed[id] = true
		}
		repo := candidatesRepo{fakeIssueRepo: newFakeIssueRepo(), candidates: candidates}
		return NewIssueService(repo, newFakeUserDirectory(), viewAuthorizer{visible: allowed})
	}

	tests := []struct {
		name     string
		svc      *IssueService
		ref      string
		wantID   uuid.UUID
		wantCode string
	}{
		{"one visible candidate", build([]uuid.UUID{projectA}, issueA), "CORE-7", issueA.ID, ""},
		{"hidden candidate is skipped", build([]uuid.UUID{projectB}, issueA, issueB), "CORE-7", issueB.ID, ""},
		{"no candidates", build(nil), "CORE-7", uuid.Nil, "issue.not_found"},
		{"candidate in an invisible project", build(nil, issueA), "CORE-7", uuid.Nil, "issue.not_found"},
		{"same key in two visible organizations", build([]uuid.UUID{projectA, projectB}, issueA, issueB), "CORE-7", uuid.Nil, "issue.key_ambiguous"},
		{"malformed key", build(nil), "core-7", uuid.Nil, "issue.invalid_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.svc.ResolveKey(ctx, uuid.New(), "user", uuid.Nil, tt.ref)
			if got := apperror.CodeOf(err); got != tt.wantCode {
				t.Fatalf("code = %q (%v), want %q", got, err, tt.wantCode)
			}
			if id != tt.wantID {
				t.Errorf("id = %s, want %s", id, tt.wantID)
			}
		})
	}
}

func TestIssueService_ResolveKeyPropagatesInternalAuthorizationErrors(t *testing.T) {
	failing := failingAuthorizer{}
	repo := candidatesRepo{fakeIssueRepo: newFakeIssueRepo(), candidates: []*domain.Issue{{ID: uuid.New(), ProjectID: uuid.New()}}}
	svc := NewIssueService(repo, newFakeUserDirectory(), failing)
	if _, err := svc.ResolveKey(context.Background(), uuid.New(), "user", uuid.Nil, "CORE-1"); apperror.KindOf(err) != apperror.KindInternal {
		t.Fatalf("kind = %v, want internal", apperror.KindOf(err))
	}
}

type failingAuthorizer struct{}

func (failingAuthorizer) Authorize(context.Context, string, uuid.UUID, string, uuid.UUID) error {
	return apperror.New(apperror.KindInternal, "boom")
}
