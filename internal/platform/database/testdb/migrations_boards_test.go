package testdb_test

import "testing"

const boardsMigBoardID = "00000000-0000-4000-8000-0000000000b1"

// TestMigrationsBoardsV2RoundTrip applies the boards v2 migration on a legacy board, checks the backfill and the
// new constraints, then runs down and up again.
func TestMigrationsBoardsV2RoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	for version := 29; version <= 38; version++ {
		env.runFile(t, version, ".up.sql")
	}
	if _, err := env.pool.Exec(`INSERT INTO boards (id, project_id, name, type, is_default) VALUES ($1, $2, 'Default', 'scrum', true)`, boardsMigBoardID, orgMigProjectID); err != nil {
		t.Fatalf("legacy board: %v", err)
	}
	if _, err := env.pool.Exec(`INSERT INTO board_columns (board_id, name, status, position, wip_limit) VALUES
		($1, 'Open', 'open', 0, 3), ($1, 'In progress', 'in_progress', 1, NULL), ($1, 'Closed', 'closed', 2, NULL)`, boardsMigBoardID); err != nil {
		t.Fatalf("legacy columns: %v", err)
	}
	if _, err := env.pool.Exec(`INSERT INTO board_column_statuses (column_id, status_id)
		SELECT c.id, (SELECT (w.definition -> 'statuses' -> 0 ->> 'id')::uuid FROM workflows w WHERE w.project_id = $2) FROM board_columns c WHERE c.board_id = $1 AND c.status = 'open'`,
		boardsMigBoardID, orgMigProjectID); err != nil {
		t.Fatalf("legacy binding: %v", err)
	}

	env.runFile(t, 39, ".up.sql")
	assertBoardsV2(t, env)

	env.runFile(t, 39, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('board_quick_filters', 'board_swimlanes')`, "0", "v2 tables removed")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'boards' AND column_name IN ('swimlane_mode', 'estimation', 'card_fields', 'show_unmapped_lane', 'done_retention_days')`, "0", "v2 board columns removed")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM board_columns WHERE board_id = $1`, "3", "legacy columns survive the down", boardsMigBoardID)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM boards`, "1", "boards that are not the default are removed by the down")
	assertOrgMigValue(t, env.pool, `SELECT is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'board_columns' AND column_name = 'status'`, "NO", "legacy status is mandatory again")

	env.runFile(t, 39, ".up.sql")
	assertBoardsV2(t, env)
}

func assertBoardsV2(t *testing.T, env *orgMigEnv) {
	t.Helper()
	assertOrgMigValue(t, env.pool, `SELECT b.name = left(p.name, 80) AND b.swimlane_mode = 'none' AND b.estimation = 'story_points' AND b.show_unmapped_lane AND b.card_fields @> '"key"'::jsonb FROM boards b JOIN projects p ON p.id = b.project_id WHERE b.id = $1`,
		"true", "legacy board takes the project name and the default settings", boardsMigBoardID)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM board_columns WHERE board_id = $1 AND wip_limit = 3 AND min_issues IS NULL AND color = ''`, "1", "legacy columns keep their limit", boardsMigBoardID)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM board_column_statuses bcs JOIN board_columns c ON c.id = bcs.column_id WHERE c.board_id = $1`, "1", "bindings survive", boardsMigBoardID)

	exec := func(query string, args ...any) error {
		_, err := env.pool.Exec(query, args...)
		return err
	}
	// Several boards per project with unique names (case-insensitive).
	if err := exec(`INSERT INTO boards (project_id, name, type) VALUES ($1, 'Soporte', 'kanban')`, orgMigProjectID); err != nil {
		t.Fatalf("second board: %v", err)
	}
	if err := exec(`INSERT INTO boards (project_id, name, type) VALUES ($1, 'soporte', 'kanban')`, orgMigProjectID); err == nil {
		t.Error("duplicate board name accepted")
	}
	if err := exec(`INSERT INTO boards (project_id, name, type, swimlane_mode) VALUES ($1, 'Otro', 'kanban', 'diagonal')`, orgMigProjectID); err == nil {
		t.Error("unknown swimlane mode accepted")
	}
	// Columns without a legacy status, limits and colors.
	if err := exec(`INSERT INTO board_columns (board_id, name, position, min_issues, wip_limit, color) VALUES ($1, 'Nueva', 3, 1, 5, 'blue')`, boardsMigBoardID); err != nil {
		t.Fatalf("v2 column: %v", err)
	}
	if err := exec(`INSERT INTO board_columns (board_id, name, position, min_issues, wip_limit) VALUES ($1, 'Mal', 4, 6, 5)`, boardsMigBoardID); err == nil {
		t.Error("min above max accepted")
	}
	if err := exec(`INSERT INTO board_columns (board_id, name, position, color) VALUES ($1, 'Rosa', 5, 'fuchsia')`, boardsMigBoardID); err == nil {
		t.Error("unknown color accepted")
	}
	// Positions are deferred-unique: a swap in one statement works.
	if err := exec(`UPDATE board_columns SET position = CASE position WHEN 0 THEN 1 WHEN 1 THEN 0 ELSE position END WHERE board_id = $1`, boardsMigBoardID); err != nil {
		t.Errorf("swap positions: %v", err)
	}
	// Quick filters and swimlanes cascade with the board.
	if err := exec(`INSERT INTO board_quick_filters (board_id, name, query_text, position) VALUES ($1, 'Mías', 'assignee = currentUser()', 0)`, boardsMigBoardID); err != nil {
		t.Fatalf("quick filter: %v", err)
	}
	if err := exec(`INSERT INTO board_swimlanes (board_id, name, query_text, position) VALUES ($1, 'Bugs', 'type = bug', 0)`, boardsMigBoardID); err != nil {
		t.Fatalf("swimlane: %v", err)
	}
	// Clean the rows the next cycle must not see.
	for _, stmt := range []string{
		`DELETE FROM board_quick_filters`, `DELETE FROM board_swimlanes`,
		`DELETE FROM board_columns WHERE name = 'Nueva'`, `DELETE FROM boards WHERE name = 'Soporte'`,
	} {
		if err := exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	// The swap above moved the first two columns; restore the order for the next assertions.
	if err := exec(`UPDATE board_columns SET position = CASE position WHEN 0 THEN 1 WHEN 1 THEN 0 ELSE position END WHERE board_id = $1`, boardsMigBoardID); err != nil {
		t.Fatalf("restore positions: %v", err)
	}
}
