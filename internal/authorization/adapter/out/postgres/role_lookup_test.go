package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jmoiron/sqlx"
)

// TestRoleLookupFindRolePostgres comprueba el rol de un miembro, la ausencia de membresia
// (nil, sin error) y que la busqueda distingue proyecto y usuario.
func TestRoleLookupFindRolePostgres(t *testing.T) {
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

	schema := "role_lookup_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE project_members (
		project_id UUID NOT NULL,
		user_id UUID NOT NULL,
		role VARCHAR(20) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (project_id, user_id)
	)`); err != nil {
		t.Fatal(err)
	}

	project, lead, viewer, outsider := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(
		`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'lead'), ($1, $3, 'viewer')`,
		project, lead, viewer,
	); err != nil {
		t.Fatal(err)
	}

	lookup := NewRoleLookup(db)
	ctx := context.Background()

	role, err := lookup.FindRole(ctx, project, lead)
	if err != nil || role == nil || *role != domain.ProjectRoleLead {
		t.Errorf("FindRole(lead) = %v, %v; want lead", role, err)
	}
	role, err = lookup.FindRole(ctx, project, viewer)
	if err != nil || role == nil || *role != domain.ProjectRoleViewer {
		t.Errorf("FindRole(viewer) = %v, %v; want viewer", role, err)
	}
	role, err = lookup.FindRole(ctx, project, outsider)
	if err != nil || role != nil {
		t.Errorf("FindRole(outsider) = %v, %v; want nil and no error", role, err)
	}
	role, err = lookup.FindRole(ctx, uuid.New(), lead)
	if err != nil || role != nil {
		t.Errorf("FindRole(other project) = %v, %v; want nil and no error", role, err)
	}
}
