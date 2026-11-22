package config

import (
	"testing"
)

func TestConfig(t *testing.T) {
	t.Run("DatabaseConfig DSN", func(t *testing.T) {
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

	})

	t.Run("Load returns valid configuration", func(t *testing.T) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error loading config: %v", err)
		}
		if cfg.Server.Port == "" {
			t.Errorf("expected default server port")
		}
	})
}
