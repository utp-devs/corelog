package main

import (
	"net/http"
	"testing"
	"time"
)

// TestWorklogsThroughTheRealRouter drives time tracking with real authentication and project permissions.
func TestWorklogsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	project := env.project.String()
	issue := env.createIssue(env.contrib)
	now := time.Now().UTC()
	from, to := now.AddDate(0, 0, -1).Format("2006-01-02"), now.AddDate(0, 0, 1).Format("2006-01-02")
	logPath := "/issues/" + issue + "/worklogs"

	// Authentication and visibility.
	if status, _ := env.client.do(http.MethodGet, logPath, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, logPath, env.outside, nil); status != http.StatusForbidden {
		t.Fatalf("outsider list: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/issues/"+project+"/worklogs", env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("project id used as issue: %d", status)
	}

	// Permission matrix: the viewer reads but cannot log.
	if status, _ := env.client.do(http.MethodGet, logPath, env.viewer, nil); status != http.StatusOK {
		t.Fatalf("viewer list: %d", status)
	}
	if status, _ := env.client.do(http.MethodPost, logPath, env.viewer, map[string]any{"time_spent": "1h"}); status != http.StatusForbidden {
		t.Fatalf("viewer log: %d", status)
	}

	// Validation: exactly one duration form, strict fields, parseable durations.
	if status, body := env.client.do(http.MethodPost, logPath, env.contrib, map[string]any{"time_spent": "1x"}); status != http.StatusUnprocessableEntity || body["error"].(map[string]any)["code"] != "worklog.duration_invalid" {
		t.Fatalf("bad duration: %d %v", status, body)
	}
	if status, _ := env.client.do(http.MethodPost, logPath, env.contrib, map[string]any{"time_spent": "1h", "unknown": 1}); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d", status)
	}
	if status, _ := env.client.do(http.MethodPost, logPath, env.contrib, map[string]any{"time_spent": "1h", "time_spent_seconds": 60}); status != http.StatusUnprocessableEntity {
		t.Fatalf("both duration forms: %d", status)
	}
	if status, _ := env.client.do(http.MethodPost, "/issues/"+project+"x/worklogs", env.contrib, map[string]any{"time_spent": "1h"}); status != http.StatusNotFound {
		t.Fatalf("bad issue id: %d", status)
	}

	// Lifecycle: the contributor logs 1h with the default auto mode.
	status, body := env.client.do(http.MethodPost, logPath, env.contrib, map[string]any{
		"time_spent_seconds": 3600, "description": "Revision", "remaining": map[string]any{"mode": "auto"},
	})
	if status != http.StatusCreated {
		t.Fatalf("log: %d %v", status, body)
	}
	data := tfData(t, body)
	worklog := data["worklog"].(map[string]any)["id"].(string)
	if data["totals"].(map[string]any)["time_spent_seconds"].(float64) != 3600 {
		t.Fatalf("totals after log: %v", data["totals"])
	}

	// Edit: the author may edit; the viewer and a contributor editing the lead's entry may not.
	if status, _ = env.client.do(http.MethodPatch, "/worklogs/"+worklog, env.viewer, map[string]any{"description": "x"}); status != http.StatusForbidden {
		t.Fatalf("viewer edit: %d", status)
	}
	if status, _ = env.client.do(http.MethodPatch, "/worklogs/"+worklog, env.lead, map[string]any{"description": "Revisado por lead"}); status != http.StatusOK {
		t.Fatalf("lead edit: %d", status)
	}
	if status, _ = env.client.do(http.MethodPatch, "/worklogs/"+worklog, env.contrib, map[string]any{"time_spent": "2h"}); status != http.StatusOK {
		t.Fatalf("author edit: %d", status)
	}

	// Estimates: the contributor may set them, the viewer may not, bad text is 422.
	if status, body = env.client.do(http.MethodPut, "/issues/"+issue+"/estimates", env.contrib, map[string]any{"original": "1d"}); status != http.StatusOK {
		t.Fatalf("estimates: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPut, "/issues/"+issue+"/estimates", env.viewer, map[string]any{"original": "1d"}); status != http.StatusForbidden {
		t.Fatalf("viewer estimates: %d", status)
	}
	if status, _ = env.client.do(http.MethodPut, "/issues/"+issue+"/estimates", env.contrib, map[string]any{"original": "5"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("bare number estimate: %d", status)
	}

	// Reads: list with totals, timesheet and summary.
	status, body = env.client.do(http.MethodGet, logPath, env.lead, nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d %v", status, body)
	}
	if got := tfData(t, body)["totals"].(map[string]any)["original_estimate_seconds"]; got == nil {
		t.Fatalf("original estimate must be set after the estimate call")
	}
	if status, body = env.client.do(http.MethodGet, "/me/timesheet?from="+from+"&to="+to, env.contrib, nil); status != http.StatusOK {
		t.Fatalf("timesheet: %d %v", status, body)
	} else if tfData(t, body)["total_seconds"].(float64) != 7200 {
		t.Fatalf("timesheet total = %v", tfData(t, body)["total_seconds"])
	}
	if status, _ = env.client.do(http.MethodGet, "/me/timesheet?from="+from, env.contrib, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("timesheet without to: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, "/me/timesheet?from="+from+"&to="+to+"&user=00000000-0000-4000-8000-0000000000ff", env.contrib, nil); status != http.StatusOK {
		t.Fatalf("timesheet of unknown user: %d", status)
	}
	summary := "/projects/" + project + "/time-summary?from=" + from + "&to=" + to
	if status, body = env.client.do(http.MethodGet, summary+"&group_by=issue", env.lead, nil); status != http.StatusOK {
		t.Fatalf("summary: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodGet, summary+"&group_by=week", env.lead, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad group_by: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, summary, env.outside, nil); status != http.StatusForbidden {
		t.Fatalf("outsider summary: %d", status)
	}

	// Delete: the author removes the entry; the spent aggregate returns to zero.
	if status, _ = env.client.do(http.MethodDelete, "/worklogs/"+worklog, env.contrib, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ = env.client.do(http.MethodDelete, "/worklogs/"+worklog, env.contrib, nil); status != http.StatusNotFound {
		t.Fatalf("second delete: %d", status)
	}
	_, body = env.client.do(http.MethodGet, logPath, env.contrib, nil)
	if tfData(t, body)["totals"].(map[string]any)["time_spent_seconds"].(float64) != 0 {
		t.Fatalf("spent after delete: %v", tfData(t, body)["totals"])
	}

	// A soft-deleted issue disappears from the time endpoints.
	env.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, issue)
	if status, _ = env.client.do(http.MethodGet, logPath, env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("deleted issue: %d", status)
	}
}
