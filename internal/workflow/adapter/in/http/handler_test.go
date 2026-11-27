package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/workflow/application/usecase"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
	"github.com/jhonsferg/corelog/internal/workflow/workflowtest"
)

const secret = "test-secret"

// resolver maps project keys to ids, like the project module does for its own routes.
type resolver struct {
	keys map[string]uuid.UUID
}

func (r resolver) ResolveProjectKey(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, key string) (uuid.UUID, error) {
	id, ok := r.keys[key]
	if !ok {
		return uuid.Nil, apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado")
	}
	return id, nil
}

// world is the in-memory world the HTTP tests drive through the real use cases.
type world struct {
	repo        *workflowtest.Repo
	handler     http.Handler
	org         uuid.UUID
	otherOrg    uuid.UUID
	project     uuid.UUID
	lead        uuid.UUID
	contributor uuid.UUID
	viewer      uuid.UUID
	orgAdmin    uuid.UUID
	orgMember   uuid.UUID
	outsider    uuid.UUID
	otherAdmin  uuid.UUID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{
		repo: workflowtest.NewRepo(), org: uuid.New(), otherOrg: uuid.New(),
		lead: uuid.New(), contributor: uuid.New(), viewer: uuid.New(), orgAdmin: uuid.New(),
		orgMember: uuid.New(), outsider: uuid.New(), otherAdmin: uuid.New(),
	}
	w.project = w.repo.SeedProject(w.org, "scrum")
	w.repo.SeedProject(w.otherOrg, "kanban")
	authz := workflowtest.Authz{
		Roles: map[uuid.UUID]string{w.lead: "lead", w.contributor: "contributor", w.viewer: "viewer", w.orgAdmin: "org_admin", w.otherAdmin: "org_admin"},
	}
	orgs := workflowtest.OrgRoles{Roles: map[workflowtest.OrgUser]string{
		{OrgID: w.org, UserID: w.orgAdmin}:        "admin",
		{OrgID: w.org, UserID: w.orgMember}:       "member",
		{OrgID: w.otherOrg, UserID: w.otherAdmin}: "admin",
	}}
	management := usecase.NewManagement(w.repo, authz, orgs, workflowtest.Tx{}, &clock.FakeClock{T: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)})
	router := chi.NewRouter()
	NewHandler(management, resolver{keys: map[string]uuid.UUID{"CORE": w.project}}, secret).RegisterRoutes(router)
	w.handler = router
	return w
}

func (w *world) token(t *testing.T, user uuid.UUID) string {
	t.Helper()
	value, err := authtoken.Generate(secret, time.Hour, user, "user")
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + value
}

func (w *world) do(t *testing.T, method, path, body string, user *uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != nil {
		req.Header.Set("Authorization", w.token(t, *user))
	}
	rec := httptest.NewRecorder()
	w.handler.ServeHTTP(rec, req)
	return rec
}

