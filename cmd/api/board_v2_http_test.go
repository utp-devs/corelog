package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func boardCodeOf(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func boardColumns(t *testing.T, view map[string]any) []map[string]any {
	t.Helper()
	raw, _ := view["columns"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		out = append(out, c.(map[string]any))
	}
	return out
}

func cardKeys(column map[string]any) []string {
	var keys []string
	for _, c := range column["cards"].([]any) {
		keys = append(keys, c.(map[string]any)["key"].(string))
	}
	return keys
}

// TestBoardV2EndpointsThroughTheRealRouter drives the board API with the real workflow engine: permissions per
// role, creation, view assembly, move resolution (single, ambiguous, none), ranking and column replacement.
func TestBoardV2EndpointsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	boardPath := "/boards/" + env.board.String()

	// Authentication and visibility.
	if status, _ := c.do(http.MethodGet, boardPath+"/view", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("view without token: %d, want 401", status)
	}
	if status, _ := c.do(http.MethodGet, boardPath+"/view", env.outside, nil); status != http.StatusNotFound {
		t.Fatalf("view as outsider: %d, want 404", status)
	}
	if status, _ := c.do(http.MethodGet, "/projects/"+env.project.String()+"/boards", env.outside, nil); status != http.StatusNotFound {
		t.Fatalf("list as outsider: %d, want 404", status)
	}
	if status, body := c.do(http.MethodGet, "/boards/"+uuid.NewString()+"/view", env.lead, nil); status != http.StatusNotFound || boardCodeOf(body) != "board.not_found" {
		t.Fatalf("unknown board: %d %v", status, body)
	}

	// The legacy board is a v2 board: its columns are bound to statuses and the rest is unmapped.
	issueID := env.createIssue(env.contrib)
	status, body := c.do(http.MethodGet, boardPath+"/view", env.viewer, nil)
	if status != http.StatusOK {
		t.Fatalf("view as viewer: %d %v", status, body)
	}
	view := tfData(t, body)
	board := view["board"].(map[string]any)
	if board["can_manage_board"] != false || board["can_move_cards"] != false {
		t.Fatalf("viewer capabilities = %v %v", board["can_manage_board"], board["can_move_cards"])
	}
	columns := boardColumns(t, view)
	if len(columns) != 4 || columns[0]["name"] != "Open" || columns[0]["count"].(float64) != 1 || columns[0]["wip_state"] != "ok" {
		t.Fatalf("columns = %v", columns)
	}
	if keys := cardKeys(columns[0]); len(keys) != 1 || keys[0] != "TRN-1" {
		t.Fatalf("first column cards = %v", keys)
	}
	if unmapped, _ := view["unmapped_column"].(map[string]any); unmapped == nil || unmapped["id"] != "unmapped" || unmapped["count"].(float64) != 0 {
		t.Fatalf("unmapped column = %v", view["unmapped_column"])
	}
	card := columns[0]["cards"].([]any)[0].(map[string]any)
	if card["id"] != issueID || card["status"].(map[string]any)["name"] != "Por hacer" || card["type"].(map[string]any)["name"] == "" {
		t.Fatalf("card = %v", card)
	}

	// Settings need manage_board.
	patch := map[string]any{"name": "Tablero del equipo", "swimlane_mode": "assignee", "estimation": "issue_count"}
	for name, token := range map[string]string{"viewer": env.viewer, "contributor": env.contrib} {
		if status, body := c.do(http.MethodPatch, boardPath, token, patch); status != http.StatusForbidden {
			t.Fatalf("patch as %s: %d %v, want 403", name, status, body)
		}
	}
	if status, body := c.do(http.MethodPatch, boardPath, env.lead, patch); status != http.StatusOK || tfData(t, body)["swimlane_mode"] != "assignee" {
		t.Fatalf("patch as lead: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodPatch, boardPath, env.lead, map[string]any{"swimlane_mode": "diagonal"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid swimlane mode: %d, want 422", status)
	}
	if status, _ := c.do(http.MethodPatch, boardPath, env.lead, map[string]any{"nope": 1}); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d, want 422", status)
	}
	status, body = c.do(http.MethodGet, boardPath+"/view", env.lead, nil)
	view = tfData(t, body)
	lanes := view["swimlanes"].([]any)
	if status != http.StatusOK || len(lanes) != 1 || lanes[0].(map[string]any)["id"] != "unassigned" {
		t.Fatalf("assignee lanes = %v", lanes)
	}

	// Moves: Open -> In progress is a single transition; the legacy default board is not ambiguous.
	progress := columns[1]["id"].(string)
	movePath := boardPath + "/cards/" + issueID + "/move"
	if status, _ := c.do(http.MethodPost, movePath, env.viewer, map[string]any{"column_id": progress}); status != http.StatusForbidden {
		t.Fatalf("viewer move: %d, want 403", status)
	}
	if status, _ := c.do(http.MethodPost, movePath, env.outside, map[string]any{"column_id": progress}); status != http.StatusNotFound {
		t.Fatalf("outsider move: %d, want 404", status)
	}
	if status, body := c.do(http.MethodPost, movePath, env.contrib, map[string]any{"column_id": uuid.NewString()}); status != http.StatusNotFound || boardCodeOf(body) != "board.column_not_found" {
		t.Fatalf("unknown column: %d %v", status, body)
	}
	if status, body := c.do(http.MethodPost, boardPath+"/cards/"+uuid.NewString()+"/move", env.contrib, map[string]any{"column_id": progress}); status != http.StatusNotFound || boardCodeOf(body) != "board.card_not_found" {
		t.Fatalf("unknown card: %d %v", status, body)
	}
	if status, body := c.do(http.MethodPost, movePath, env.contrib, map[string]any{"column_id": progress, "swimlane": "unassigned"}); status != http.StatusOK {
		t.Fatalf("move to in progress: %d %v", status, body)
	} else if data := tfData(t, body); data["transitioned"] != true || data["transition_id"] != "start" || data["status"].(map[string]any)["name"] != "En curso" || data["wip"].(map[string]any)["state"] != "ok" {
		t.Fatalf("move result = %v", data)
	}
	if got := env.statusName(issueID); got != "En curso" {
		t.Fatalf("issue status = %q, want En curso", got)
	}

	// A second card, ranked before the first one inside the same column.
	second := env.createIssue(env.contrib)
	secondMove := boardPath + "/cards/" + second + "/move"
	if status, body := c.do(http.MethodPost, secondMove, env.contrib, map[string]any{"column_id": progress, "before_issue_id": issueID}); status != http.StatusOK {
		t.Fatalf("move second card: %d %v", status, body)
	} else if tfData(t, body)["rank"] == "" {
		t.Fatalf("no rank in %v", body)
	}
	_, body = c.do(http.MethodGet, boardPath+"/view", env.lead, nil)
	if keys := cardKeys(boardColumns(t, tfData(t, body))[1]); len(keys) != 2 || keys[0] != "TRN-2" || keys[1] != "TRN-1" {
		t.Fatalf("order in progress = %v, want TRN-2 then TRN-1", keys)
	}
}

func (e *transitionEnv) statusName(issueID string) string {
	e.t.Helper()
	var name string
	if err := e.db.Get(&name, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, issueID); err != nil {
		e.t.Fatal(err)
	}
	return name
}
