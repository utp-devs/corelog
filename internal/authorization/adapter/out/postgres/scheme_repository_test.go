package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// seedProject creates an owner, an organization and one project, and returns the project id.
func seedProject(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	owner, org, project := uuid.New(), uuid.New(), uuid.New()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, name, display_name, email, password_hash, role)
		  VALUES ($1, 'Owner', 'Owner', $2, 'not-a-real-hash', 'user')`, []any{owner, owner.String() + "@example.test"}},
		{`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Org', $2)`, []any{org, "org-" + org.String()}},
		{`INSERT INTO projects (id, key, name, lead_id, organization_id) VALUES ($1, 'SCHM', 'Scheme', $2, $3)`, []any{project, owner, org}},
	}
	for _, s := range statements {
		if _, err := db.Exec(s.query, s.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return project
}

func customScheme(t *testing.T) domain.Scheme {
	t.Helper()
	defaults := domain.DefaultScheme().Grants()
	for i := range defaults {
		if defaults[i].Permission == domain.PermissionCreateIssue {
			defaults[i].Holders = []domain.Holder{domain.HolderLead, domain.HolderViewer}
		}
	}
	scheme, err := domain.NewScheme(defaults)
	if err != nil {
		t.Fatalf("build scheme: %v", err)
	}
	return scheme
}

func countRows(t *testing.T, db *sqlx.DB, projectID uuid.UUID) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT count(*) FROM project_permissions WHERE project_id = $1`, projectID); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSchemeRepositoryLoadReplaceReset(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewSchemeRepository(db)
	project := seedProject(t, db)

	stored, err := repo.Load(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if stored != nil {
		t.Fatal("a project without rows must load as nil (default scheme)")
	}

	custom := customScheme(t)
	if err := repo.Replace(ctx, project, custom); err != nil {
		t.Fatalf("replace: %v", err)
	}
	stored, err = repo.Load(ctx, project)
	if err != nil || stored == nil {
		t.Fatalf("load after replace: %v, %v", stored, err)
	}
	if got := stored.Holders(domain.PermissionCreateIssue); len(got) != 2 {
		t.Fatalf("create_issue holders = %v, want lead and viewer", got)
	}

	if err := repo.Replace(ctx, project, domain.DefaultScheme()); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if n := countRows(t, db, project); n == 0 {
		t.Fatal("replace must write rows for the default grants too")
	}
	if err := repo.Reset(ctx, project); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if n := countRows(t, db, project); n != 0 {
		t.Fatalf("reset left %d rows", n)
	}
	stored, err = repo.Load(ctx, project)
	if err != nil || stored != nil {
		t.Fatalf("after reset want nil scheme, got %v, %v", stored, err)
	}
}

func TestSchemeRepositoryReplaceIsAtomic(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewSchemeRepository(db)
	project := seedProject(t, db)

	if err := repo.Replace(ctx, project, customScheme(t)); err != nil {
		t.Fatal(err)
	}
	// A project that does not exist violates the foreign key on insert. The delete of the previous
	// rows must be rolled back with it, so nothing changes for the existing project.
	if err := repo.Replace(ctx, uuid.New(), customScheme(t)); err == nil {
		t.Fatal("replace for a missing project must fail")
	}
	if n := countRows(t, db, project); n == 0 {
		t.Fatal("failed replace must not touch other projects")
	}
}

func TestSchemeRowsCascadeWithProject(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewSchemeRepository(db)
	project := seedProject(t, db)

	if err := repo.Replace(ctx, project, customScheme(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM projects WHERE id = $1`, project); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, project); n != 0 {
		t.Fatalf("rows left after project delete: %d", n)
	}
}
