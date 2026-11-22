package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

func TestAttachmentRepositoryPostgresCRUD(t *testing.T) {
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

	schema := "attachment_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE users (id UUID PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE issues (id UUID PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	migrationPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000011_issue_attachments.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE attachments ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	issueID, uploaderID := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users(id) VALUES($1)`, uploaderID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues(id) VALUES($1)`, issueID); err != nil {
		t.Fatal(err)
	}
	want, err := domain.NewAttachment(issueID, uploaderID, "spec.pdf", "application/pdf", 1024)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewAttachmentRepository(db)
	if err := repo.SaveAttachment(ctx, want); err != nil {
		t.Fatalf("SaveAttachment: %v", err)
	}
	got, err := repo.FindAttachment(ctx, issueID, want.ID)
	if err != nil || got.StorageKey != want.StorageKey || got.Size != want.Size {
		t.Fatalf("FindAttachment = %#v, %v", got, err)
	}
	items, err := repo.ListAttachments(ctx, issueID)
	if err != nil || len(items) != 1 || items[0].ID != want.ID {
		t.Fatalf("ListAttachments = %#v, %v", items, err)
	}
	if err := repo.DeleteAttachment(ctx, issueID, want.ID); err != nil {
		t.Fatalf("DeleteAttachment: %v", err)
	}
	if _, err := repo.FindAttachment(ctx, issueID, want.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("FindAttachment after delete error = %v, want not found", err)
	}
	if err := repo.DeleteAttachment(ctx, issueID, uuid.New()); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("DeleteAttachment missing error = %v, want not found", err)
	}
	downPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000011_issue_attachments.down.sql")
	down, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}
}
