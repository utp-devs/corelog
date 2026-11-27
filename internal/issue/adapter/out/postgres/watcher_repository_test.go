package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jmoiron/sqlx"
)

func TestWatcherRepositoryAndCommentAutoFollowPostgres(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	schema := "watcher_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users(id UUID PRIMARY KEY)`,
		`CREATE TABLE issues(id UUID PRIMARY KEY)`,
		`CREATE TABLE comments(id UUID PRIMARY KEY, issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE, author_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT, body TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE comment_mentions(comment_id UUID NOT NULL REFERENCES comments(id) ON DELETE CASCADE, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, PRIMARY KEY(comment_id,user_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	upPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000012_issue_watchers.up.sql")
	up, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(up)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE comments ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}

	issueID, authorID, otherID := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO issues(id) VALUES($1)`, issueID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{authorID, otherID} {
		if _, err := db.Exec(`INSERT INTO users(id) VALUES($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	watchers := NewWatcherRepository(db)
	state, err := watchers.State(context.Background(), issueID, authorID)
	if err != nil || state.Count != 0 || state.IsWatching {
		t.Fatalf("initial state=%+v err=%v", state, err)
	}
	comment, err := domain.NewComment(issueID, authorID, "Following automatically")
	if err != nil {
		t.Fatal(err)
	}
	if err := NewCollaborationRepository(db).SaveComment(context.Background(), comment); err != nil {
		t.Fatal(err)
	}
	state, err = watchers.State(context.Background(), issueID, authorID)
	if err != nil || !state.IsWatching || state.Count != 1 {
		t.Fatalf("comment auto-follow state=%+v err=%v", state, err)
	}
	if err := watchers.Add(context.Background(), issueID, otherID); err != nil {
		t.Fatal(err)
	}
	if err := watchers.Add(context.Background(), issueID, otherID); err != nil {
		t.Fatal(err)
	}
	state, err = watchers.State(context.Background(), issueID, authorID)
	if err != nil || state.Count != 2 {
		t.Fatalf("manual watcher count state=%+v err=%v", state, err)
	}
	if err := watchers.Remove(context.Background(), issueID, otherID); err != nil {
		t.Fatal(err)
	}
	state, err = watchers.State(context.Background(), issueID, otherID)
	if err != nil || state.IsWatching || state.Count != 1 {
		t.Fatalf("unwatch state=%+v err=%v", state, err)
	}
	downPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000012_issue_watchers.down.sql")
	down, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}
}
