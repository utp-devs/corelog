package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const authzSecret = "authz-audit-secret"

// Actor names of the audit. Every actor is a real user with a signed token; "anonymous" has no token.
const (
	actAnonymous   = "anonymous"
	actNoOrg       = "noorg"
	actGuest       = "guest"
	actMember      = "member"
	actViewer      = "viewer"
	actContributor = "contributor"
	actLead        = "lead"
	actOrgAdmin    = "orgadmin"
	actOrgOwner    = "orgowner"
	actPlatform    = "platform"
	actOutsider    = "outsider"
)

var authzActors = []string{actAnonymous, actNoOrg, actGuest, actMember, actViewer, actContributor, actLead, actOrgAdmin, actOrgOwner, actPlatform, actOutsider}

// authzFixture is two organizations with a private, an open and an archived project in the first one and the
// resources of every kind the routes take as path parameters. Everything is created through the real router
// where the API allows it, so the audit exercises real identifiers.
type authzFixture struct {
	t      *testing.T
	db     *sqlx.DB
	router http.Handler
	tokens map[string]string
	users  map[string]uuid.UUID
	orgA   string
	orgB   string
	// ids holds the resources of the private project (and organization A) by kind.
	ids map[string]string
	// open and archived hold the project, issue and sprint of the open and the archived project.
	open     map[string]string
	archived map[string]string
	// other holds the resources of organization B.
	other map[string]string
	// unseeded lists the kinds that could not be created; a request using one resolves to a random UUID.
	unseeded map[string]bool
	// secrets are strings that must never appear in the response to a caller who may not see the resource.
	secrets []string
}

func (f *authzFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(query, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, query)
	}
}

