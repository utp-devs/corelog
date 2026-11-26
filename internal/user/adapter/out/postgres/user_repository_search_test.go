package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// newUserFixture saves a user through the repository, so the row matches the real schema.
func newUserFixture(t *testing.T, repo *UserRepository, name string) *domain.User {
	t.Helper()
	user, err := domain.NewUser(name, name+"@corelog.test", "x", domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), user); err != nil {
		t.Fatalf("Save(%s): %v", name, err)
	}
	return user
}

func insertOrganization(t *testing.T, db *sqlx.DB, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := db.Get(&id, `INSERT INTO organizations (slug, name) VALUES ($1, $2) RETURNING id`, slug, slug); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTeam(t *testing.T, db *sqlx.DB, org uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO teams (id, name, organization_id) VALUES ($1, $2, $3)`, id, name, org); err != nil {
		t.Fatal(err)
	}
	return id
}

// TestUserRepositorySearchByTeamUsesMembershipsPostgres checks that the team filter reads
// team_members: a person who belongs to the team only through the membership is found, and a
// person of another team is not.
func TestUserRepositorySearchByTeamUsesMembershipsPostgres(t *testing.T) {
	db := testdb.New(t)
	repo := NewUserRepository(db)
	org := insertOrganization(t, db, "team-search")
	teamA := insertTeam(t, db, org, "A")
	teamB := insertTeam(t, db, org, "B")

	ana := newUserFixture(t, repo, "Ana Torres")
	carla := newUserFixture(t, repo, "Carla Ruiz")
	luis := newUserFixture(t, repo, "Luis Paredes")
	if _, err := db.Exec(
		`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2), ($1, $3), ($4, $5)`,
		teamA, ana.ID, carla.ID, teamB, luis.ID,
	); err != nil {
		t.Fatal(err)
	}

	found, err := repo.Search(context.Background(), port.SearchFilter{TeamID: &teamA})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	got := map[uuid.UUID]bool{}
	for _, user := range found {
		got[user.ID] = true
	}
	if !got[ana.ID] || !got[carla.ID] {
		t.Errorf("team A search missed members: ana=%v carla=%v", got[ana.ID], got[carla.ID])
	}
	if got[luis.ID] {
		t.Errorf("team A search returned a member of team B")
	}
}

// TestUserRepositoryOrganizationSearchPostgres checks the org scope: only live members of the
// organization are returned, guests only see non-guest members or people who share a project of
// this organization with them, and users of another organization never appear.
func TestUserRepositoryOrganizationSearchPostgres(t *testing.T) {
	db := testdb.New(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	org := insertOrganization(t, db, "org-search")
	otherOrg := insertOrganization(t, db, "org-other")

	owner := newUserFixture(t, repo, "Owner Olga")
	member := newUserFixture(t, repo, "Member Mario")
	guestA := newUserFixture(t, repo, "Guest Ada")
	guestB := newUserFixture(t, repo, "Guest Bruno")
	guestC := newUserFixture(t, repo, "Guest Carmen")
	deactivated := newUserFixture(t, repo, "Member Dario")
	foreign := newUserFixture(t, repo, "Foreign Fer")

	members := []struct {
		org  uuid.UUID
		user uuid.UUID
		role string
	}{
		{org, owner.ID, "owner"},
		{org, member.ID, "member"},
		{org, guestA.ID, "guest"},
		{org, guestB.ID, "guest"},
		{org, guestC.ID, "guest"},
		{org, deactivated.ID, "member"},
		{otherOrg, foreign.ID, "member"},
	}
	for _, m := range members {
		if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, m.org, m.user, m.role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE users SET deactivated_at = now() WHERE id = $1`, deactivated.ID); err != nil {
		t.Fatal(err)
	}

	// Project P of this organization: guestA and guestB share it. guestC has no project.
	projectID := uuid.New()
	if _, err := db.Exec(
		`INSERT INTO projects (id, key, name, lead_id, organization_id) VALUES ($1, 'SCOPE', 'Scope', $2, $3)`,
		projectID, owner.ID, org,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'contributor'), ($1, $3, 'viewer')`,
		projectID, guestA.ID, guestB.ID,
	); err != nil {
		t.Fatal(err)
	}

	names := func(users []*domain.User) map[uuid.UUID]bool {
		got := map[uuid.UUID]bool{}
		for _, u := range users {
			got[u.ID] = true
		}
		return got
	}

	t.Run("member sees every live member of the organization", func(t *testing.T) {
		found, err := repo.SearchOrganizationMembers(ctx, port.OrganizationSearchFilter{OrganizationID: org, ActorID: member.ID})
		if err != nil {
			t.Fatal(err)
		}
		got := names(found)
		for _, want := range []uuid.UUID{owner.ID, member.ID, guestA.ID, guestB.ID, guestC.ID} {
			if !got[want] {
				t.Errorf("member must see %s", want)
			}
		}
		if got[deactivated.ID] || got[foreign.ID] {
			t.Errorf("deactivated users and users of other organizations must not appear")
		}
	})

	t.Run("guest only sees non-guests and guests sharing a project", func(t *testing.T) {
		found, err := repo.SearchOrganizationMembers(ctx, port.OrganizationSearchFilter{OrganizationID: org, ActorID: guestA.ID, ActorIsGuest: true})
		if err != nil {
			t.Fatal(err)
		}
		got := names(found)
		if !got[owner.ID] || !got[member.ID] {
			t.Errorf("guest must see non-guest members")
		}
		if !got[guestB.ID] {
			t.Errorf("guest must see a guest who shares a project")
		}
		if got[guestC.ID] {
			t.Errorf("guest must not see a guest without a shared project")
		}
	})

	t.Run("query matches name and email inside the scope", func(t *testing.T) {
		found, err := repo.SearchOrganizationMembers(ctx, port.OrganizationSearchFilter{OrganizationID: org, ActorID: member.ID, Query: "mario"})
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].ID != member.ID {
			t.Errorf("query 'mario' returned %d users, want only the member", len(found))
		}
	})

	t.Run("FindOrganizationRole reports membership", func(t *testing.T) {
		role, found, err := repo.FindOrganizationRole(ctx, org, guestA.ID)
		if err != nil || !found || role != "guest" {
			t.Errorf("FindOrganizationRole = %q, %v, %v; want guest, true, nil", role, found, err)
		}
		_, found, err = repo.FindOrganizationRole(ctx, org, foreign.ID)
		if err != nil || found {
			t.Errorf("non-member must not be found: found=%v err=%v", found, err)
		}
	})
}

// TestUserRepositoryProfileAndLoginPostgres checks the profile columns round trip, that the
// display name and timezone defaults are written on insert, and that RecordLogin stores last_login_at.
func TestUserRepositoryProfileAndLoginPostgres(t *testing.T) {
	db := testdb.New(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := newUserFixture(t, repo, "Ana Perfil")
	stored, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DisplayName != "Ana Perfil" {
		t.Errorf("display_name must be set on insert, got %q", stored.DisplayName)
	}
	if stored.Timezone != domain.DefaultTimezone {
		t.Errorf("timezone must default to UTC, got %q", stored.Timezone)
	}
	if stored.LastLoginAt != nil {
		t.Error("a new user has no last login")
	}

	avatar := "avatars/ana.png"
	stored.DisplayName = "Ana P."
	stored.JobTitle = "Analista"
	stored.Department = "Operaciones"
	stored.Timezone = "Europe/Madrid"
	stored.AvatarKey = &avatar
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update: %v", err)
	}
	reloaded, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DisplayName != "Ana P." || reloaded.JobTitle != "Analista" || reloaded.Department != "Operaciones" || reloaded.Timezone != "Europe/Madrid" {
		t.Errorf("profile columns did not round trip: %+v", reloaded)
	}
	if reloaded.AvatarKey == nil || *reloaded.AvatarKey != avatar {
		t.Errorf("avatar_key did not round trip: %v", reloaded.AvatarKey)
	}

	if err := repo.RecordLogin(ctx, user.ID, time.Now().UTC()); err != nil {
		t.Fatalf("RecordLogin: %v", err)
	}
	afterLogin, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterLogin.LastLoginAt == nil {
		t.Fatal("RecordLogin must store last_login_at")
	}
	if err := repo.RecordLogin(ctx, uuid.New(), time.Now().UTC()); err != domain.ErrUserNotFound {
		t.Errorf("RecordLogin for a missing user: want ErrUserNotFound, got %v", err)
	}
}
