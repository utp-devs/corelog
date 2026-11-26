package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

// TestTeamRepository_ListMembersExcludesDeactivatedPostgres checks that closed accounts are not listed
// and not counted as team members.
func TestTeamRepository_ListMembersExcludesDeactivatedPostgres(t *testing.T) {
	db := testdb.New(t)
	repo := NewTeamRepository(db)
	ctx := context.Background()
	owner := insertUser(t, repo, "closed-owner@example.test")
	closed := insertUser(t, repo, "closed-member@example.test")
	org := uuid.MustParse(defaultOrgID)

	team := newTeam(t, org, owner, "Cerrados")
	if err := repo.Save(ctx, team); err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, user := range []uuid.UUID{owner, closed} {
		m, _ := domain.NewMember(team.ID, user, domain.RoleMember)
		if err := repo.AddMember(ctx, org, m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET deactivated_at = now() WHERE id = $1`, closed); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	members, err := repo.ListMembers(ctx, org, team.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 || members[0].UserID != owner {
		t.Fatalf("members = %+v, want only the live user", members)
	}
	got, err := repo.FindByID(ctx, team.ID, owner)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.MemberCount != 1 {
		t.Fatalf("MemberCount = %d, want 1", got.MemberCount)
	}
}
