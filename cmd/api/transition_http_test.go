package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	workflowpostgres "github.com/jhonsferg/corelog/internal/workflow/adapter/out/postgres"
	workflowdomain "github.com/jhonsferg/corelog/internal/workflow/domain"
)

const transitionTestSecret = "transition-e2e-secret"

// transitionEnv is a project with a lead, a contributor, a viewer and an outsider on the real router.
type transitionEnv struct {
	t       *testing.T
	db      *sqlx.DB
	client  tfClient
	project uuid.UUID
	lead    string
	contrib string
	viewer  string
	outside string
	board   uuid.UUID
}

func newTransitionEnv(t *testing.T, template string) *transitionEnv {
	t.Helper()
	db := testdb.New(t)
	cfg := &config.Config{Auth: config.AuthConfig{JWTSecret: transitionTestSecret, JWTExpiration: time.Hour}}
	router, err := buildRouter(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	env := &transitionEnv{t: t, db: db, client: tfClient{t: t, router: router}, project: uuid.New(), board: uuid.New()}
	const orgID = "00000000-0000-4000-8000-000000000002"
	users := map[string]*string{"lead": &env.lead, "contributor": &env.contrib, "viewer": &env.viewer, "outsider": &env.outside}
	ids := map[string]uuid.UUID{}
	for name, token := range users {
		id := uuid.New()
		ids[name] = id
		env.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
		signed, err := authtoken.Generate(transitionTestSecret, time.Hour, id, "user")
		if err != nil {
			t.Fatal(err)
		}
		*token = signed
	}
	env.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, 'TRN', 'Transitions', 'kanban', $3, $4, 'private')`, env.project, orgID, template, ids["lead"])
	for name, role := range map[string]string{"lead": "lead", "contributor": "contributor", "viewer": "viewer"} {
		env.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, orgID, ids[name])
		env.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`, env.project, ids[name], role)
	}
	env.exec(`INSERT INTO boards (id, project_id, name, type, is_default) VALUES ($1, $2, 'Main', 'kanban', true)`, env.board, env.project)
	env.exec(`INSERT INTO board_columns (board_id, name, status, position) VALUES
		($1, 'Open', 'open', 0), ($1, 'In progress', 'in_progress', 1), ($1, 'Resolved', 'resolved', 2), ($1, 'Closed', 'closed', 3)`, env.board)
	env.exec(`SELECT corelog_ensure_workflow($1)`, env.project)
	return env
}

func (e *transitionEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		e.t.Fatalf("%v\n%s", err, query)
	}
}

func (e *transitionEnv) createIssue(token string) string {
	e.t.Helper()
	status, body := e.client.do(http.MethodPost, "/issues", token, map[string]any{
		"summary": "Incidencia de prueba", "description": "Texto", "project_id": e.project.String(),
	})
	if status != http.StatusCreated {
		e.t.Fatalf("create issue: %d %v", status, body)
	}
	return tfData(e.t, body)["id"].(string)
}

