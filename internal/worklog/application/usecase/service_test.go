package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/worklog/application/port"
	"github.com/jhonsferg/corelog/internal/worklog/domain"
)

var now = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

// fakeAuthz grants actions per project and actor; archived projects reject write actions with a conflict.
type fakeAuthz struct {
	grants   map[uuid.UUID]map[uuid.UUID]map[string]bool
	archived map[uuid.UUID]bool
}

var writeActions = map[string]bool{port.ActionLogWork: true, port.ActionEditIssue: true, port.ActionManageWorklog: true}

func (a *fakeAuthz) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, project uuid.UUID) error {
	if !a.grants[project][actor][action] {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	if a.archived[project] && writeActions[action] {
		return apperror.WithCode(apperror.KindConflict, "project.archived", "archived")
	}
	return nil
}

// fakeRepo keeps everything in memory. Issue time values are copied on read like rows would be.
type fakeRepo struct {
	issues     map[uuid.UUID]port.IssueTime
	worklogs   map[uuid.UUID]domain.Worklog
	activities []port.Activity
	orgs       map[uuid.UUID]domain.OrgTime
}

func ip(v int64) *int64 { return &v }

func (r *fakeRepo) IssueTime(_ context.Context, id uuid.UUID, _ bool) (port.IssueTime, error) {
	it, ok := r.issues[id]
	if !ok {
		return port.IssueTime{}, domain.ErrIssueNotFound
	}
	return it, nil
}

func (r *fakeRepo) Worklog(_ context.Context, id uuid.UUID, _ bool) (domain.Worklog, error) {
	w, ok := r.worklogs[id]
	if !ok || w.DeletedAt != nil {
		return domain.Worklog{}, domain.ErrWorklogNotFound
	}
	return w, nil
}

func (r *fakeRepo) ListIssueWorklogs(_ context.Context, issue uuid.UUID) ([]domain.Worklog, error) {
	var out []domain.Worklog
	for _, w := range r.worklogs {
		if w.IssueID == issue && w.DeletedAt == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

func (r *fakeRepo) InsertWorklog(_ context.Context, w *domain.Worklog) error {
	w.ID = uuid.New()
	w.CreatedAt, w.UpdatedAt = now, now
	r.worklogs[w.ID] = *w
	return nil
}

func (r *fakeRepo) UpdateWorklog(_ context.Context, w *domain.Worklog) error {
	if _, ok := r.worklogs[w.ID]; !ok {
		return domain.ErrWorklogNotFound
	}
	r.worklogs[w.ID] = *w
	return nil
}

func (r *fakeRepo) SoftDeleteWorklog(_ context.Context, id uuid.UUID, at time.Time) error {
	w, ok := r.worklogs[id]
	if !ok {
		return domain.ErrWorklogNotFound
	}
	w.DeletedAt = &at
	r.worklogs[id] = w
	return nil
}

func (r *fakeRepo) RecomputeSpent(_ context.Context, issue uuid.UUID) error {
	var sum int64
	for _, w := range r.worklogs {
		if w.IssueID == issue && w.DeletedAt == nil {
			sum += w.TimeSpentSeconds
		}
	}
	it := r.issues[issue]
	it.Spent = sum
	r.issues[issue] = it
	return nil
}

func (r *fakeRepo) SetRemaining(_ context.Context, issue uuid.UUID, v *int64) error {
	it := r.issues[issue]
	it.Remaining = v
	r.issues[issue] = it
	return nil
}

func (r *fakeRepo) SetOriginal(_ context.Context, issue uuid.UUID, v *int64) error {
	it := r.issues[issue]
	it.Original = v
	r.issues[issue] = it
	return nil
}

func (r *fakeRepo) Window(_ context.Context, w port.Window) ([]domain.Entry, error) {
	var out []domain.Entry
	for _, wl := range r.worklogs {
		it := r.issues[wl.IssueID]
		if wl.DeletedAt != nil || wl.StartedAt.Before(w.Lo) || !wl.StartedAt.Before(w.Hi) {
			continue
		}
		if w.AuthorID != nil && wl.AuthorID != *w.AuthorID {
			continue
		}
		if w.ProjectID != nil && it.ProjectID != *w.ProjectID {
			continue
		}
		out = append(out, domain.Entry{
			ID: wl.ID, IssueID: wl.IssueID, IssueKey: it.IssueKey, IssueTitle: it.IssueTitle,
			ProjectID: it.ProjectID, OrganizationID: it.OrganizationID, AuthorID: wl.AuthorID,
			AuthorName: wl.AuthorName, StartedAt: wl.StartedAt, Seconds: wl.TimeSpentSeconds,
		})
	}
	return out, nil
}

func (r *fakeRepo) OrgTimes(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.OrgTime, error) {
	out := map[uuid.UUID]domain.OrgTime{}
	for _, id := range ids {
		if z, ok := r.orgs[id]; ok {
			out[id] = z
		}
	}
	return out, nil
}

func (r *fakeRepo) AddActivities(_ context.Context, rows []port.Activity) error {
	r.activities = append(r.activities, rows...)
	return nil
}

type fixture struct {
	svc                   *Service
	repo                  *fakeRepo
	authz                 *fakeAuthz
	project, org, issue   uuid.UUID
	lead, contrib, viewer uuid.UUID
	outsider              uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{project: uuid.New(), org: uuid.New(), issue: uuid.New()}
	f.lead, f.contrib, f.viewer, f.outsider = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	f.authz = &fakeAuthz{
		grants: map[uuid.UUID]map[uuid.UUID]map[string]bool{
			f.project: {
				f.lead:    {port.ActionViewProject: true, port.ActionLogWork: true, port.ActionEditIssue: true, port.ActionManageWorklog: true, port.ActionViewReports: true},
				f.contrib: {port.ActionViewProject: true, port.ActionLogWork: true, port.ActionEditIssue: true, port.ActionViewReports: true},
				f.viewer:  {port.ActionViewProject: true, port.ActionViewReports: true},
			},
		},
		archived: map[uuid.UUID]bool{},
	}
	f.repo = &fakeRepo{
		issues:   map[uuid.UUID]port.IssueTime{f.issue: {IssueID: f.issue, IssueKey: "P-1", IssueTitle: "Login", ProjectID: f.project, OrganizationID: f.org, Remaining: ip(10 * 3600)}},
		worklogs: map[uuid.UUID]domain.Worklog{},
		orgs:     map[uuid.UUID]domain.OrgTime{f.org: {Location: time.UTC, Config: domain.DefaultConfig()}},
	}
	f.svc = NewService(f.repo, f.authz, &database.FakeTxManager{}, nil, clock.NewFixed(now))
	return f
}

func text(s string) *port.Duration { return &port.Duration{Text: &s} }

func logInput(spent string, mode domain.RemainingMode) port.LogInput {
	return port.LogInput{TimeSpent: port.Duration{Text: &spent}, Remaining: port.RemainingInput{Mode: mode}}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got nil", code)
	}
	if got := apperror.CodeOf(err); got != code {
		t.Fatalf("code = %q (%v), want %q", got, err, code)
	}
}

func TestLogWorkPermissionMatrix(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.LogWork(context.Background(), f.lead, "user", f.issue, logInput("1h", domain.RemainingAuto)); err != nil {
		t.Fatalf("lead: %v", err)
	}
	if _, err := f.svc.LogWork(context.Background(), f.contrib, "user", f.issue, logInput("1h", domain.RemainingAuto)); err != nil {
		t.Fatalf("contributor: %v", err)
	}
	for name, actor := range map[string]uuid.UUID{"viewer": f.viewer, "outsider": f.outsider} {
		_, err := f.svc.LogWork(context.Background(), actor, "user", f.issue, logInput("1h", domain.RemainingAuto))
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("%s: kind = %v", name, apperror.KindOf(err))
		}
	}
	f.authz.archived[f.project] = true
	_, err := f.svc.LogWork(context.Background(), f.lead, "user", f.issue, logInput("1h", domain.RemainingAuto))
	wantCode(t, err, "project.archived")
}

func TestLogWorkRemainingModes(t *testing.T) {
	cases := []struct {
		name       string
		mode       domain.RemainingMode
		value      *port.Duration
		wantRemain int64
		wantAdjust int64
		wantErr    string
	}{
		{name: "auto reduces by spent", mode: domain.RemainingAuto, wantRemain: 9 * 3600, wantAdjust: 3600},
		{name: "leave keeps estimate", mode: domain.RemainingLeave, wantRemain: 10 * 3600},
		{name: "set replaces", mode: domain.RemainingSet, value: text("2d"), wantRemain: 16 * 3600},
		{name: "set zero allowed", mode: domain.RemainingSet, value: text("0m"), wantRemain: 0},
		{name: "reduce by amount", mode: domain.RemainingReduce, value: text("30m"), wantRemain: 9*3600 + 1800, wantAdjust: 1800},
		{name: "reduce needs value", mode: domain.RemainingReduce, wantErr: domain.CodeDurationRequired},
		{name: "unknown mode", mode: "bogus", wantErr: domain.CodeModeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			in := logInput("1h", tc.mode)
			in.Remaining.Value = tc.value
			out, err := f.svc.LogWork(context.Background(), f.lead, "user", f.issue, in)
			if tc.wantErr != "" {
				wantCode(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if out.Totals.RemainingSeconds == nil || *out.Totals.RemainingSeconds != tc.wantRemain {
				t.Fatalf("remaining = %v, want %d", out.Totals.RemainingSeconds, tc.wantRemain)
			}
			if out.Worklog.RemainingAdjustSeconds != tc.wantAdjust {
				t.Fatalf("adjust = %d, want %d", out.Worklog.RemainingAdjustSeconds, tc.wantAdjust)
			}
			if out.Totals.SpentSeconds != 3600 {
				t.Fatalf("spent = %d", out.Totals.SpentSeconds)
			}
		})
	}
}

func TestAutoModeFloorsAtZeroAndWithoutEstimate(t *testing.T) {
	f := newFixture(t)
	f.repo.issues[f.issue] = port.IssueTime{IssueID: f.issue, ProjectID: f.project, OrganizationID: f.org, Remaining: ip(1800)}
	out, err := f.svc.LogWork(context.Background(), f.lead, "user", f.issue, logInput("1h", domain.RemainingAuto))
	if err != nil || *out.Totals.RemainingSeconds != 0 || out.Worklog.RemainingAdjustSeconds != 1800 {
		t.Fatalf("floor: %v %+v", err, out)
	}
	f.repo.issues[f.issue] = port.IssueTime{IssueID: f.issue, ProjectID: f.project, OrganizationID: f.org}
	out, err = f.svc.LogWork(context.Background(), f.lead, "user", f.issue, logInput("1h", domain.RemainingAuto))
	if err != nil || out.Totals.RemainingSeconds != nil {
		t.Fatalf("nil estimate must stay nil: %v %+v", err, out.Totals)
	}
}

func TestWorklogLifecycleRestoresEstimate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	out, err := f.svc.LogWork(ctx, f.contrib, "user", f.issue, logInput("2h", domain.RemainingAuto))
	if err != nil {
		t.Fatal(err)
	}
	id := out.Worklog.ID
	if *f.repo.issues[f.issue].Remaining != 8*3600 {
		t.Fatalf("after log remaining = %d", *f.repo.issues[f.issue].Remaining)
	}

	// Editing the time re-applies auto from the restored estimate: 1h instead of 2h gives 9h left.
	newTime := "1h"
	edited, err := f.svc.UpdateWorklog(ctx, f.contrib, "user", id, port.UpdateInput{TimeSpent: text(newTime)})
	if err != nil {
		t.Fatal(err)
	}
	if *f.repo.issues[f.issue].Remaining != 9*3600 || edited.Totals.SpentSeconds != 3600 {
		t.Fatalf("after edit remaining=%d spent=%d", *f.repo.issues[f.issue].Remaining, edited.Totals.SpentSeconds)
	}

	// Deleting gives the adjust back and clears the spent aggregate.
	if err := f.svc.DeleteWorklog(ctx, f.contrib, "user", id); err != nil {
		t.Fatal(err)
	}
	it := f.repo.issues[f.issue]
	if *it.Remaining != 10*3600 || it.Spent != 0 {
		t.Fatalf("after delete remaining=%d spent=%d", *it.Remaining, it.Spent)
	}
	if len(f.repo.activities) == 0 {
		t.Fatalf("activity rows must be recorded")
	}
}

func TestEditAndDeleteOwnershipMatrix(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	out, err := f.svc.LogWork(ctx, f.contrib, "user", f.issue, logInput("1h", domain.RemainingLeave))
	if err != nil {
		t.Fatal(err)
	}
	id := out.Worklog.ID
	desc := "x"
	// The lead has manage_worklogs and may edit others' entries; a contributor may not touch the lead's.
	if _, err := f.svc.UpdateWorklog(ctx, f.lead, "user", id, port.UpdateInput{Description: &desc}); err != nil {
		t.Fatalf("lead edit: %v", err)
	}
	lead, err := f.svc.LogWork(ctx, f.lead, "user", f.issue, logInput("1h", domain.RemainingLeave))
	if err != nil {
		t.Fatal(err)
	}
	if apperror.KindOf(errOnly(f.svc.UpdateWorklog(ctx, f.contrib, "user", lead.Worklog.ID, port.UpdateInput{Description: &desc}))) != apperror.KindForbidden {
		t.Fatalf("contributor editing lead entry must be forbidden")
	}
	if apperror.KindOf(errOf(f.svc.DeleteWorklog(ctx, f.contrib, "user", lead.Worklog.ID))) != apperror.KindForbidden {
		t.Fatalf("contributor deleting lead entry must be forbidden")
	}
	if apperror.KindOf(errOf(f.svc.DeleteWorklog(ctx, f.viewer, "user", id))) != apperror.KindForbidden {
		t.Fatalf("viewer delete must be forbidden")
	}
	if err := f.svc.DeleteWorklog(ctx, f.contrib, "user", id); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	if apperror.KindOf(errOf(f.svc.DeleteWorklog(ctx, f.contrib, "user", id))) != apperror.KindNotFound {
		t.Fatalf("deleted worklog must be not found")
	}
}

func errOnly[T any](_ T, err error) error { return err }

func errOf(err error) error { return err }

func errOf3[A, B any](_ A, _ B, err error) error { return err }

func TestLogWorkValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	future := now.Add(48 * time.Hour)
	in := logInput("1h", domain.RemainingLeave)
	in.StartedAt = &future
	wantCode(t, errOnly(f.svc.LogWork(ctx, f.lead, "user", f.issue, in)), domain.CodeStartedInFuture)

	in = logInput("1h", domain.RemainingLeave)
	in.TimeSpent = port.Duration{}
	wantCode(t, errOnly(f.svc.LogWork(ctx, f.lead, "user", f.issue, in)), domain.CodeDurationRequired)

	in = logInput("1x", domain.RemainingLeave)
	wantCode(t, errOnly(f.svc.LogWork(ctx, f.lead, "user", f.issue, in)), domain.CodeDurationInvalid)

	missing := uuid.New()
	wantCode(t, errOnly(f.svc.LogWork(ctx, f.lead, "user", missing, logInput("1h", domain.RemainingLeave))), "issue.not_found")
}

