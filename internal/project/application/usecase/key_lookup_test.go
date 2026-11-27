package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// recordingRecents keeps every recorded view. With fail set, RecordView returns an error.
type recordingRecents struct {
	views []string
	fail  bool
}

func (r *recordingRecents) RecordView(_ context.Context, userID uuid.UUID, entityType string, entityID uuid.UUID) error {
	r.views = append(r.views, userID.String()+":"+entityType+":"+entityID.String())
	if r.fail {
		return errors.New("store unavailable")
	}
	return nil
}

func TestFindByKeyWithoutOrganizationResolvesSingleCandidate(t *testing.T) {
	f := newFixture(t)
	p := createFromTemplate(t, f, f.owner, f.orgA, "ONLY", domain.TemplateKanban)

	got, err := f.svc.FindByKey(f.ctx, f.member, "requester", uuid.Nil, "only")
	if err != nil || got.ID != p.ID {
		t.Fatalf("FindByKey = %v, %v; want project %s", got, err, p.ID)
	}
	// The outsider belongs to no organization of the project.
	_, err = f.svc.FindByKey(f.ctx, f.outsider, "requester", uuid.Nil, "ONLY")
	wantCode(t, err, "project.not_found")
	// A malformed key is rejected before any lookup.
	_, err = f.svc.FindByKey(f.ctx, f.member, "requester", uuid.Nil, "1bad")
	wantCode(t, err, "project.invalid_key")
}

func TestFindByKeyAmbiguousAcrossOrganizations(t *testing.T) {
	f := newFixture(t)
	f.orgs.join(f.orgB, f.owner, domain.OrgRoleMember)
	inA := createFromTemplate(t, f, f.owner, f.orgA, "SAME", domain.TemplateKanban)
	inB, err := f.svc.CreateProject(f.ctx, f.owner, f.orgB, port.CreateInput{
		Key: "SAME", Name: "Same in B", Template: domain.TemplateKanban, Visibility: domain.VisibilityOpen,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.svc.FindByKey(f.ctx, f.owner, "requester", uuid.Nil, "SAME")
	wantCode(t, err, "project.key_ambiguous")
	wantError(t, err, apperror.KindConflict, "project.key_ambiguous")

	got, err := f.svc.FindByKey(f.ctx, f.owner, "requester", f.orgB, "SAME")
	if err != nil || got.ID != inB.ID {
		t.Fatalf("FindByKey with organization B = %v, %v", got, err)
	}
	got, err = f.svc.FindByKey(f.ctx, f.owner, "requester", f.orgA, "same")
	if err != nil || got.ID != inA.ID {
		t.Fatalf("FindByKey with organization A = %v, %v", got, err)
	}
}

func TestFindByKeyHidesPrivateAndForeignProjects(t *testing.T) {
	f := newFixture(t)
	createFromTemplate(t, f, f.owner, f.orgA, "HID", domain.TemplateKanban)
	private, err := f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{
		Key: "PRIVK", Name: "Private", Template: domain.TemplateKanban, Visibility: domain.VisibilityPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}

	// A member of the organization does not see a private project without a grant.
	_, err = f.svc.FindByKey(f.ctx, f.member, "requester", f.orgA, "PRIVK")
	wantCode(t, err, "project.not_found")
	got, err := f.svc.FindByKey(f.ctx, f.owner, "requester", f.orgA, "PRIVK")
	if err != nil || got.ID != private.ID {
		t.Fatalf("owner FindByKey = %v, %v", got, err)
	}
	// A key of organization A asked with organization B is not found.
	_, err = f.svc.FindByKey(f.ctx, f.outsider, "requester", f.orgB, "HID")
	wantCode(t, err, "project.not_found")
}

func TestGetRecordsRecentViewAndIgnoresRecorderFailure(t *testing.T) {
	f := newFixture(t)
	p := createFromTemplate(t, f, f.owner, f.orgA, "REC", domain.TemplateKanban)
	recents := &recordingRecents{}
	f.svc.WithRecents(recents)

	if _, err := f.svc.Get(f.ctx, f.member, "requester", p.ID); err != nil {
		t.Fatal(err)
	}
	want := f.member.String() + ":project:" + p.ID.String()
	if len(recents.views) != 1 || recents.views[0] != want {
		t.Fatalf("views = %v, want [%s]", recents.views, want)
	}

	// A denied read records nothing.
	if _, err := f.svc.Get(f.ctx, f.outsider, "requester", p.ID); err == nil {
		t.Fatal("outsider must not read the project")
	}
	if len(recents.views) != 1 {
		t.Fatalf("denied read recorded a view: %v", recents.views)
	}

	recents.fail = true
	got, err := f.svc.Get(f.ctx, f.member, "requester", p.ID)
	if err != nil || got.ID != p.ID {
		t.Fatalf("Get must succeed when the recorder fails: %v, %v", got, err)
	}
}

func TestListFilterByLegacyType(t *testing.T) {
	f := newFixture(t)
	kanban := createFromTemplate(t, f, f.owner, f.orgA, "TKAN", domain.TemplateKanban)
	taskTracking := createFromTemplate(t, f, f.owner, f.orgA, "TTRK", domain.TemplateTaskTracking)
	scrum := createFromTemplate(t, f, f.owner, f.orgA, "TSCR", domain.TemplateScrum)

	list := func(filter port.ProjectFilter) map[uuid.UUID]bool {
		t.Helper()
		page, err := f.svc.ListProjects(f.ctx, f.owner, "requester", f.orgA, filter)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[uuid.UUID]bool{}
		for _, p := range page.Items {
			ids[p.ID] = true
		}
		return ids
	}

	kanbanOnly := list(port.ProjectFilter{Type: domain.ProjectTypeKanban})
	if !kanbanOnly[kanban.ID] || !kanbanOnly[taskTracking.ID] || kanbanOnly[scrum.ID] {
		t.Fatalf("type=kanban matched %v", kanbanOnly)
	}
	scrumOnly := list(port.ProjectFilter{Type: domain.ProjectTypeScrum})
	if len(scrumOnly) != 1 || !scrumOnly[scrum.ID] {
		t.Fatalf("type=scrum matched %v", scrumOnly)
	}
}