// currentDefinition builds the body that re-submits the stored workflow unchanged.
func (w *world) currentDefinition(t *testing.T, version int) string {
	t.Helper()
	stored := w.repo.Workflows[w.project].Workflow
	type statusRef struct {
		Ref string `json:"ref"`
		ID  string `json:"id"`
	}
	body := struct {
		ExpectedVersion int               `json:"expected_version"`
		InitialRef      string            `json:"initial_ref"`
		Statuses        []statusRef       `json:"statuses"`
		Transitions     []json.RawMessage `json:"transitions"`
	}{ExpectedVersion: version, InitialRef: stored.Initial}
	for _, s := range stored.Statuses {
		body.Statuses = append(body.Statuses, statusRef{Ref: s.ID, ID: s.ID})
	}
	for _, tr := range stored.Transitions {
		raw, err := domain.EncodeTransition(tr)
		if err != nil {
			t.Fatal(err)
		}
		body.Transitions = append(body.Transitions, raw)
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func errorCodeOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

func TestWorkflowRoleMatrix(t *testing.T) {
	w := newWorld(t)
	cases := []struct {
		name   string
		user   uuid.UUID
		method string
		want   int
	}{
		{"lead reads", w.lead, http.MethodGet, http.StatusOK},
		{"contributor reads", w.contributor, http.MethodGet, http.StatusOK},
		{"viewer reads", w.viewer, http.MethodGet, http.StatusOK},
		{"org admin reads", w.orgAdmin, http.MethodGet, http.StatusOK},
		{"outsider cannot see the project", w.outsider, http.MethodGet, http.StatusNotFound},
		{"lead replaces", w.lead, http.MethodPut, http.StatusOK},
		{"org admin replaces", w.orgAdmin, http.MethodPut, http.StatusOK},
		{"contributor cannot replace", w.contributor, http.MethodPut, http.StatusForbidden},
		{"viewer cannot replace", w.viewer, http.MethodPut, http.StatusForbidden},
		{"outsider cannot replace", w.outsider, http.MethodPut, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user := c.user
			body := ""
			if c.method == http.MethodPut {
				body = w.currentDefinition(t, w.repo.Workflows[w.project].Version)
			}
			rec := w.do(t, c.method, "/projects/"+w.project.String()+"/workflow", body, &user)
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
		})
	}
}

func TestWorkflowRequiresAuthentication(t *testing.T) {
	w := newWorld(t)
	rec := w.do(t, http.MethodGet, "/projects/"+w.project.String()+"/workflow", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	rec = w.do(t, http.MethodGet, "/workflow-templates", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("templates without token = %d, want 401", rec.Code)
	}
}

func TestGetWorkflowShapeAndKeyResolution(t *testing.T) {
	w := newWorld(t)
	rec := w.do(t, http.MethodGet, "/projects/CORE/workflow", "", &w.contributor)
	if rec.Code != http.StatusOK {
		t.Fatalf("key lookup: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Statuses          []map[string]any `json:"statuses"`
			InitialStatusID   string           `json:"initial_status_id"`
			Transitions       []map[string]any `json:"transitions"`
			Version           int              `json:"version"`
			IsTemplateDefault bool             `json:"is_template_default"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Statuses) != 4 || body.Data.Version != 1 || !body.Data.IsTemplateDefault || body.Data.InitialStatusID == "" {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if _, ok := body.Data.Statuses[0]["issue_count"]; !ok {
		t.Fatal("statuses lack issue_count")
	}
	if _, ok := body.Data.Transitions[0]["post_functions"]; !ok {
		t.Fatal("transitions lack the domain encoding")
	}

	rec = w.do(t, http.MethodGet, "/projects/UNKNOWN/workflow", "", &w.contributor)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown key = %d, want 404", rec.Code)
	}
}

func TestValidateDryRunAnswersWithPaths(t *testing.T) {
	w := newWorld(t)
	body := strings.Replace(w.currentDefinition(t, 1), `"to":"`, `"to":"nowhere`, 1)
	rec := w.do(t, http.MethodPost, "/projects/"+w.project.String()+"/workflow/validate", body, &w.lead)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate = %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Valid  bool `json:"valid"`
			Errors []struct {
				Path string `json:"path"`
				Code string `json:"code"`
			} `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Data.Valid || len(out.Data.Errors) == 0 || out.Data.Errors[0].Path != "transitions[0].to" {
		t.Fatalf("validate body = %s", rec.Body.String())
	}
}

func TestReplaceRefusesMissingMappingWithDetails(t *testing.T) {
	w := newWorld(t)
	review, _ := w.repo.StatusByName(w.org, "En revisión")
	w.repo.AddIssues(w.project, review.ID, 2)
	body := w.dropReviewDefinition(t)
	rec := w.do(t, http.MethodPut, "/projects/"+w.project.String()+"/workflow", body, &w.lead)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCodeOf(t, rec); code != "workflow.status_mapping_required" {
		t.Fatalf("code = %q", code)
	}
	if !strings.Contains(rec.Body.String(), review.ID.String()) || !strings.Contains(rec.Body.String(), `"issue_count":2`) {
		t.Fatalf("mapping_required missing: %s", rec.Body.String())
	}
}

// dropReviewDefinition is the stored scrum workflow without its "En revisión" status and transitions.
func (w *world) dropReviewDefinition(t *testing.T) string {
	t.Helper()
	review, _ := w.repo.StatusByName(w.org, "En revisión")
	stored := w.repo.Workflows[w.project].Workflow
	type statusRef struct {
		Ref string `json:"ref"`
		ID  string `json:"id"`
	}
	body := struct {
		ExpectedVersion int               `json:"expected_version"`
		InitialRef      string            `json:"initial_ref"`
		Statuses        []statusRef       `json:"statuses"`
		Transitions     []json.RawMessage `json:"transitions"`
	}{ExpectedVersion: w.repo.Workflows[w.project].Version, InitialRef: stored.Initial}
	for _, s := range stored.Statuses {
		if s.ID != review.ID.String() {
			body.Statuses = append(body.Statuses, statusRef{Ref: s.ID, ID: s.ID})
		}
	}
	for _, tr := range stored.Transitions {
		if tr.To == review.ID.String() || contains(tr.From, review.ID.String()) {
			continue
		}
		raw, err := domain.EncodeTransition(tr)
		if err != nil {
			t.Fatal(err)
		}
		body.Transitions = append(body.Transitions, raw)
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func TestReplaceVersionConflictAndUnknownFields(t *testing.T) {
	w := newWorld(t)
	rec := w.do(t, http.MethodPut, "/projects/"+w.project.String()+"/workflow", w.currentDefinition(t, 9), &w.lead)
	if rec.Code != http.StatusConflict || errorCodeOf(t, rec) != "workflow.version_conflict" {
		t.Fatalf("stale version = %d %s", rec.Code, rec.Body.String())
	}
	bad := strings.Replace(w.currentDefinition(t, 1), `{"expected_version"`, `{"surprise":true,"expected_version"`, 1)
	rec = w.do(t, http.MethodPut, "/projects/"+w.project.String()+"/workflow", bad, &w.lead)
	if rec.Code != http.StatusUnprocessableEntity || errorCodeOf(t, rec) != "validation.failed" {
		t.Fatalf("unknown field = %d %s", rec.Code, rec.Body.String())
	}
}

func TestReplaceSucceedsAndReturnsVersion(t *testing.T) {
	w := newWorld(t)
	rec := w.do(t, http.MethodPut, "/projects/"+w.project.String()+"/workflow", w.currentDefinition(t, 1), &w.lead)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"version":2`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestMigrateIssuesEndpoint(t *testing.T) {
	w := newWorld(t)
	review, _ := w.repo.StatusByName(w.org, "En revisión")
	progress, _ := w.repo.StatusByName(w.org, "En curso")
	w.repo.AddIssues(w.project, review.ID, 3)
	body := `{"status_mapping":{"` + review.ID.String() + `":"` + progress.ID.String() + `"}}`
	rec := w.do(t, http.MethodPost, "/projects/"+w.project.String()+"/workflow/migrate-issues", body, &w.lead)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"moved":3`) {
		t.Fatalf("migrate = %d %s", rec.Code, rec.Body.String())
	}
	rec = w.do(t, http.MethodPost, "/projects/"+w.project.String()+"/workflow/migrate-issues", `{"status_mapping":{"not-a-uuid":"x"}}`, &w.lead)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad mapping = %d", rec.Code)
	}
}

func TestResetEndpointAndTemplates(t *testing.T) {
	w := newWorld(t)
	rec := w.do(t, http.MethodGet, "/workflow-templates", "", &w.viewer)
	if rec.Code != http.StatusOK {
		t.Fatalf("templates = %d", rec.Code)
	}
	var out struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data) != 4 || out.Data[2].Name != "Seguimiento de tareas" {
		t.Fatalf("templates = %s", rec.Body.String())
	}

	rec = w.do(t, http.MethodPost, "/projects/"+w.project.String()+"/workflow/reset", `{"template":"task_tracking","expected_version":1}`, &w.contributor)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("contributor reset = %d", rec.Code)
	}
	rec = w.do(t, http.MethodPost, "/projects/"+w.project.String()+"/workflow/reset", `{"template":"task_tracking","expected_version":1}`, &w.lead)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":2`) {
		t.Fatalf("reset = %d %s", rec.Code, rec.Body.String())
	}
}

func TestOrganizationStatusEndpoints(t *testing.T) {
	w := newWorld(t)
	path := "/organizations/" + w.org.String() + "/statuses"

	if rec := w.do(t, http.MethodGet, path, "", &w.orgMember); rec.Code != http.StatusOK {
		t.Fatalf("member list = %d", rec.Code)
	}
	if rec := w.do(t, http.MethodGet, path, "", &w.outsider); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider list = %d", rec.Code)
	}
	if rec := w.do(t, http.MethodPost, path, `{"name":"Bloqueado","category":"in_progress"}`, &w.orgMember); rec.Code != http.StatusForbidden {
		t.Fatalf("member create = %d", rec.Code)
	}
	rec := w.do(t, http.MethodPost, path, `{"name":"Bloqueado","category":"in_progress"}`, &w.orgAdmin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.do(t, http.MethodPost, path, `{"name":"Bloqueado","category":"in_progress","extra":1}`, &w.orgAdmin); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field = %d", rec.Code)
	}

	hecho, _ := w.repo.StatusByName(w.org, "Hecho")
	if rec := w.do(t, http.MethodPatch, path+"/"+hecho.ID.String(), `{"name":"Completado"}`, &w.orgAdmin); rec.Code != http.StatusOK {
		t.Fatalf("rename = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.do(t, http.MethodPatch, path+"/"+hecho.ID.String(), `{"category":"in_progress"}`, &w.orgAdmin); rec.Code != http.StatusConflict || errorCodeOf(t, rec) != "workflow.category_in_use" {
		t.Fatalf("invalidating category = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.do(t, http.MethodPatch, path+"/"+hecho.ID.String(), `{"name":"x"}`, &w.otherAdmin); rec.Code != http.StatusNotFound {
		t.Fatalf("other org admin patch = %d", rec.Code)
	}

	progress, _ := w.repo.StatusByName(w.org, "En curso")
	w.repo.AddIssues(w.project, progress.ID, 1)
	if rec := w.do(t, http.MethodDelete, path+"/"+progress.ID.String(), "", &w.orgAdmin); rec.Code != http.StatusConflict || errorCodeOf(t, rec) != "workflow.status_in_use" {
		t.Fatalf("delete in use = %d %s", rec.Code, rec.Body.String())
	}
	blocked, _ := w.repo.StatusByName(w.org, "Bloqueado")
	rec = w.do(t, http.MethodDelete, path+"/"+progress.ID.String()+"?reassign_to="+blocked.ID.String(), "", &w.orgAdmin)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"reassigned_issues":1`) {
		t.Fatalf("delete with reassignment = %d %s", rec.Code, rec.Body.String())
	}

	statuses, _ := w.repo.ListStatuses(context.Background(), w.org)
	reversed := make([]string, 0, len(statuses))
	for i := len(statuses) - 1; i >= 0; i-- {
		reversed = append(reversed, `"`+statuses[i].ID.String()+`"`)
	}
	if rec := w.do(t, http.MethodPut, path+"/reorder", `{"status_ids":[`+strings.Join(reversed, ",")+`]}`, &w.orgAdmin); rec.Code != http.StatusOK {
		t.Fatalf("reorder = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.do(t, http.MethodPut, path+"/reorder", `{"status_ids":["`+statuses[0].ID.String()+`"]}`, &w.orgAdmin); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short reorder = %d", rec.Code)
	}
}

func TestReplaceRefusesUnknownRuleWithTransitionPath(t *testing.T) {
	w := newWorld(t)
	body := `{"expected_version":1,"initial_ref":"a","statuses":[{"ref":"a","name":"A","category":"todo"}],` +
		`"transitions":[{"id":"t","name":"T","from":["a"],"to":"a","conditions":[{"type":"no_such_rule"}],"validators":[],"post_functions":[],"required_fields":[]}]}`
	rec := w.do(t, http.MethodPut, "/projects/"+w.project.String()+"/workflow", body, &w.lead)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"path":"transitions[0]"`) || !strings.Contains(rec.Body.String(), `"code":"workflow.unknown_rule_type"`) {
		t.Fatalf("details = %s", rec.Body.String())
	}
}
