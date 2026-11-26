package domain

import (
	"testing"

	"github.com/jhonsferg/corelog/internal/authorization/authztest"
)

// projectRoleOf converts the actor's project role name into the domain value (nil = no membership).
func projectRoleOf(a authztest.Actor) *ProjectRole {
	if a.ProjectRole == "" {
		return nil
	}
	return role(ProjectRole(a.ProjectRole))
}

// TestPolicyMatrix verifies every project permission against every standard actor, following the
// permission matrix of docs/05-architecture.md section 3. Admin is a global role and is evaluated
// without a project role. Any actor not listed for a permission is denied.
func TestPolicyMatrix(t *testing.T) {
	policy := Policy{}

	checks := map[string]func(globalRole string, projectRole *ProjectRole) bool{
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
		"manage_sprints":  policy.CanManageSprints,
	}

	permissions := []string{
		"view_project", "edit_project", "delete_project", "manage_members",
		"create_issue", "edit_issue", "comment", "watch_issue",
		"move_cards", "manage_board", "manage_workflow", "manage_sprints",
	}

	allowed := map[string][]string{
		"view_project":    {"admin", "lead", "contributor", "viewer"},
		"edit_project":    {"admin", "lead"},
		"delete_project":  {"admin"},
		"manage_members":  {"admin", "lead"},
		"create_issue":    {"admin", "lead", "contributor"},
		"edit_issue":      {"admin", "lead", "contributor"},
		"comment":         {"admin", "lead", "contributor"},
		"watch_issue":     {"admin", "lead", "contributor", "viewer"},
		"move_cards":      {"admin", "lead", "contributor"},
		"manage_board":    {"admin", "lead"},
		"manage_workflow": {"admin", "lead"},
		"manage_sprints":  {"admin", "lead"},
	}

	authztest.RunMatrix(t, authztest.StandardActors, permissions, allowed, func(a authztest.Actor, permission string) bool {
		return checks[permission](a.PlatformRole, projectRoleOf(a))
	})
}
