package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

// CodeProjectArchived is the stable error code of a write that targets an archived project. It
// matches the code of the project module so clients handle both alike.
const CodeProjectArchived = "project.archived"

// ErrProjectArchived is returned for every write on an archived project that the archive rules do
// not allow. It is a conflict (409): the project exists and the caller may write to it once it is
// restored.
var ErrProjectArchived = apperror.WithCode(apperror.KindConflict, CodeProjectArchived, "el proyecto esta archivado y no admite cambios")

// readOnlyPermissions never change project data. They stay available on archived projects.
var readOnlyPermissions = map[Permission]bool{
	PermissionBrowseProject: true,
	PermissionViewReports:   true,
	PermissionExportIssues:  true,
}

// archivedWritePermissions are write permissions that an archived project still accepts. Deleting
// the project is the only one: it is the way out of the archive for data that must go.
var archivedWritePermissions = map[Permission]bool{
	PermissionDeleteProject: true,
}

// archivedActions are caller action names that stay allowed on archived projects even though their
// permission is a write. Archive and restore change the archive state itself, so they must work;
// restore needs edit_project, which on its own is blocked (project field updates stay read-only).
var archivedActions = map[string]bool{
	"archive_project": true,
	"restore_project": true,
}

// IsReadOnly reports whether the permission never changes project data.
func (p Permission) IsReadOnly() bool {
	return readOnlyPermissions[p]
}

// ArchivedAllowed reports whether an operation stays allowed on an archived project. action is the
// caller's action name, or "" when the caller checks a permission directly; p is the permission the
// operation needs. Every other permission is a write and is blocked.
func ArchivedAllowed(action string, p Permission) bool {
	return readOnlyPermissions[p] || archivedWritePermissions[p] || archivedActions[action]
}

// ArchivedActionAllowed resolves an action name and reports ArchivedAllowed for it. Unknown actions
// are not allowed. Callers outside the authorization module use it to apply the same rules.
func ArchivedActionAllowed(action string) bool {
	p, ok := PermissionForAction(action)
	return ok && ArchivedAllowed(action, p)
}

// ArchiveBlocks reports whether the archive state withholds p from access, for an operation named by
// action ("" when unnamed). It only matters when the caller already holds the permission.
func ArchiveBlocks(access Access, action string, p Permission) bool {
	return access.Archived && !ArchivedAllowed(action, p)
}
