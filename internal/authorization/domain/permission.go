package domain

// Permission is a capability of a project that a scheme grants to roles and special holders.
type Permission string

// The permission catalog. Names are stable: they are stored in project_permissions and exposed by
// the API.
const (
	PermissionBrowseProject       Permission = "browse_project"
	PermissionCreateIssue         Permission = "create_issue"
	PermissionEditIssue           Permission = "edit_issue"
	PermissionEditOwnIssue        Permission = "edit_own_issue"
	PermissionDeleteIssue         Permission = "delete_issue"
	PermissionAssignIssue         Permission = "assign_issue"
	PermissionTransitionIssue     Permission = "transition_issue"
	PermissionCommentIssue        Permission = "comment_issue"
	PermissionEditOwnComment      Permission = "edit_own_comment"
	PermissionDeleteOwnComment    Permission = "delete_own_comment"
	PermissionManageComments      Permission = "manage_comments"
	PermissionAddAttachment       Permission = "add_attachment"
	PermissionDeleteOwnAttachment Permission = "delete_own_attachment"
	PermissionManageAttachments   Permission = "manage_attachments"
	PermissionLinkIssue           Permission = "link_issue"
	PermissionLogWork             Permission = "log_work"
	PermissionManageWorklogs      Permission = "manage_worklogs"
	PermissionWatchIssue          Permission = "watch_issue"
	PermissionVoteIssue           Permission = "vote_issue"
	PermissionMoveCards           Permission = "move_cards"
	PermissionManageBoard         Permission = "manage_board"
	PermissionManageSprints       Permission = "manage_sprints"
	PermissionManageVersions      Permission = "manage_versions"
	PermissionManageComponents    Permission = "manage_components"
	PermissionManageWorkflow      Permission = "manage_workflow"
	PermissionManageMembers       Permission = "manage_members"
	PermissionEditProject         Permission = "edit_project"
	PermissionDeleteProject       Permission = "delete_project"
	PermissionViewReports         Permission = "view_reports"
	PermissionManageAutomation    Permission = "manage_automation"
	PermissionBulkChange          Permission = "bulk_change"
	PermissionExportIssues        Permission = "export_issues"
)

// Permissions lists the whole catalog in the order used by the API and the matrix.
var Permissions = []Permission{
	PermissionBrowseProject,
	PermissionCreateIssue,
	PermissionEditIssue,
	PermissionEditOwnIssue,
	PermissionDeleteIssue,
	PermissionAssignIssue,
	PermissionTransitionIssue,
	PermissionCommentIssue,
	PermissionEditOwnComment,
	PermissionDeleteOwnComment,
	PermissionManageComments,
	PermissionAddAttachment,
	PermissionDeleteOwnAttachment,
	PermissionManageAttachments,
	PermissionLinkIssue,
	PermissionLogWork,
	PermissionManageWorklogs,
	PermissionWatchIssue,
	PermissionVoteIssue,
	PermissionMoveCards,
	PermissionManageBoard,
	PermissionManageSprints,
	PermissionManageVersions,
	PermissionManageComponents,
	PermissionManageWorkflow,
	PermissionManageMembers,
	PermissionEditProject,
	PermissionDeleteProject,
	PermissionViewReports,
	PermissionManageAutomation,
	PermissionBulkChange,
	PermissionExportIssues,
}

// Valid reports whether p is part of the catalog.
func (p Permission) Valid() bool {
	for _, known := range Permissions {
		if p == known {
			return true
		}
	}
	return false
}

// Holder is who a grant applies to: a project role (lead, contributor, viewer), the organization
// owners and admins (org_admin), or a special holder of the object being checked (reporter,
// assignee, watcher).
type Holder string

const (
	HolderLead        Holder = "lead"
	HolderContributor Holder = "contributor"
	HolderViewer      Holder = "viewer"
	HolderOrgAdmin    Holder = "org_admin"
	HolderReporter    Holder = "reporter"
	HolderAssignee    Holder = "assignee"
	HolderWatcher     Holder = "watcher"
)

// Holders lists every holder accepted by a scheme, roles first.
var Holders = []Holder{
	HolderLead,
	HolderContributor,
	HolderViewer,
	HolderOrgAdmin,
	HolderReporter,
	HolderAssignee,
	HolderWatcher,
}

// Valid reports whether h is a known holder.
func (h Holder) Valid() bool {
	for _, known := range Holders {
		if h == known {
			return true
		}
	}
	return false
}

// actionPermissions maps the action names used by callers (port.Action) that differ from the
// permission catalog. Every other action name is the permission of the same name.
var actionPermissions = map[string]Permission{
	"view_project":    PermissionBrowseProject,
	"comment":         PermissionCommentIssue,
	"manage_sprint":   PermissionManageSprints,
	"archive_project": PermissionEditProject,
	"restore_project": PermissionEditProject,
}

// PermissionForAction resolves an action name to its permission. It reports false for names that
// are neither a catalog permission nor a known action alias.
func PermissionForAction(action string) (Permission, bool) {
	if p, ok := actionPermissions[action]; ok {
		return p, true
	}
	p := Permission(action)
	return p, p.Valid()
}

// SpecialHolders returns the special holders an organization role grants on every project.
// Organization owners and admins are holders org_admin (the project role lead applies to them too).
func SpecialHolders(a Access) []Holder {
	switch a.OrgRole {
	case OrgRoleOwner, OrgRoleAdmin:
		return []Holder{HolderOrgAdmin}
	}
	return nil
}
