package postgres

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

type v2Env struct {
	db      *sqlx.DB
	repo    *IssueRepository
	collab  *CollaborationRepository
	catalog *CatalogRepository
	org     uuid.UUID
	project uuid.UUID
	lead    uuid.UUID
}

func newV2Env(t *testing.T) v2Env {
	t.Helper()
	db := testdb.New(t)
	catalog := NewCatalogRepository(db)
	org, project, lead := catalogTestOrg(t, catalog)
	return v2Env{db: db, repo: NewIssueRepository(db), collab: NewCollaborationRepository(db), catalog: catalog, org: org, project: project, lead: lead}
}

func (e v2Env) service(publisher events.Publisher) *usecase.IssueService {
	svc := usecase.NewIssueService(e.repo, sameTeamDirectory{}, e.collab, NewWatcherRepository(e.db), allowAllAuthorizer{}).
		WithTxManager(database.NewTxManager(e.db)).WithCatalog(e.catalog)
	if publisher != nil {
		svc.WithEvents(publisher)
	}
	return svc
}

func (e v2Env) publisher() events.Publisher {
	return events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, e.db) })
}

func (e v2Env) newIssue(t *testing.T, summary string) *domain.Issue {
	t.Helper()
	issue := newKeyTestIssue(e.project, e.lead, summary)
	if err := e.repo.Save(context.Background(), issue); err != nil {
		t.Fatal(err)
	}
	return issue
}

func TestIssueV2FieldsRoundTrip(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := newKeyTestIssue(e.project, e.lead, "Campos nuevos")
	original, remaining, points := int64(28800), int64(7200), 13.5
	start, due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)
	issue.OriginalEstimateSeconds, issue.RemainingEstimateSeconds = &original, &remaining
	issue.StoryPoints, issue.StartDate, issue.DueDate, issue.Flagged = &points, &start, &due, true
	if err := e.repo.Save(ctx, issue); err != nil {
		t.Fatal(err)
	}
	got, err := e.repo.FindByID(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if *got.OriginalEstimateSeconds != original || *got.RemainingEstimateSeconds != remaining || *got.StoryPoints != 13.5 || !got.Flagged {
		t.Fatalf("round trip lost fields: %+v", got)
	}
	if domain.FormatDate(got.StartDate) != "2026-12-01" || domain.FormatDate(got.DueDate) != "2026-12-15" {
		t.Fatalf("dates = %v %v", got.StartDate, got.DueDate)
	}
	// The database enforces the same rules as the domain.
	if _, err := e.db.Exec(`UPDATE issues SET start_date = $2 WHERE id = $1`, issue.ID, due.AddDate(0, 0, 1)); err == nil {
		t.Error("start after due accepted by the database")
	}
	if _, err := e.db.Exec(`UPDATE issues SET story_points = 1000 WHERE id = $1`, issue.ID); err == nil {
		t.Error("story points above 999.5 accepted by the database")
	}
}

func TestSoftDeletedIssuesAreInvisibleToEveryIssueQuery(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	kept, gone := e.newIssue(t, "Visible"), e.newIssue(t, "Eliminada")
	if err := e.repo.SoftDelete(ctx, gone.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.SoftDelete(ctx, gone.ID, time.Now().UTC()); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("second delete = %v, want not found", err)
	}
	if _, err := e.repo.FindByID(ctx, gone.ID); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("FindByID = %v", err)
	}
	filter := port.ListFilter{ProjectIDs: []uuid.UUID{e.project}, Page: 1, PageSize: 50}
	items, err := e.repo.FindAll(ctx, filter)
	if err != nil || len(items) != 1 || items[0].ID != kept.ID {
		t.Fatalf("FindAll = %v err %v", items, err)
	}
	if total, _ := e.repo.Count(ctx, filter); total != 1 {
		t.Fatalf("Count = %d", total)
	}
	backlog, err := e.repo.ListBacklog(ctx, e.project)
	if err != nil || len(backlog) != 1 {
		t.Fatalf("ListBacklog = %d err %v", len(backlog), err)
	}
	candidates, err := e.repo.FindKeyCandidates(ctx, "CAT", gone.KeyNumber, e.org, e.lead, true)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("FindKeyCandidates = %d err %v", len(candidates), err)
	}
	gone.Summary = "cambio"
	if err := e.repo.Update(ctx, gone); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("Update on a deleted issue = %v", err)
	}
	if err := e.repo.LockForUpdate(ctx, gone.ID); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("LockForUpdate = %v", err)
	}
	if _, _, err := e.repo.ViewExtras(ctx, gone.ID); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("ViewExtras = %v", err)
	}

	found, err := e.repo.FindDeleted(ctx, gone.ID)
	if err != nil || found.DeletedAt == nil {
		t.Fatalf("FindDeleted = %v err %v", found, err)
	}
	if _, err := e.repo.FindDeleted(ctx, kept.ID); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("FindDeleted on a live issue = %v", err)
	}
	if err := e.repo.Restore(ctx, gone.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.FindByID(ctx, gone.ID); err != nil {
		t.Fatalf("restored issue not found: %v", err)
	}
}

