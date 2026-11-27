package usecase_test

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/sprint/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/application/usecase"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

const testOrg = "00000000-0000-4000-8000-000000000002"

// roleAuthorizer answers like the project matrix: unknown users do not see the project (404), viewers read
// only, contributors also move cards and leads manage sprints.
type roleAuthorizer struct {
	roles map[uuid.UUID]string
}

func (a roleAuthorizer) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, _ uuid.UUID) error {
	role, ok := a.roles[actor]
	if !ok {
		return apperror.WithCode(apperror.KindNotFound, "not_found", "No encontrado")
	}
	allowed := map[string][]string{
		port.ActionViewProject:   {"viewer", "contributor", "lead"},
		port.ActionMoveCards:     {"contributor", "lead"},
		port.ActionManageSprints: {"lead"},
	}[action]
	for _, r := range allowed {
		if r == role {
			return nil
		}
	}
	return apperror.WithCode(apperror.KindForbidden, "forbidden", "Sin permiso")
}

type env struct {
	t       *testing.T
	db      *sqlx.DB
	svc     *usecase.SprintService
	clk     *clock.FakeClock
	project uuid.UUID
	lead    uuid.UUID
	contrib uuid.UUID
	viewer  uuid.UUID
	outside uuid.UUID
	keyNum  int
	rankSeq int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db := testdb.New(t)
	e := &env{t: t, db: db, clk: &clock.FakeClock{T: time.Now().UTC().Add(-72 * time.Hour)},
		project: uuid.New(), lead: uuid.New(), contrib: uuid.New(), viewer: uuid.New(), outside: uuid.New()}
	for name, id := range map[string]uuid.UUID{"lead": e.lead, "contrib": e.contrib, "viewer": e.viewer, "outside": e.outside} {
		e.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
	}
	e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, 'SPR', 'Sprints', 'scrum', 'scrum', $3, 'private')`, e.project, testOrg, e.lead)
	e.exec(`SELECT corelog_ensure_workflow($1)`, e.project)
	roles := roleAuthorizer{roles: map[uuid.UUID]string{e.lead: "lead", e.contrib: "contributor", e.viewer: "viewer"}}
	e.svc = usecase.NewSprintService(postgres.NewSprintRepository(db), roles, database.NewTxManager(db), e.clk)
	return e
}

func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		e.t.Fatalf("%v\n%s", err, query)
	}
}

func (e *env) issue(project uuid.UUID, summary, itype string, points *float64) uuid.UUID {
	e.t.Helper()
	e.keyNum++
	e.rankSeq++
	id := uuid.New()
	e.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, story_points, issue_type_id, priority_id)
		VALUES ($1, $2, $3, '', 'open', 'medium', $4, $5::text, $6, $7, $8,
			(SELECT id FROM issue_types WHERE organization_id = $9 AND legacy_key = $5::text),
			(SELECT id FROM priorities WHERE organization_id = $9 AND legacy_key = 'medium'))`,
		id, project, summary, e.lead, itype, e.keyNum, "r"+string(rune('a'+e.rankSeq%26))+string(rune('a'+e.rankSeq/26)), points, testOrg)
	return id
}

func (e *env) pts(v float64) *float64 { return &v }

func (e *env) done(issue uuid.UUID) {
	e.exec(`UPDATE issues SET status = 'closed', closed_at = $2 WHERE id = $1`, issue, e.clk.Now())
}

