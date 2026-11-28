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

func TestRetentionDefaultsAndValidation(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Retention
	if r.Enabled || !r.DryRun || r.DeleteOrphans || r.SoftDeletedDays != 30 || r.ReadNotificationDays != 180 || r.FingerprintDays != 90 {
		t.Fatalf("unexpected defaults: %+v", r)
	}
	t.Setenv("PURGE_RETENTION_DAYS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("a zero retention window must be rejected")
	}
	t.Setenv("PURGE_RETENTION_DAYS", "45")
	t.Setenv("PURGE_NOTIFICATION_RETENTION_DAYS", "99999")
	if _, err := Load(); err == nil {
		t.Fatal("an out of range notification window must be rejected")
	}
}
