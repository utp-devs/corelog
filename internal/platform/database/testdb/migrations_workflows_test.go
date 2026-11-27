package testdb_test

import (
	"encoding/json"
	"testing"

	workflowdomain "github.com/jhonsferg/corelog/internal/workflow/domain"
)

const (
	wfMigBugProjectID = "00000000-0000-4000-8000-0000000000c2"
	wfMigIssueDone    = "00000000-0000-4000-8000-0000000000e3"
	wfMigIssueBugRes  = "00000000-0000-4000-8000-0000000000e4"
	wfMigIssueBugOpen = "00000000-0000-4000-8000-0000000000e5"
	wfMigBoardID      = "00000000-0000-4000-8000-0000000000d1"
)

// TestMigrationsWorkflowsRoundTrip applies the workflow migration on legacy data: every issue gets a valid
// status, every project a valid workflow, the legacy mirror stays in sync, then down and up run again.
func TestMigrationsWorkflowsRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	for version := 29; version <= 36; version++ {
		env.runFile(t, version, ".up.sql")
	}
	legacy := []string{
		`UPDATE projects SET template = 'bug_tracking' WHERE id = '` + orgMigProjectID + `'`,
		`INSERT INTO projects (id, key, name, lead_id, organization_id, template) VALUES
			('` + wfMigBugProjectID + `', 'KAN', 'Kanban', '` + orgMigAdminID + `', '` + orgMigDefaultOrgID + `', 'kanban')`,
		`UPDATE issues SET status = 'in_progress' WHERE id = '` + issueCoreFirstIssue + `'`,
	}
	for _, stmt := range legacy {
		if _, err := env.pool.Exec(stmt); err != nil {
			t.Fatalf("prepare legacy data: %v\n%s", err, stmt)
		}
	}
	issueInsert := func(id, project, status string) {
		t.Helper()
		if _, err := env.pool.Exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, key_number, issue_type_id, priority_id)
			SELECT $1, $2, 'S', 'D', $3, 'medium', $4,
			       (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2),
			       (SELECT it.id FROM issue_types it JOIN projects p ON p.organization_id = it.organization_id WHERE p.id = $2 AND it.is_default),
			       (SELECT pr.id FROM priorities pr JOIN projects p ON p.organization_id = pr.organization_id WHERE p.id = $2 AND pr.is_default)`,
			id, project, status, orgMigRequesterID); err != nil {
			t.Fatalf("insert issue %s: %v", id, err)
		}
	}
	issueInsert(wfMigIssueBugRes, orgMigProjectID, "resolved")
	issueInsert(wfMigIssueBugOpen, orgMigProjectID, "closed")
	issueInsert(wfMigIssueDone, wfMigBugProjectID, "resolved")
	if _, err := env.pool.Exec(`INSERT INTO boards (id, project_id, name, type, is_default) VALUES ($1, $2, 'B', 'kanban', true)`, wfMigBoardID, wfMigBugProjectID); err != nil {
		t.Fatalf("legacy board: %v", err)
	}
	if _, err := env.pool.Exec(`INSERT INTO board_columns (board_id, name, status, position) VALUES
		($1, 'Open', 'open', 0), ($1, 'In progress', 'in_progress', 1), ($1, 'Resolved', 'resolved', 2), ($1, 'Closed', 'closed', 3)`, wfMigBoardID); err != nil {
		t.Fatalf("legacy columns: %v", err)
	}
	issues := countOrgMig(t, env.pool, "issues")

	env.runFile(t, 37, ".up.sql")
	assertWorkflowMigration(t, env, issues, true)

	// New writers keep both columns in sync.
	if _, err := env.pool.Exec(`UPDATE issues SET status_id = (SELECT (definition->>'initial')::uuid FROM workflows WHERE project_id = issues.project_id) WHERE id = $1`, wfMigIssueDone); err != nil {
		t.Fatalf("move to initial status: %v", err)
	}
	assertOrgMigValue(t, env.pool, `SELECT status FROM issues WHERE id = $1`, "open", "legacy mirror follows status_id", wfMigIssueDone)
	if _, err := env.pool.Exec(`UPDATE issues SET status = 'in_progress' WHERE id = $1`, wfMigIssueDone); err != nil {
		t.Fatalf("legacy writer: %v", err)
	}
	assertOrgMigValue(t, env.pool, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, "En curso", "legacy writer moves status_id", wfMigIssueDone)

	env.runFile(t, 37, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'status_id'`, "0", "status_id removed")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('statuses', 'issue_status_history', 'board_column_statuses')`, "0", "new tables removed")
	assertOrgMigValue(t, env.pool, `SELECT (count(*) = 6 * (SELECT count(*) FROM workflows))::text FROM workflow_transitions`, "true", "legacy transitions restored for every project")
	if got := countOrgMig(t, env.pool, "issues"); got != issues {
		t.Fatalf("issues after down = %d, want %d", got, issues)
	}

	env.runFile(t, 37, ".up.sql")
	assertWorkflowMigration(t, env, issues, false)
}

func assertWorkflowMigration(t *testing.T, env *orgMigEnv, issues int, exactNames bool) {
	t.Helper()
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM issues WHERE status_id IS NULL`, "0", "every issue has a status")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM issues i LEFT JOIN statuses s ON s.id = i.status_id WHERE s.id IS NULL`, "0", "every status exists")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM projects p LEFT JOIN workflows w ON w.project_id = p.id WHERE w.id IS NULL`, "0", "every project has a workflow")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.status <> CASE s.category WHEN 'todo' THEN 'open' WHEN 'in_progress' THEN 'in_progress' ELSE 'closed' END`, "0", "legacy mirror in sync")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM issues WHERE status = 'resolved'`, "0", "resolved rows mirror their done status")
	if got := countOrgMigWhere(t, env.pool, "issue_status_history", "true"); got < issues {
		t.Fatalf("status history rows = %d, want at least %d", got, issues)
	}
	if exactNames {
		assertOrgMigValue(t, env.pool, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, "En curso", "in_progress maps to En curso", issueCoreFirstIssue)
		assertOrgMigValue(t, env.pool, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, "Resuelto", "bug project resolved maps to Resuelto", wfMigIssueBugRes)
		assertOrgMigValue(t, env.pool, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, "Cerrado", "bug project closed maps to Cerrado", wfMigIssueBugOpen)
		assertOrgMigValue(t, env.pool, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = $1`, "Hecho", "kanban project resolved maps to Hecho", wfMigIssueDone)

	}
	rows, err := env.pool.Query(`SELECT p.template, w.definition FROM workflows w JOIN projects p ON p.id = w.project_id`)
	if err != nil {
		t.Fatalf("read workflows: %v", err)
	}
	defer func() { _ = rows.Close() }()
	seen := 0
	for rows.Next() {
		var template string
		var raw []byte
		if err := rows.Scan(&template, &raw); err != nil {
			t.Fatalf("scan workflow: %v", err)
		}
		def, err := workflowdomain.Decode(raw)
		if err != nil {
			t.Fatalf("decode workflow of %s: %v", template, err)
		}
		if issues := def.Validate(); len(issues) > 0 {
			t.Fatalf("workflow of %s is invalid: %+v", template, issues)
		}
		want, err := workflowdomain.TemplateWorkflow(template)
		if err != nil {
			t.Fatalf("template %s: %v", template, err)
		}
		if len(def.Statuses) != len(want.Statuses) || len(def.Transitions) != len(want.Transitions) {
			t.Fatalf("workflow of %s differs from its template", template)
		}
		for i, tr := range want.Transitions {
			got := def.Transitions[i]
			if got.ID != tr.ID || len(got.From) != len(tr.From) || len(got.PostFunctions) != len(tr.PostFunctions) {
				t.Fatalf("transition %d of %s differs from the template", i, template)
			}
		}
		if _, err := json.Marshal(def.Initial); err != nil {
			t.Fatal(err)
		}
		seen++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen < 2 {
		t.Fatalf("workflows checked = %d, want at least 2", seen)
	}

	// Board columns of the legacy board follow the legacy mirror of the statuses.
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM board_column_statuses bcs JOIN board_columns c ON c.id = bcs.column_id WHERE c.board_id = $1 AND c.status = 'closed'`, "1", "done status bound to the closed column", wfMigBoardID)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM board_column_statuses bcs JOIN board_columns c ON c.id = bcs.column_id WHERE c.board_id = $1 AND c.status = 'resolved'`, "0", "no status bound to the resolved column", wfMigBoardID)
}
