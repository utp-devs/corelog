package domain

import (
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Template is the blueprint a project is created from. It decides the default board,
// the default statuses and issue types, and whether sprints are available.
type Template string

const (
	TemplateScrum        Template = "scrum"
	TemplateKanban       Template = "kanban"
	TemplateTaskTracking Template = "task_tracking"
	TemplateBugTracking  Template = "bug_tracking"
)

// StatusCategory groups statuses for reporting and board logic.
type StatusCategory string

const (
	StatusCategoryTodo       StatusCategory = "todo"
	StatusCategoryInProgress StatusCategory = "in_progress"
	StatusCategoryDone       StatusCategory = "done"
)

// ErrInvalidTemplate is returned for an unknown template name.
var ErrInvalidTemplate = apperror.WithCode(apperror.KindValidation, "project.invalid_template", "plantilla de proyecto no valida")

// StatusDefinition is one status of a template's default flow.
type StatusDefinition struct {
	Name     string
	Category StatusCategory
}

// TemplateDefinition is the static data of one template.
type TemplateDefinition struct {
	Template       Template
	Name           string
	Description    string
	ProjectType    ProjectType
	SprintsEnabled bool
	BoardColumns   []string
	Statuses       []StatusDefinition
	IssueTypes     []string
}

var (
	standardColumns  = []string{"Por hacer", "En curso", "En revisión", "Hecho"}
	standardStatuses = []StatusDefinition{
		{Name: "Por hacer", Category: StatusCategoryTodo},
		{Name: "En curso", Category: StatusCategoryInProgress},
		{Name: "En revisión", Category: StatusCategoryInProgress},
		{Name: "Hecho", Category: StatusCategoryDone},
	}
	standardIssueTypes = []string{"Épica", "Historia", "Tarea", "Error", "Subtarea"}
)

// templateCatalog lists the templates in the order the UI presents them.
var templateCatalog = []TemplateDefinition{
	{
		Template:       TemplateScrum,
		Name:           "Scrum",
		Description:    "Backlog, sprints y tablero para equipos que planifican por iteraciones.",
		ProjectType:    ProjectTypeScrum,
		SprintsEnabled: true,
		BoardColumns:   standardColumns,
		Statuses:       standardStatuses,
		IssueTypes:     standardIssueTypes,
	},
	{
		Template:       TemplateKanban,
		Name:           "Kanban",
		Description:    "Tablero de flujo continuo, sin sprints.",
		ProjectType:    ProjectTypeKanban,
		SprintsEnabled: false,
		BoardColumns:   standardColumns,
		Statuses:       standardStatuses,
		IssueTypes:     standardIssueTypes,
	},
	{
		Template:       TemplateTaskTracking,
		Name:           "Seguimiento de tareas",
		Description:    "Flujo simple de tres estados para listas de tareas, sin sprints.",
		ProjectType:    ProjectTypeKanban,
		SprintsEnabled: false,
		BoardColumns:   []string{"Por hacer", "En curso", "Hecho"},
		Statuses: []StatusDefinition{
			{Name: "Por hacer", Category: StatusCategoryTodo},
			{Name: "En curso", Category: StatusCategoryInProgress},
			{Name: "Hecho", Category: StatusCategoryDone},
		},
		IssueTypes: []string{"Tarea", "Subtarea"},
	},
	{
		Template:       TemplateBugTracking,
		Name:           "Seguimiento de errores",
		Description:    "Tipos centrados en errores y un flujo de cinco estados.",
		ProjectType:    ProjectTypeKanban,
		SprintsEnabled: false,
		BoardColumns:   []string{"Nuevo", "Confirmado", "En curso", "Resuelto", "Cerrado"},
		Statuses: []StatusDefinition{
			{Name: "Nuevo", Category: StatusCategoryTodo},
			{Name: "Confirmado", Category: StatusCategoryTodo},
			{Name: "En curso", Category: StatusCategoryInProgress},
			{Name: "Resuelto", Category: StatusCategoryDone},
			{Name: "Cerrado", Category: StatusCategoryDone},
		},
		IssueTypes: []string{"Error", "Regresión", "Incidencia", "Mejora"},
	},
}

// ParseTemplate validates a template name. Matching ignores case and surrounding spaces.
func ParseTemplate(value string) (Template, error) {
	template := Template(strings.ToLower(strings.TrimSpace(value)))
	if _, err := TemplateDefinitionOf(template); err != nil {
		return "", err
	}
	return template, nil
}

// TemplateDefinitionOf returns a copy of the definition of template.
func TemplateDefinitionOf(template Template) (TemplateDefinition, error) {
	for _, definition := range templateCatalog {
		if definition.Template == template {
			return cloneDefinition(definition), nil
		}
	}
	return TemplateDefinition{}, ErrInvalidTemplate
}

// Templates returns copies of every template definition, in presentation order.
func Templates() []TemplateDefinition {
	out := make([]TemplateDefinition, 0, len(templateCatalog))
	for _, definition := range templateCatalog {
		out = append(out, cloneDefinition(definition))
	}
	return out
}

// TemplateFromProjectType maps the legacy project type to the template of the same family.
func TemplateFromProjectType(projectType ProjectType) Template {
	if projectType == ProjectTypeScrum {
		return TemplateScrum
	}
	return TemplateKanban
}

// ProjectTypeOf returns the legacy project type that matches a template. Only scrum keeps the
// scrum type; every other template is a kanban project.
func ProjectTypeOf(template Template) ProjectType {
	if template == TemplateScrum {
		return ProjectTypeScrum
	}
	return ProjectTypeKanban
}

func cloneDefinition(definition TemplateDefinition) TemplateDefinition {
	definition.BoardColumns = append([]string(nil), definition.BoardColumns...)
	definition.Statuses = append([]StatusDefinition(nil), definition.Statuses...)
	definition.IssueTypes = append([]string(nil), definition.IssueTypes...)
	return definition
}
