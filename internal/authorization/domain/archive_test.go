package domain

import "testing"

// TestArchivedAllowedPermissions pins the permission-level archive rules: reads and delete stay
// available, every other permission of the catalog is a write that archived projects reject.
func TestArchivedAllowedPermissions(t *testing.T) {
	allowed := map[Permission]bool{
		PermissionBrowseProject: true,
		PermissionViewReports:   true,
		PermissionExportIssues:  true,
		PermissionDeleteProject: true,
	}
	for _, p := range Permissions {
		if got := ArchivedAllowed("", p); got != allowed[p] {
			t.Errorf("ArchivedAllowed(\"\", %s) = %v, want %v", p, got, allowed[p])
		}
	}
}

// TestArchivedAllowedActions checks that restore and archive stay allowed even though their
// permission (edit_project) is a write, and that a bare edit_project is still blocked.
func TestArchivedAllowedActions(t *testing.T) {
	cases := map[string]bool{
		"view_project":    true,
		"delete_project":  true,
		"restore_project": true,
		"archive_project": true,
		"edit_project":    false,
		"create_issue":    false,
		"comment":         false,
		"manage_members":  false,
		"unknown":         false,
	}
	for action, want := range cases {
		if got := ArchivedActionAllowed(action); got != want {
			t.Errorf("ArchivedActionAllowed(%q) = %v, want %v", action, got, want)
		}
	}
}

func TestArchiveBlocksOnlyArchivedProjects(t *testing.T) {
	if ArchiveBlocks(Access{Archived: false}, "", PermissionEditIssue) {
		t.Error("active project must never be blocked by the archive rules")
	}
	if !ArchiveBlocks(Access{Archived: true}, "", PermissionEditIssue) {
		t.Error("archived project must block edit_issue")
	}
	if ArchiveBlocks(Access{Archived: true}, "restore_project", PermissionEditProject) {
		t.Error("restore_project must pass on an archived project")
	}
}
