package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/roadmap/application/port"
	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

type fakeRepo struct {
	epics    []domain.EpicRow
	versions []domain.PeriodRow
	sprints  []domain.PeriodRow
	deps     []domain.Dependency
	ranked   []uuid.UUID
	calls    int
}

func (f *fakeRepo) Epics(context.Context, uuid.UUID) ([]domain.EpicRow, error) {
	f.calls++
	return f.epics, nil
}
func (f *fakeRepo) Versions(context.Context, uuid.UUID) ([]domain.PeriodRow, error) {
	f.calls++
	return f.versions, nil
}
func (f *fakeRepo) Sprints(context.Context, uuid.UUID) ([]domain.PeriodRow, error) {
	f.calls++
	return f.sprints, nil
}
func (f *fakeRepo) Dependencies(context.Context, uuid.UUID) ([]domain.Dependency, error) {
	f.calls++
	return f.deps, nil
}
func (f *fakeRepo) SaveRank(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
	f.ranked = ids
	return nil
}

type fakeAuth struct{ allowed map[string]bool }

func (f fakeAuth) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, _ uuid.UUID) error {
	if f.allowed[action] {
		return nil
	}
	if action == port.ActionViewProject {
		return apperror.New(apperror.KindNotFound, "not found")
	}
	return apperror.New(apperror.KindForbidden, "forbidden")
}

