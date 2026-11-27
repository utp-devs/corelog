package domain

import "strings"

// TemplateLabel returns the Spanish name and description shown for a template. Unknown templates return
// empty strings; callers check the template with TemplateWorkflow first.
func TemplateLabel(template string) (name, description string) {
	switch strings.ToLower(strings.TrimSpace(template)) {
	case TemplateScrum:
		return "Scrum", "Flujo con revisión antes de dar por cerrado el trabajo, pensado para equipos que planifican por sprints."
	case TemplateKanban:
		return "Kanban", "Flujo continuo con revisión, sin fases de cierre separadas, pensado para equipos que trabajan por flujo."
	case TemplateTaskTracking:
		return "Seguimiento de tareas", "Flujo sencillo de tres estados para llevar tareas pendientes, en curso y hechas."
	case TemplateBugTracking:
		return "Seguimiento de errores", "Flujo para incidencias de calidad: confirmación, resolución y cierre."
	}
	return "", ""
}
