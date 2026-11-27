package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/favorite/domain"
)

// openFavoriteTestSchema creates a temporary schema with the tables used by the favorites queries.
// The schema is dropped when the test ends.
func openFavoriteTestSchema(t *testing.T) *sqlx.DB {
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

	schema := "favorite_repo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE projects (
			id UUID PRIMARY KEY, organization_id UUID NOT NULL, key TEXT NOT NULL, name TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT 'blue', visibility TEXT NOT NULL DEFAULT 'open',
			deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE organization_members (organization_id UUID NOT NULL, user_id UUID NOT NULL, role TEXT NOT NULL,
			PRIMARY KEY (organization_id, user_id))`,
		`CREATE TABLE project_members (project_id UUID NOT NULL, user_id UUID NOT NULL, role TEXT NOT NULL,
			PRIMARY KEY (project_id, user_id))`,
		`CREATE TABLE project_team_roles (project_id UUID NOT NULL, team_id UUID NOT NULL, role TEXT NOT NULL,
			PRIMARY KEY (project_id, team_id))`,
		`CREATE TABLE team_members (team_id UUID NOT NULL, user_id UUID NOT NULL, PRIMARY KEY (team_id, user_id))`,
		`CREATE TABLE favorites (
			user_id UUID NOT NULL, entity_type TEXT NOT NULL, entity_id UUID NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, entity_type, entity_id))`,
		`CREATE TABLE recent_items (
			user_id UUID NOT NULL, entity_type TEXT NOT NULL, entity_id UUID NOT NULL,
			viewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, entity_type, entity_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// insertProject stores a live project of the organization with the given visibility.
func insertProject(t *testing.T, db *sqlx.DB, org uuid.UUID, name, visibility string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, visibility) VALUES ($1, $2, $3, $4, $5)`,
		id, org, strings.ToUpper(name[:3]), name, visibility); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFavoriteRepositoryAddIsIdempotentAndListsProjects(t *testing.T) {
	db := openFavoriteTestSchema(t)
	repo := NewRepository(db)
	ctx := context.Background()
	user, org := uuid.New(), uuid.New()
	project := insertProject(t, db, org, "Alpha", "open")

	for i := 0; i < 2; i++ {
		fav, err := domain.NewFavorite(user, domain.EntityProject, project, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.AddFavorite(ctx, fav); err != nil {
			t.Fatalf("add #%d: %v", i+1, err)
		}
	}
	entries, err := repo.ListFavorites(ctx, user, domain.EntityProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != project || entries[0].Name != "Alpha" || entries[0].Key != "ALP" {
		t.Fatalf("favorites = %+v, want one Alpha project", entries)
	}

	if err := repo.RemoveFavorite(ctx, user, domain.EntityProject, project); err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveFavorite(ctx, user, domain.EntityProject, project); err != nil {
		t.Fatalf("second remove must be a no-op: %v", err)
	}
	entries, _ = repo.ListFavorites(ctx, user, domain.EntityProject)
	if len(entries) != 0 {
		t.Fatalf("favorites after remove = %d, want 0", len(entries))
	}
}

func TestFavoriteRepositoryHidesDeletedProjects(t *testing.T) {
	db := openFavoriteTestSchema(t)
	repo := NewRepository(db)
	ctx := context.Background()
	user := uuid.New()
	project := insertProject(t, db, uuid.New(), "Gone", "open")
	fav, _ := domain.NewFavorite(user, domain.EntityProject, project, time.Now().UTC())
	if err := repo.AddFavorite(ctx, fav); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, project); err != nil {
		t.Fatal(err)
	}
	entries, err := repo.ListFavorites(ctx, user, domain.EntityProject)
	if err != nil || len(entries) != 0 {
		t.Fatalf("deleted project in favorites: %d entries, err %v", len(entries), err)
	}
}

func TestFavoriteRepositoryRecentOrderingAndPruning(t *testing.T) {
	db := openFavoriteTestSchema(t)
	repo := NewRepository(db)
	ctx := context.Background()
	user, org := uuid.New(), uuid.New()
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	const total = domain.RecentRetention + 5
	ids := make([]uuid.UUID, 0, total)
	for i := 0; i < total; i++ {
		project := insertProject(t, db, org, "Proj"+uuid.NewString()[:6], "open")
		ids = append(ids, project)
		item := domain.RecentItem{UserID: user, Type: domain.EntityProject, EntityID: project, ViewedAt: base.Add(time.Duration(i) * time.Minute)}
		if err := repo.TouchRecent(ctx, item); err != nil {
			t.Fatalf("touch %d: %v", i, err)
		}
	}

	var stored int
	if err := db.Get(&stored, `SELECT COUNT(*) FROM recent_items WHERE user_id = $1 AND entity_type = 'project'`, user); err != nil {
		t.Fatal(err)
	}
	if stored != domain.RecentRetention {
		t.Fatalf("stored recents = %d, want pruned to %d", stored, domain.RecentRetention)
	}

	entries, err := repo.ListRecent(ctx, user, domain.EntityProject, domain.RecentRetention)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != domain.RecentRetention {
		t.Fatalf("listed recents = %d, want %d", len(entries), domain.RecentRetention)
	}
	if entries[0].ID != ids[total-1] {
		t.Fatalf("first recent is not the last viewed project")
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].ViewedAt.Before(*entries[i].ViewedAt) {
			t.Fatalf("recents not ordered by viewed_at desc at index %d", i)
		}
	}

	// Re-viewing the oldest kept project moves it to the front without growing the list.
	oldest := ids[total-domain.RecentRetention]
	again := domain.RecentItem{UserID: user, Type: domain.EntityProject, EntityID: oldest, ViewedAt: base.Add(time.Hour)}
	if err := repo.TouchRecent(ctx, again); err != nil {
		t.Fatal(err)
	}
	entries, _ = repo.ListRecent(ctx, user, domain.EntityProject, 3)
	if len(entries) != 3 || entries[0].ID != oldest || entries[0].ViewedAt == nil {
		t.Fatalf("re-viewed project not first: %+v", entries)
	}
}

func TestProjectVisibilityRules(t *testing.T) {
	db := openFavoriteTestSchema(t)
	visibility := NewProjectVisibility(db)
	ctx := context.Background()
	org := uuid.New()
	owner, admin, member, guest, teamMember, outsider := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for user, role := range map[uuid.UUID]string{owner: "owner", admin: "admin", member: "member", guest: "guest", teamMember: "member"} {
		if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, org, user, role); err != nil {
			t.Fatal(err)
		}
	}

	openProject := insertProject(t, db, org, "Open", "open")
	privateProject := insertProject(t, db, org, "Private", "private")
	deletedProject := insertProject(t, db, org, "Deleted", "open")
	teamProject := insertProject(t, db, org, "Team", "private")
	if _, err := db.Exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, deletedProject); err != nil {
		t.Fatal(err)
	}
	team := uuid.New()
	if _, err := db.Exec(`INSERT INTO project_team_roles (project_id, team_id, role) VALUES ($1, $2, 'viewer')`, teamProject, team); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)`, team, teamMember); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		user    uuid.UUID
		project uuid.UUID
		want    bool
	}{
		{"owner sees private", owner, privateProject, true},
		{"admin sees private", admin, privateProject, true},
		{"member sees open", member, openProject, true},
		{"member does not see private", member, privateProject, false},
		{"member does not see deleted", member, deletedProject, false},
		{"guest does not see open", guest, openProject, false},
		{"team member sees granted project", teamMember, teamProject, true},
		{"outsider sees nothing", outsider, openProject, false},
	}
	for _, tc := range cases {
		got, err := visibility.CanView(ctx, tc.user, tc.project)
		if err != nil || got != tc.want {
			t.Errorf("%s: CanView = %v, err %v; want %v", tc.name, got, err, tc.want)
		}
	}
}
