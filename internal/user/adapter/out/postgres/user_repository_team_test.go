package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/user/domain"
	"github.com/jmoiron/sqlx"
)

// TestUserRepositoryTeamMembershipPostgres comprueba que Save crea la membresía del equipo y que
// FindByID lee el equipo desde team_members: aunque el espejo users.team_id quede vacio, el
// usuario conserva su equipo.
func TestUserRepositoryTeamMembershipPostgres(t *testing.T) {
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

	schema := "user_team_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(120) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role VARCHAR(20) NOT NULL,
			team_id UUID,
			deactivated_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE team_members (team_id UUID NOT NULL, user_id UUID NOT NULL, role VARCHAR(20) NOT NULL DEFAULT 'member', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (team_id, user_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewUserRepository(db)
	ctx := context.Background()
	team := uuid.New()
	now := time.Now().UTC()
	user := &domain.User{Name: "Ana Torres", Email: "ana@corelog.test", PasswordHash: "x", Role: domain.Role("agent"), TeamID: &team, CreatedAt: now, UpdatedAt: now}
	if err := repo.Save(ctx, user); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if user.ID == uuid.Nil {
		t.Fatal("Save did not return the generated id")
	}

	var memberships int
	if err := db.Get(&memberships, `SELECT COUNT(1) FROM team_members WHERE team_id = $1 AND user_id = $2`, team, user.ID); err != nil {
		t.Fatal(err)
	}
	if memberships != 1 {
		t.Fatalf("Save must create the team membership, found %d rows", memberships)
	}

	// Simula el espejo vacio: la lectura debe seguir devolviendo el equipo desde la membresia.
	if _, err := db.Exec(`UPDATE users SET team_id = NULL WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	found, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if found.TeamID == nil || *found.TeamID != team {
		t.Fatalf("FindByID team = %v, want %s from team_members", found.TeamID, team)
	}
}
