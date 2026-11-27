package domain

import "testing"

func TestEffectiveRole(t *testing.T) {
	viewer := roleOf(ProjectRoleViewer)
	contributor := roleOf(ProjectRoleContributor)
	lead := roleOf(ProjectRoleLead)

	cases := []struct {
		name   string
		access Access
		want   *ProjectRole
	}{
		{"org owner is lead on a private project without role", Access{OrgRole: OrgRoleOwner, Visibility: VisibilityPrivate}, lead},
		{"org admin is lead even with a lower explicit role", Access{OrgRole: OrgRoleAdmin, ProjectRole: viewer, Visibility: VisibilityPrivate}, lead},
		{"explicit role applies", Access{OrgRole: OrgRoleMember, ProjectRole: contributor, Visibility: VisibilityPrivate}, contributor},
		{"org member sees open project as viewer", Access{OrgRole: OrgRoleMember, Visibility: VisibilityOpen}, viewer},
		{"org member has no access to private project", Access{OrgRole: OrgRoleMember, Visibility: VisibilityPrivate}, nil},
		{"guest has no access to open project", Access{OrgRole: OrgRoleGuest, Visibility: VisibilityOpen}, nil},
		{"guest with explicit role keeps it", Access{OrgRole: OrgRoleGuest, ProjectRole: viewer, Visibility: VisibilityOpen}, viewer},
		{"outsider has nothing", Access{OrgRole: OrgRoleNone, Visibility: VisibilityOpen}, nil},
		{"platform admin reads everything as viewer", Access{PlatformRole: PlatformRoleAdmin, OrgRole: OrgRoleNone, Visibility: VisibilityPrivate}, viewer},
		{"legacy context without visibility is not open", Access{OrgRole: OrgRoleMember}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveRole(tc.access)
			switch {
			case got == nil && tc.want == nil:
			case got == nil || tc.want == nil || *got != *tc.want:
				t.Fatalf("EffectiveRole(%+v) = %v, want %v", tc.access, got, tc.want)
			}
		})
	}
}

func TestPlatformAdminCannotManage(t *testing.T) {
	policy := Policy{}
	admin := Access{PlatformRole: PlatformRoleAdmin, OrgRole: OrgRoleNone, Visibility: VisibilityPrivate}
	if !policy.CanViewProject(admin) {
		t.Fatal("platform admin should view any project")
	}
	if policy.CanEditProject(admin) || policy.CanManageSprints(admin) || policy.CanDeleteProject(admin) {
		t.Fatal("platform admin must not manage projects without an organization role")
	}
}

// TestLeadOrOrgManagerDeletesProject: the project lead may delete the project (with the typed name
// checked by the use case), and so may an organization owner or admin. Contributors, viewers and
// platform admins may not.
func TestLeadOrOrgManagerDeletesProject(t *testing.T) {
	policy := Policy{}
	lead := roleOf(ProjectRoleLead)
	contributor := roleOf(ProjectRoleContributor)
	if !policy.CanDeleteProject(Access{OrgRole: OrgRoleMember, ProjectRole: lead}) {
		t.Fatal("project lead should be able to delete the project")
	}
	if !policy.CanDeleteProject(Access{OrgRole: OrgRoleAdmin, Visibility: VisibilityPrivate}) {
		t.Fatal("org admin should delete the project")
	}
	if policy.CanDeleteProject(Access{OrgRole: OrgRoleMember, ProjectRole: contributor}) {
		t.Fatal("contributor must not delete the project")
	}
	if policy.CanDeleteProject(Access{PlatformRole: PlatformRoleAdmin, OrgRole: OrgRoleMember, Visibility: VisibilityOpen}) {
		t.Fatal("platform admin support access must not delete the project")
	}
}
