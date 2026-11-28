package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/application/usecase"
)

// testOrg is the organization the migrations seed (with its catalogs).
const testOrg = "00000000-0000-4000-8000-000000000002"

// testNow is the fixed instant of the behavior tests: a Friday.
var testNow = time.Date(2026, 11, 27, 15, 30, 0, 0, time.UTC)

type searchEnv struct {
	t   *testing.T
	db  *sqlx.DB
	ctx context.Context

	alice, bob, carol, owner, dave, outsider, admin uuid.UUID
	org2                                            uuid.UUID
	pub, priv, oth                                  uuid.UUID
	teamBackend, teamOther                          uuid.UUID
	sprintActive, sprintClosed                      uuid.UUID
	fieldClient, fieldHours, fieldEnv, fieldUrgent  uuid.UUID
	keyNum                                          map[uuid.UUID]int

	repo *postgres.IssueSearchRepository
	jql  *usecase.JQLService
}

func (e *searchEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		e.t.Fatalf("%v\n%s", err, query)
	}
}

func (e *searchEnv) user(name, role string) uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	e.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', $4)`,
		id, name, name+"@example.test", role)
	return id
}

func (e *searchEnv) member(org string, user uuid.UUID, role string) {
	e.t.Helper()
	e.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, org, user, role)
}

func (e *searchEnv) project(org, key, name, visibility string, lead uuid.UUID) uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, $3, $4, 'scrum', 'scrum', $5, $6)`, id, org, key, name, lead, visibility)
	e.exec(`SELECT corelog_ensure_workflow($1)`, id)
	e.keyNum[id] = 0
	return id
}

// issueSpec describes a fixture issue; zero values mean "unset".
type issueSpec struct {
	project    uuid.UUID
	summary    string
	desc       string
	typeName   string // legacy key: task, bug, story, epic
	priority   string // legacy key: low, medium, high, critical
	status     string // legacy: open, in_progress, closed
	assignee   *uuid.UUID
	reporter   uuid.UUID
	createdAgo time.Duration
	due        string
	points     *float64
	flagged    bool
	votes      int
	parent     *uuid.UUID
	sprint     *uuid.UUID
	resolved   bool
	deleted    bool
	custom     string
	keyNo      int // explicit key number; 0 takes the next one
}

func (e *searchEnv) issue(s issueSpec) uuid.UUID {
	e.t.Helper()
	number := s.keyNo
	if number == 0 {
		e.keyNum[s.project]++
		number = e.keyNum[s.project]
	}
	id := uuid.New()
	if s.typeName == "" {
		s.typeName = "task"
	}
	if s.priority == "" {
		s.priority = "medium"
	}
	if s.status == "" {
		s.status = "open"
	}
	if s.custom == "" {
		s.custom = "{}"
	}
	created := testNow.Add(-s.createdAgo)
	var due any
	if s.due != "" {
		due = s.due
	}
	e.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, assignee_id, issue_type, key_number, rank,
			story_points, issue_type_id, priority_id, due_date, flagged, vote_count, parent_id, sprint_id, created_at, updated_at, custom)
		VALUES ($1, $2, $3, $4, $5, $6::text, $7, $8, $9::text, $10, $11, $12,
			(SELECT id FROM issue_types WHERE organization_id = (SELECT organization_id FROM projects WHERE id = $2) AND legacy_key = $9),
			(SELECT id FROM priorities WHERE organization_id = (SELECT organization_id FROM projects WHERE id = $2) AND legacy_key = $6 ORDER BY position LIMIT 1),
			$13, $14, $15, $16, $17, $18, $18, $19::jsonb)`,
		id, s.project, s.summary, s.desc, s.status, s.priority, s.reporter, s.assignee, s.typeName, number,
		"r"+string(rune('a'+number)), s.points, due, s.flagged, s.votes, s.parent, s.sprint, created, s.custom)
	if s.resolved {
		e.exec(`UPDATE issues SET resolution_id = (SELECT id FROM resolutions WHERE organization_id = $2 AND name = 'Hecho'), resolved_at = $3 WHERE id = $1`,
			id, testOrg, testNow.Add(-24*time.Hour))
	}
	if s.deleted {
		e.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, id)
	}
	return id
}