func TestNextRankSortsAfterTheBacklog(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	first, err := e.repo.NextRank(ctx, e.project)
	if err != nil {
		t.Fatal(err)
	}
	a := e.newIssue(t, "A")
	a.Rank = first
	if err := e.repo.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	second, err := e.repo.NextRank(ctx, e.project)
	if err != nil || second <= first {
		t.Fatalf("second rank %q must sort after %q (err %v)", second, first, err)
	}
}

func TestDefaultAssigneePolicyAndChildLevels(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	policy, lead, err := e.repo.DefaultAssigneePolicy(ctx, e.project)
	if err != nil || policy != "unassigned" || lead == nil || *lead != e.lead {
		t.Fatalf("policy = %q lead %v err %v", policy, lead, err)
	}
	if _, err := e.db.Exec(`UPDATE projects SET default_assignee_policy = 'lead' WHERE id = $1`, e.project); err != nil {
		t.Fatal(err)
	}
	if policy, _, _ = e.repo.DefaultAssigneePolicy(ctx, e.project); policy != "lead" {
		t.Fatalf("policy = %q", policy)
	}

	svc := e.service(nil)
	epic, err := svc.Create(ctx, port.CreateInput{Summary: "Epica", Description: "d", ReporterID: e.lead, ProjectID: &e.project, IssueType: domain.IssueTypeEpic})
	if err != nil {
		t.Fatal(err)
	}
	if epic.AssigneeID == nil || *epic.AssigneeID != e.lead {
		t.Fatalf("lead policy did not assign the lead: %v", epic.AssigneeID)
	}
	child, err := svc.Create(ctx, port.CreateInput{Summary: "Hija", Description: "d", ReporterID: e.lead, ProjectID: &e.project, ParentID: &epic.ID})
	if err != nil {
		t.Fatalf("standard under epic: %v", err)
	}
	levels, err := e.repo.ChildLevels(ctx, epic.ID)
	if err != nil || len(levels) != 1 || levels[0] != domain.HierarchyStandard {
		t.Fatalf("levels = %v err %v", levels, err)
	}
	// An epic with a standard child cannot become a task: the child would have a standard parent.
	_, err = svc.Patch(ctx, epic.ID, e.lead, "user", port.PatchInput{IssueType: port.Some(domain.IssueTypeTask)})
	if apperror.CodeOf(err) != "issue.hierarchy_children_invalid" {
		t.Fatalf("epic to task = %v", err)
	}
	// The reporter and the assignee follow the new issue.
	var watchers int
	if err := e.db.Get(&watchers, `SELECT count(*) FROM watchers WHERE issue_id = $1`, child.ID); err != nil || watchers != 1 {
		t.Fatalf("watchers = %d err %v (the reporter is the lead and the assignee)", watchers, err)
	}
}

type failingPublisher struct{}

func (failingPublisher) Publish(context.Context, events.Event) error {
	return errors.New("outbox down")
}

