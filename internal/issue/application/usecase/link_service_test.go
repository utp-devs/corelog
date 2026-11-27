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

// linkFakeAuthorizer allows every action except the ones on denied projects. editDenied makes edit fail everywhere.
type linkFakeAuthorizer struct {
	denyView   map[uuid.UUID]bool
	editDenied bool
}

func (a linkFakeAuthorizer) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, projectID uuid.UUID) error {
	if a.denyView[projectID] {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	if action == "edit_issue" && a.editDenied {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

// linkFakeRepo keeps issue summaries and links in memory. A soft-deleted issue is missing from issues.
type linkFakeRepo struct {
	issues map[uuid.UUID]port.IssueSummary
	links  []*domain.IssueLink
}

func newLinkFakeRepo() *linkFakeRepo {
	return &linkFakeRepo{issues: map[uuid.UUID]port.IssueSummary{}}
}

func (r *linkFakeRepo) add(org, project uuid.UUID, key string) uuid.UUID {
	id := uuid.New()
	r.issues[id] = port.IssueSummary{ID: id, ProjectID: project, OrganizationID: org, Key: key, Summary: key, Status: domain.StatusOpen}
	return id
}

func (r *linkFakeRepo) FindLinkable(_ context.Context, id uuid.UUID) (port.IssueSummary, error) {
	issue, ok := r.issues[id]
	if !ok {
		return port.IssueSummary{}, domain.ErrIssueNotFound
	}
	return issue, nil
}

func (r *linkFakeRepo) ListLinks(_ context.Context, id uuid.UUID) ([]port.LinkedIssue, error) {
	var out []port.LinkedIssue
	for _, l := range r.links {
		if l.SourceIssueID != id && l.TargetIssueID != id {
			continue
		}
		other := l.TargetIssueID
		if l.TargetIssueID == id {
			other = l.SourceIssueID
		}
		if _, live := r.issues[other]; !live {
			continue
		}
		out = append(out, port.LinkedIssue{LinkID: l.ID, Kind: l.Kind, Outward: l.SourceIssueID == id, Other: r.issues[other]})
	}
	return out, nil
}

func (r *linkFakeRepo) FindLinkForIssue(_ context.Context, issueID, linkID uuid.UUID) (*domain.IssueLink, error) {
	for _, l := range r.links {
		if l.ID == linkID && (l.SourceIssueID == issueID || l.TargetIssueID == issueID) {
			return l, nil
		}
	}
	return nil, domain.ErrIssueLinkNotFound
}

func (r *linkFakeRepo) PairExists(_ context.Context, a, b uuid.UUID, kind domain.IssueLinkKind) (bool, error) {
	for _, l := range r.links {
		if l.Kind == kind && ((l.SourceIssueID == a && l.TargetIssueID == b) || (l.SourceIssueID == b && l.TargetIssueID == a)) {
			return true, nil
		}
	}
	return false, nil
}

func (r *linkFakeRepo) BlocksReach(_ context.Context, from, to uuid.UUID) (bool, error) {
	seen := map[uuid.UUID]bool{}
	stack := []uuid.UUID{from}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, l := range r.links {
			if l.Kind != domain.IssueLinkBlocks || l.SourceIssueID != cur || seen[l.TargetIssueID] {
				continue
			}
			if l.TargetIssueID == to {
				return true, nil
			}
			seen[l.TargetIssueID] = true
			stack = append(stack, l.TargetIssueID)
		}
	}
	return false, nil
}

func (r *linkFakeRepo) Save(_ context.Context, l *domain.IssueLink) error {
	l.ID = uuid.New()
	r.links = append(r.links, l)
	return nil
}

func (r *linkFakeRepo) Delete(_ context.Context, linkID uuid.UUID) error {
	for i, l := range r.links {
		if l.ID == linkID {
			r.links = append(r.links[:i], r.links[i+1:]...)
			return nil
		}
	}
	return domain.ErrIssueLinkNotFound
}

func TestIssueLinkRules(t *testing.T) {
	ctx := context.Background()
	org, otherOrg := uuid.New(), uuid.New()
	project, otherProject, hidden := uuid.New(), uuid.New(), uuid.New()
	actor := uuid.New()

	cases := []struct {
		name  string
		setup func(r *linkFakeRepo) (source uuid.UUID, target string, kind domain.IssueLinkKind)
		auth  linkFakeAuthorizer
		want  error
		kind  apperror.Kind
	}{
		{name: "self link", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a := r.add(org, project, "P-1")
			return a, a.String(), domain.IssueLinkRelates
		}, want: domain.ErrSelfLink},
		{name: "duplicate pair", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a, b := r.add(org, project, "P-1"), r.add(org, project, "P-2")
			_ = r.Save(context.Background(), &domain.IssueLink{SourceIssueID: a, TargetIssueID: b, Kind: domain.IssueLinkDuplicates})
			return a, b.String(), domain.IssueLinkDuplicates
		}, want: domain.ErrDuplicateLink},
		{name: "inverse duplicate", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a, b := r.add(org, project, "P-1"), r.add(org, project, "P-2")
			_ = r.Save(context.Background(), &domain.IssueLink{SourceIssueID: a, TargetIssueID: b, Kind: domain.IssueLinkDuplicates})
			return b, a.String(), domain.IssueLinkDuplicates
		}, want: domain.ErrDuplicateLink},
		{name: "blocking cycle", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a, b, c := r.add(org, project, "P-1"), r.add(org, project, "P-2"), r.add(org, project, "P-3")
			_ = r.Save(context.Background(), &domain.IssueLink{SourceIssueID: a, TargetIssueID: b, Kind: domain.IssueLinkBlocks})
			_ = r.Save(context.Background(), &domain.IssueLink{SourceIssueID: b, TargetIssueID: c, Kind: domain.IssueLinkBlocks})
			return c, a.String(), domain.IssueLinkBlocks
		}, want: domain.ErrLinkCycle},
		{name: "relates never forms a cycle", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a, b, c := r.add(org, project, "P-1"), r.add(org, project, "P-2"), r.add(org, project, "P-3")
			_ = r.Save(context.Background(), &domain.IssueLink{SourceIssueID: a, TargetIssueID: b, Kind: domain.IssueLinkRelates})
			_ = r.Save(context.Background(), &domain.IssueLink{SourceIssueID: b, TargetIssueID: c, Kind: domain.IssueLinkRelates})
			return c, a.String(), domain.IssueLinkRelates
		}},
		{name: "cross organization target is hidden", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a := r.add(org, project, "P-1")
			b := r.add(otherOrg, otherProject, "Q-1")
			return a, b.String(), domain.IssueLinkRelates
		}, want: domain.ErrIssueNotFound},
		{name: "target the caller cannot browse is hidden", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a := r.add(org, project, "P-1")
			b := r.add(org, hidden, "H-1")
			return a, b.String(), domain.IssueLinkRelates
		}, auth: linkFakeAuthorizer{denyView: map[uuid.UUID]bool{hidden: true}}, want: domain.ErrIssueNotFound},
		{name: "soft deleted target is missing", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a := r.add(org, project, "P-1")
			return a, uuid.New().String(), domain.IssueLinkRelates
		}, want: domain.ErrIssueNotFound},
		{name: "edit denied on source", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a, b := r.add(org, project, "P-1"), r.add(org, project, "P-2")
			return a, b.String(), domain.IssueLinkRelates
		}, auth: linkFakeAuthorizer{editDenied: true}, kind: apperror.KindForbidden},
		{name: "unknown kind is rejected before the service", setup: func(r *linkFakeRepo) (uuid.UUID, string, domain.IssueLinkKind) {
			a, b := r.add(org, project, "P-1"), r.add(org, project, "P-2")
			return a, b.String(), domain.IssueLinkKind("unknown")
		}, want: domain.ErrUnknownLinkKind},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newLinkFakeRepo()
			source, target, kind := tc.setup(repo)
			svc := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), repo, tc.auth)
			_, err := svc.CreateIssueLink(ctx, source, actor, "agent", target, kind)
			switch {
			case tc.want != nil:
				if !errors.Is(err, tc.want) {
					t.Fatalf("CreateIssueLink = %v, want %v", err, tc.want)
				}
			case tc.kind != "":
				if apperror.KindOf(err) != tc.kind {
					t.Fatalf("CreateIssueLink = %v, want kind %s", err, tc.kind)
				}
			default:
				if err != nil {
					t.Fatalf("CreateIssueLink: %v", err)
				}
			}
		})
	}
}