func (e *transitionEnv) statusID(issueID string) string {
	e.t.Helper()
	var id string
	if err := e.db.Get(&id, `SELECT status_id::text FROM issues WHERE id = $1`, issueID); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func transitionIDs(t *testing.T, body map[string]any) []string {
	t.Helper()
	list, _ := body["data"].([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.(map[string]any)["id"].(string))
	}
	return out
}

func TestTransitionEndpointsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	issueID := env.createIssue(env.contrib)
	path := "/issues/" + issueID + "/transitions"

	created := env.client
	status, body := created.do(http.MethodGet, "/issues/"+issueID, env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("get issue: %d", status)
	}
	data := tfData(t, body)
	ws, _ := data["workflow_status"].(map[string]any)
	if data["status"] != "open" || ws == nil || ws["name"] != "Por hacer" || ws["category"] != "todo" {
		t.Fatalf("new issue status = %v / %v, want the initial status of the template", data["status"], ws)
	}
	todoID := ws["id"].(string)

	// Authentication and visibility.
	if status, _ := env.client.do(http.MethodGet, path, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("GET without token: %d, want 401", status)
	}
	if status, _ := env.client.do(http.MethodPost, path, "", map[string]any{"transition_id": "start"}); status != http.StatusUnauthorized {
		t.Fatalf("POST without token: %d, want 401", status)
	}
	if status, _ := env.client.do(http.MethodGet, path, env.outside, nil); status != http.StatusNotFound {
		t.Fatalf("outsider: %d, want 404", status)
	}
	if status, _ := env.client.do(http.MethodGet, path, env.viewer, nil); status != http.StatusForbidden {
		t.Fatalf("viewer GET: %d, want 403", status)
	}
	if status, _ := env.client.do(http.MethodPost, path, env.viewer, map[string]any{"transition_id": "start"}); status != http.StatusForbidden {
		t.Fatalf("viewer POST: %d, want 403", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/issues/"+uuid.New().String()+"/transitions", env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("unknown issue: %d, want 404", status)
	}

	// Only the transitions from the current status, with the shape the client needs.
	status, body = env.client.do(http.MethodGet, path, env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d %v", status, body)
	}
	if ids := transitionIDs(t, body); len(ids) != 2 || ids[0] != "start" || ids[1] != "complete" {
		t.Fatalf("available transitions = %v, want [start complete]", ids)
	}
	first := body["data"].([]any)[0].(map[string]any)
	to := first["to"].(map[string]any)
	if first["to_status"] != "in_progress" || to["name"] != "En curso" || to["category"] != "in_progress" || first["needs_screen"] != false {
		t.Fatalf("transition shape = %v", first)
	}
	// The same list is part of the issue view.
	status, body = env.client.do(http.MethodGet, "/issues/"+issueID, env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("view: %d", status)
	}
	if view, _ := tfData(t, body)["available_transitions"].([]any); len(view) != 2 {
		t.Fatalf("view transitions = %v", view)
	}

	// Errors: unknown member, missing id, unavailable transition, stale expected status.
	if status, _ := env.client.do(http.MethodPost, path, env.contrib, map[string]any{"transition_id": "start", "colour": "x"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown member: %d, want 422", status)
	}
	if status, _ := env.client.do(http.MethodPost, path, env.contrib, map[string]any{}); status != http.StatusUnprocessableEntity {
		t.Fatalf("missing transition: %d, want 422", status)
	}
	status, body = env.client.do(http.MethodPost, path, env.contrib, map[string]any{"transition_id": "approve"})
	if status != http.StatusForbidden || tfErrorCode(t, body) != "workflow.transition_not_allowed" {
		t.Fatalf("transition from the wrong status: %d %v", status, body)
	}
	status, body = env.client.do(http.MethodPost, path, env.contrib, map[string]any{"transition_id": "start", "expected_status_id": uuid.NewString()})
	if status != http.StatusConflict || tfErrorCode(t, body) != "issue.status_conflict" {
		t.Fatalf("stale expected status: %d %v", status, body)
	}

	// The transition itself.
	status, body = env.client.do(http.MethodPost, path, env.contrib, map[string]any{"transition_id": "start", "expected_status_id": todoID})
	if status != http.StatusOK {
		t.Fatalf("start: %d %v", status, body)
	}
	data = tfData(t, body)
	ws, _ = data["workflow_status"].(map[string]any)
	applied, _ := data["transition"].(map[string]any)
	if data["status"] != "in_progress" || ws["name"] != "En curso" || applied["id"] != "start" || applied["from_status_id"] != todoID {
		t.Fatalf("start response = %v", data)
	}
	var history, activities, events int
	if err := env.db.Get(&history, `SELECT count(*) FROM issue_status_history WHERE issue_id = $1 AND transition_id = 'start' AND from_status_id::text = $2`, issueID, todoID); err != nil {
		t.Fatal(err)
	}
	if err := env.db.Get(&activities, `SELECT count(*) FROM activities WHERE issue_id = $1 AND field_name = 'status' AND new_value = 'En curso'`, issueID); err != nil {
		t.Fatal(err)
	}
	if err := env.db.Get(&events, `SELECT count(*) FROM domain_events WHERE entity_id = $1 AND type = 'issue.transitioned'`, issueID); err != nil {
		t.Fatal(err)
	}
	if history != 1 || activities != 1 || events != 1 {
		t.Fatalf("history=%d activities=%d events=%d, want one of each", history, activities, events)
	}

	// Completing sets the default Done resolution and the legacy mirror that sprints and dashboards read.
	status, body = env.client.do(http.MethodPost, path, env.lead, map[string]any{"transition_id": "complete"})
	if status != http.StatusOK {
		t.Fatalf("complete: %d %v", status, body)
	}
	data = tfData(t, body)
	if data["status"] != "closed" || data["resolution"] == nil || data["resolved_at"] == nil {
		t.Fatalf("done issue = status %v resolution %v resolved_at %v", data["status"], data["resolution"], data["resolved_at"])
	}
	status, body = env.client.do(http.MethodGet, path, env.lead, nil)
	if ids := transitionIDs(t, body); status != http.StatusOK || len(ids) != 1 || ids[0] != "reopen" {
		t.Fatalf("transitions of a done issue = %v", body)
	}
	status, body = env.client.do(http.MethodPost, path, env.lead, map[string]any{"transition_id": "reopen"})
	if status != http.StatusOK || tfData(t, body)["resolution"] != nil || tfData(t, body)["status"] != "open" {
		t.Fatalf("reopen: %d %v", status, body)
	}
}

func TestTransitionValidatorsAndRequiredFieldsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "kanban")
	issueID := env.createIssue(env.contrib)
	path := "/issues/" + issueID + "/transitions"

	// Require a comment and story points to complete.
	repo := workflowpostgres.NewRepository(env.db)
	record, err := repo.FindWorkflow(context.Background(), env.project)
	if err != nil {
		t.Fatal(err)
	}
	w := record.Workflow
	for i := range w.Transitions {
		if w.Transitions[i].ID == "complete" {
			w.Transitions[i].Validators = []workflowdomain.Validator{workflowdomain.CommentRequired{}, workflowdomain.RequiredField{Field: "story_points"}}
		}
	}
	if _, err := repo.SaveWorkflow(context.Background(), env.project, w, record.Version, time.Now()); err != nil {
		t.Fatal(err)
	}

	status, body := env.client.do(http.MethodGet, path, env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d", status)
	}
	var complete map[string]any
	for _, item := range body["data"].([]any) {
		if item.(map[string]any)["id"] == "complete" {
			complete = item.(map[string]any)
		}
	}
	if complete == nil || complete["requires_comment"] != true || complete["needs_screen"] != true || len(complete["required_fields"].([]any)) != 1 {
		t.Fatalf("complete option = %v, want a screen with a comment and one field", complete)
	}

	status, body = env.client.do(http.MethodPost, path, env.contrib, map[string]any{"transition_id": "complete"})
	if status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "workflow.validation_failed" {
		t.Fatalf("incomplete input: %d %v", status, body)
	}
	details, _ := body["error"].(map[string]any)["details"].([]any)
	fields := map[string]bool{}
	for _, d := range details {
		fields[d.(map[string]any)["field"].(string)] = true
	}
	if !fields["comment"] || !fields["fields.story_points"] {
		t.Fatalf("validation details = %v, want comment and fields.story_points", details)
	}
	var stillTodo int
	if err := env.db.Get(&stillTodo, `SELECT count(*) FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1 AND s.category = 'todo'`, issueID); err != nil || stillTodo != 1 {
		t.Fatalf("a failed validation moved the issue: %d %v", stillTodo, err)
	}

	status, body = env.client.do(http.MethodPost, path, env.contrib, map[string]any{
		"transition_id": "complete", "comment": "Hecho y verificado", "fields": map[string]string{"story_points": "8"},
	})
	if status != http.StatusOK {
		t.Fatalf("complete with input: %d %v", status, body)
	}
	if data := tfData(t, body); data["story_points"] != float64(8) || data["status"] != "closed" {
		t.Fatalf("completed issue = %v", data)
	}
	var comments int
	if err := env.db.Get(&comments, `SELECT count(*) FROM comments WHERE issue_id = $1 AND body = 'Hecho y verificado'`, issueID); err != nil || comments != 1 {
		t.Fatalf("transition comment saved %d times (%v)", comments, err)
	}
}

