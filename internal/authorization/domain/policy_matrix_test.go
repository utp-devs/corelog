package domain

import "testing"

// TestPolicyMatrix verifica cada accion del proyecto contra cada rol, segun la matriz de
// permisos de ARQUITECTURA.md (seccion 5). Cada fila es una accion; cada columna, un rol.
// Admin es rol global y se evalua sin rol de proyecto.
func TestPolicyMatrix(t *testing.T) {
	policy := Policy{}

	type check func(globalRole string, projectRole *ProjectRole) bool

	// Orden de las columnas: admin global, lead, contributor, viewer, sin acceso.
	const (
		colAdmin = iota
		colLead
		colContributor
		colViewer
		colNone
	)

	matrix := []struct {
		action string
		fn     check
		want   [5]bool
	}{
		{"ver proyecto e incidencias", policy.CanViewProject, [5]bool{true, true, true, true, false}},
		{"editar datos del proyecto", policy.CanEditProject, [5]bool{true, true, false, false, false}},
		{"eliminar proyecto", policy.CanDeleteProject, [5]bool{true, false, false, false, false}},
		{"gestionar miembros y roles", policy.CanManageProjectMembers, [5]bool{true, true, false, false, false}},
		{"crear incidencia", policy.CanCreateIssue, [5]bool{true, true, true, false, false}},
		{"editar incidencia", policy.CanEditIssue, [5]bool{true, true, true, false, false}},
		{"comentar y adjuntar", policy.CanComment, [5]bool{true, true, true, false, false}},
		{"seguir incidencia", policy.CanWatchIssue, [5]bool{true, true, true, true, false}},
		{"mover tarjetas y backlog", policy.CanMoveCards, [5]bool{true, true, true, false, false}},
		{"gestionar tablero y columnas", policy.CanManageBoard, [5]bool{true, true, false, false, false}},
		{"gestionar flujo de estados", policy.CanManageWorkflow, [5]bool{true, true, false, false, false}},
		{"gestionar sprints", policy.CanManageSprints, [5]bool{true, true, false, false, false}},
	}

	roles := [5]struct {
		global  string
		project *ProjectRole
	}{
		{"admin", nil},
		{"requester", role(ProjectRoleLead)},
		{"requester", role(ProjectRoleContributor)},
		{"requester", role(ProjectRoleViewer)},
		{"requester", nil},
	}
	names := [5]string{"admin", "lead", "contributor", "viewer", "sin acceso"}

	for _, row := range matrix {
		for col := colAdmin; col <= colNone; col++ {
			got := row.fn(roles[col].global, roles[col].project)
			if got != row.want[col] {
				t.Errorf("%s para %s: got %v, want %v", row.action, names[col], got, row.want[col])
			}
		}
	}
}
