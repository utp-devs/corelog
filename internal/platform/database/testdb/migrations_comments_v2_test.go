package testdb_test

import "testing"

// TestMigrationsCommentsV2RoundTrip applies the comments v2 migration over an existing public comment, checks the
// backfill, the new constraints and tables, then runs the down and up migrations again.
func TestMigrationsCommentsV2RoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 43)
	const (
		org     = "00000000-0000-4000-8000-000000000002"
		owner   = "00000000-0000-4000-8000-0000000000a1"
		bob     = "00000000-0000-4000-8000-0000000000a2"
		project = "00000000-0000-4000-8000-0000000000b1"
		issue   = "00000000-0000-4000-8000-0000000000c1"
		legacy  = "00000000-0000-4000-8000-0000000000d1"
	)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	scalar := func(q string, args ...any) string {
		t.Helper()
		var v string
		if err := env.pool.QueryRow(q, args...).Scan(&v); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
		return v
	}
	exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'o', 'o', 'o@example.test', 'x', 'user'), ($2, 'b', 'b', 'b@example.test', 'x', 'user')`, owner, bob)
	exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, org, owner)
	exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'CMT', 'Comentarios', 'scrum', 'scrum', $3, 'open')`, project, org, owner)
	exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id)
		SELECT $3, p.id, 'Con comentario', 'd', 'open', 'medium', $1, 'task', 1, 'm',
		       (SELECT id FROM issue_types WHERE organization_id = p.organization_id AND legacy_key = 'task'),
		       (SELECT id FROM priorities WHERE organization_id = p.organization_id AND legacy_key = 'medium')
		FROM projects p WHERE p.id = $2`, owner, project, issue)
	exec(`INSERT INTO comments (id, issue_id, author_id, body, created_at, updated_at) VALUES ($1, $2, $3, 'antiguo', '2026-01-01T10:00:00Z', '2026-01-02T10:00:00Z')`, legacy, issue, owner)

	env.runFile(t, 44, ".up.sql")

	if got := scalar(`SELECT visibility FROM comments WHERE id = $1`, legacy); got != "public" {
		t.Fatalf("legacy comment visibility = %q, want public", got)
	}
	if got := scalar(`SELECT (edited_at IS NOT NULL)::text FROM comments WHERE id = $1`, legacy); got != "true" {
		t.Fatalf("edited comment not backfilled: %q", got)
	}
	if _, err := env.pool.Exec(`INSERT INTO comments (issue_id, author_id, body, visibility) VALUES ($1, $2, 'x', 'internal')`, issue, owner); err == nil {
		t.Fatal("internal comment without a minimum role was accepted")
	}
	if _, err := env.pool.Exec(`INSERT INTO comments (issue_id, author_id, body, visibility, internal_role) VALUES ($1, $2, 'x', 'internal', 'viewer')`, issue, owner); err == nil {
		t.Fatal("viewer minimum role was accepted for an internal comment")
	}
	if _, err := env.pool.Exec(`UPDATE comments SET parent_id = id WHERE id = $1`, legacy); err == nil {
		t.Fatal("a comment must not be its own parent")
	}
	exec(`INSERT INTO comments (id, issue_id, author_id, body, visibility, internal_role, parent_id) VALUES (gen_random_uuid(), $1, $2, 'respuesta interna', 'internal', 'contributor', $3)`, issue, owner, legacy)
	exec(`INSERT INTO comment_reactions (comment_id, user_id, emoji) VALUES ($1, $2, 'thumbs_up')`, legacy, bob)
	if _, err := env.pool.Exec(`INSERT INTO comment_reactions (comment_id, user_id, emoji) VALUES ($1, $2, 'laugh')`, legacy, bob); err == nil {
		t.Fatal("unknown reaction emoji was accepted")
	}
	exec(`INSERT INTO comment_revisions (comment_id, body, edited_by) VALUES ($1, 'antes', $2)`, legacy, owner)
	exec(`INSERT INTO comment_notices (comment_id, user_id, kind) VALUES ($1, $2, 'mention')`, legacy, bob)
	if _, err := env.pool.Exec(`INSERT INTO comment_notices (comment_id, user_id, kind) VALUES ($1, $2, 'bogus')`, legacy, bob); err == nil {
		t.Fatal("unknown notice kind was accepted")
	}
	if got := scalar(`SELECT count(*)::text FROM comment_mentions`); got != "0" {
		t.Fatalf("mentions table changed unexpectedly: %s", got)
	}

	env.runFile(t, 44, ".down.sql")
	if got := scalar(`SELECT count(*)::text FROM comments WHERE body = 'respuesta interna'`); got != "0" {
		t.Fatal("internal comments must not survive the down migration")
	}
	if got := scalar(`SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'comment_reactions'`); got != "0" {
		t.Fatal("comment_reactions must be removed by the down migration")
	}
	if got := scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'comments' AND column_name = 'visibility'`); got != "0" {
		t.Fatal("visibility column must be removed by the down migration")
	}

	env.runFile(t, 44, ".up.sql")
	if got := scalar(`SELECT count(*)::text FROM comments WHERE id = $1`, legacy); got != "1" {
		t.Fatal("the public comment must survive a down and up")
	}
	if got := scalar(`SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'comment_notices'`); got != "1" {
		t.Fatal("comment_notices must exist after the second up")
	}
}
