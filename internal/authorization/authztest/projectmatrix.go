package authztest

// Visibility names of a project, as used by ProjectMatrix. Legacy "team" is read as "open".
const (
	VisibilityOpen    = "open"
	VisibilityPrivate = "private"
)

// ProjectPermissions lists every project permission checked by the matrix. It mirrors the actions of
// internal/authorization/application/port.
var ProjectPermissions = []string{
	"view_project", "edit_project", "delete_project", "manage_members",
	"create_issue", "edit_issue", "comment", "watch_issue",
	"move_cards", "manage_board", "manage_workflow", "manage_sprint",
}

// ProjectMatrix is the declarative permission table per project visibility. The rows are the
// permissions and each value lists the actors (StandardActors names) that are allowed. Anyone not
// listed is denied. Rules:
//   - platform admin: view and watch only (support access), no manage rights.
//   - org owner/admin: lead everywhere, private projects included; delete_project is owner/admin only.
//   - org member: viewer on open projects only; guests get nothing without an explicit role.
//   - explicit project roles: lead, contributor, viewer.
//   - outsider and anonymous: nothing.
var ProjectMatrix = map[string]map[string][]string{
	VisibilityOpen: {
		"view_project":    {"platform-admin", "org-owner", "org-admin", "org-member", "lead", "contributor", "viewer"},
		"edit_project":    {"org-owner", "org-admin", "lead"},
		"delete_project":  {"org-owner", "org-admin"},
		"manage_members":  {"org-owner", "org-admin", "lead"},
		"create_issue":    {"org-owner", "org-admin", "lead", "contributor"},
		"edit_issue":      {"org-owner", "org-admin", "lead", "contributor"},
		"comment":         {"org-owner", "org-admin", "lead", "contributor"},
		"watch_issue":     {"platform-admin", "org-owner", "org-admin", "org-member", "lead", "contributor", "viewer"},
		"move_cards":      {"org-owner", "org-admin", "lead", "contributor"},
		"manage_board":    {"org-owner", "org-admin", "lead"},
		"manage_workflow": {"org-owner", "org-admin", "lead"},
		"manage_sprint":   {"org-owner", "org-admin", "lead"},
	},
	VisibilityPrivate: {
		"view_project":    {"platform-admin", "org-owner", "org-admin", "lead", "contributor", "viewer"},
		"edit_project":    {"org-owner", "org-admin", "lead"},
		"delete_project":  {"org-owner", "org-admin"},
		"manage_members":  {"org-owner", "org-admin", "lead"},
		"create_issue":    {"org-owner", "org-admin", "lead", "contributor"},
		"edit_issue":      {"org-owner", "org-admin", "lead", "contributor"},
		"comment":         {"org-owner", "org-admin", "lead", "contributor"},
		"watch_issue":     {"platform-admin", "org-owner", "org-admin", "lead", "contributor", "viewer"},
		"move_cards":      {"org-owner", "org-admin", "lead", "contributor"},
		"manage_board":    {"org-owner", "org-admin", "lead"},
		"manage_workflow": {"org-owner", "org-admin", "lead"},
		"manage_sprint":   {"org-owner", "org-admin", "lead"},
	},
}