func TestUpdateEstimates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	orig := text("3d")
	rem := &port.Duration{Clear: true}
	out, err := f.svc.UpdateEstimates(ctx, f.contrib, "user", f.issue, port.EstimatesInput{Original: orig, Remaining: rem})
	if err != nil {
		t.Fatal(err)
	}
	if out.Totals.OriginalSeconds == nil || *out.Totals.OriginalSeconds != 3*8*3600 || out.Totals.RemainingSeconds != nil {
		t.Fatalf("estimates = %+v", out.Totals)
	}
	if len(f.repo.activities) != 2 {
		t.Fatalf("activities = %d", len(f.repo.activities))
	}
	if apperror.KindOf(errOnly(f.svc.UpdateEstimates(ctx, f.viewer, "user", f.issue, port.EstimatesInput{Original: orig}))) != apperror.KindForbidden {
		t.Fatalf("viewer estimate edit must be forbidden")
	}
	wantCode(t, errOnly(f.svc.UpdateEstimates(ctx, f.contrib, "user", f.issue, port.EstimatesInput{Original: text("-1h")})), domain.CodeDurationInvalid)
}

func TestListIssueWorklogsForOutsiderIsForbidden(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.ListIssueWorklogs(context.Background(), f.outsider, "user", f.issue)
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("kind = %v", apperror.KindOf(err))
	}
	out, err := f.svc.ListIssueWorklogs(context.Background(), f.viewer, "user", f.issue)
	if err != nil || out.Config != domain.DefaultConfig() {
		t.Fatalf("viewer list: %v", err)
	}
}

