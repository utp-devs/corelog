package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func sprintList(t *testing.T, body map[string]any) []any {
	t.Helper()
	list, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("response has no data list: %v", body)
	}
	return list
}

// TestSprintAndBacklogThroughTheRealRouter drives the whole planning flow with real permissions.
func TestSprintAndBacklogThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	project := env.project.String()
	q := "?project_id=" + project

	// Authentication, visibility and permission matrix.
	if status, _ := env.client.do(http.MethodGet, "/sprints"+q, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/sprints"+q, env.outside, nil); status != http.StatusForbidden {
		t.Fatalf("outsider list: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/projects/"+project+"/backlog", env.outside, nil); status != http.StatusForbidden {
		t.Fatalf("outsider backlog: %d", status)
	}
	for name, token := range map[string]string{"viewer": env.viewer, "contributor": env.contrib} {
		if status, _ := env.client.do(http.MethodPost, "/sprints"+q, token, map[string]any{"name": "X"}); status != http.StatusForbidden {
			t.Fatalf("%s create sprint: %d", name, status)
		}
		if status, _ := env.client.do(http.MethodGet, "/projects/"+project+"/backlog", token, nil); status != http.StatusOK {
			t.Fatalf("%s backlog: %d", name, status)
		}
	}
	if status, _ := env.client.do(http.MethodPost, "/sprints"+q, env.lead, map[string]any{"name": "X", "unknown": 1}); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d", status)
	}

	status, body := env.client.do(http.MethodPost, "/sprints"+q, env.lead, map[string]any{"name": "Sprint 1", "goal": "Probar"})
	if status != http.StatusCreated {
		t.Fatalf("create sprint: %d %v", status, body)
	}
	sprint := tfData(t, body)["id"].(string)
	_, body = env.client.do(http.MethodPost, "/sprints"+q, env.lead, map[string]any{"name": "Sprint 2"})
	next := tfData(t, body)["id"].(string)

	a, b, c := env.createIssue(env.contrib), env.createIssue(env.contrib), env.createIssue(env.contrib)
	for id, pts := range map[string]int{a: 5, b: 3, c: 2} {
		env.exec(`UPDATE issues SET story_points = $2 WHERE id = $1`, id, pts)
	}

	// Empty start is a 422; the get route works without project_id and a foreign project is a 404.
	if status, body = env.client.do(http.MethodPost, "/sprints/"+sprint+"/start", env.lead, nil); status != http.StatusUnprocessableEntity || body["error"].(map[string]any)["code"] != "sprint.empty" {
		t.Fatalf("empty start: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodGet, "/sprints/"+sprint, env.lead, nil); status != http.StatusOK {
		t.Fatalf("get without project_id: %d", status)
	}
	foreign := uuid.New()
	env.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		SELECT $1, organization_id, 'FOR', 'Otro', 'kanban', 'scrum', lead_id, 'private' FROM projects WHERE id = $2`, foreign, env.project)
	env.exec(`INSERT INTO project_members (project_id, user_id, role) SELECT $1, user_id, role FROM project_members WHERE project_id = $2`, foreign, env.project)
	if status, _ = env.client.do(http.MethodGet, "/sprints/"+sprint+"?project_id="+foreign.String(), env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("get with another project: %d", status)
	}

	// Backlog rank: contributors cannot touch planned sprints, leads can; the static route wins over /backlog/{issue_id}.
	rank := map[string]any{"issue_ids": []string{a, b}, "target": map[string]any{"sprint_id": sprint}}
	if status, _ = env.client.do(http.MethodPut, "/backlog/rank", env.contrib, rank); status != http.StatusForbidden {
		t.Fatalf("contributor into planned sprint: %d", status)
	}
	if status, body = env.client.do(http.MethodPut, "/backlog/rank", env.lead, rank); status != http.StatusOK {
		t.Fatalf("lead rank: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPut, "/backlog/rank", env.lead, map[string]any{"issue_ids": []string{a}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("missing target: %d", status)
	}
	if status, _ = env.client.do(http.MethodPut, "/backlog/rank", "", rank); status != http.StatusUnauthorized {
		t.Fatalf("rank without token: %d", status)
	}

	// The old per-issue backlog endpoints keep working next to the new one.
	if status, _ = env.client.do(http.MethodGet, "/backlog"+q, env.lead, nil); status != http.StatusOK {
		t.Fatalf("legacy backlog list: %d", status)
	}

	status, body = env.client.do(http.MethodPost, "/sprints/"+sprint+"/start", env.lead, map[string]any{"start_date": "2026-11-02", "end_date": "2026-11-13", "goal": "Salir"})
	if status != http.StatusOK || tfData(t, body)["status"] != "active" || tfData(t, body)["committed_points"].(float64) != 8 {
		t.Fatalf("start: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPost, "/sprints/"+next+"/start", env.lead, map[string]any{"force": true}); status != http.StatusConflict {
		t.Fatalf("second active sprint: %d", status)
	}
	if status, _ = env.client.do(http.MethodDelete, "/sprints/"+sprint, env.lead, nil); status != http.StatusConflict {
		t.Fatalf("delete active: %d", status)
	}

	// Contributors may add to the active sprint; the backlog read shows sections and totals.
	if status, _ = env.client.do(http.MethodPut, "/backlog/rank", env.contrib, map[string]any{"issue_ids": []string{c}, "target": map[string]any{"sprint_id": sprint}}); status != http.StatusOK {
		t.Fatalf("contributor into active sprint: %d", status)
	}
	status, body = env.client.do(http.MethodGet, "/projects/"+project+"/backlog", env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("backlog: %d", status)
	}
	sections := tfData(t, body)["sections"].([]any)
	first := sections[0].(map[string]any)
	if first["kind"] != "active" || len(first["issues"].([]any)) != 3 || first["totals"].(map[string]any)["points"].(float64) != 10 {
		t.Fatalf("active section %v", first)
	}

	// Finish one issue, complete to a new sprint, read the report.
	env.exec(`UPDATE issues SET status = 'closed', closed_at = now() WHERE id = $1`, a)
	status, body = env.client.do(http.MethodPost, "/sprints/"+sprint+"/complete", env.lead, map[string]any{"destination": "new"})
	done := tfData(t, body)
	if status != http.StatusOK || done["completed_issues"].(float64) != 1 || done["moved_issues"].(float64) != 2 || done["next_sprint"] == nil {
		t.Fatalf("complete: %d %v", status, body)
	}
	status, body = env.client.do(http.MethodGet, "/sprints/"+sprint+"/report", env.viewer, nil)
	report := tfData(t, body)
	if status != http.StatusOK || report["committed_points"].(float64) != 8 || report["completed_points"].(float64) != 5 ||
		len(report["added_after_start"].([]any)) != 1 || len(report["burndown"].([]any)) == 0 {
		t.Fatalf("report: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPost, "/sprints/"+sprint+"/complete", env.lead, nil); status != http.StatusConflict {
		t.Fatalf("complete twice: %d", status)
	}
	if status, body = env.client.do(http.MethodGet, "/sprints/velocity"+q, env.viewer, nil); status != http.StatusOK || sprintList(t, body)[0].(map[string]any)["points"].(float64) != 5 {
		t.Fatalf("velocity: %d %v", status, body)
	}

	// Edit and delete a planned sprint.
	if status, body = env.client.do(http.MethodPatch, "/sprints/"+next, env.lead, map[string]any{"goal": "Nuevo objetivo", "end_date": "2026-12-01"}); status != http.StatusOK || tfData(t, body)["goal"] != "Nuevo objetivo" {
		t.Fatalf("patch: %d %v", status, body)
	}
	if status, _ = env.client.do(http.MethodPatch, "/sprints/"+sprint, env.lead, map[string]any{"name": "Otro"}); status != http.StatusConflict {
		t.Fatalf("patch closed: %d", status)
	}
	if status, _ = env.client.do(http.MethodDelete, "/sprints/"+next, env.contrib, nil); status != http.StatusForbidden {
		t.Fatalf("contributor delete: %d", status)
	}
	if status, _ = env.client.do(http.MethodDelete, "/sprints/"+next, env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("delete planned: %d", status)
	}

	// Archived projects are read-only.
	env.exec(`UPDATE projects SET archived_at = now() WHERE id = $1`, env.project)
	if status, _ = env.client.do(http.MethodPost, "/sprints"+q, env.lead, map[string]any{"name": "Z"}); status != http.StatusConflict {
		t.Fatalf("create on archived project: %d", status)
	}
	if status, _ = env.client.do(http.MethodGet, "/projects/"+project+"/backlog", env.lead, nil); status != http.StatusOK {
		t.Fatalf("read archived backlog: %d", status)
	}
}