// call performs one request on the real router, with a JSON body when body is not nil.
func (f *authzFixture) call(method, target, token string, body any) (int, []byte) {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "/api/v1"+target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// The live stream never ends by itself: the audit only needs its status line.
	if strings.Contains(target, "/notifications/stream") {
		ctx, cancel := context.WithTimeout(req.Context(), 300*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// create calls the API as the given actor and returns the "id" of the created resource ("" and a flag when it failed).
func (f *authzFixture) create(kind, actor, method, target string, body any) string {
	f.t.Helper()
	status, raw := f.call(method, target, f.tokens[actor], body)
	if status < 200 || status > 299 {
		f.t.Logf("fixture: %s not created (%s %s -> %d %s)", kind, method, target, status, truncate(raw, 160))
		f.unseeded[kind] = true
		return ""
	}
	var out struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Data == nil {
		f.t.Logf("fixture: %s created but the response has no data object: %s", kind, truncate(raw, 160))
		f.unseeded[kind] = true
		return ""
	}
	id, _ := out.Data["id"].(string)
	if id == "" {
		f.unseeded[kind] = true
	}
	return id
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func (f *authzFixture) set(m map[string]string, kind, id string) {
	if id != "" {
		m[kind] = id
	}
}

// fixtureOptions tune the fixture of a test that needs a particular logger or environment.
type fixtureOptions struct {
	logger *slog.Logger
	env    string
}

func newAuthzFixture(t *testing.T, opts ...func(*fixtureOptions)) *authzFixture {
	t.Helper()
	// Uploads of the audited routes go to a temporary directory, never to the working tree.
	for _, name := range []string{"IMPORTS_DIR", "ATTACHMENTS_DIR", "AVATARS_DIR", "LOGOS_DIR"} {
		t.Setenv(name, t.TempDir())
	}
	db := testdb.New(t)
	previous := middleware.DefaultLogger
	middleware.DefaultLogger = middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log.New(io.Discard, "", 0)})
	t.Cleanup(func() { middleware.DefaultLogger = previous })
	options := fixtureOptions{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, apply := range opts {
		apply(&options)
	}
	cfg := &config.Config{Auth: config.AuthConfig{JWTSecret: authzSecret, JWTExpiration: time.Hour}}
	cfg.Server.Env = options.env
	router, err := buildRouter(cfg, db, options.logger)
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	f := &authzFixture{
		t: t, db: db, router: router,
		tokens: map[string]string{}, users: map[string]uuid.UUID{},
		ids: map[string]string{}, open: map[string]string{}, archived: map[string]string{}, other: map[string]string{},
		unseeded: map[string]bool{},
	}

	for _, name := range authzActors {
		if name == actAnonymous {
			continue
		}
		id := uuid.New()
		role := "user"
		if name == actPlatform {
			role = "admin"
		}
		display := "Zq " + name
		f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', $4)`,
			id, display, "zq-"+name+"@authz.test", role)
		token, err := authtoken.Generate(authzSecret, time.Hour, id, role)
		if err != nil {
			t.Fatal(err)
		}
		f.users[name] = id
		f.tokens[name] = token
		f.secrets = append(f.secrets, "zq-"+name+"@authz.test")
	}

	// Organizations are created through the API so their catalogs are seeded like in production.
	f.orgA = f.create("org", actOrgOwner, http.MethodPost, "/organizations", map[string]any{"name": "Zorbax Corp", "slug": "zorbax"})
	f.orgB = f.create("orgB", actOutsider, http.MethodPost, "/organizations", map[string]any{"name": "Quimera Ltd", "slug": "quimera"})
	if f.orgA == "" || f.orgB == "" {
		t.Fatal("fixture: organizations were not created")
	}
	f.secrets = append(f.secrets, "Zorbax", "zorbax")
	for name, role := range map[string]string{actOrgAdmin: "admin", actGuest: "guest", actMember: "member", actViewer: "member", actContributor: "member", actLead: "member"} {
		f.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, f.orgA, f.users[name], role)
	}
	f.ids["org"] = f.orgA

	project := func(m map[string]string, key, name, visibility string, owner string) string {
		id := f.create("project", owner, http.MethodPost, "/projects", map[string]any{
			"organization_id": f.orgA, "key": key, "name": name, "template": "scrum", "type": "scrum", "visibility": visibility,
		})
		if id == "" {
			t.Fatalf("fixture: project %s not created", key)
		}
		m["project"] = id
		f.secrets = append(f.secrets, name, key)
		return id
	}
	prv := project(f.ids, "ZQPRV", "Zorbax Secreto", "private", actOrgOwner)
	opn := project(f.open, "ZQOPN", "Zorbax Abierto", "open", actOrgOwner)
	arc := project(f.archived, "ZQARC", "Zorbax Archivado", "private", actOrgOwner)
	for _, p := range []string{prv, opn, arc} {
		for name, role := range map[string]string{actLead: "lead", actContributor: "contributor", actViewer: "viewer"} {
			if p == opn && name != actLead {
				continue // the open project is read by every member through the organization
			}
			f.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role`, p, f.users[name], role)
		}
	}
	// Organization B has its own project and the same kinds of resources, so a request that mixes the
	// identifiers of both organizations can be built for every route.
	f.other["project"] = f.create("projectB", actOutsider, http.MethodPost, "/projects", map[string]any{
		"organization_id": f.orgB, "key": "QMB", "name": "Quimera Interno", "template": "scrum", "type": "scrum", "visibility": "private",
	})
	f.other["org"] = f.orgB
	f.seedProject(f.ids, prv, actLead, "Zorbax")
	f.seedProject(f.other, f.other["project"], actOutsider, "Quimera")
	f.seedOrg(f.ids, f.orgA, actOrgAdmin, "Zorbax")
	f.seedOrg(f.other, f.orgB, actOutsider, "Quimera")
	f.set(f.open, "issue", f.create("issue", actLead, http.MethodPost, "/issues", issueBody(opn, "Zorbax incidencia abierta")))
	f.set(f.open, "sprint", f.create("sprint", actLead, http.MethodPost, "/sprints?project_id="+opn, map[string]any{"name": "Zorbax Sprint abierto"}))
	f.set(f.archived, "issue", f.create("issue", actLead, http.MethodPost, "/issues", issueBody(arc, "Zorbax incidencia archivada")))
	f.open["org"], f.archived["org"] = f.orgA, f.orgA
	f.secrets = append(f.secrets, "Zorbax incidencia", "Quimera")
	// A spare member the destructive member routes can act on without disturbing the actors of the audit.
	spare := uuid.New()
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Zq spare', 'Zq spare', 'zq-spare@authz.test', 'x', 'user')`, spare)
	f.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, f.orgA, spare)
	f.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'viewer')`, prv, spare)
	f.set(f.ids, "user", spare.String())
	f.set(f.ids, "member", spare.String())
	f.set(f.other, "user", f.users[actOutsider].String())
	f.set(f.other, "member", f.users[actOutsider].String())
	f.set(f.ids, "team", f.create("team", actOrgAdmin, http.MethodPost, "/teams", map[string]any{"organization_id": f.orgA, "name": "Zorbax Equipo"}))
	f.set(f.other, "team", f.create("team", actOutsider, http.MethodPost, "/teams", map[string]any{"organization_id": f.orgB, "name": "Quimera Equipo"}))
	delete(f.unseeded, "watcher")

	// Archive the third project last, once its issue exists.
	if status, raw := f.call(http.MethodPost, "/projects/"+arc+"/archive", f.tokens[actOrgOwner], nil); status != http.StatusOK && status != http.StatusNoContent {
		t.Fatalf("fixture: archive: %d %s", status, truncate(raw, 200))
	}
	sort.Strings(f.secrets)
	return f
}

func (f *authzFixture) firstID(query string, args ...any) string {
	var id string
	if err := f.db.Get(&id, query, args...); err != nil {
		f.t.Logf("fixture: lookup failed: %v", err)
		return ""
	}
	return id
}

func issueBody(project, summary string) map[string]any {
	return map[string]any{"summary": summary, "description": "texto", "project_id": project}
}

// seedProject creates the resources that live inside one project, as the given actor (its lead).
func (f *authzFixture) seedProject(m map[string]string, prj, actor, word string) {
	f.set(m, "issue", f.create("issue", actor, http.MethodPost, "/issues", issueBody(prj, word+" incidencia secreta")))
	f.set(m, "issue2", f.create("issue", actor, http.MethodPost, "/issues", issueBody(prj, word+" incidencia hermana")))
	issue := m["issue"]
	if issue != "" {
		f.set(m, "comment", f.create("comment", actor, http.MethodPost, "/issues/"+issue+"/comments", map[string]any{"body": word + " comentario"}))
		f.create("worklog", actor, http.MethodPost, "/issues/"+issue+"/worklogs", map[string]any{"time_spent": "1h"})
		delete(f.unseeded, "worklog")
		f.set(m, "worklog", f.firstID("SELECT id::text FROM worklogs WHERE issue_id = $1 LIMIT 1", issue))
		f.set(m, "link", f.create("link", actor, http.MethodPost, "/issues/"+issue+"/links", map[string]any{"target": m["issue2"], "kind": "relates"}))
		_ = f.create("watcher", actor, http.MethodPost, "/issues/"+issue+"/watchers", map[string]any{})
	}
	f.set(m, "sprint", f.create("sprint", actor, http.MethodPost, "/sprints?project_id="+prj, map[string]any{"name": word + " Sprint privado"}))
	f.set(m, "version", f.create("version", actor, http.MethodPost, "/projects/"+prj+"/versions", map[string]any{"name": strings.ToLower(word) + "-1.0"}))
	f.set(m, "label", f.create("label", actor, http.MethodPost, "/projects/"+prj+"/labels", map[string]any{"name": strings.ToLower(word) + "-label", "color": "blue"}))
	f.set(m, "component", f.create("component", actor, http.MethodPost, "/projects/"+prj+"/components", map[string]any{"name": strings.ToLower(word) + "-comp"}))
	f.set(m, "rule", f.create("rule", actor, http.MethodPost, "/projects/"+prj+"/automation-rules", map[string]any{
		"name": word + " regla", "trigger": map[string]any{"type": "issue_created"}, "actions": []any{map[string]any{"type": "add_label", "label": "auto"}},
	}))
	f.set(m, "board", f.firstID("SELECT id::text FROM boards WHERE project_id = $1 ORDER BY is_default DESC LIMIT 1", prj))
	f.set(m, "filter", f.create("filter", actor, http.MethodPost, "/filters", map[string]any{"name": word + " filtro", "query": "statusCategory != done"}))
	f.set(m, "savedfilter", f.create("savedfilter", actor, http.MethodPost, "/saved-filters", map[string]any{"name": word + " guardado", "filters": map[string]any{"query": strings.ToLower(word)}}))
	f.set(m, "dashboard", f.create("dashboard", actor, http.MethodPost, "/dashboards", map[string]any{"name": word + " panel"}))
	f.set(m, "note", f.create("note", actor, http.MethodPost, "/notes", map[string]any{"title": word + " nota", "body": "privada"}))
	f.set(m, "event", f.create("event", actor, http.MethodPost, "/calendar/events", map[string]any{"title": word + " evento", "starts_at": "2026-12-09T10:00:00Z", "ends_at": "2026-12-09T11:00:00Z", "visibility": "private"}))
	f.set(m, "apitoken", f.create("apitoken", actor, http.MethodPost, "/me/api-tokens", map[string]any{"name": strings.ToLower(word) + " token", "scopes": []string{"read"}}))
}

// seedOrg creates the organization level catalogs and integrations, as the given actor (an administrator).
func (f *authzFixture) seedOrg(m map[string]string, org, actor, word string) {
	base := "/organizations/" + org
	f.set(m, "webhook", f.create("webhook", actor, http.MethodPost, base+"/webhooks", map[string]any{"name": word + " hook", "url": "https://example.com/hook", "events": []string{"issue.created"}}))
	f.set(m, "priority", f.create("priority", actor, http.MethodPost, base+"/priorities", map[string]any{"name": word + " prioridad", "icon": "flag", "color": "red"}))
	f.set(m, "resolution", f.create("resolution", actor, http.MethodPost, base+"/resolutions", map[string]any{"name": word + " resolucion"}))
	f.set(m, "status", f.create("status", actor, http.MethodPost, base+"/statuses", map[string]any{"name": word + " estado", "category": "todo"}))
	f.set(m, "issuetype", f.create("issuetype", actor, http.MethodPost, base+"/issue-types", map[string]any{"name": word + " tipo", "icon": "task", "color": "blue"}))
	f.set(m, "customfield", f.create("customfield", actor, http.MethodPost, base+"/custom-fields", map[string]any{"name": word + " campo", "type": "text"}))
}
