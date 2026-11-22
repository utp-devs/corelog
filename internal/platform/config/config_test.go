package config

import (
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	t.Run("DatabaseConfig DSN and SQLitePath", func(t *testing.T) {
		dbCfg := DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "corelog",
			Password: "password",
			Name:     "testdb",
			SSLMode:  "disable",
		}

		expectedDSN := "host=localhost port=5432 user=corelog password=password dbname=testdb sslmode=disable"
		if dbCfg.DSN() != expectedDSN {
			t.Errorf("got DSN %q, want %q", dbCfg.DSN(), expectedDSN)
		}

		expectedPath := filepath.Join("data", "testdb.db")
		if dbCfg.SQLitePath() != expectedPath {
			t.Errorf("got SQLitePath %q, want %q", dbCfg.SQLitePath(), expectedPath)
		}
	})

	t.Run("Load returns valid configuration", func(t *testing.T) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error loading config: %v", err)
		}
		if cfg.Server.Port == "" {
			t.Errorf("expected default server port")
		}
		if cfg.Database.Driver == "" {
			t.Errorf("expected default database driver")
		}
	})
}
