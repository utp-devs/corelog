package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const linkTestOrg = "00000000-0000-4000-8000-000000000002"

// linkEnv holds a project with two issues, versions and components for the link tests.
type linkEnv struct {
	db       *sqlx.DB
	repo     *VersionLinkRepository
	issues   *IssueRepository
	project  uuid.UUID
	other    uuid.UUID
	reporter uuid.UUID
}

func newLinkEnv(t *testing.T) *linkEnv {
	t.Helper()
	db := testdb.New(t)
	e := &linkEnv{db: db, repo: NewVersionLinkRepository(db), issues: NewIssueRepository(db), project: uuid.New(), other: uuid.New(), reporter: uuid.New()}
	e.exec(t, `INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'rep', 'rep', 'rep@example.test', 'x', 'user')`, e.reporter)
	for _, p := range []uuid.UUID{e.project, e.other} {
		e.exec(t, `INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
			VALUES ($1, $2, $3, 'P', 'scrum', 'scrum', $4, 'private')`, p, linkTestOrg, "K"+p.String()[:6], e.reporter)
		e.exec(t, `SELECT corelog_ensure_workflow($1)`, p)
	}
	return e
}

func (e *linkEnv) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
}

func (e *linkEnv) issue(t *testing.T, project uuid.UUID, key int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	e.exec(t, `INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id)
		VALUES ($1, $2, 'Incidencia', '', 'open', 'medium', $3, 'task', $4, $5,
			(SELECT id FROM issue_types WHERE organization_id = $6 AND legacy_key = 'task'),
			(SELECT id FROM priorities WHERE organization_id = $6 AND legacy_key = 'medium'))`,
		id, project, e.reporter, key, "r"+string(rune('a'+key)), linkTestOrg)
	return id
}

func (e *linkEnv) version(t *testing.T, project uuid.UUID, name, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	e.exec(t, `INSERT INTO versions (id, project_id, name, status) VALUES ($1, $2, $3, $4)`, id, project, name, status)
	return id
}

func TestVersionLinksResolveWithinProjectAndBatchLoad(t *testing.T) {
	e := newLinkEnv(t)
	ctx := context.Background()
	own := e.version(t, e.project, "1.0", "unreleased")
	foreign := e.version(t, e.other, "9.9", "unreleased")
	component := uuid.New()
	e.exec(t, `INSERT INTO components (id, project_id, name) VALUES ($1, $2, 'Pagos')`, component, e.project)
	first := e.issue(t, e.project, 1)
	second := e.issue(t, e.project, 2)

	versions, err := e.repo.ResolveVersions(ctx, e.project, []uuid.UUID{own, foreign})
	if err != nil || len(versions) != 1 || versions[0].ID != own || versions[0].Status != "unreleased" {
		t.Fatalf("cross-project version resolved: %+v %v", versions, err)
	}
	if err := e.repo.ReplaceIssueVersions(ctx, first, "fix", []uuid.UUID{own}); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.ReplaceIssueVersions(ctx, first, "affects", []uuid.UUID{own}); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.ReplaceIssueComponents(ctx, first, []uuid.UUID{component}); err != nil {
		t.Fatal(err)
	}

	links, err := e.repo.LoadLinks(ctx, []uuid.UUID{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if got := links[first]; len(got.FixVersions) != 1 || len(got.AffectsVersions) != 1 || len(got.Components) != 1 || got.Components[0].Name != "Pagos" {
		t.Fatalf("links of first = %+v", got)
	}
	if got := links[second]; len(got.FixVersions)+len(got.AffectsVersions)+len(got.Components) != 0 {
		t.Fatalf("links of second = %+v", got)
	}

	// Replacing with an empty set clears the kind only.
	if err := e.repo.ReplaceIssueVersions(ctx, first, "fix", nil); err != nil {
		t.Fatal(err)
	}
	links, _ = e.repo.LoadLinks(ctx, []uuid.UUID{first})
	if len(links[first].FixVersions) != 0 || len(links[first].AffectsVersions) != 1 {
		t.Fatalf("after clearing fix: %+v", links[first])
	}
}

func TestComponentDefaultsReadPolicyAndLead(t *testing.T) {
	e := newLinkEnv(t)
	ctx := context.Background()
	id := uuid.New()
	e.exec(t, `INSERT INTO components (id, project_id, name, lead_id, default_assignee_policy) VALUES ($1, $2, 'Pagos', $3, 'component_lead')`, id, e.project, e.reporter)
	defaults, err := e.repo.ComponentDefaults(ctx, e.project, []uuid.UUID{id})
	if err != nil || len(defaults) != 1 || defaults[0].Policy != "component_lead" || defaults[0].LeadID == nil || *defaults[0].LeadID != e.reporter {
		t.Fatalf("defaults = %+v %v", defaults, err)
	}
	if other, _ := e.repo.ComponentDefaults(ctx, e.other, []uuid.UUID{id}); len(other) != 0 {
		t.Fatal("components of another project must not resolve")
	}
}

func TestIssueListFiltersByFixVersionAndComponent(t *testing.T) {
	e := newLinkEnv(t)
	ctx := context.Background()
	version := e.version(t, e.project, "2.0", "unreleased")
	component := uuid.New()
	e.exec(t, `INSERT INTO components (id, project_id, name) VALUES ($1, $2, 'API')`, component, e.project)
	linked := e.issue(t, e.project, 1)
	e.issue(t, e.project, 2)
	if err := e.repo.ReplaceIssueVersions(ctx, linked, "fix", []uuid.UUID{version}); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.ReplaceIssueComponents(ctx, linked, []uuid.UUID{component}); err != nil {
		t.Fatal(err)
	}

	byVersion, err := e.issues.FindAll(ctx, port.ListFilter{ProjectIDs: []uuid.UUID{e.project}, FixVersionID: version, Page: 1, PageSize: 10})
	if err != nil || len(byVersion) != 1 || byVersion[0].ID != linked {
		t.Fatalf("fix_version filter = %d %v", len(byVersion), err)
	}
	byComponent, err := e.issues.FindAll(ctx, port.ListFilter{ProjectIDs: []uuid.UUID{e.project}, ComponentID: component, Page: 1, PageSize: 10})
	if err != nil || len(byComponent) != 1 || byComponent[0].ID != linked {
		t.Fatalf("component filter = %d %v", len(byComponent), err)
	}
	total, err := e.issues.Count(ctx, port.ListFilter{ProjectIDs: []uuid.UUID{e.project}, ComponentID: component})
	if err != nil || total != 1 {
		t.Fatalf("count with component = %d %v", total, err)
	}
}
