package usecase

import (
	"testing"

	"github.com/google/uuid"
)

// TestListFilterByMember covers the member_id filter of the team list: only teams the user belongs to,
// the same visibility rules as the unfiltered list, and no leakage across organizations.
func TestListFilterByMember(t *testing.T) {
	f := newFixture()
	ctx := t.Context()
	alfa := f.seedTeam(t, f.orgA, "Alfa", f.owner)
	f.addTeamMember(t, alfa, f.member, "member")
	f.seedTeam(t, f.orgA, "Beta", f.owner)
	gamma := f.seedTeam(t, f.orgB, "Gamma", f.outsider)

	// A member of org A sees only the teams of A that the given user belongs to.
	teams, err := f.svc.List(ctx, f.owner, f.orgA, f.member)
	if err != nil || len(teams) != 1 || teams[0].ID != alfa.ID {
		t.Fatalf("filter by member: %v / %+v", err, teams)
	}
	// Without the filter nothing changes.
	if teams, _ := f.svc.List(ctx, f.owner, f.orgA, uuid.Nil); len(teams) != 2 {
		t.Fatalf("unfiltered list = %d, want 2", len(teams))
	}

	// A guest may use the filter like the list itself: an empty result is valid, a teams-of-user result too.
	if teams, err := f.svc.List(ctx, f.guest, f.orgA, f.guest); err != nil || len(teams) != 0 {
		t.Fatalf("guest with own id: %v / %d", err, len(teams))
	}
	if teams, err := f.svc.List(ctx, f.guest, f.orgA, f.member); err != nil || len(teams) != 1 {
		t.Fatalf("guest filtering another member: %v / %d", err, len(teams))
	}

	// Outsiders get the organization-level not found, with or without the filter.
	wantCode(t, errOf(f.svc.List(ctx, f.outsider, f.orgA, f.member)), "team.organization_not_found")

	// Isolation: the filter never returns teams of another organization, even for a user that belongs to both.
	if teams, err := f.svc.List(ctx, f.owner, f.orgA, f.outsider); err != nil || len(teams) != 0 {
		t.Fatalf("org A filtered by org B user: %v / %d", err, len(teams))
	}
	wantCode(t, errOf(f.svc.List(ctx, f.owner, f.orgB, f.outsider)), "team.organization_not_found")
	if teams, err := f.svc.List(ctx, f.outsider, f.orgB, f.outsider); err != nil || len(teams) != 1 || teams[0].ID != gamma.ID {
		t.Fatalf("org B filter: %v / %+v", err, teams)
	}
}
