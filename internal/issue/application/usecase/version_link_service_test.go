package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// fakeLinks keeps the versions and components of one project in memory.
type fakeLinks struct {
	project    uuid.UUID
	versions   map[uuid.UUID]domain.VersionRef
	components map[uuid.UUID]domain.ComponentDefault
	linked     map[uuid.UUID][]domain.LinkedRef // issue -> currently linked fix versions
	replaced   []uuid.UUID
}

func (f *fakeLinks) ResolveVersions(_ context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.VersionRef, error) {
	out := []domain.VersionRef{}
	for _, id := range ids {
		if v, ok := f.versions[id]; ok && projectID == f.project {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeLinks) ResolveComponents(_ context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.LinkedRef, error) {
	out := []domain.LinkedRef{}
	for _, id := range ids {
		if c, ok := f.components[id]; ok && projectID == f.project {
			out = append(out, domain.LinkedRef{ID: c.ID, Name: c.Name})
		}
	}
	return out, nil
}

func (f *fakeLinks) ComponentDefaults(_ context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.ComponentDefault, error) {
	out := []domain.ComponentDefault{}
	for _, id := range ids {
		if c, ok := f.components[id]; ok && projectID == f.project {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeLinks) LoadLinks(_ context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]domain.IssueLinks, error) {
	out := map[uuid.UUID]domain.IssueLinks{}
	for _, id := range issueIDs {
		out[id] = domain.IssueLinks{FixVersions: f.linked[id]}
	}
	return out, nil
}

func (f *fakeLinks) ReplaceIssueVersions(_ context.Context, _ uuid.UUID, _ string, ids []uuid.UUID) error {
	f.replaced = ids
	return nil
}

func (f *fakeLinks) ReplaceIssueComponents(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
	f.replaced = ids
	return nil
}

// fakeDefaults answers the project lead of the issue defaults port.
type fakeDefaults struct{ lead *uuid.UUID }

func (d fakeDefaults) DefaultAssigneePolicy(context.Context, uuid.UUID) (string, *uuid.UUID, error) {
	return "unassigned", d.lead, nil
}

type linkFixture struct {
	links    *fakeLinks
	svc      *IssueService
	project  uuid.UUID
	issue    *domain.Issue
	released uuid.UUID
	archived uuid.UUID
	open     uuid.UUID
	foreign  uuid.UUID
}

func newLinkFixture(t *testing.T, lead *uuid.UUID) *linkFixture {
	t.Helper()
	f := &linkFixture{project: uuid.New(), released: uuid.New(), archived: uuid.New(), open: uuid.New(), foreign: uuid.New()}
	f.links = &fakeLinks{
		project: f.project,
		versions: map[uuid.UUID]domain.VersionRef{
			f.released: {ID: f.released, Name: "1.0", Status: "released"},
			f.archived: {ID: f.archived, Name: "0.9", Status: "archived"},
			f.open:     {ID: f.open, Name: "1.1", Status: "unreleased"},
		},
		components: map[uuid.UUID]domain.ComponentDefault{},
		linked:     map[uuid.UUID][]domain.LinkedRef{},
	}
	f.svc = &IssueService{versionLinks: f.links, defaults: fakeDefaults{lead: lead}}
	f.issue = &domain.Issue{ID: uuid.New(), ProjectID: f.project}
	return f
}

func codeOfLink(err error) string { return apperror.CodeOf(err) }

func TestFixVersionsRejectForeignAndClosedVersions(t *testing.T) {
	f := newLinkFixture(t, nil)
	ctx := context.Background()

	if _, err := f.svc.replaceVersionKind(ctx, f.issue, domain.VersionLinkFix, &[]uuid.UUID{f.foreign}, nil, false); codeOfLink(err) != "issue.version_invalid" {
		t.Fatalf("foreign version: %v", err)
	}
	if _, err := f.svc.replaceVersionKind(ctx, f.issue, domain.VersionLinkFix, &[]uuid.UUID{f.released}, nil, false); codeOfLink(err) != "issue.version_closed" {
		t.Fatalf("released version for a fix link: %v", err)
	}
	if _, err := f.svc.replaceVersionKind(ctx, f.issue, domain.VersionLinkFix, &[]uuid.UUID{f.archived}, nil, true); codeOfLink(err) != "" {
		t.Fatalf("admin override should pass: %v", err)
	}
	if _, err := f.svc.replaceVersionKind(ctx, f.issue, domain.VersionLinkFix, &[]uuid.UUID{f.open}, nil, false); err != nil {
		t.Fatalf("open version: %v", err)
	}
}

func TestAlreadyLinkedClosedVersionIsKept(t *testing.T) {
	f := newLinkFixture(t, nil)
	f.links.linked[f.issue.ID] = []domain.LinkedRef{{ID: f.released, Name: "1.0"}}
	refs, err := f.svc.replaceVersionKind(context.Background(), f.issue, domain.VersionLinkFix, &[]uuid.UUID{f.released}, f.links.linked[f.issue.ID], false)
	if err != nil || len(refs) != 1 {
		t.Fatalf("unchanged closed link must not be re-validated: %v %v", err, refs)
	}
}

func TestAffectsVersionsAcceptReleasedButNotArchived(t *testing.T) {
	f := newLinkFixture(t, nil)
	ctx := context.Background()
	if _, err := f.svc.replaceVersionKind(ctx, f.issue, domain.VersionLinkAffects, &[]uuid.UUID{f.released}, nil, false); err != nil {
		t.Fatalf("released version should accept affects: %v", err)
	}
	if _, err := f.svc.replaceVersionKind(ctx, f.issue, domain.VersionLinkAffects, &[]uuid.UUID{f.archived}, nil, false); codeOfLink(err) != "issue.version_closed" {
		t.Fatalf("archived version for affects: %v", err)
	}
}

func TestTooManyVersionsAreRejected(t *testing.T) {
	f := newLinkFixture(t, nil)
	ids := make([]uuid.UUID, 0, domain.MaxIssueVersionLinks+1)
	for i := 0; i <= domain.MaxIssueVersionLinks; i++ {
		ids = append(ids, uuid.New())
	}
	if _, err := f.svc.replaceVersionKind(context.Background(), f.issue, domain.VersionLinkFix, &ids, nil, false); codeOfLink(err) != "issue.version_invalid" {
		t.Fatalf("list over the limit: %v", err)
	}
}

func TestComponentsMustBelongToProject(t *testing.T) {
	f := newLinkFixture(t, nil)
	known := uuid.New()
	f.links.components[known] = domain.ComponentDefault{ID: known, Name: "Pagos", Policy: domain.ComponentPolicyNone}
	if _, err := f.svc.replaceComponents(context.Background(), f.issue, &[]uuid.UUID{known, uuid.New()}); codeOfLink(err) != "issue.component_invalid" {
		t.Fatalf("unknown component: %v", err)
	}
	refs, err := f.svc.replaceComponents(context.Background(), f.issue, &[]uuid.UUID{known})
	if err != nil || len(refs) != 1 || refs[0].Name != "Pagos" {
		t.Fatalf("known component: %v %v", err, refs)
	}
}

func TestComponentAssigneeFollowsPolicies(t *testing.T) {
	projectLead := uuid.New()
	componentLead := uuid.New()
	f := newLinkFixture(t, &projectLead)
	lead := componentLead
	none, byLead, byProject, noLead := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	f.links.components[none] = domain.ComponentDefault{ID: none, Name: "A", Policy: domain.ComponentPolicyNone}
	f.links.components[byLead] = domain.ComponentDefault{ID: byLead, Name: "B", Policy: domain.ComponentPolicyComponentLead, LeadID: &lead}
	f.links.components[byProject] = domain.ComponentDefault{ID: byProject, Name: "C", Policy: domain.ComponentPolicyProjectLead}
	f.links.components[noLead] = domain.ComponentDefault{ID: noLead, Name: "D", Policy: domain.ComponentPolicyComponentLead}
	ctx := context.Background()

	got, found, err := f.svc.componentAssignee(ctx, f.project, []uuid.UUID{none})
	if err != nil || found || got != nil {
		t.Fatalf("policy none must not assign: %v %v", found, got)
	}
	got, found, _ = f.svc.componentAssignee(ctx, f.project, []uuid.UUID{byLead})
	if !found || got == nil || *got != componentLead {
		t.Fatalf("component lead not applied: %v", got)
	}
	got, found, _ = f.svc.componentAssignee(ctx, f.project, []uuid.UUID{byProject})
	if !found || got == nil || *got != projectLead {
		t.Fatalf("project lead not applied: %v", got)
	}
	got, found, _ = f.svc.componentAssignee(ctx, f.project, []uuid.UUID{noLead})
	if found || got != nil {
		t.Fatalf("component_lead without lead must fall through: %v", got)
	}
	got, found, _ = f.svc.componentAssignee(ctx, f.project, []uuid.UUID{none, byLead})
	if !found || *got != componentLead {
		t.Fatalf("first policy with an opinion wins: %v", got)
	}
	if _, found, _ := f.svc.componentAssignee(ctx, uuid.New(), []uuid.UUID{byLead}); found {
		t.Fatal("components of another project must not assign")
	}
}
