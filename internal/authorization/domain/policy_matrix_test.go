package domain

import (
	"testing"

	"github.com/jhonsferg/corelog/internal/authorization/authztest"
)

// accessOf converts a test actor into the domain context of one project with the given visibility.
func accessOf(a authztest.Actor, visibility Visibility) Access {
	access := Access{PlatformRole: a.PlatformRole, OrgRole: OrgRole(a.OrgRole), Visibility: visibility}
	if a.ProjectRole != "" {
		access.ProjectRole = roleOf(ProjectRole(a.ProjectRole))
	}
	return access
}

// TestPolicyMatrix verifies every project permission against every standard actor, for open and
// private projects (ProjectMatrix). Any actor not listed for a permission is denied.
func TestPolicyMatrix(t *testing.T) {
	policy := Policy{}

	checks := map[string]func(Access) bool{
		"view_project":    policy.CanViewProject,
		"edit_project":    policy.CanEditProject,
		"delete_project":  policy.CanDeleteProject,
		"manage_members":  policy.CanManageProjectMembers,
		"create_issue":    policy.CanCreateIssue,
		"edit_issue":      policy.CanEditIssue,
		"comment":         policy.CanComment,
		"watch_issue":     policy.CanWatchIssue,
		"move_cards":      policy.CanMoveCards,
		"manage_board":    policy.CanManageBoard,
		"manage_workflow": policy.CanManageWorkflow,
		"manage_sprint":   policy.CanManageSprints,
	}

	for _, visibility := range []string{authztest.VisibilityOpen, authztest.VisibilityPrivate} {
		t.Run(visibility, func(t *testing.T) {
			authztest.RunMatrix(t, authztest.StandardActors, authztest.ProjectPermissions, authztest.ProjectMatrix[visibility],
				func(a authztest.Actor, permission string) bool {
					return checks[permission](accessOf(a, Visibility(visibility)))
				})
		})
	}
}