func TestPatchWritesOneBatchOneEventAndRollsBackTogether(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := e.newIssue(t, "Patch")
	svc := e.service(e.publisher())

	res, err := svc.Patch(ctx, issue.ID, e.lead, "user", port.PatchInput{
		Summary: port.Some("Nuevo"), StoryPoints: port.Some(5.0), Flagged: port.Some(true), DueDate: port.Some(time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatal(err)
	}
	var batches, rows int
	if err := e.db.QueryRow(`SELECT count(DISTINCT batch_id), count(*) FROM activities WHERE issue_id = $1 AND batch_id IS NOT NULL`, issue.ID).Scan(&batches, &rows); err != nil {
		t.Fatal(err)
	}
	if batches != 1 || rows != len(res.Changes) || rows != 4 {
		t.Fatalf("batches = %d rows = %d changes = %d", batches, rows, len(res.Changes))
	}
	var updatedEvents int
	if err := e.db.Get(&updatedEvents, `SELECT count(*) FROM domain_events WHERE type = 'issue.updated' AND entity_id = $1`, issue.ID); err != nil || updatedEvents != 1 {
		t.Fatalf("issue.updated events = %d err %v", updatedEvents, err)
	}
	stored, _ := e.repo.FindByID(ctx, issue.ID)
	if !stored.UpdatedAt.Equal(res.Issue.UpdatedAt) {
		t.Fatalf("returned updated_at %v differs from stored %v", res.Issue.UpdatedAt, stored.UpdatedAt)
	}

	// A failing outbox rolls the whole patch back.
	broken := e.service(failingPublisher{})
	if _, err := broken.Patch(ctx, issue.ID, e.lead, "user", port.PatchInput{Summary: port.Some("No debe quedar")}); err == nil {
		t.Fatal("patch succeeded although the event failed")
	}
	after, _ := e.repo.FindByID(ctx, issue.ID)
	if after.Summary != "Nuevo" {
		t.Fatalf("summary = %q, the failed patch must roll back", after.Summary)
	}
	var total int
	_ = e.db.Get(&total, `SELECT count(*) FROM activities WHERE issue_id = $1 AND batch_id IS NOT NULL`, issue.ID)
	if total != 4 {
		t.Fatalf("activities = %d after rollback, want 4", total)
	}

	// Stale precondition against the real timestamps.
	stale := res.Issue.UpdatedAt.Add(-time.Second)
	if _, err := svc.Patch(ctx, issue.ID, e.lead, "user", port.PatchInput{ExpectedUpdatedAt: &stale, Summary: port.Some("x")}); apperror.CodeOf(err) != "issue.stale" {
		t.Fatalf("stale = %v", err)
	}
	fresh := stored.UpdatedAt
	if _, err := svc.Patch(ctx, issue.ID, e.lead, "user", port.PatchInput{ExpectedUpdatedAt: &fresh, Summary: port.Some("ok")}); err != nil {
		t.Fatalf("fresh precondition: %v", err)
	}
}

type queryCounter struct{ n atomic.Int64 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// countingDB opens a second pool on the schema of db that counts every statement it runs.
func countingDB(t *testing.T, db *sqlx.DB) (*sqlx.DB, *queryCounter) {
	t.Helper()
	var schema string
	if err := db.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(os.Getenv("CORELOG_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.Tracer = counter
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["search_path"] = schema
	pool := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	t.Cleanup(func() { _ = pool.Close() })
	return pool, counter
}

func TestGetViewStatementCountDoesNotGrowWithData(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := e.newIssue(t, "Vista")
	pool, counter := countingDB(t, e.db)
	svc := usecase.NewIssueService(NewIssueRepository(pool), sameTeamDirectory{}, NewCollaborationRepository(pool), NewWatcherRepository(pool), allowAllAuthorizer{}).
		WithCatalog(NewCatalogRepository(pool))

	measure := func() int64 {
		counter.n.Store(0)
		if _, err := svc.GetView(ctx, issue.ID, e.lead, "user"); err != nil {
			t.Fatal(err)
		}
		return counter.n.Load()
	}
	small := measure()

	for i := 0; i < 25; i++ {
		if _, err := e.db.Exec(`INSERT INTO comments (issue_id, author_id, body) VALUES ($1, $2, 'c')`, issue.ID, e.lead); err != nil {
			t.Fatal(err)
		}
		child := newKeyTestIssue(e.project, e.lead, "Hija")
		child.ParentID = &issue.ID
		if err := e.repo.Save(ctx, child); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, 'relates', $3)`, issue.ID, child.ID, e.lead); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`INSERT INTO watchers (issue_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, child.ID, e.lead); err != nil {
			t.Fatal(err)
		}
	}
	large := measure()
	if small != large {
		t.Fatalf("statements: %d with no data, %d with 25 comments, children and links", small, large)
	}
	if small > 4 {
		t.Fatalf("the view costs %d statements, expected a small constant", small)
	}
	view, err := svc.GetView(ctx, issue.ID, e.lead, "user")
	if err != nil {
		t.Fatal(err)
	}
	if view.Counts.Comments != 25 || view.Counts.Children != 25 || view.Counts.Links != 25 || view.Breadcrumb.ProjectKey != "CAT" {
		t.Fatalf("counts = %+v breadcrumb = %+v", view.Counts, view.Breadcrumb)
	}
}
