package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/domain"
)

type fakeAuth struct{ err error }

func (f fakeAuth) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, _ uuid.UUID) error {
	if action != port.ActionViewReports {
		return errors.New("wrong action")
	}
	return f.err
}

type fakeReader struct {
	sprint    domain.SprintInfo
	sprintErr error
	events    []domain.ScopeEvent
	issues    []domain.IssueRow
	history   []domain.StatusChange
	worklogs  []domain.WorklogRow
	version   domain.VersionInfo
	versErr   error
	tz        string
	// recorded
	issueQuery port.IssueQuery
	calls      int
}

func (f *fakeReader) Project(context.Context, uuid.UUID) (domain.ProjectInfo, error) {
	f.calls++
	return domain.ProjectInfo{Name: "P", Timezone: f.tz}, nil
}
func (f *fakeReader) Sprint(_ context.Context, _ uuid.UUID, id *uuid.UUID) (domain.SprintInfo, error) {
	f.calls++
	return f.sprint, f.sprintErr
}
func (f *fakeReader) RecentSprints(context.Context, uuid.UUID, int) ([]domain.SprintInfo, error) {
	f.calls++
	return []domain.SprintInfo{f.sprint}, nil
}
func (f *fakeReader) ScopeEvents(context.Context, uuid.UUID) ([]domain.ScopeEvent, error) {
	f.calls++
	return f.events, nil
}
func (f *fakeReader) Version(context.Context, uuid.UUID, uuid.UUID) (domain.VersionInfo, error) {
	f.calls++
	return f.version, f.versErr
}
func (f *fakeReader) Issues(_ context.Context, q port.IssueQuery) ([]domain.IssueRow, error) {
	f.calls++
	f.issueQuery = q
	return f.issues, nil
}
func (f *fakeReader) StatusHistory(context.Context, port.HistoryQuery) ([]domain.StatusChange, error) {
	f.calls++
	return f.history, nil
}
func (f *fakeReader) Worklogs(context.Context, port.WorklogQuery) ([]domain.WorklogRow, error) {
	f.calls++
	return f.worklogs, nil
}

type fakeFilter struct {
	ids []uuid.UUID
	err error
}

func (f fakeFilter) MatchIssues(context.Context, uuid.UUID, string, uuid.UUID, string, int) ([]uuid.UUID, error) {
	return f.ids, f.err
}

type fakeSprints struct{}

func (fakeSprints) SprintSummary(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) (domain.SprintSummary, error) {
	return domain.SprintSummary{Name: "S", Committed: 4, Completed: 2, Incomplete: 2}, nil
}

var now = time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC)

func newSvc(r *fakeReader, auth error, f port.IssueFilter, s port.SprintSummarizer) *Service {
	return NewService(r, fakeAuth{err: auth}, f, s, clock.NewFixed(now))
}

func run(s *Service, kind string, raw domain.RawParams) (*port.Result, error) {
	return s.Run(context.Background(), uuid.New(), "user", uuid.New(), kind, raw)
}

func TestRunDeniesBeforeReading(t *testing.T) {
	for _, denied := range []error{
		apperror.New(apperror.KindForbidden, "no"), apperror.New(apperror.KindNotFound, "oculto"),
	} {
		r := &fakeReader{}
		s := newSvc(r, denied, nil, nil)
		if _, err := run(s, "workload", domain.RawParams{}); apperror.KindOf(err) != apperror.KindOf(denied) {
			t.Fatalf("err = %v", err)
		}
		if _, err := s.List(context.Background(), uuid.New(), "user", uuid.New()); apperror.KindOf(err) != apperror.KindOf(denied) {
			t.Fatalf("list err = %v", err)
		}
		if r.calls != 0 {
			t.Fatalf("reader was used before authorization: %d calls", r.calls)
		}
	}
}

