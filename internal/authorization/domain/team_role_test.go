package domain

import "testing"

func roleP(r ProjectRole) *ProjectRole { return &r }

// TestEffectiveRoleWithTeamGrants covers the best-role rule: the strongest of the direct membership
// and every team grant wins, and org owners and admins stay lead regardless of grants.
func TestEffectiveRoleWithTeamGrants(t *testing.T) {
	cases := []struct {
		name   string
		access Access
		want   *ProjectRole
	}{
		{"team viewer only", Access{OrgRole: OrgRoleMember, TeamRole: roleP(ProjectRoleViewer), Visibility: VisibilityPrivate}, roleP(ProjectRoleViewer)},
		{"team contributor only", Access{OrgRole: OrgRoleGuest, TeamRole: roleP(ProjectRoleContributor), Visibility: VisibilityPrivate}, roleP(ProjectRoleContributor)},
		{"team lead beats direct viewer", Access{OrgRole: OrgRoleMember, ProjectRole: roleP(ProjectRoleViewer), TeamRole: roleP(ProjectRoleLead), Visibility: VisibilityPrivate}, roleP(ProjectRoleLead)},
		{"direct contributor beats team viewer", Access{OrgRole: OrgRoleMember, ProjectRole: roleP(ProjectRoleContributor), TeamRole: roleP(ProjectRoleViewer), Visibility: VisibilityPrivate}, roleP(ProjectRoleContributor)},
		{"org admin stays lead with a team viewer grant", Access{OrgRole: OrgRoleAdmin, TeamRole: roleP(ProjectRoleViewer), Visibility: VisibilityPrivate}, roleP(ProjectRoleLead)},
		{"team grant does not reach a platform admin role", Access{PlatformRole: PlatformRoleAdmin, OrgRole: OrgRoleNone, Visibility: VisibilityPrivate}, roleP(ProjectRoleViewer)},
		{"no grant on private project", Access{OrgRole: OrgRoleMember, Visibility: VisibilityPrivate}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveRole(tc.access)
			switch {
			case got == nil && tc.want == nil:
			case got == nil || tc.want == nil || *got != *tc.want:
				t.Fatalf("EffectiveRole = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBestRoleOrdersLeadContributorViewer(t *testing.T) {
	if got := BestRole(roleP(ProjectRoleViewer), roleP(ProjectRoleContributor)); *got != ProjectRoleContributor {
		t.Fatalf("best of viewer and contributor = %v", *got)
	}
	if got := BestRole(nil, roleP(ProjectRoleViewer)); *got != ProjectRoleViewer {
		t.Fatalf("best of nil and viewer = %v", *got)
	}
	if got := BestRole(nil, nil); got != nil {
		t.Fatalf("best of nil and nil = %v", *got)
	}
}