func TestTimesheetVisibilityAndTimezone(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	bogota, _ := time.LoadLocation("America/Bogota")
	f.repo.orgs[f.org] = domain.OrgTime{Location: bogota, Config: domain.DefaultConfig()}
	other := uuid.New()
	f.repo.issues[other] = port.IssueTime{IssueID: other, ProjectID: uuid.New(), OrganizationID: f.org}
	// Lead logs 23:30 Bogota (04:30 UTC of the next day) on 26th local, and 1h on 27th local.
	f.repo.worklogs[uuid.New()] = domain.Worklog{IssueID: f.issue, AuthorID: f.contrib, StartedAt: time.Date(2026, 11, 27, 4, 30, 0, 0, time.UTC), TimeSpentSeconds: 1800}
	f.repo.worklogs[uuid.New()] = domain.Worklog{IssueID: f.issue, AuthorID: f.contrib, StartedAt: time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC), TimeSpentSeconds: 3600}
	// Entry in a project the contributor cannot browse must be dropped.
	f.repo.worklogs[uuid.New()] = domain.Worklog{IssueID: other, AuthorID: f.contrib, StartedAt: time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC), TimeSpentSeconds: 999}

	ts, _, err := f.svc.Timesheet(ctx, f.contrib, "user", port.TimesheetQuery{From: "2026-11-26", To: "2026-11-27"})
	if err != nil {
		t.Fatal(err)
	}
	if ts.TotalSeconds != 5400 || len(ts.Days) != 2 || ts.Days[0].Date != "2026-11-26" || ts.Days[0].TotalSeconds != 1800 {
		t.Fatalf("timesheet = %+v", ts)
	}

	// Another user's timesheet: the contributor is not a lead, so nothing is visible.
	ts, _, err = f.svc.Timesheet(ctx, f.lead, "user", port.TimesheetQuery{From: "2026-11-26", To: "2026-11-27", UserID: &f.contrib})
	if err != nil || ts.TotalSeconds != 5400 {
		t.Fatalf("lead viewing contributor = %v %+v", err, ts)
	}
	ts, _, err = f.svc.Timesheet(ctx, f.viewer, "user", port.TimesheetQuery{From: "2026-11-26", To: "2026-11-27", UserID: &f.contrib})
	if err != nil || ts.TotalSeconds != 0 {
		t.Fatalf("viewer viewing contributor must see nothing: %v %+v", err, ts)
	}

	wantCode(t, errOf3(f.svc.Timesheet(ctx, f.contrib, "user", port.TimesheetQuery{From: "2026-11-27", To: "2026-11-26"})), domain.CodeRangeInvalid)
	wantCode(t, errOf3(f.svc.Timesheet(ctx, f.contrib, "user", port.TimesheetQuery{From: "2026-01-01", To: "2027-12-31"})), domain.CodeRangeInvalid)
}

func TestTimeSummaryGroupingAndAccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.repo.worklogs[uuid.New()] = domain.Worklog{IssueID: f.issue, AuthorID: f.contrib, AuthorName: "Contrib", StartedAt: now.Add(-time.Hour), TimeSpentSeconds: 3600}
	f.repo.worklogs[uuid.New()] = domain.Worklog{IssueID: f.issue, AuthorID: f.lead, AuthorName: "Lead", StartedAt: now.Add(-2 * time.Hour), TimeSpentSeconds: 7200}

	sum, _, err := f.svc.TimeSummary(ctx, f.viewer, "user", f.project, port.SummaryQuery{From: "2026-11-01", To: "2026-11-30", GroupBy: domain.GroupByUser})
	if err != nil || sum.TotalSeconds != 10800 || len(sum.Groups) != 2 || sum.Groups[0].Label != "Lead" {
		t.Fatalf("summary = %v %+v", err, sum)
	}
	sum, _, err = f.svc.TimeSummary(ctx, f.viewer, "user", f.project, port.SummaryQuery{From: "2026-11-01", To: "2026-11-30", GroupBy: domain.GroupByIssue})
	if err != nil || len(sum.Groups) != 1 || sum.Groups[0].TotalSeconds != 10800 {
		t.Fatalf("by issue = %v %+v", err, sum)
	}
	sum, _, err = f.svc.TimeSummary(ctx, f.viewer, "user", f.project, port.SummaryQuery{From: "2026-11-27", To: "2026-11-27", GroupBy: domain.GroupByDay})
	if err != nil || len(sum.Groups) != 1 {
		t.Fatalf("by day = %v %+v", err, sum)
	}
	_, _, err = f.svc.TimeSummary(ctx, f.outsider, "user", f.project, port.SummaryQuery{From: "2026-11-01", To: "2026-11-30", GroupBy: domain.GroupByUser})
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("outsider summary kind = %v", apperror.KindOf(err))
	}
	wantCode(t, errOf3(f.svc.TimeSummary(ctx, f.viewer, "user", f.project, port.SummaryQuery{From: "2026-11-01", To: "2026-11-30", GroupBy: "week"})), domain.CodeGroupByInvalid)
}
