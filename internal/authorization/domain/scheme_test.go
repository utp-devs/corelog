package domain

import (
	"testing"

	"github.com/jhonsferg/corelog/internal/authorization/authztest"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// expectedDefault is the default scheme written as a specification table: for each permission, the
// project roles and special holders that pass. It is compared with every combination of role and
// special holder below.
var expectedDefault = map[Permission][]Holder{
	PermissionBrowseProject:       {HolderLead, HolderContributor, HolderViewer},
	PermissionCreateIssue:         {HolderLead, HolderContributor},
	PermissionEditIssue:           {HolderLead, HolderContributor},
	PermissionEditOwnIssue:        {HolderReporter},
	PermissionDeleteIssue:         {HolderLead},
	PermissionAssignIssue:         {HolderLead, HolderContributor},
	PermissionTransitionIssue:     {HolderLead, HolderContributor, HolderAssignee},
	PermissionCommentIssue:        {HolderLead, HolderContributor},
	PermissionEditOwnComment:      {HolderLead, HolderContributor},
	PermissionDeleteOwnComment:    {HolderLead, HolderContributor},
	PermissionManageComments:      {HolderLead},
	PermissionAddAttachment:       {HolderLead, HolderContributor},
	PermissionDeleteOwnAttachment: {HolderLead, HolderContributor},
	PermissionManageAttachments:   {HolderLead},
	PermissionLinkIssue:           {HolderLead, HolderContributor},
	PermissionLogWork:             {HolderLead, HolderContributor},
	PermissionManageWorklogs:      {HolderLead},
	PermissionWatchIssue:          {HolderLead, HolderContributor, HolderViewer},
	PermissionVoteIssue:           {HolderLead, HolderContributor, HolderViewer},
	PermissionMoveCards:           {HolderLead, HolderContributor},
	PermissionManageBoard:         {HolderLead},
	PermissionManageSprints:       {HolderLead},
	PermissionManageVersions:      {HolderLead},
	PermissionManageComponents:    {HolderLead},
	PermissionManageWorkflow:      {HolderLead},
	PermissionManageMembers:       {HolderLead},
	PermissionEditProject:         {HolderLead},
	PermissionDeleteProject:       {HolderLead, HolderOrgAdmin},
	PermissionViewReports:         {HolderLead, HolderContributor, HolderViewer},
	PermissionManageAutomation:    {HolderLead},
	PermissionBulkChange:          {HolderLead, HolderContributor},
	PermissionExportIssues:        {HolderLead, HolderContributor, HolderViewer},
}

func contains(list []Holder, h Holder) bool {
	for _, x := range list {
		if x == h {
			return true
		}
	}
	return false
}

// TestDefaultSchemeMatrix checks every permission against every project role (or none) and every
// combination of special holder, using the default scheme.
func TestDefaultSchemeMatrix(t *testing.T) {
	scheme := DefaultScheme()
	roles := []*ProjectRole{nil, roleOf(ProjectRoleLead), roleOf(ProjectRoleContributor), roleOf(ProjectRoleViewer)}
	specialSets := [][]Holder{
		nil,
		{HolderReporter},
		{HolderAssignee},
		{HolderWatcher},
		{HolderOrgAdmin},
		{HolderReporter, HolderAssignee, HolderWatcher, HolderOrgAdmin},
	}

	if len(expectedDefault) != len(Permissions) {
		t.Fatalf("expectation covers %d permissions, catalog has %d", len(expectedDefault), len(Permissions))
	}
	for _, p := range Permissions {
		want, ok := expectedDefault[p]
		if !ok {
			t.Fatalf("no expectation for %s", p)
		}
		for _, role := range roles {
			for _, specials := range specialSets {
				expected := false
				if role != nil && contains(want, Holder(*role)) {
					expected = true
				}
				for _, s := range specials {
					if contains(want, s) {
						expected = true
					}
				}
				if got := Allowed(scheme, p, role, specials...); got != expected {
					t.Errorf("%s role=%v specials=%v: got %v, want %v", p, role, specials, got, expected)
				}
			}
		}
	}
}

// TestDefaultSchemeMatchesPolicy checks that the default scheme decides the mapped actions exactly like
// the existing policy, for every standard actor and visibility.
func TestDefaultSchemeMatchesPolicy(t *testing.T) {
	policy := Policy{}
	checks := map[Permission]func(Access) bool{
		PermissionBrowseProject:  policy.CanViewProject,
		PermissionEditProject:    policy.CanEditProject,
		PermissionDeleteProject:  policy.CanDeleteProject,
		PermissionManageMembers:  policy.CanManageProjectMembers,
		PermissionCreateIssue:    policy.CanCreateIssue,
		PermissionEditIssue:      policy.CanEditIssue,
		PermissionCommentIssue:   policy.CanComment,
		PermissionWatchIssue:     policy.CanWatchIssue,
		PermissionMoveCards:      policy.CanMoveCards,
		PermissionManageBoard:    policy.CanManageBoard,
		PermissionManageWorkflow: policy.CanManageWorkflow,
		PermissionManageSprints:  policy.CanManageSprints,
	}
	scheme := DefaultScheme()
	for _, visibility := range []Visibility{VisibilityOpen, VisibilityPrivate} {
		for _, actor := range authztest.StandardActors {
			access := accessOf(actor, visibility)
			for p, check := range checks {
				if got, want := Decide(scheme, p, access), check(access); got != want {
					t.Errorf("%s %s %s: scheme %v, policy %v", actor.Name, visibility, p, got, want)
				}
			}
		}
	}
}

func TestDefaultSchemeIsValid(t *testing.T) {
	if err := DefaultScheme().Validate(); err != nil {
		t.Fatalf("default scheme must be valid: %v", err)
	}
}

func TestNewSchemeValidation(t *testing.T) {
	// base returns the default grants, so each case changes one thing.
	base := func() []Grant { return DefaultScheme().Grants() }
	replace := func(grants []Grant, p Permission, holders ...Holder) []Grant {
		for i := range grants {
			if grants[i].Permission == p {
				grants[i].Holders = holders
			}
		}
		return grants
	}

	cases := []struct {
		name   string
		grants []Grant
		code   string
	}{
		{"default is accepted", base(), ""},
		{"lead loses manage_members", replace(base(), PermissionManageMembers, HolderContributor), CodeInvalidScheme},
		{"lead loses edit_project", replace(base(), PermissionEditProject, HolderViewer), CodeInvalidScheme},
		{"delete_project without lead or org admin", replace(base(), PermissionDeleteProject, HolderContributor), CodeInvalidScheme},
		{"delete_project granted to lead is accepted", replace(base(), PermissionDeleteProject, HolderLead), ""},
		{"delete_project granted to org_admin only is accepted", replace(base(), PermissionDeleteProject, HolderOrgAdmin), ""},
		{"browse_project without viewer or contributor", replace(base(), PermissionBrowseProject, HolderReporter), CodeInvalidScheme},
		{"browse_project for lead only is accepted", replace(base(), PermissionBrowseProject, HolderLead), ""},
		{"unknown permission", append(base(), Grant{Permission: "fly_drone", Holders: []Holder{HolderLead}}), CodeInvalidScheme},
		{"unknown holder", replace(base(), PermissionCreateIssue, HolderLead, "janitor"), CodeInvalidScheme},
		{"duplicate permission", append(base(), Grant{Permission: PermissionCreateIssue, Holders: []Holder{HolderLead}}), CodeInvalidScheme},
		{"duplicate holder is merged", replace(base(), PermissionCreateIssue, HolderLead, HolderLead, HolderContributor), ""},
		{"empty grants fail the lead rules", nil, CodeInvalidScheme},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewScheme(tc.grants)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %s, got success", tc.code)
			}
			if got := apperror.CodeOf(err); got != tc.code {
				t.Fatalf("code = %q, want %q", got, tc.code)
			}
			if apperror.KindOf(err) != apperror.KindValidation {
				t.Fatalf("kind = %v, want validation", apperror.KindOf(err))
			}
		})
	}
}

func TestPermissionForAction(t *testing.T) {
	cases := map[string]Permission{
		"view_project":    PermissionBrowseProject,
		"comment":         PermissionCommentIssue,
		"manage_sprint":   PermissionManageSprints,
		"create_issue":    PermissionCreateIssue,
		"manage_workflow": PermissionManageWorkflow,
	}
	for action, want := range cases {
		got, ok := PermissionForAction(action)
		if !ok || got != want {
			t.Errorf("%s: got %q/%v, want %q", action, got, ok, want)
		}
	}
	if _, ok := PermissionForAction("unknown"); ok {
		t.Error("unknown action must not map to a permission")
	}
}

func TestCatalogHasNoDuplicates(t *testing.T) {
	seen := map[Permission]bool{}
	for _, p := range Permissions {
		if seen[p] {
			t.Fatalf("duplicate permission %s", p)
		}
		seen[p] = true
	}
}