func TestIssueLinkCreateAndDeleteByIssueScope(t *testing.T) {
	ctx := context.Background()
	org := uuid.New()
	project := uuid.New()
	actor := uuid.New()
	repo := newLinkFakeRepo()
	a, b, c := repo.add(org, project, "P-1"), repo.add(org, project, "P-2"), repo.add(org, project, "P-3")
	svc := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), repo, linkFakeAuthorizer{})

	created, err := svc.CreateIssueLink(ctx, a, actor, "agent", b.String(), domain.IssueLinkBlocks)
	if err != nil {
		t.Fatalf("CreateIssueLink: %v", err)
	}
	if !created.Outward || created.Other.ID != b || created.Kind != domain.IssueLinkBlocks {
		t.Fatalf("created link = %+v", created)
	}
	if err := svc.DeleteIssueLink(ctx, c, created.LinkID, actor, "agent"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("deleting through an unrelated issue = %v, want not found", err)
	}
	if err := svc.DeleteIssueLink(ctx, b, created.LinkID, actor, "agent"); err != nil {
		t.Fatalf("DeleteIssueLink from the target end: %v", err)
	}
	if err := svc.DeleteIssueLink(ctx, a, created.LinkID, actor, "agent"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("deleting twice = %v, want not found", err)
	}
}

func TestIssueLinkKindLabelsAndParsing(t *testing.T) {
	want := map[domain.IssueLinkKind][2]string{
		domain.IssueLinkBlocks:     {"bloquea", "bloqueada por"},
		domain.IssueLinkRelates:    {"se relaciona con", "se relaciona con"},
		domain.IssueLinkDuplicates: {"duplica a", "es duplicada por"},
		domain.IssueLinkClones:     {"clona a", "es clonada por"},
		domain.IssueLinkCauses:     {"causa", "es causada por"},
		domain.IssueLinkImplements: {"implementa", "es implementada por"},
	}
	for kind, labels := range want {
		if got := kind.Label(true); got != labels[0] {
			t.Fatalf("%s outward = %q, want %q", kind, got, labels[0])
		}
		if got := kind.Label(false); got != labels[1] {
			t.Fatalf("%s inward = %q, want %q", kind, got, labels[1])
		}
	}
	if _, err := domain.ParseIssueLinkKind("blocked"); !errors.Is(err, domain.ErrUnknownLinkKind) {
		t.Fatalf("unknown kind parsed: %v", err)
	}
}