func (e *env) sprintOf(issue uuid.UUID) *uuid.UUID {
	e.t.Helper()
	var id *uuid.UUID
	if err := e.db.Get(&id, `SELECT sprint_id FROM issues WHERE id = $1`, issue); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) newSprint(name string) *domain.Sprint {
	e.t.Helper()
	sp, err := e.svc.Create(context.Background(), e.lead, "user", e.project, name, "meta", nil, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return sp
}

func (e *env) assign(sprint, issue uuid.UUID) {
	e.t.Helper()
	if err := e.svc.AssignIssue(context.Background(), e.lead, "user", e.project, sprint, issue); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) start(sprint uuid.UUID) *domain.Sprint {
	e.t.Helper()
	sp, err := e.svc.Start(context.Background(), e.lead, "user", e.project, sprint, port.StartInput{})
	if err != nil {
		e.t.Fatal(err)
	}
	return sp
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if apperror.CodeOf(err) != code {
		t.Fatalf("error = %v (code %q), want code %q", err, apperror.CodeOf(err), code)
	}
}

func (e *env) eventCount(sprint uuid.UUID, kind string) int {
	e.t.Helper()
	var n int
	if err := e.db.Get(&n, `SELECT count(*) FROM sprint_issue_events WHERE sprint_id = $1 AND event = $2`, sprint, kind); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func TestLifecycleRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	s1, s2, s3 := e.newSprint("Sprint 1"), e.newSprint("Sprint 2"), e.newSprint("Sprint 3")
	if s1.Status != domain.StatusPlanned {
		t.Fatalf("status %s", s1.Status)
	}

	// Empty sprint cannot start unless forced.
	_, err := e.svc.Start(ctx, e.lead, "user", e.project, s1.ID, port.StartInput{})
	wantCode(t, err, "sprint.empty")
	if apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("kind %v", apperror.KindOf(err))
	}
	forced, err := e.svc.Start(ctx, e.lead, "user", e.project, s1.ID, port.StartInput{Force: true})
	if err != nil || forced.Status != domain.StatusActive || forced.StartedAt == nil || forced.CommittedPoints == nil || *forced.CommittedPoints != 0 {
		t.Fatalf("forced start: %+v, %v", forced, err)
	}
	if forced.StartDate == nil || forced.EndDate == nil {
		t.Fatal("start must fix dates")
	}

	// Only one active sprint per project.
	e.assign(s2.ID, e.issue(e.project, "tarea", "task", e.pts(1)))
	_, err = e.svc.Start(ctx, e.lead, "user", e.project, s2.ID, port.StartInput{})
	wantCode(t, err, "sprint.already_active")
	if apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("kind %v", apperror.KindOf(err))
	}

	// Delete: only planned sprints; the issues go back to the backlog.
	err = e.svc.Delete(ctx, e.lead, "user", e.project, s1.ID)
	wantCode(t, err, "sprint.not_planned")
	if err := e.svc.Delete(ctx, e.lead, "user", e.project, s2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.lead, "user", e.project, s2.ID); apperror.CodeOf(err) != "sprint.not_found" {
		t.Fatalf("deleted sprint still visible: %v", err)
	}

	// Update: planned and active edit, closed does not.
	name := "Sprint tres"
	up, err := e.svc.Update(ctx, e.lead, "user", e.project, s3.ID, domain.Patch{Name: &name})
	if err != nil || up.Name != name {
		t.Fatalf("update: %+v %v", up, err)
	}
	if _, err := e.svc.Complete(ctx, e.lead, "user", e.project, s1.ID, port.CompleteInput{}); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Update(ctx, e.lead, "user", e.project, s1.ID, domain.Patch{Name: &name})
	wantCode(t, err, "sprint.closed")
	_, err = e.svc.Complete(ctx, e.lead, "user", e.project, s1.ID, port.CompleteInput{})
	wantCode(t, err, "sprint.not_active")
	_, err = e.svc.Start(ctx, e.lead, "user", e.project, s1.ID, port.StartInput{Force: true})
	wantCode(t, err, "sprint.not_started")

	// A sprint of another project is not found.
	other := uuid.New()
	e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'OTH', 'Otro', 'scrum', 'scrum', $3, 'private')`, other, testOrg, e.lead)
	_, err = e.svc.Get(ctx, e.lead, "user", other, s3.ID)
	wantCode(t, err, "sprint.not_found")
}

func TestPermissionsPerRole(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sp := e.newSprint("Sprint 1")
	issue := e.issue(e.project, "a", "task", e.pts(2))

	for name, actor := range map[string]uuid.UUID{"viewer": e.viewer, "contributor": e.contrib} {
		if _, err := e.svc.Create(ctx, actor, "user", e.project, "X", "", nil, nil); apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("%s create: %v", name, err)
		}
		if err := e.svc.AssignIssue(ctx, actor, "user", e.project, sp.ID, issue); apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("%s assign: %v", name, err)
		}
		if _, err := e.svc.Start(ctx, actor, "user", e.project, sp.ID, port.StartInput{Force: true}); apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("%s start: %v", name, err)
		}
		if err := e.svc.Delete(ctx, actor, "user", e.project, sp.ID); apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("%s delete: %v", name, err)
		}
		if _, err := e.svc.Backlog(ctx, actor, "user", e.project, port.BacklogFilter{}); err != nil {
			t.Fatalf("%s backlog: %v", name, err)
		}
	}
	// The outsider does not even see the project.
	if _, err := e.svc.Backlog(ctx, e.outside, "user", e.project, port.BacklogFilter{}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider backlog: %v", err)
	}
	if _, err := e.svc.Report(ctx, e.outside, "user", e.project, sp.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider report: %v", err)
	}
	if _, err := e.svc.Rank(ctx, e.outside, "user", port.RankInput{IssueIDs: []uuid.UUID{issue}}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider rank: %v", err)
	}
	if _, err := e.svc.Rank(ctx, e.viewer, "user", port.RankInput{IssueIDs: []uuid.UUID{issue}}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer rank: %v", err)
	}
}

// TestScopeHistoryAndReport runs add, remove, re-add, move, points change and completion to a new sprint.
func TestScopeHistoryAndReport(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sp := e.newSprint("Sprint 12")
	a := e.issue(e.project, "A", "story", e.pts(5))
	b := e.issue(e.project, "B", "task", e.pts(3))
	c := e.issue(e.project, "C", "task", e.pts(2))
	for _, id := range []uuid.UUID{a, b, c} {
		e.assign(sp.ID, id)
	}
	if n := e.eventCount(sp.ID, "added"); n != 0 {
		t.Fatalf("planned sprint must not record scope events, got %d", n)
	}
	started := e.start(sp.ID)
	if started.CommittedPoints == nil || *started.CommittedPoints != 10 {
		t.Fatalf("committed = %v", started.CommittedPoints)
	}
	var snapshot int
	_ = e.db.Get(&snapshot, `SELECT count(*) FROM sprint_issue_events WHERE sprint_id = $1 AND event = 'added' AND NOT after_start`, sp.ID)
	if snapshot != 3 {
		t.Fatalf("snapshot events = %d", snapshot)
	}

	// Day 1: D joins (8 points), B leaves.
	e.clk.Advance(24 * time.Hour)
	d := e.issue(e.project, "D", "task", e.pts(8))
	e.assign(sp.ID, d)
	if err := e.svc.RemoveIssue(ctx, e.lead, "user", e.project, sp.ID, b); err != nil {
		t.Fatal(err)
	}
	// Day 2: B comes back; A is finished; C is re-estimated from 2 to 4 (a trigger records it).
	e.clk.Advance(24 * time.Hour)
	e.assign(sp.ID, b)
	e.done(a)
	e.exec(`UPDATE issues SET story_points = 4 WHERE id = $1`, c)
	if n := e.eventCount(sp.ID, "points_changed"); n != 1 {
		t.Fatalf("points_changed events = %d", n)
	}

	live, err := e.svc.Report(ctx, e.lead, "user", e.project, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.CommittedPoints != 10 || live.CompletedPoints != 5 || live.IncompletePoints != 15 || len(live.AddedAfterStart) != 1 || len(live.Removed) != 0 {
		t.Fatalf("live report committed=%v completed=%v incomplete=%v added=%d removed=%d",
			live.CommittedPoints, live.CompletedPoints, live.IncompletePoints, len(live.AddedAfterStart), len(live.Removed))
	}

	// Completion to a brand new sprint in one transaction.
	res, err := e.svc.Complete(ctx, e.lead, "user", e.project, sp.ID, port.CompleteInput{Destination: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if res.NextSprint == nil || res.NextSprint.Name != "Sprint 13" || res.NextSprint.Status != domain.StatusPlanned {
		t.Fatalf("next sprint %+v", res.NextSprint)
	}
	if res.CompletedIssues != 1 || res.MovedIssues != 3 || res.CompletedPoints != 5 || res.CommittedPoints != 10 {
		t.Fatalf("result %+v", res)
	}
	if got := e.sprintOf(a); got == nil || *got != sp.ID {
		t.Fatal("completed issues stay in the closed sprint")
	}
	for _, id := range []uuid.UUID{b, c, d} {
		if got := e.sprintOf(id); got == nil || *got != res.NextSprint.ID {
			t.Fatalf("incomplete issue %s not moved to the new sprint", id)
		}
	}
	if res.Sprint.CompletedPoints == nil || *res.Sprint.CompletedPoints != 5 || res.Sprint.CompletedAt == nil || res.Sprint.Status != domain.StatusClosed {
		t.Fatalf("closed sprint %+v", res.Sprint)
	}

	report, err := e.svc.Report(ctx, e.lead, "user", e.project, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.CommittedPoints != 10 || report.CompletedPoints != 5 || report.IncompletePoints != 15 || report.AddedPoints != 8 {
		t.Fatalf("report %+v", report.Report)
	}
	if len(report.Completed) != 1 || report.Completed[0].Key != "SPR-"+keyOf(t, e, a) || len(report.Incomplete) != 3 || len(report.AddedAfterStart) != 1 || len(report.Removed) != 0 {
		t.Fatalf("buckets: completed=%d incomplete=%d added=%d removed=%d", len(report.Completed), len(report.Incomplete), len(report.AddedAfterStart), len(report.Removed))
	}
	for _, i := range report.Incomplete {
		if i.Summary == "C" && (i.Points == nil || *i.Points != 4) {
			t.Fatalf("C must report its re-estimated points, got %v", i.Points)
		}
	}
	if len(report.Burndown) == 0 || len(report.Velocity) != 1 || report.Velocity[0].Points != 5 {
		t.Fatalf("burndown %d velocity %+v", len(report.Burndown), report.Velocity)
	}
	if first := report.Burndown[0]; first.Scope != 10 || first.Remaining != 10 {
		t.Fatalf("first burndown day %+v", first)
	}

	// The old burndown endpoint still answers from the same history.
	if points, err := e.svc.Burndown(ctx, e.lead, "user", e.project, sp.ID); err != nil || len(points) == 0 {
		t.Fatalf("burndown: %d %v", len(points), err)
	}
}

func keyOf(t *testing.T, e *env, issue uuid.UUID) string {
	t.Helper()
	var n string
	if err := e.db.Get(&n, `SELECT key_number::text FROM issues WHERE id = $1`, issue); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRemovedIssuesAppearInTheReport(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sp := e.newSprint("Sprint 1")
	a, b := e.issue(e.project, "A", "task", e.pts(5)), e.issue(e.project, "B", "task", e.pts(3))
	e.assign(sp.ID, a)
	e.assign(sp.ID, b)
	e.start(sp.ID)
	e.clk.Advance(time.Hour)
	if err := e.svc.RemoveIssue(ctx, e.lead, "user", e.project, sp.ID, b); err != nil {
		t.Fatal(err)
	}
	r, err := e.svc.Report(ctx, e.lead, "user", e.project, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Removed) != 1 || r.Removed[0].IssueID != b || r.RemovedPoints != 3 || r.CommittedPoints != 8 || r.IncompletePoints != 5 {
		t.Fatalf("report %+v removed %+v", r.Report, r.Removed)
	}
	// Soft-deleted issues disappear from the report and from the completion.
	e.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, a)
	r, _ = e.svc.Report(ctx, e.lead, "user", e.project, sp.ID)
	if len(r.Incomplete) != 0 || len(r.Completed) != 0 {
		t.Fatalf("deleted issue still reported: %+v", r.Incomplete)
	}
	res, err := e.svc.Complete(ctx, e.lead, "user", e.project, sp.ID, port.CompleteInput{})
	if err != nil || res.MovedIssues != 0 {
		t.Fatalf("complete: %+v %v", res, err)
	}
}

func TestCompleteDestinations(t *testing.T) {
	ctx := context.Background()
	t.Run("backlog", func(t *testing.T) {
		e := newEnv(t)
		sp := e.newSprint("S1")
		open, finished := e.issue(e.project, "open", "task", e.pts(2)), e.issue(e.project, "fin", "task", e.pts(3))
		e.assign(sp.ID, open)
		e.assign(sp.ID, finished)
		e.start(sp.ID)
		e.done(finished)
		if _, err := e.svc.Complete(ctx, e.lead, "user", e.project, sp.ID, port.CompleteInput{Destination: "backlog"}); err != nil {
			t.Fatal(err)
		}
		if e.sprintOf(open) != nil || e.sprintOf(finished) == nil {
			t.Fatal("open goes to the backlog, finished stays")
		}
		if e.eventCount(sp.ID, "completed") != 1 || e.eventCount(sp.ID, "incomplete") != 1 {
			t.Fatal("completion events missing")
		}
	})
	t.Run("planned sprint and legacy options", func(t *testing.T) {
		e := newEnv(t)
		sp, next := e.newSprint("S1"), e.newSprint("S2")
		open := e.issue(e.project, "open", "task", e.pts(2))
		e.assign(sp.ID, open)
		e.start(sp.ID)
		res, err := e.svc.Complete(ctx, e.lead, "user", e.project, sp.ID, port.CompleteInput{Unfinished: "sprint", TargetSprintID: &next.ID})
		if err != nil || res.NextSprint == nil || res.NextSprint.ID != next.ID {
			t.Fatalf("%+v %v", res, err)
		}
		if got := e.sprintOf(open); got == nil || *got != next.ID {
			t.Fatal("not moved to the planned sprint")
		}
	})
	t.Run("invalid destinations change nothing", func(t *testing.T) {
		e := newEnv(t)
		sp, other := e.newSprint("S1"), e.newSprint("S2")
		open := e.issue(e.project, "open", "task", e.pts(2))
		e.assign(sp.ID, open)
		e.start(sp.ID)
		e.exec(`UPDATE sprints SET status = 'closed' WHERE id = $1`, other.ID)
		foreign := uuid.New()
		e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'OTH', 'Otro', 'scrum', 'scrum', $3, 'private')`, foreign, testOrg, e.lead)
		foreignSprint, _ := e.svc.Create(ctx, e.lead, "user", foreign, "X", "", nil, nil)
		for _, dest := range []string{"sprint:" + sp.ID.String(), "sprint:" + other.ID.String(), "sprint:" + foreignSprint.ID.String(), "sprint:" + uuid.NewString(), "nope"} {
			_, err := e.svc.Complete(ctx, e.lead, "user", e.project, sp.ID, port.CompleteInput{Destination: dest})
			if apperror.KindOf(err) != apperror.KindValidation {
				t.Fatalf("destination %q: %v", dest, err)
			}
		}
		got, _ := e.svc.Get(ctx, e.lead, "user", e.project, sp.ID)
		if got.Status != domain.StatusActive || e.eventCount(sp.ID, "incomplete") != 0 || e.sprintOf(open) == nil {
			t.Fatal("a rejected completion must leave the sprint untouched")
		}
	})
}

