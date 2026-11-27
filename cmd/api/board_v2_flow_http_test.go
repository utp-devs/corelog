package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestBoardV2CreateReplaceAndAmbiguousMoves covers board creation, the mapping validation and the
// resolution of moves into columns that hold several statuses.
func TestBoardV2CreateReplaceAndAmbiguousMoves(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	list := "/projects/" + env.project.String() + "/boards"

	if status, _ := c.do(http.MethodPost, list, env.contrib, map[string]any{"name": "Nuevo"}); status != http.StatusForbidden {
		t.Fatalf("create as contributor: %d, want 403", status)
	}
	if status, _ := c.do(http.MethodPost, list, env.lead, map[string]any{"name": ""}); status != http.StatusUnprocessableEntity {
		t.Fatalf("create without name: %d, want 422", status)
	}
	status, body := c.do(http.MethodPost, list, env.lead, map[string]any{"name": "Kanban de apoyo"})
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	created := tfData(t, body)
	if created["is_default"] != false || len(created["columns"].([]any)) != 3 || created["show_unmapped_lane"] != true {
		t.Fatalf("created board = %v", created)
	}
	if status, body := c.do(http.MethodPost, list, env.lead, map[string]any{"name": "kanban DE apoyo"}); status != http.StatusConflict || boardCodeOf(body) != "board.name_taken" {
		t.Fatalf("duplicate name: %d %v", status, body)
	}
	if status, body := c.do(http.MethodGet, list, env.viewer, nil); status != http.StatusOK || len(body["data"].([]any)) != 2 {
		t.Fatalf("list: %d %v", status, body)
	}

	ids := map[string]string{}
	for _, s := range created["workflow_statuses"].([]any) {
		m := s.(map[string]any)
		ids[m["name"].(string)] = m["id"].(string)
	}
	boardPath := "/boards/" + created["id"].(string)

	// Mapping validation: a status twice is rejected with field details; an unmapped remainder is allowed.
	dup := []map[string]any{{"name": "A", "status_ids": []string{ids["Por hacer"]}}, {"name": "B", "status_ids": []string{ids["Por hacer"]}}}
	if status, body := c.do(http.MethodPut, boardPath+"/columns", env.lead, map[string]any{"columns": dup}); status != http.StatusUnprocessableEntity || boardCodeOf(body) != "board.invalid_columns" {
		t.Fatalf("duplicate mapping: %d %v", status, body)
	}
	foreign := []map[string]any{{"name": "A", "status_ids": []string{uuid.NewString()}}}
	if status, _ := c.do(http.MethodPut, boardPath+"/columns", env.lead, map[string]any{"columns": foreign}); status != http.StatusUnprocessableEntity {
		t.Fatalf("foreign status: %d, want 422", status)
	}
	if status, _ := c.do(http.MethodPut, boardPath+"/columns", env.lead, map[string]any{"columns": []map[string]any{}}); status != http.StatusUnprocessableEntity {
		t.Fatalf("replace with no columns: %d, want 422", status)
	}
	if status, _ := c.do(http.MethodPut, boardPath+"/columns", env.contrib, map[string]any{"columns": dup[:1]}); status != http.StatusForbidden {
		t.Fatalf("replace as contributor: %d, want 403", status)
	}
	columns := []map[string]any{
		{"name": "Pendiente", "status_ids": []string{ids["Por hacer"], ids["En revisión"]}, "max_issues": 1},
		{"name": "Vacia", "status_ids": []string{}},
		{"name": "Hecho", "status_ids": []string{ids["Hecho"]}},
	}
	status, body = c.do(http.MethodPut, boardPath+"/columns", env.lead, map[string]any{"columns": columns})
	if status != http.StatusOK {
		t.Fatalf("replace columns: %d %v", status, body)
	}
	detail := tfData(t, body)
	cols := detail["columns"].([]any)
	pending, empty, done := cols[0].(map[string]any)["id"].(string), cols[1].(map[string]any)["id"].(string), cols[2].(map[string]any)["id"].(string)
	if len(detail["unmapped_status_ids"].([]any)) != 1 {
		t.Fatalf("unmapped statuses = %v, want only En curso", detail["unmapped_status_ids"])
	}

	// Quick filters round trip and the view applies them.
	if status, body := c.do(http.MethodPut, boardPath+"/quick-filters", env.lead, map[string]any{"quick_filters": []map[string]any{{"name": "Mal", "query": "nope"}}}); status != http.StatusUnprocessableEntity || boardCodeOf(body) != "board.invalid_query" {
		t.Fatalf("invalid quick filter: %d %v", status, body)
	}
	status, body = c.do(http.MethodPut, boardPath+"/quick-filters", env.lead, map[string]any{"quick_filters": []map[string]any{{"name": "Mías", "query": "assignee = currentUser()"}}})
	if status != http.StatusOK {
		t.Fatalf("quick filters: %d %v", status, body)
	}
	filterID := tfData(t, body)["quick_filters"].([]any)[0].(map[string]any)["id"].(string)

	issueID := env.createIssue(env.contrib)
	_, body = c.do(http.MethodGet, boardPath+"/view?quick_filters="+filterID, env.contrib, nil)
	first := boardColumns(t, tfData(t, body))[0]
	if first["count"].(float64) != 0 || first["total"].(float64) != 1 {
		t.Fatalf("filtered column count/total = %v/%v, want 0/1", first["count"], first["total"])
	}
	if status, _ := c.do(http.MethodGet, boardPath+"/view?quick_filters="+uuid.NewString(), env.contrib, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown quick filter: %d, want 422", status)
	}
	if status, _ := c.do(http.MethodGet, boardPath+"/view?sprint=zzz", env.contrib, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid sprint parameter: %d, want 422", status)
	}

	// A column without statuses is not reachable.
	move := boardPath + "/cards/" + issueID + "/move"
	if status, body := c.do(http.MethodPost, move, env.contrib, map[string]any{"column_id": empty}); status != http.StatusConflict || boardCodeOf(body) != "issue.transition_unavailable" {
		t.Fatalf("move into an empty column: %d %v", status, body)
	}
	// Por hacer -> Hecho (complete) and back to Pendiente (reopen leads to Por hacer only).
	if status, body := c.do(http.MethodPost, move, env.contrib, map[string]any{"column_id": done}); status != http.StatusOK {
		t.Fatalf("move to done: %d %v", status, body)
	}
	if status, body := c.do(http.MethodPost, move, env.contrib, map[string]any{"column_id": pending}); status != http.StatusOK || tfData(t, body)["transition_id"] != "reopen" {
		t.Fatalf("move back: %d %v", status, body)
	}
	// From En curso both pause (to Por hacer) and send_to_review (to En revisión) lead into Pendiente.
	if status, body := c.do(http.MethodPost, "/issues/"+issueID+"/transitions", env.contrib, map[string]any{"transition_id": "start"}); status != http.StatusOK {
		t.Fatalf("start: %d %v", status, body)
	}
	status, body = c.do(http.MethodPost, move, env.contrib, map[string]any{"column_id": pending})
	if status != http.StatusConflict || boardCodeOf(body) != "board.ambiguous_transition" {
		t.Fatalf("ambiguous move: %d %v", status, body)
	}
	if details, _ := body["error"].(map[string]any)["details"].([]any); len(details) != 2 {
		t.Fatalf("candidates = %v", details)
	}
	if status, body := c.do(http.MethodPost, move, env.contrib, map[string]any{"column_id": pending, "transition_id": "send_to_review"}); status != http.StatusOK || tfData(t, body)["wip"].(map[string]any)["state"] != "at_max" {
		t.Fatalf("explicit transition: %d %v", status, body)
	}

	// Soft-deleted issues leave the board; deleting boards needs manage_board and spares the default one.
	env.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, issueID)
	_, body = c.do(http.MethodGet, boardPath+"/view", env.contrib, nil)
	if boardColumns(t, tfData(t, body))[0]["total"].(float64) != 0 {
		t.Fatalf("deleted issue still on the board: %v", body)
	}
	if status, _ := c.do(http.MethodDelete, boardPath, env.contrib, nil); status != http.StatusForbidden {
		t.Fatalf("delete as contributor: %d, want 403", status)
	}
	if status, body := c.do(http.MethodDelete, "/boards/"+env.board.String(), env.lead, nil); status != http.StatusConflict || boardCodeOf(body) != "board.default_not_deletable" {
		t.Fatalf("delete default: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodDelete, boardPath, env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("delete board: %d, want 204", status)
	}
	if status, _ := c.do(http.MethodGet, boardPath, env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("deleted board: %d, want 404", status)
	}
	// The legacy endpoint keeps answering.
	if status, _ := c.do(http.MethodGet, "/projects/"+env.project.String()+"/board", env.viewer, nil); status != http.StatusOK {
		t.Fatalf("legacy board: %d, want 200", status)
	}
}
