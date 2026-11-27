package domain

import "strings"

// Template names, matching the project templates.
const (
	TemplateScrum        = "scrum"
	TemplateKanban       = "kanban"
	TemplateTaskTracking = "task_tracking"
	TemplateBugTracking  = "bug_tracking"
)

// TemplateNames lists the supported templates in presentation order.
func TemplateNames() []string {
	return []string{TemplateScrum, TemplateKanban, TemplateTaskTracking, TemplateBugTracking}
}

func doneResolution() []PostFunction {
	return []PostFunction{SetResolution{Resolution: DoneResolution}}
}

// TemplateWorkflow returns a fresh copy of the default workflow of a template. Matching
// ignores case and surrounding spaces. Status ids are stable slugs; persistence maps them to
// real ids.
func TemplateWorkflow(template string) (Workflow, error) {
	switch strings.ToLower(strings.TrimSpace(template)) {
	case TemplateScrum, TemplateKanban:
		return Workflow{
			Initial: "todo",
			Statuses: []Status{
				{ID: "todo", Name: "Por hacer", Category: CategoryTodo},
				{ID: "in_progress", Name: "En curso", Category: CategoryInProgress},
				{ID: "in_review", Name: "En revisión", Category: CategoryInProgress},
				{ID: "done", Name: "Hecho", Category: CategoryDone},
			},
			Transitions: []Transition{
				{ID: "start", Name: "Iniciar", From: []string{"todo"}, To: "in_progress"},
				{ID: "send_to_review", Name: "Enviar a revisión", From: []string{"in_progress"}, To: "in_review"},
				{ID: "approve", Name: "Aprobar", From: []string{"in_review"}, To: "done", PostFunctions: doneResolution()},
				{ID: "return", Name: "Devolver", From: []string{"in_review"}, To: "in_progress"},
				{ID: "pause", Name: "Pausar", From: []string{"in_progress"}, To: "todo"},
				{ID: "complete", Name: "Completar", From: []string{"todo", "in_progress"}, To: "done", PostFunctions: doneResolution()},
				{ID: "reopen", Name: "Reabrir", From: []string{"done"}, To: "todo", PostFunctions: []PostFunction{ClearResolution{}}},
			},
		}, nil
	case TemplateTaskTracking:
		return Workflow{
			Initial: "todo",
			Statuses: []Status{
				{ID: "todo", Name: "Por hacer", Category: CategoryTodo},
				{ID: "in_progress", Name: "En curso", Category: CategoryInProgress},
				{ID: "done", Name: "Hecho", Category: CategoryDone},
			},
			Transitions: []Transition{
				{ID: "start", Name: "Iniciar", From: []string{"todo"}, To: "in_progress"},
				{ID: "pause", Name: "Pausar", From: []string{"in_progress"}, To: "todo"},
				{ID: "complete", Name: "Completar", From: []string{"todo", "in_progress"}, To: "done", PostFunctions: doneResolution()},
				{ID: "reopen", Name: "Reabrir", From: []string{"done"}, To: "todo", PostFunctions: []PostFunction{ClearResolution{}}},
			},
		}, nil
	case TemplateBugTracking:
		return Workflow{
			Initial: "new",
			Statuses: []Status{
				{ID: "new", Name: "Nuevo", Category: CategoryTodo},
				{ID: "confirmed", Name: "Confirmado", Category: CategoryTodo},
				{ID: "in_progress", Name: "En curso", Category: CategoryInProgress},
				{ID: "resolved", Name: "Resuelto", Category: CategoryDone},
				{ID: "closed", Name: "Cerrado", Category: CategoryDone},
			},
			Transitions: []Transition{
				{ID: "confirm", Name: "Confirmar", From: []string{"new"}, To: "confirmed"},
				{ID: "start", Name: "Iniciar", From: []string{"new", "confirmed"}, To: "in_progress"},
				{ID: "resolve", Name: "Resolver", From: []string{"in_progress"}, To: "resolved", PostFunctions: doneResolution()},
				{ID: "close", Name: "Cerrar", From: []string{"new", "confirmed", "in_progress", "resolved"}, To: "closed", PostFunctions: doneResolution()},
				{ID: "reopen", Name: "Reabrir", From: []string{"resolved"}, To: "in_progress", PostFunctions: []PostFunction{ClearResolution{}}},
			},
		}, nil
	}
	return Workflow{}, ErrUnknownTemplate
}