func TestConcurrentTransitionsSecondOneConflicts(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	issueID := env.createIssue(env.contrib)
	path := "/issues/" + issueID + "/transitions"
	todoID := env.statusID(issueID)

	const attempts = 2
	results := make([]int, attempts)
	codes := make([]string, attempts)
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(attempts)
	done.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			transition := "start"
			if i == 1 {
				transition = "complete"
			}
			status, body := env.client.do(http.MethodPost, path, env.lead, map[string]any{"transition_id": transition, "expected_status_id": todoID})
			results[i] = status
			if status != http.StatusOK {
				codes[i] = tfErrorCode(t, body)
			}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	wins, conflicts := 0, 0
	for i, status := range results {
		switch status {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
			conflicts++
			if codes[i] != "issue.status_conflict" {
				t.Errorf("conflict code = %q, want issue.status_conflict", codes[i])
			}
		default:
			t.Errorf("unexpected status %d (%s)", status, codes[i])
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d (%v), want exactly one of each", wins, conflicts, results)
	}
	var history int
	if err := env.db.Get(&history, `SELECT count(*) FROM issue_status_history WHERE issue_id = $1 AND transition_id IS NOT NULL`, issueID); err != nil || history != 1 {
		t.Fatalf("status history rows of transitions = %d (%v), want 1", history, err)
	}
}

func TestBoardMoveAndSprintVelocityFollowTransitions(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	issueID := env.createIssue(env.contrib)
	var progressColumn, openColumn, closedColumn uuid.UUID
	for status, target := range map[string]*uuid.UUID{"in_progress": &progressColumn, "open": &openColumn, "closed": &closedColumn} {
		if err := env.db.Get(target, `SELECT id FROM board_columns WHERE board_id = $1 AND status = $2`, env.board, status); err != nil {
			t.Fatal(err)
		}
	}
	move := func(token string, column uuid.UUID) (int, map[string]any) {
		return env.client.do(http.MethodPost, "/projects/"+env.project.String()+"/board/cards/"+issueID+"/move", token, map[string]any{"column_id": column.String()})
	}

	// The column resolves to a transition of the card.
	if status, body := move(env.contrib, progressColumn); status != http.StatusOK {
		t.Fatalf("move to in progress: %d %v", status, body)
	}
	if got := env.statusID(issueID); got == "" {
		t.Fatal("issue lost its status")
	}
	var name string
	if err := env.db.Get(&name, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, issueID); err != nil || name != "En curso" {
		t.Fatalf("status after the board move = %q (%v)", name, err)
	}
	status, body := env.client.do(http.MethodGet, "/projects/"+env.project.String()+"/board", env.contrib, nil)
	if status != http.StatusOK {
		t.Fatalf("board: %d", status)
	}
	cards := 0
	for _, column := range tfData(t, body)["columns"].([]any) {
		c := column.(map[string]any)
		if c["status"] == "in_progress" {
			cards = len(c["cards"].([]any))
		}
	}
	if cards != 1 {
		t.Fatalf("cards in the in-progress column = %d, want 1", cards)
	}

	// A column no transition reaches is a conflict with its own code; a viewer cannot move cards.
	env.exec(`UPDATE board_columns SET name = 'Review' WHERE board_id = $1 AND status = 'resolved'`, env.board)
	var unreachable uuid.UUID
	if err := env.db.Get(&unreachable, `SELECT id FROM board_columns WHERE board_id = $1 AND name = 'Review'`, env.board); err != nil {
		t.Fatal(err)
	}
	status, body = move(env.contrib, unreachable)
	if status != http.StatusConflict || tfErrorCode(t, body) != "issue.transition_unavailable" {
		t.Fatalf("move to a column without statuses: %d %v", status, body)
	}
	if status, _ := move(env.viewer, openColumn); status != http.StatusForbidden {
		t.Fatalf("viewer move: %d, want 403", status)
	}

	// Done in a closed sprint counts for the velocity through the legacy mirror.
	var sprintID uuid.UUID
	if err := env.db.Get(&sprintID, `INSERT INTO sprints (project_id, name, goal, status, start_date, end_date, closed_at) VALUES ($1, 'S1', '', 'closed', CURRENT_DATE - 14, CURRENT_DATE, now()) RETURNING id`, env.project); err != nil {
		t.Fatal(err)
	}
	env.exec(`UPDATE issues SET sprint_id = $2, story_points = 5 WHERE id = $1`, issueID, sprintID)
	if status, body := move(env.contrib, closedColumn); status != http.StatusOK {
		t.Fatalf("move to the done column: %d %v", status, body)
	}
	status, body = env.client.do(http.MethodGet, "/sprints/velocity?project_id="+env.project.String()+"&limit=3", env.lead, nil)
	points, _ := body["data"].([]any)
	if status != http.StatusOK || len(points) != 1 || points[0].(map[string]any)["points"] != float64(5) {
		t.Fatalf("velocity after the issue was done = %d %v", status, body)
	}
}
