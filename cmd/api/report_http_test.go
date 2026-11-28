package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

func (e *transitionEnv) rawGet(path, bearer string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.client.router.ServeHTTP(rec, req)
	return rec
}

func reportErrCode(body map[string]any) string {
	if e, ok := body["error"].(map[string]any); ok {
		code, _ := e["code"].(string)
		return code
	}
	return ""
}

// TestProjectReportsThroughTheRealRouter drives the report endpoints with real authentication,
// project permissions, CoreQL filters and CSV export.
func TestProjectReportsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	project := env.project.String()
	base := "/projects/" + project + "/reports"
	now := time.Now().UTC()
	from, to := now.AddDate(0, 0, -7).Format("2006-01-02"), now.Format("2006-01-02")

	// Authentication and visibility. The project is private: the outsider cannot see it.
	if status, _ := env.client.do(http.MethodGet, base, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, base+"/workload", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token run: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, base+"/workload", env.outside, nil); status != http.StatusNotFound && status != http.StatusForbidden {
		t.Fatalf("outsider: %d, want 404 or 403", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/projects/"+uuid.NewString()+"/reports/workload", env.lead, nil); status != http.StatusNotFound && status != http.StatusForbidden {
		t.Fatalf("unknown project: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/projects/not-a-uuid/reports/workload", env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("bad project id: %d", status)
	}

	// Permission matrix (default scheme: lead, contributor and viewer hold view_reports).
	for name, token := range map[string]string{"lead": env.lead, "contributor": env.contrib, "viewer": env.viewer} {
		if status, _ := env.client.do(http.MethodGet, base+"/workload", token, nil); status != http.StatusOK {
			t.Fatalf("%s: %d", name, status)
		}
	}
	admin := uuid.New()
	env.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'admin', 'admin', 'admin@example.test', 'x', 'user')`, admin)
	env.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ('00000000-0000-4000-8000-000000000002', $1, 'admin')`, admin)
	adminToken, err := authtoken.Generate(transitionTestSecret, time.Hour, admin, "user")
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := env.client.do(http.MethodGet, base+"/workload", adminToken, nil); status != http.StatusOK {
		t.Fatalf("org admin: %d", status)
	}

	// Catalog.
	status, body := env.client.do(http.MethodGet, base, env.lead, nil)
	list, _ := body["data"].([]any)
	if status != http.StatusOK || len(list) != 11 || list[0].(map[string]any)["kind"] != "burndown" || list[0].(map[string]any)["description"] == "" {
		t.Fatalf("catalog: %d %v", status, body)
	}

	// Data: two issues, one resolved, with estimate and worklog.
	open, done := env.createIssue(env.contrib), env.createIssue(env.contrib)
	env.exec(`UPDATE issues SET assignee_id = (SELECT id FROM users WHERE name = 'contributor'), original_estimate_seconds = 7200, remaining_estimate_seconds = 3600, story_points = 5 WHERE id = $1`, open)
	env.exec(`UPDATE issues SET status_id = (SELECT id FROM statuses WHERE category = 'done' ORDER BY position LIMIT 1), resolved_at = now() WHERE id = $1`, done)
	if status, _ := env.client.do(http.MethodPost, "/issues/"+open+"/worklogs", env.contrib, map[string]any{"time_spent": "1h"}); status != http.StatusCreated {
		t.Fatalf("worklog: %d", status)
	}

	for _, kind := range []string{"cumulative-flow", "control", "created-vs-resolved", "resolution-time", "time-tracking", "workload", "velocity"} {
		status, body = env.client.do(http.MethodGet, base+"/"+kind+"?from="+from+"&to="+to, env.lead, nil)
		if status != http.StatusOK {
			t.Fatalf("%s: %d %v", kind, status, body)
		}
		data := tfData(t, body)
		for _, key := range []string{"kind", "title", "generated_at", "params", "series", "table", "summary"} {
			if _, ok := data[key]; !ok {
				t.Fatalf("%s lacks %q: %v", kind, key, data)
			}
		}
		if data["kind"] != kind {
			t.Fatalf("kind = %v", data["kind"])
		}
	}
	status, body = env.client.do(http.MethodGet, base+"/workload", env.lead, nil)
	summary := tfData(t, body)["summary"].(map[string]any)
	if summary["open"].(float64) != 1 || summary["remaining_hours"].(float64) != 0 {
		t.Fatalf("workload summary = %v", summary)
	}
	status, body = env.client.do(http.MethodGet, base+"/created-vs-resolved?from="+from+"&to="+to+"&interval=week", env.lead, nil)
	summary = tfData(t, body)["summary"].(map[string]any)
	if summary["created"].(float64) != 2 || summary["resolved"].(float64) != 1 {
		t.Fatalf("created-vs-resolved summary = %v", summary)
	}
	status, body = env.client.do(http.MethodGet, base+"/time-tracking?from="+from+"&to="+to, env.lead, nil)
	summary = tfData(t, body)["summary"].(map[string]any)
	if summary["spent_hours"].(float64) != 1 || summary["estimate_hours"].(float64) != 2 || summary["accuracy_percent"].(float64) != 50 {
		t.Fatalf("time-tracking summary = %v", summary)
	}

	// Sprint reports: no sprint yet.
	if status, body = env.client.do(http.MethodGet, base+"/burndown", env.lead, nil); status != http.StatusNotFound || reportErrCode(body) != "report.sprint_not_found" {
		t.Fatalf("burndown without sprint: %d %v", status, body)
	}
	sprint := uuid.NewString()
	if status, body = env.client.do(http.MethodGet, base+"/sprint-report?sprint="+sprint, env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("sprint report with a foreign sprint: %d %v", status, body)
	}
	status, body = env.client.do(http.MethodPost, "/sprints?project_id="+project, env.lead, map[string]any{"name": "S1", "start_date": from, "end_date": to})
	if status != http.StatusCreated {
		t.Fatalf("create sprint: %d %v", status, body)
	}
	sprint = tfData(t, body)["id"].(string)
	if status, body = env.client.do(http.MethodPut, "/sprints/"+sprint+"/issues/"+open, env.lead, nil); status != http.StatusNoContent && status != http.StatusOK {
		t.Fatalf("assign: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodPost, "/sprints/"+sprint+"/start", env.lead, map[string]any{"start_date": from, "end_date": to}); status != http.StatusOK {
		t.Fatalf("start: %d %v", status, body)
	}
	for _, kind := range []string{"burndown", "burnup", "sprint-report"} {
		if status, body = env.client.do(http.MethodGet, base+"/"+kind+"?sprint="+sprint, env.lead, nil); status != http.StatusOK {
			t.Fatalf("%s: %d %v", kind, status, body)
		}
	}
	status, body = env.client.do(http.MethodGet, base+"/burndown", env.lead, nil)
	data := tfData(t, body)
	if status != http.StatusOK || data["summary"].(map[string]any)["committed"].(float64) != 5 || data["params"].(map[string]any)["sprint"] != sprint {
		t.Fatalf("default (active) sprint burndown: %d %v", status, data)
	}
	if status, body = env.client.do(http.MethodGet, base+"/burndown?group_by=issues", env.lead, nil); status != http.StatusOK || tfData(t, body)["summary"].(map[string]any)["committed"].(float64) != 1 {
		t.Fatalf("burndown by issues: %d %v", status, body)
	}

	// Version report.
	version := uuid.NewString()
	env.exec(`INSERT INTO versions (id, project_id, name) VALUES ($1, $2, '1.0')`, version, project)
	env.exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES ($1, $2, 'fix'), ($3, $2, 'fix')`, open, version, done)
	status, body = env.client.do(http.MethodGet, base+"/version?version="+version, env.lead, nil)
	summary = tfData(t, body)["summary"].(map[string]any)
	if status != http.StatusOK || summary["total"].(float64) != 2 || summary["done"].(float64) != 1 || summary["percent_done"].(float64) != 50 {
		t.Fatalf("version: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodGet, base+"/version", env.lead, nil); status != http.StatusUnprocessableEntity || reportErrCode(body) != "report.version_required" {
		t.Fatalf("version without id: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodGet, base+"/version?version="+uuid.NewString(), env.lead, nil); status != http.StatusNotFound || reportErrCode(body) != "report.version_not_found" {
		t.Fatalf("foreign version: %d %v", status, body)
	}

	// CoreQL filter through the search module: only the open issue matches.
	status, body = env.client.do(http.MethodGet, base+"/workload?filter="+strings.ReplaceAll("status = \"Por hacer\"", " ", "%20"), env.lead, nil)
	if status != http.StatusOK {
		t.Fatalf("filter: %d %v", status, body)
	}
	if tfData(t, body)["params"].(map[string]any)["filter"] == nil {
		t.Fatalf("filter not echoed: %v", body)
	}
	if status, body = env.client.do(http.MethodGet, base+"/workload?filter=%3D%3D%3D", env.lead, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid filter: %d %v", status, body)
	}
	if status, body = env.client.do(http.MethodGet, base+"/workload?filter="+strings.Repeat("a", 2001), env.lead, nil); status != http.StatusUnprocessableEntity || reportErrCode(body) != "report.filter_invalid" {
		t.Fatalf("long filter: %d %v", status, body)
	}

	// Validation (422) and unknown report (404), including hostile values.
	for name, path := range map[string]string{
		"bad date":     "/control?from=2024-02-31",
		"sql in date":  "/control?from=1%27%3B%20DROP%20TABLE%20issues%3B--",
		"too long":     "/control?from=2020-01-01&to=2024-01-01",
		"reversed":     "/control?from=2024-02-02&to=2024-02-01",
		"group_by":     "/time-tracking?group_by=%27%20OR%201%3D1",
		"interval":     "/created-vs-resolved?interval=year",
		"limit":        "/velocity?limit=99999999999999999999",
		"sprint":       "/burndown?sprint=abc",
		"version uuid": "/version?version=1%20OR%201%3D1",
	} {
		if status, body = env.client.do(http.MethodGet, base+path, env.lead, nil); status != http.StatusUnprocessableEntity || reportErrCode(body) == "" {
			t.Fatalf("%s: %d %v", name, status, body)
		}
	}
	if status, body = env.client.do(http.MethodGet, base+"/nope", env.lead, nil); status != http.StatusNotFound || reportErrCode(body) != "report.kind_unknown" {
		t.Fatalf("unknown kind: %d %v", status, body)
	}

	// Cache headers and CSV.
	rec := env.rawGet(base+"/workload", env.lead)
	if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=30" {
		t.Fatalf("cache-control = %q", cc)
	}
	rec = env.rawGet(base+"/workload.csv", env.lead)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "informe-workload-") {
		t.Fatalf("csv: %d %v", rec.Code, rec.Header())
	}
	csvBody := rec.Body.String()
	if !strings.HasPrefix(csvBody, "\xEF\xBB\xBF") || !strings.Contains(csvBody, "Persona,Abiertas,En curso,Puntos") {
		t.Fatalf("csv body = %q", csvBody)
	}
	if rec = env.rawGet(base+"/workload.csv", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("csv without token: %d", rec.Code)
	}
	if rec = env.rawGet(base+"/workload.csv", env.outside); rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("csv outsider: %d", rec.Code)
	}

	// A soft-deleted project disappears.
	env.exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, project)
	if status, _ = env.client.do(http.MethodGet, base+"/workload", env.lead, nil); status != http.StatusNotFound && status != http.StatusForbidden {
		t.Fatalf("deleted project: %d", status)
	}
}
