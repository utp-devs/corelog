package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// newSearchTestDB creates an isolated schema with projects, project members and issues.
func newSearchTestDB(t *testing.T) *sqlx.DB {
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

	schema := "search_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE projects (id UUID PRIMARY KEY, key TEXT NOT NULL, name TEXT NOT NULL, archived_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE project_members (project_id UUID NOT NULL, user_id UUID NOT NULL, PRIMARY KEY (project_id, user_id))`,
		`CREATE TABLE issues (id UUID PRIMARY KEY, project_id UUID NOT NULL REFERENCES projects (id), title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

// TestSearchIndexVisibilityAndLiteralPatternsPostgres comprueba que un miembro solo ve sus proyectos y
// sus incidencias, que el admin ve todo, y que "%" y "_" se buscan literalmente.
func TestSearchIndexVisibilityAndLiteralPatternsPostgres(t *testing.T) {
	db := newSearchTestDB(t)

	member, outsider := uuid.New(), uuid.New()
	mine, theirs := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, key, name) VALUES ($1, 'CORE', 'Core platform'), ($2, 'OPS', 'Operaciones')`, mine, theirs); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project_members VALUES ($1, $2)`, mine, member); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, title) VALUES ($1, $2, 'Exportar 100% de datos'), ($3, $4, 'Exportar 100% tambien')`,
		uuid.New(), mine, uuid.New(), theirs); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, title) VALUES ($1, $2, 'Exportar 1000 datos')`, uuid.New(), mine); err != nil {
		t.Fatal(err)
	}

	index := NewSearchIndex(db)
	ctx := context.Background()

	memberHits, err := index.Search(ctx, member, false, "exportar", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberHits) != 2 {
		t.Fatalf("member sees %d hits for 'exportar', want 2 (its own project only)", len(memberHits))
	}

	outsiderHits, err := index.Search(ctx, outsider, false, "core", 10)
	if err != nil || len(outsiderHits) != 0 {
		t.Fatalf("non member found %d hits in a project it does not belong to", len(outsiderHits))
	}

	adminHits, err := index.Search(ctx, outsider, true, "exportar", 10)
	if err != nil || len(adminHits) != 3 {
		t.Fatalf("admin sees %d hits, want 3 (all projects)", len(adminHits))
	}

	literal, err := index.Search(ctx, outsider, true, "100%", 10)
	if err != nil || len(literal) != 2 {
		t.Fatalf("'100%%' matched %d hits, want 2 (the literal percent sign only)", len(literal))
	}
	for _, hit := range literal {
		if !strings.Contains(hit.Title, "100%") {
			t.Fatalf("wildcard leaked into results: %q", hit.Title)
		}
	}
}

// TestSearchSkipsDeletedProjectsKeepsArchivedPostgres checks that issues and project names of a
// soft-deleted project never match, while an archived project stays searchable.
func TestSearchSkipsDeletedProjectsKeepsArchivedPostgres(t *testing.T) {
	db := newSearchTestDB(t)
	ctx := context.Background()
	index := NewSearchIndex(db)
	admin := uuid.New()

	live, deleted, archived := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, key, name) VALUES ($1, 'CORE', 'Core')`, live); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, key, name, deleted_at) VALUES ($1, 'GONE', 'Exportar archivo', now())`, deleted); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, key, name, archived_at) VALUES ($1, 'OLD', 'Exportar historico', now())`, archived); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, title) VALUES ($1, $2, 'Exportar 7 borrado'), ($3, $4, 'Exportar 8 archivado'), ($5, $6, 'Exportar 9 vivo')`,
		uuid.New(), deleted, uuid.New(), archived, uuid.New(), live); err != nil {
		t.Fatal(err)
	}

	hits, err := index.Search(ctx, admin, true, "exportar", 20)
	if err != nil {
		t.Fatal(err)
	}
	// The live issue, the archived project name and the archived project issue match; the deleted ones do not.
	if len(hits) != 3 {
		t.Fatalf("admin sees %d hits for 'exportar', want 3", len(hits))
	}
	for _, hit := range hits {
		if hit.Title == "Exportar archivo" || hit.Title == "Exportar 7 borrado" {
			t.Fatalf("hit of a deleted project leaked: %q", hit.Title)
		}
	}
	if gone, err := index.Search(ctx, admin, true, "borrado", 20); err != nil || len(gone) != 0 {
		t.Fatalf("deleted project issue found: %d hits, err %v", len(gone), err)
	}
}