func (e *searchEnv) label(issue, project uuid.UUID, name string) {
	e.t.Helper()
	var id uuid.UUID
	if err := e.db.Get(&id, `INSERT INTO labels (id, project_id, name) VALUES (gen_random_uuid(), $1, $2)
		ON CONFLICT (project_id, name) WHERE deleted_at IS NULL DO UPDATE SET name = EXCLUDED.name RETURNING id`, project, name); err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO issue_labels (issue_id, label_id) VALUES ($1, $2)`, issue, id)
}

func (e *searchEnv) component(issue, project uuid.UUID, name string) {
	e.t.Helper()
	var id uuid.UUID
	if err := e.db.Get(&id, `INSERT INTO components (id, project_id, name) VALUES (gen_random_uuid(), $1, $2)
		ON CONFLICT (project_id, name) WHERE deleted_at IS NULL DO UPDATE SET name = EXCLUDED.name RETURNING id`, project, name); err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO issue_components (issue_id, component_id) VALUES ($1, $2)`, issue, id)
}

func (e *searchEnv) setStatus(issue uuid.UUID, name string) {
	e.t.Helper()
	e.exec(`UPDATE issues SET status_id = (SELECT s.id FROM statuses s WHERE s.organization_id = $2 AND s.name = $3) WHERE id = $1`, issue, testOrg, name)
}

func fptr(f float64) *float64 { return &f }

func uptr(u uuid.UUID) *uuid.UUID { return &u }

