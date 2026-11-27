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
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jmoiron/sqlx"
)

func TestSavedFilterRepositoryPostgresCRUDAndOwnership(t *testing.T) {
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
	schema := "saved_filter_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE users(id UUID PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	upPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000013_saved_filters.up.sql")
	up, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(up)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE saved_filters ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}
	downPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000013_saved_filters.down.sql")
	down, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(string(down)) })
	ownerID, otherID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{ownerID, otherID} {
		if _, err := db.Exec(`INSERT INTO users(id) VALUES($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	repository := NewSavedFilterRepository(db)
	filter, err := domain.NewSavedFilter(ownerID, "My bugs", domain.SavedFilterCriteria{Status: domain.StatusOpen, IssueType: domain.IssueTypeBug})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveSavedFilter(context.Background(), filter); err != nil {
		t.Fatal(err)
	}
	duplicate, err := domain.NewSavedFilter(ownerID, "My bugs", domain.SavedFilterCriteria{Status: domain.StatusOpen})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveSavedFilter(context.Background(), duplicate); apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("duplicate filter error=%v, want conflict", err)
	}
	filters, err := repository.ListSavedFilters(context.Background(), ownerID)
	if err != nil || len(filters) != 1 || filters[0].Criteria.IssueType != domain.IssueTypeBug {
		t.Fatalf("owner filters=%+v err=%v", filters, err)
	}
	otherFilters, err := repository.ListSavedFilters(context.Background(), otherID)
	if err != nil || len(otherFilters) != 0 {
		t.Fatalf("other user's filters=%+v err=%v", otherFilters, err)
	}
	if err := repository.DeleteSavedFilter(context.Background(), otherID, filter.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("cross-owner delete error=%v, want not found", err)
	}
	if err := repository.DeleteSavedFilter(context.Background(), ownerID, filter.ID); err != nil {
		t.Fatal(err)
	}
	filters, err = repository.ListSavedFilters(context.Background(), ownerID)
	if err != nil || len(filters) != 0 {
		t.Fatalf("filters after delete=%+v err=%v", filters, err)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}
}
