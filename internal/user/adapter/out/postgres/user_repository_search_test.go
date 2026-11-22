package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jmoiron/sqlx"
)

// TestUserRepositorySearchByTeamUsesMembershipsPostgres comprueba que el filtro por equipo usa
// team_members: una persona que pertenece al equipo solo por membresia aparece aunque su
// users.team_id este vacio, y una persona de otro equipo no aparece.
func TestUserRepositorySearchByTeamUsesMembershipsPostgres(t *testing.T) {
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

	schema := "user_search_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE users (
			id UUID PRIMARY KEY,
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
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	teamA, teamB := uuid.New(), uuid.New()
	ana, carla, luis := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	// Ana: equipo A en users.team_id y en team_members. Carla: solo en team_members (team_id vacio).
	// Luis: solo equipo B.
	insertUser := func(id uuid.UUID, name string, teamID *uuid.UUID) {
		if _, err := db.Exec(
			`INSERT INTO users (id, name, email, password_hash, role, team_id, created_at, updated_at)
			 VALUES ($1, $2, $3, 'x', 'agent', $4, $5, $5)`,
			id, name, name+"@corelog.test", teamID, now,
		); err != nil {
			t.Fatal(err)
		}
	}
	insertUser(ana, "Ana Torres", &teamA)
	insertUser(carla, "Carla Ruiz", nil)
	insertUser(luis, "Luis Paredes", &teamB)
	if _, err := db.Exec(
		`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2), ($1, $3), ($4, $5)`,
		teamA, ana, carla, teamB, luis,
	); err != nil {
		t.Fatal(err)
	}

	repo := NewUserRepository(db)
	teamFilter := teamA
	found, err := repo.Search(context.Background(), port.SearchFilter{TeamID: &teamFilter})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	got := map[uuid.UUID]bool{}
	for _, user := range found {
		got[user.ID] = true
	}
	if !got[ana] || !got[carla] {
		t.Errorf("team A search missed members: ana=%v carla=%v", got[ana], got[carla])
	}
	if got[luis] {
		t.Errorf("team A search returned a member of team B")
	}
}
