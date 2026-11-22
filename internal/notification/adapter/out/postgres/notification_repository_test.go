package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// TestNotificationRepositoryPostgres recorre los metodos contra PostgreSQL en un esquema temporal.
// Comprueba que cada usuario solo ve y marca sus propias notificaciones.
func TestNotificationRepositoryPostgres(t *testing.T) {
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

	schema := "notification_repo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (id UUID PRIMARY KEY)`,
		`CREATE TABLE notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			kind VARCHAR(40) NOT NULL, message VARCHAR(300) NOT NULL, link VARCHAR(300) NOT NULL DEFAULT '',
			read_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	ana, luis := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{ana, luis} {
		if _, err := db.Exec(`INSERT INTO users (id) VALUES ($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	first, _ := domain.New(ana, "issue_assigned", "Primera", "/issues/1")
	if err := repo.Save(ctx, first); err != nil || first.ID == uuid.Nil {
		t.Fatalf("Save = %v, id %s", err, first.ID)
	}
	second, _ := domain.New(ana, "issue_assigned", "Segunda", "")
	if err := repo.Save(ctx, second); err != nil {
		t.Fatal(err)
	}
	foreign, _ := domain.New(luis, "issue_assigned", "De Luis", "")
	if err := repo.Save(ctx, foreign); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListByUser(ctx, ana, 50)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByUser(ana) = %d, %v; want 2", len(list), err)
	}
	if count, err := repo.UnreadCount(ctx, ana); err != nil || count != 2 {
		t.Fatalf("UnreadCount(ana) = %d, %v; want 2", count, err)
	}

	if err := repo.MarkRead(ctx, luis, first.ID); !errors.Is(err, domain.ErrNotificationNotFound) {
		t.Fatalf("Luis marking Ana's notification = %v, want not found", err)
	}
	if err := repo.MarkRead(ctx, ana, first.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if err := repo.MarkRead(ctx, ana, first.ID); err != nil {
		t.Fatalf("marking twice must succeed: %v", err)
	}
	if count, _ := repo.UnreadCount(ctx, ana); count != 1 {
		t.Fatalf("unread after one mark = %d, want 1", count)
	}

	if err := repo.MarkAllRead(ctx, ana); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if count, _ := repo.UnreadCount(ctx, ana); count != 0 {
		t.Fatalf("unread after mark all = %d, want 0", count)
	}
	if count, _ := repo.UnreadCount(ctx, luis); count != 1 {
		t.Fatalf("Luis's notification must not be marked by Ana: unread = %d", count)
	}
}