func TestConcurrentStartsOnlyOneWins(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var ids []uuid.UUID
	for _, name := range []string{"A", "B", "C"} {
		sp := e.newSprint(name)
		e.assign(sp.ID, e.issue(e.project, name, "task", e.pts(1)))
		ids = append(ids, sp.ID)
	}
	var wg sync.WaitGroup
	errs := make([]error, len(ids))
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.svc.Start(ctx, e.lead, "user", e.project, id, port.StartInput{})
		}()
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case apperror.CodeOf(err) != "sprint.already_active" || apperror.KindOf(err) != apperror.KindConflict:
			t.Fatalf("loser must get 409 sprint.already_active, got %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d sprints started, want exactly 1", wins)
	}
	var active int
	_ = e.db.Get(&active, `SELECT count(*) FROM sprints WHERE project_id = $1 AND status = 'active'`, e.project)
	if active != 1 {
		t.Fatalf("active sprints = %d", active)
	}
}

func TestBacklogSectionsFiltersAndTotals(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	epic := e.issue(e.project, "Epica pagos", "epic", nil)
	active, planned := e.newSprint("Activo"), e.newSprint("Planificado")
	closed := e.newSprint("Viejo")
	inActive := e.issue(e.project, "Pagar con tarjeta", "story", e.pts(5))
	inPlanned := e.issue(e.project, "Reembolsos", "task", e.pts(3))
	loose := e.issue(e.project, "Suelta uno", "task", e.pts(2))
	looseDone := e.issue(e.project, "Hecha sin sprint", "task", e.pts(1))
	child := e.issue(e.project, "Hija de epica", "task", e.pts(1))
	e.exec(`UPDATE issues SET parent_id = $2, original_estimate_seconds = 3600 WHERE id = $1`, child, epic)
	e.exec(`UPDATE issues SET assignee_id = $2 WHERE id = $1`, inActive, e.contrib)
	e.assign(active.ID, inActive)
	e.assign(planned.ID, inPlanned)
	e.done(looseDone)
	deleted := e.issue(e.project, "Borrada", "task", e.pts(9))
	e.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, deleted)
	e.exec(`UPDATE sprints SET status = 'closed' WHERE id = $1`, closed.ID)
	oldIssue := e.issue(e.project, "En sprint cerrado", "task", e.pts(4))
	e.exec(`UPDATE issues SET sprint_id = $2 WHERE id = $1`, oldIssue, closed.ID)
	e.start(active.ID)
	e.exec(`INSERT INTO labels (id, project_id, name, color) VALUES ($1, $2, 'api', 'blue')`, uuid.New(), e.project)
	e.exec(`INSERT INTO issue_labels (issue_id, label_id) SELECT $1, id FROM labels WHERE project_id = $2`, inActive, e.project)

	b, err := e.svc.Backlog(ctx, e.contrib, "user", e.project, port.BacklogFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Sections) != 3 || b.Sections[0].Kind != "active" || b.Sections[1].Kind != "planned" || b.Sections[2].Kind != "backlog" {
		t.Fatalf("sections %+v", kinds(b))
	}
	if !b.CanMoveCards || b.CanManageSprint {
		t.Fatalf("contributor flags move=%v manage=%v", b.CanMoveCards, b.CanManageSprint)
	}
	if got := ids(b.Sections[0]); len(got) != 1 || got[0] != inActive {
		t.Fatalf("active section %v", got)
	}
	issue := b.Sections[0].Issues[0]
	if len(issue.Labels) != 1 || issue.Labels[0].Name != "api" || issue.AssigneeName != "contrib" || issue.Key == "" || issue.StatusCategory != "todo" {
		t.Fatalf("issue row %+v", issue)
	}
	if got := ids(b.Sections[1]); len(got) != 1 || got[0] != inPlanned {
		t.Fatalf("planned section %v", got)
	}
	// Backlog: loose + child, without the epic, the done one, the deleted one and the closed sprint issue.
	if got := ids(b.Sections[2]); len(got) != 2 || !contains(got, loose) || !contains(got, child) {
		t.Fatalf("backlog section %v", got)
	}
	if tot := b.Sections[2].Totals; tot.Issues != 2 || tot.Points != 3 || tot.EstimateSeconds != 3600 || tot.ByCategory["todo"].Issues != 2 {
		t.Fatalf("totals %+v", tot)
	}
	if len(b.Epics) != 1 || b.Epics[0].Total != 1 || b.Epics[0].Done != 0 || b.Epics[0].ID != epic {
		t.Fatalf("epics %+v", b.Epics)
	}
	if child := b.Sections[2].Issues; child[0].EpicID == nil && child[1].EpicID == nil {
		t.Fatal("epic reference missing on the child issue")
	}

	byEpic, _ := e.svc.Backlog(ctx, e.contrib, "user", e.project, port.BacklogFilter{EpicID: &epic})
	if total := len(ids(byEpic.Sections[0])) + len(ids(byEpic.Sections[1])) + len(ids(byEpic.Sections[2])); total != 1 {
		t.Fatalf("epic filter returned %d issues", total)
	}
	byQuery, _ := e.svc.Backlog(ctx, e.contrib, "user", e.project, port.BacklogFilter{Query: "tarjeta"})
	if len(ids(byQuery.Sections[0])) != 1 || len(ids(byQuery.Sections[2])) != 0 {
		t.Fatal("text filter wrong")
	}
	byKey, _ := e.svc.Backlog(ctx, e.contrib, "user", e.project, port.BacklogFilter{Query: "spr-" + keyOf(t, e, loose)})
	if len(ids(byKey.Sections[2])) != 1 {
		t.Fatal("key filter wrong")
	}
	// A % in the query is literal text, not a wildcard.
	if wild, _ := e.svc.Backlog(ctx, e.contrib, "user", e.project, port.BacklogFilter{Query: "%"}); len(ids(wild.Sections[2]))+len(ids(wild.Sections[0])) != 0 {
		t.Fatal("percent sign must not act as a wildcard")
	}
}