// newSearchEnv builds the shared fixture: two organizations, users with different access, a public and a
// private project, a project of another organization and a set of issues exercising every field.
func newSearchEnv(t *testing.T) *searchEnv {
	t.Helper()
	db := testdb.New(t)
	e := &searchEnv{t: t, db: db, ctx: context.Background(), keyNum: map[uuid.UUID]int{}}
	e.alice, e.bob, e.carol = e.user("alice", "user"), e.user("bob", "user"), e.user("carol", "user")
	e.owner, e.dave, e.outsider, e.admin = e.user("owner", "user"), e.user("dave", "user"), e.user("outsider", "user"), e.user("admin", "admin")
	e.member(testOrg, e.alice, "member")
	e.member(testOrg, e.bob, "member")
	e.member(testOrg, e.carol, "guest")
	e.member(testOrg, e.owner, "owner")
	e.org2 = uuid.New()
	e.exec(`INSERT INTO organizations (id, slug, name, created_by) VALUES ($1, 'otra', 'Otra', $2)`, e.org2, e.dave)
	e.member(e.org2.String(), e.dave, "owner")
	e.exec(`INSERT INTO issue_types (organization_id, name, description, icon, color, hierarchy_level, position, is_default, legacy_key)
		SELECT $1, name, description, icon, color, hierarchy_level, position, is_default, legacy_key FROM issue_types WHERE organization_id = $2`, e.org2, testOrg)
	e.exec(`INSERT INTO priorities (organization_id, name, icon, color, position, is_default, legacy_key)
		SELECT $1, name, icon, color, position, is_default, legacy_key FROM priorities WHERE organization_id = $2`, e.org2, testOrg)

	e.pub = e.project(testOrg, "PUB", "Publico", "open", e.alice)
	e.priv = e.project(testOrg, "PRIV", "Privado", "private", e.alice)
	e.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'lead')`, e.priv, e.alice)
	e.oth = e.project(e.org2.String(), "OTH", "Otra org", "open", e.dave)

	e.teamBackend, e.teamOther = uuid.New(), uuid.New()
	e.exec(`INSERT INTO teams (id, name, organization_id) VALUES ($1, 'Backend', $3), ($2, 'Diseno', $3)`, e.teamBackend, e.teamOther, testOrg)
	e.exec(`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'member'), ($1, $3, 'member'), ($4, $5, 'member')`,
		e.teamBackend, e.alice, e.bob, e.teamOther, e.carol)

	e.sprintActive, e.sprintClosed = uuid.New(), uuid.New()
	e.exec(`INSERT INTO sprints (id, project_id, name, status) VALUES ($1, $2, 'Sprint 1', 'active'), ($3, $2, 'Sprint 0', 'closed')`,
		e.sprintActive, e.pub, e.sprintClosed)

	// Custom fields of the main organization.
	e.fieldClient, e.fieldHours, e.fieldEnv, e.fieldUrgent = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	e.exec(`INSERT INTO custom_fields (id, organization_id, name, field_type, options) VALUES
		($1, $5, 'Cliente', 'text', '[]'), ($2, $5, 'Horas', 'number', '[]'),
		($3, $5, 'Entorno', 'select', '[{"id":"opt-prod","label":"Producción"},{"id":"opt-dev","label":"Desarrollo"}]'),
		($4, $5, 'Urgente', 'checkbox', '[]')`, e.fieldClient, e.fieldHours, e.fieldEnv, e.fieldUrgent, testOrg)

	e.repo = postgres.NewIssueSearchRepository(db).WithVersions(postgres.VersionTables{})
	e.jql = usecase.NewJQLService(e.repo, e.repo, e.repo, clock.NewFixed(testNow)).WithSuggestions(e.repo)

	e.seedIssues()
	return e
}

func (e *searchEnv) seedIssues() {
	day := 24 * time.Hour
	epic := e.issue(issueSpec{project: e.pub, summary: "Plataforma de pagos", typeName: "epic", reporter: e.alice, createdAgo: 30 * day, priority: "high", keyNo: 5})
	e.setStatus(epic, "En curso")
	i1 := e.issue(issueSpec{project: e.pub, summary: "Login falla en móvil", desc: "El botón de login no responde en Android",
		typeName: "bug", priority: "high", assignee: uptr(e.alice), reporter: e.bob, createdAgo: 10 * day, due: "2026-11-29",
		points: fptr(5), flagged: true, sprint: &e.sprintActive,
		custom: `{"` + e.fieldClient.String() + `":"Acme","` + e.fieldHours.String() + `":8,"` + e.fieldEnv.String() + `":"opt-prod","` + e.fieldUrgent.String() + `":true}`})
	e.label(i1, e.pub, "bug")
	e.label(i1, e.pub, "mobile")
	e.component(i1, e.pub, "Frontend")
	e.setStatus(i1, "Por hacer")
	e.exec(`INSERT INTO comments (id, issue_id, author_id, body) VALUES (gen_random_uuid(), $1, $2, 'Revisar el despliegue de la versión nueva')`, i1, e.bob)

	i2 := e.issue(issueSpec{project: e.pub, summary: "Mejorar el rendimiento del dashboard", typeName: "story", assignee: uptr(e.bob),
		reporter: e.alice, createdAgo: 3 * day, due: "2026-11-26", points: fptr(3), parent: &epic, status: "in_progress",
		custom: `{"` + e.fieldClient.String() + `":"Globex","` + e.fieldHours.String() + `":20}`})
	e.label(i2, e.pub, "perf")
	e.setStatus(i2, "En curso")

	i3 := e.issue(issueSpec{project: e.pub, summary: "Documentar la API", reporter: e.alice, createdAgo: 20 * day, priority: "low",
		points: fptr(1), sprint: &e.sprintClosed, status: "closed", resolved: true})
	e.label(i3, e.pub, "docs")
	e.setStatus(i3, "Hecho")

	i4 := e.issue(issueSpec{project: e.pub, summary: "Pago con tarjeta rechazado", desc: "El proveedor stripe devuelve error 402",
		typeName: "bug", priority: "critical", assignee: uptr(e.alice), reporter: e.carol, createdAgo: day, votes: 3, parent: &epic})
	e.label(i4, e.pub, "bug")
	e.label(i4, e.pub, "payments")
	e.setStatus(i4, "Por hacer")

	e.issue(issueSpec{project: e.pub, summary: "Tarea eliminada", reporter: e.alice, deleted: true, keyNo: 6})
	e.issue(issueSpec{project: e.priv, summary: "Secreto de la adquisición", assignee: uptr(e.alice), reporter: e.alice, createdAgo: 2 * day})
	e.issue(issueSpec{project: e.priv, summary: "Login secreto eliminado", reporter: e.alice, deleted: true})
	e.issue(issueSpec{project: e.oth, summary: "Login en otra organización", assignee: uptr(e.dave), reporter: e.dave, createdAgo: 4 * day})
}

// viewer builds the search viewer of a user.
func (e *searchEnv) viewer(id uuid.UUID) port.Viewer {
	return port.Viewer{UserID: id, PlatformAdmin: id == e.admin, Timezone: "UTC"}
}

// keys runs a query as a user and returns the issue keys in result order.
func (e *searchEnv) keys(user uuid.UUID, jql string) []string {
	e.t.Helper()
	out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(user), Query: jql, PageSize: 100, WithTotal: true})
	if err != nil {
		e.t.Fatalf("search %q: %v", jql, err)
	}
	keys := make([]string, 0, len(out.Page.Items))
	for _, it := range out.Page.Items {
		keys = append(keys, it.Key)
	}
	if out.Page.Total != len(keys) {
		e.t.Fatalf("search %q: total %d, items %d", jql, out.Page.Total, len(keys))
	}
	return keys
}