func TestRunEveryKind(t *testing.T) {
	sprint := domain.SprintInfo{ID: uuid.New(), Name: "S1", Status: "closed", StartDate: &now, EndDate: &now, CompletedAt: &now}
	version := domain.VersionInfo{ID: uuid.New(), Name: "1.0", Status: "unreleased"}
	for _, d := range domain.Catalog() {
		r := &fakeReader{sprint: sprint, version: version, tz: "America/Bogota"}
		s := newSvc(r, nil, nil, fakeSprints{})
		raw := domain.RawParams{}
		if d.Kind == domain.KindVersion {
			raw.Version = uuid.NewString()
		}
		res, err := run(s, string(d.Kind), raw)
		if err != nil {
			t.Fatalf("%s: %v", d.Kind, err)
		}
		if res.Report.Kind != d.Kind || !res.GeneratedAt.Equal(now) || res.Params["timezone"] != "America/Bogota" {
			t.Fatalf("%s: %+v", d.Kind, res)
		}
	}
	if _, err := run(newSvc(&fakeReader{}, nil, nil, nil), "nope", domain.RawParams{}); !errors.Is(err, domain.ErrKindUnknown) {
		t.Fatalf("unknown kind: %v", err)
	}
	if _, err := run(newSvc(&fakeReader{sprint: sprint}, nil, nil, nil), "sprint-report", domain.RawParams{}); err == nil {
		t.Fatal("sprint report without summarizer must fail")
	}
}

func TestRunPropagatesNotFound(t *testing.T) {
	r := &fakeReader{sprintErr: domain.ErrSprintNotFound}
	if _, err := run(newSvc(r, nil, nil, nil), "burndown", domain.RawParams{}); !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("err = %v", err)
	}
	r = &fakeReader{versErr: domain.ErrVersionNotFound}
	if _, err := run(newSvc(r, nil, nil, nil), "version", domain.RawParams{Version: uuid.NewString()}); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := run(newSvc(&fakeReader{}, nil, nil, nil), "control", domain.RawParams{From: "x"}); !errors.Is(err, domain.ErrRangeInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestFilterNarrowsIssues(t *testing.T) {
	keep := []uuid.UUID{uuid.New()}
	r := &fakeReader{}
	s := newSvc(r, nil, fakeFilter{ids: keep}, nil)
	res, err := run(s, "workload", domain.RawParams{Filter: "type = Bug"})
	if err != nil || r.issueQuery.OnlyIDs == nil || len(*r.issueQuery.OnlyIDs) != 1 || res.Params["filter"] != "type = Bug" {
		t.Fatalf("query = %+v err = %v params = %v", r.issueQuery, err, res)
	}
	// The velocity and sprint reports ignore the filter.
	r = &fakeReader{sprint: domain.SprintInfo{ID: uuid.New()}}
	res, err = run(newSvc(r, nil, fakeFilter{err: errors.New("must not run")}, nil), "velocity", domain.RawParams{Filter: "x"})
	if err != nil || res.Params["filter"] != nil {
		t.Fatalf("velocity with filter: %v %v", err, res)
	}
	// An empty match is an empty list, not "no filter".
	r = &fakeReader{}
	if _, err = run(newSvc(r, nil, fakeFilter{ids: []uuid.UUID{}}, nil), "workload", domain.RawParams{Filter: "x"}); err != nil || r.issueQuery.OnlyIDs == nil {
		t.Fatalf("empty match: %v %+v", err, r.issueQuery)
	}
	tooMany := make([]uuid.UUID, port.MaxFilterIssues+1)
	if _, err = run(newSvc(&fakeReader{}, nil, fakeFilter{ids: tooMany}, nil), "workload", domain.RawParams{Filter: "x"}); !errors.Is(err, domain.ErrFilterTooBroad) {
		t.Fatalf("too broad: %v", err)
	}
	bad := apperror.WithCode(apperror.KindValidation, "search.query_invalid", "mal")
	if _, err = run(newSvc(&fakeReader{}, nil, fakeFilter{err: bad}, nil), "workload", domain.RawParams{Filter: "x"}); apperror.CodeOf(err) != "search.query_invalid" {
		t.Fatalf("invalid filter: %v", err)
	}
	if _, err = run(newSvc(&fakeReader{}, nil, nil, nil), "workload", domain.RawParams{Filter: "x"}); !errors.Is(err, domain.ErrFilterInvalid) {
		t.Fatalf("no filter port: %v", err)
	}
}

func TestBurndownFilterIntersectsScope(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	r := &fakeReader{
		sprint: domain.SprintInfo{ID: uuid.New(), Status: "closed", StartDate: &now, EndDate: &now, CompletedAt: &now},
		events: []domain.ScopeEvent{{IssueID: a, Kind: "added", At: now}, {IssueID: b, Kind: "added", At: now}, {IssueID: a, Kind: "points_changed", At: now}},
	}
	if _, err := run(newSvc(r, nil, fakeFilter{ids: []uuid.UUID{b, uuid.New()}}, nil), "burndown", domain.RawParams{Filter: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := *r.issueQuery.OnlyIDs; len(got) != 1 || got[0] != b {
		t.Fatalf("ids = %v, want only %v", got, b)
	}
}