func day(m time.Month, d int) *time.Time {
	t := time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func newService() (*Service, *fakeRepo) {
	a, b := uuid.New(), uuid.New()
	repo := &fakeRepo{
		epics: []domain.EpicRow{
			{ID: a, Key: "P-1", Title: "Plataforma", Start: day(1, 1), Due: day(2, 1), Total: 4, Done: 2},
			{ID: b, Key: "P-2", Title: "Pagos", ChildStart: day(3, 1), ChildDue: day(3, 31), Total: 2, Done: 2},
			{ID: uuid.New(), Key: "P-3", Title: "Sin fechas", Total: 1, Todo: 1},
		},
		versions: []domain.PeriodRow{{ID: uuid.New(), Kind: "version", Name: "v1", Status: "unreleased", End: day(2, 15)}, {ID: uuid.New(), Kind: "version", Name: "v0", Status: "released", End: day(1, 10)}},
		sprints:  []domain.PeriodRow{{ID: uuid.New(), Kind: "sprint", Name: "S1", Status: "closed", Start: day(1, 5), End: day(1, 18)}},
		deps:     []domain.Dependency{{From: a, To: b, Kind: "blocks"}, {From: b, To: uuid.New(), Kind: "blocks"}},
	}
	return NewService(repo, fakeAuth{allowed: map[string]bool{port.ActionViewProject: true, port.ActionEditIssue: true}}), repo
}

func TestRoadmapGroupsAndRange(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()
	project := uuid.New()

	all, err := svc.Roadmap(ctx, uuid.New(), "user", project, domain.Query{})
	if err != nil || all.Group != domain.GroupEpic || len(all.Bars) != 3 || len(all.Versions) != 2 || len(all.Sprints) != 1 {
		t.Fatalf("all: %+v %v", all, err)
	}
	if repo.calls != 4 {
		t.Fatalf("repository calls = %d, want a fixed 4", repo.calls)
	}
	if all.Bars[1].Start.Format("2006-01-02") != "2026-03-01" || all.Bars[1].StartSource != domain.SourceDerived || !all.Bars[1].Complete {
		t.Fatalf("derived epic: %+v", all.Bars[1])
	}
	// Only the dependency whose two ends are shown survives.
	if len(all.Dependencies) != 1 || all.Dependencies[0].Kind != "blocks" {
		t.Fatalf("dependencies: %+v", all.Dependencies)
	}

	// A window keeps overlapping bars and the undated ones; flags and bands follow the window.
	win, _ := svc.Roadmap(ctx, uuid.New(), "user", project, domain.Query{From: day(2, 10), To: day(2, 28), Filter: domain.FilterAll})
	keys := []string{}
	for _, b := range win.Bars {
		keys = append(keys, b.Key)
	}
	if len(keys) != 1 || keys[0] != "P-3" || len(win.Versions) != 1 || win.Versions[0].Title != "v1" || len(win.Sprints) != 0 {
		t.Fatalf("window: %v %+v", keys, win)
	}
	if len(win.Dependencies) != 0 {
		t.Fatalf("dependencies with a hidden end: %+v", win.Dependencies)
	}

	open, _ := svc.Roadmap(ctx, uuid.New(), "user", project, domain.Query{Filter: domain.FilterOpen})
	if len(open.Bars) != 2 {
		t.Fatalf("open: %d", len(open.Bars))
	}
	// The filter and the text do not hide the versions and sprints of the axis.
	if len(open.Versions) != 2 {
		t.Fatalf("versions hidden by the epic filter: %d", len(open.Versions))
	}

	byVersion, _ := svc.Roadmap(ctx, uuid.New(), "user", project, domain.Query{Group: domain.GroupVersion})
	if byVersion.Group != domain.GroupVersion || len(byVersion.Bars) != 2 || byVersion.Bars[0].Kind != "version" || len(byVersion.Dependencies) != 0 {
		t.Fatalf("by version: %+v", byVersion)
	}
	bySprint, _ := svc.Roadmap(ctx, uuid.New(), "user", project, domain.Query{Group: domain.GroupSprint, Text: "s1"})
	if len(bySprint.Bars) != 1 || bySprint.Bars[0].Title != "S1" {
		t.Fatalf("by sprint: %+v", bySprint.Bars)
	}
}

func TestRoadmapPermissionsAndValidation(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	project := uuid.New()
	hidden := NewService(&fakeRepo{}, fakeAuth{})
	if _, err := hidden.Roadmap(ctx, uuid.New(), "user", project, domain.Query{}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("hidden project: %v", err)
	}
	if _, err := hidden.Dependencies(ctx, uuid.New(), "user", project); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("hidden dependencies: %v", err)
	}
	if _, err := svc.Roadmap(ctx, uuid.New(), "user", project, domain.Query{From: day(5, 1), To: day(4, 1)}); apperror.CodeOf(err) != domain.CodeInvalidRange {
		t.Fatalf("reversed range: %v", err)
	}
	deps, err := svc.Dependencies(ctx, uuid.New(), "user", project)
	if err != nil || len(deps) != 2 {
		t.Fatalf("dependencies: %v %v", deps, err)
	}
	empty, _ := NewService(&fakeRepo{}, fakeAuth{allowed: map[string]bool{port.ActionViewProject: true}}).Dependencies(ctx, uuid.New(), "user", project)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty dependencies must be a non-nil slice: %#v", empty)
	}
}

func TestRankPermissionsAndValidation(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.Background()
	project := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	viewer := NewService(repo, fakeAuth{allowed: map[string]bool{port.ActionViewProject: true}})
	if err := viewer.Rank(ctx, uuid.New(), "user", project, ids); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer: %v", err)
	}
	if err := NewService(repo, fakeAuth{}).Rank(ctx, uuid.New(), "user", project, ids); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("hidden: %v", err)
	}
	editor := NewService(repo, fakeAuth{allowed: map[string]bool{port.ActionViewProject: true, port.ActionEditIssue: true}})
	if err := editor.Rank(ctx, uuid.New(), "user", project, []uuid.UUID{ids[0], ids[0]}); apperror.CodeOf(err) != domain.CodeInvalidRank {
		t.Fatalf("duplicates: %v", err)
	}
	if repo.ranked != nil {
		t.Fatal("an invalid rank reached the repository")
	}
	if err := editor.Rank(ctx, uuid.New(), "user", project, ids); err != nil || len(repo.ranked) != 2 || repo.ranked[0] != ids[0] {
		t.Fatalf("rank: %v %v", err, repo.ranked)
	}
	if err := editor.Rank(ctx, uuid.New(), "user", project, nil); err != nil {
		t.Fatalf("clearing the order: %v", err)
	}
}
