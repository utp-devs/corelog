package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/config"
)

func TestNewSQLite(t *testing.T) {
	dbName := "test_db_" + uuid.New().String()
	dbPath := filepath.Join("data", dbName+".db")
	defer func() { _ = os.Remove(dbPath) }()

	cfg := config.DatabaseConfig{
		Name:            dbName,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := NewSQLite(cfg)
	if err != nil {
		t.Fatalf("failed to create sqlite database: %v", err)
	}
	defer func() { _ = db.Close() }()

	var applied int
	err = db.Get(&applied, "SELECT COUNT(1) FROM schema_migrations")
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if applied == 0 {
		t.Errorf("expected migrations to be applied, got 0")
	}
}
