package directory

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// openTestSchema abre la base de pruebas y crea un schema temporal que se elimina al terminar.
// Devuelve false (y omite la prueba) si no hay CORELOG_TEST_DATABASE_URL.
func openTestSchema(t *testing.T) (*sqlx.DB, bool) {
	t.Helper()
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

	schema := "directory_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	return db, true
}

// TestUserDirectoryNamesOfPostgres comprueba la busqueda de nombres por lote: los ids
// existentes aparecen en el mapa y los inexistentes se omiten sin error.
func TestUserDirectoryNamesOfPostgres(t *testing.T) {
	db, _ := openTestSchema(t)
	if _, err := db.Exec(`CREATE TABLE users (id UUID PRIMARY KEY, name VARCHAR(120) NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	ana, luis := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name) VALUES ($1, 'Ana Torres'), ($2, 'Luis Paredes')`, ana, luis); err != nil {
		t.Fatal(err)
	}

	dir := NewUserDirectory(db)
	names, err := dir.NamesOf(context.Background(), []uuid.UUID{ana, luis, uuid.New()})
	if err != nil {
		t.Fatalf("NamesOf returned error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2 (unknown ids are omitted)", len(names))
	}
	if names[ana] != "Ana Torres" || names[luis] != "Luis Paredes" {
		t.Errorf("unexpected names: %v", names)
	}

	empty, err := dir.NamesOf(context.Background(), nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("NamesOf(nil) = %v, %v; want empty map and no error", empty, err)
	}
}

// TestUserDirectorySharesTeamPostgres comprueba que dos usuarios comparten equipo solo si
// existe una membresia en comun en team_members.
func TestUserDirectorySharesTeamPostgres(t *testing.T) {
	db, _ := openTestSchema(t)
	if _, err := db.Exec(`CREATE TABLE team_members (team_id UUID NOT NULL, user_id UUID NOT NULL, PRIMARY KEY (team_id, user_id))`); err != nil {
		t.Fatal(err)
	}

	teamA, teamB := uuid.New(), uuid.New()
	ana, luis, nadie := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(
		`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2), ($3, $4), ($3, $5)`,
		teamA, ana, teamB, luis, ana,
	); err != nil {
		t.Fatal(err)
	}

	dir := NewUserDirectory(db)
	ctx := context.Background()
	shared, err := dir.SharesTeam(ctx, ana, luis)
	if err != nil || !shared {
		t.Errorf("SharesTeam(ana, luis) = %v, %v; want true (both belong to teamB)", shared, err)
	}
	none, err := dir.SharesTeam(ctx, ana, nadie)
	if err != nil || none {
		t.Errorf("SharesTeam(ana, nadie) = %v, %v; want false", none, err)
	}
}
