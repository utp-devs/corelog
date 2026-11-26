package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// TestUserRepositoryTeamMembershipPostgres checks that Save creates the team membership and that
// FindByID reads the team from team_members.
func TestUserRepositoryTeamMembershipPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	org := insertOrganization(t, db, "team-membership")
	team := insertTeam(t, db, org, "Plataforma")

	repo := NewUserRepository(db)
	user, err := domain.NewUser("Ana Torres", "ana@corelog.test", "x", domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	user.TeamID = &team
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

	found, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if found.TeamID == nil || *found.TeamID != team {
		t.Fatalf("FindByID team = %v, want %s from team_members", found.TeamID, team)
	}
	if found.Role != domain.RoleUser {
		t.Errorf("role = %q, want %q", found.Role, domain.RoleUser)
	}
}