func kinds(b *port.Backlog) []string {
	var out []string
	for _, s := range b.Sections {
		out = append(out, s.Kind)
	}
	return out
}

func ids(s port.BacklogSection) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(s.Issues))
	for _, i := range s.Issues {
		out = append(out, i.ID)
	}
	return out
}

func contains(list []uuid.UUID, id uuid.UUID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func TestBacklogIsolationBetweenProjects(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	other := uuid.New()
	e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'OTH', 'Otro', 'scrum', 'scrum', $3, 'private')`, other, testOrg, e.lead)
	mine := e.issue(e.project, "mia", "task", nil)
	theirs := e.issue(other, "ajena", "task", nil)
	b, _ := e.svc.Backlog(ctx, e.lead, "user", e.project, port.BacklogFilter{})
	if got := ids(b.Sections[0]); len(got) != 1 || got[0] != mine {
		t.Fatalf("backlog leaks other projects: %v", got)
	}
	// A move that mixes projects, or names a foreign issue for this project, is rejected.
	_, err := e.svc.Rank(ctx, e.lead, "user", port.RankInput{IssueIDs: []uuid.UUID{mine, theirs}})
	wantCode(t, err, "sprint.issue_not_in_project")
	_, err = e.svc.Rank(ctx, e.lead, "user", port.RankInput{ProjectID: &e.project, IssueIDs: []uuid.UUID{theirs}})
	wantCode(t, err, "sprint.issue_not_in_project")
	sp := e.newSprint("S")
	_, err = e.svc.Rank(ctx, e.lead, "user", port.RankInput{ProjectID: &other, IssueIDs: []uuid.UUID{theirs}, TargetSprintID: &sp.ID})
	wantCode(t, err, "sprint.not_found")
}

func TestRankMultiMoveAndNeighbours(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var order []uuid.UUID
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		order = append(order, e.issue(e.project, n, "task", e.pts(1)))
	}
	e.exec(`UPDATE issues SET rank = 'm' WHERE project_id = $1`, e.project) // collisions force a rebalance
	sectionOrder := func() []uuid.UUID {
		b, err := e.svc.Backlog(ctx, e.lead, "user", e.project, port.BacklogFilter{})
		if err != nil {
			t.Fatal(err)
		}
		return ids(b.Sections[len(b.Sections)-1])
	}
	// Give them a deterministic order first: a b c d e.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: order}); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, sectionOrder(), order)

	// Move d and b (in that order) before a.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{order[3], order[1]}, BeforeIssueID: &order[0]}); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, sectionOrder(), []uuid.UUID{order[3], order[1], order[0], order[2], order[4]})

	// Move e after b.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{order[4]}, AfterIssueID: &order[1]}); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, sectionOrder(), []uuid.UUID{order[3], order[1], order[4], order[0], order[2]})

	// Anchor rules.
	_, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{order[0]}, BeforeIssueID: &order[0]})
	wantCode(t, err, "backlog.invalid_anchor")
	stranger := uuid.New()
	_, err = e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{order[0]}, AfterIssueID: &stranger})
	wantCode(t, err, "backlog.invalid_anchor")
	_, err = e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{order[0]}, BeforeIssueID: &order[1], AfterIssueID: &order[2]})
	wantCode(t, err, "backlog.invalid_anchor")
	_, err = e.svc.Rank(ctx, e.contrib, "user", port.RankInput{})
	wantCode(t, err, "backlog.no_issues")

	// Duplicated ids move once.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{order[2], order[2]}, BeforeIssueID: &order[3]}); err != nil {
		t.Fatal(err)
	}
	if got := sectionOrder(); got[0] != order[2] || len(got) != 5 {
		t.Fatalf("after duplicate move: %v", got)
	}
}

func assertOrder(t *testing.T, got, want []uuid.UUID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order has %d issues, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("position %d differs: got %v want %v", i, got, want)
		}
	}
}

func TestRankBetweenSectionsRecordsScopeAndChecksPermissions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	active, planned := e.newSprint("Activo"), e.newSprint("Plan")
	seed := e.issue(e.project, "seed", "task", e.pts(1))
	e.assign(active.ID, seed)
	e.start(active.ID)
	e.clk.Advance(time.Hour)
	x, y := e.issue(e.project, "x", "task", e.pts(3)), e.issue(e.project, "y", "task", e.pts(5))
	z := e.issue(e.project, "z", "task", e.pts(2))

	// Contributors may pull backlog issues into the active sprint: recorded as added after start.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{x, y}, TargetSprintID: &active.ID, AfterIssueID: &seed}); err != nil {
		t.Fatal(err)
	}
	var added int
	_ = e.db.Get(&added, `SELECT count(*) FROM sprint_issue_events WHERE sprint_id = $1 AND event = 'added' AND after_start`, active.ID)
	if added != 2 || e.sprintOf(x) == nil {
		t.Fatalf("added after start = %d", added)
	}
	// ...but a planned sprint needs manage_sprint.
	_, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{z}, TargetSprintID: &planned.ID})
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor into planned sprint: %v", err)
	}
	if e.sprintOf(z) != nil {
		t.Fatal("a forbidden move must not change anything")
	}
	if _, err := e.svc.Rank(ctx, e.lead, "user", port.RankInput{IssueIDs: []uuid.UUID{z}, TargetSprintID: &planned.ID}); err != nil {
		t.Fatal(err)
	}
	if e.eventCount(planned.ID, "added") != 0 {
		t.Fatal("planned sprints record no scope events")
	}
	// Active to planned is a move out of the active scope; lead only.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{y}, TargetSprintID: &planned.ID}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor active->planned: %v", err)
	}
	if _, err := e.svc.Rank(ctx, e.lead, "user", port.RankInput{IssueIDs: []uuid.UUID{y}, TargetSprintID: &planned.ID}); err != nil {
		t.Fatal(err)
	}
	if e.eventCount(active.ID, "moved_out") != 1 {
		t.Fatal("moved_out event missing")
	}
	// Back to the backlog from the active sprint needs only move_cards: removed.
	if _, err := e.svc.Rank(ctx, e.contrib, "user", port.RankInput{IssueIDs: []uuid.UUID{x}}); err != nil {
		t.Fatal(err)
	}
	if e.eventCount(active.ID, "removed") != 1 || e.sprintOf(x) != nil {
		t.Fatal("removed event or sprint change missing")
	}
	// Reordering inside the backlog needs only move_cards; a closed sprint cannot be a target.
	closed := e.newSprint("Cerrado")
	e.exec(`UPDATE sprints SET status = 'closed' WHERE id = $1`, closed.ID)
	_, err = e.svc.Rank(ctx, e.lead, "user", port.RankInput{IssueIDs: []uuid.UUID{x}, TargetSprintID: &closed.ID})
	wantCode(t, err, "sprint.closed")
}

// TestBacklogStatementCountIsConstant is the N+1 guard: the read costs the same with many issues.
func TestBacklogStatementCountIsConstant(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var schema string
	if err := e.db.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(os.Getenv("CORELOG_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.Tracer = counter
	cfg.RuntimeParams = map[string]string{"search_path": schema}
	pool := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	t.Cleanup(func() { _ = pool.Close() })
	repo := postgres.NewSprintRepository(pool)

	sp := e.newSprint("S")
	e.assign(sp.ID, e.issue(e.project, "uno", "task", e.pts(1)))
	e.start(sp.ID)
	measure := func() int64 {
		counter.n.Store(0)
		if _, err := repo.Backlog(ctx, e.project, port.BacklogFilter{}); err != nil {
			t.Fatal(err)
		}
		return counter.n.Load()
	}
	small := measure()
	e.exec(`INSERT INTO labels (id, project_id, name, color) VALUES (gen_random_uuid(), $1, 'l', 'blue')`, e.project)
	for i := 0; i < 30; i++ {
		id := e.issue(e.project, "n", "task", e.pts(1))
		e.exec(`INSERT INTO issue_labels (issue_id, label_id) SELECT $1, id FROM labels WHERE project_id = $2`, id, e.project)
		if i%3 == 0 {
			e.assign(sp.ID, id)
		}
		e.issue(e.project, "epica", "epic", nil)
	}
	if large := measure(); small != large || small > 4 {
		t.Fatalf("statements: %d with one issue, %d with 60 issues and labels (want equal and <= 4)", small, large)
	}
}

type queryCounter struct{ n atomic.Int64 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
