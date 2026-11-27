package domain

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// SwimlaneMode says how the cards of a board are grouped into horizontal lanes.
type SwimlaneMode string

const (
	SwimlanesNone     SwimlaneMode = "none"
	SwimlanesAssignee SwimlaneMode = "assignee"
	SwimlanesEpic     SwimlaneMode = "epic"
	SwimlanesQuery    SwimlaneMode = "query"
	SwimlanesStory    SwimlaneMode = "story"
)

// Valid reports whether the mode is one of the known ones.
func (m SwimlaneMode) Valid() bool {
	switch m {
	case SwimlanesNone, SwimlanesAssignee, SwimlanesEpic, SwimlanesQuery, SwimlanesStory:
		return true
	}
	return false
}

// Estimation is the statistic the column headers sum.
type Estimation string

const (
	EstimationStoryPoints Estimation = "story_points"
	EstimationIssueCount  Estimation = "issue_count"
	EstimationTime        Estimation = "time"
)

// Valid reports whether the estimation is one of the known ones.
func (e Estimation) Valid() bool {
	return e == EstimationStoryPoints || e == EstimationIssueCount || e == EstimationTime
}

// Limits of a board.
const (
	MaxBoardsPerProject = 20
	MaxColumns          = 20
	MaxQuickFilters     = 20
	MaxSwimlanes        = 20
	MaxNameLength       = 80
	MaxFilterNameLength = 60
	MaxQueryLength      = 500
)

// CardFieldKeys are the fields a card can show.
var CardFieldKeys = []string{"key", "type", "priority", "assignee", "labels", "points", "epic", "subtasks", "flag", "estimate"}

// DefaultCardFields are the fields a new board shows.
var DefaultCardFields = []string{"key", "type", "priority", "assignee", "labels", "points"}

var columnColors = map[string]bool{"": true, "gray": true, "red": true, "orange": true, "amber": true, "green": true, "teal": true, "blue": true, "indigo": true, "purple": true, "pink": true}

// QuickFilter is a named query a user toggles on the board.
type QuickFilter struct {
	ID       uuid.UUID
	BoardID  uuid.UUID
	Name     string
	Query    string
	Position int
}

// Swimlane is a named query lane of a board in query swimlane mode.
type Swimlane struct {
	ID       uuid.UUID
	BoardID  uuid.UUID
	Name     string
	Query    string
	Position int
}

// Settings are the editable board settings.
type Settings struct {
	Name              string
	SwimlaneMode      SwimlaneMode
	Estimation        Estimation
	CardFields        []string
	ShowUnmappedLane  bool
	DoneRetentionDays *int
}

func invalid(base *apperror.Error, details ...apperror.FieldError) error {
	return apperror.WithCode(base.Kind, base.Code, base.Message).WithDetails(details...)
}

// ValidateSettings checks the settings and normalizes the name and the card fields.
func ValidateSettings(s *Settings) error {
	var problems []apperror.FieldError
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" || utf8.RuneCountInString(s.Name) > MaxNameLength {
		problems = append(problems, apperror.FieldError{Field: "name", Message: "el nombre es obligatorio y admite hasta 80 caracteres"})
	}
	if !s.SwimlaneMode.Valid() {
		problems = append(problems, apperror.FieldError{Field: "swimlane_mode", Message: "debe ser none, assignee, epic, query o story"})
	}
	if !s.Estimation.Valid() {
		problems = append(problems, apperror.FieldError{Field: "estimation", Message: "debe ser story_points, issue_count o time"})
	}
	if s.DoneRetentionDays != nil && (*s.DoneRetentionDays < 0 || *s.DoneRetentionDays > 3650) {
		problems = append(problems, apperror.FieldError{Field: "done_retention_days", Message: "debe estar entre 0 y 3650"})
	}
	known := map[string]bool{}
	for _, key := range CardFieldKeys {
		known[key] = true
	}
	seen := map[string]bool{}
	fields := make([]string, 0, len(s.CardFields))
	for _, key := range s.CardFields {
		if !known[key] {
			problems = append(problems, apperror.FieldError{Field: "card_fields", Message: "campo de tarjeta desconocido: " + key})
			continue
		}
		if !seen[key] {
			seen[key] = true
			fields = append(fields, key)
		}
	}
	s.CardFields = fields
	if len(problems) > 0 {
		return invalid(ErrInvalidSettings, problems...)
	}
	return nil
}

// ColumnSpec is the requested definition of a column in a replace-all.
type ColumnSpec struct {
	// ID is the id of an existing column to keep, or the nil uuid for a new one.
	ID        uuid.UUID
	Name      string
	MinIssues *int
	MaxIssues *int
	Color     string
	StatusIDs []uuid.UUID
}

// ValidateColumns checks a replace-all of the columns: at least one column, valid limits, and every status
// of the project workflow mapped at most once. Statuses mapped nowhere are shown in the implicit "unmapped"
// lane. workflow is the set of status ids of the project workflow.
func ValidateColumns(specs []ColumnSpec, workflow map[uuid.UUID]bool) error {
	var problems []apperror.FieldError
	if len(specs) == 0 {
		return invalid(ErrInvalidColumns, apperror.FieldError{Field: "columns", Message: "el tablero necesita al menos una columna"})
	}
	if len(specs) > MaxColumns {
		return invalid(ErrInvalidColumns, apperror.FieldError{Field: "columns", Message: "un tablero admite hasta 20 columnas"})
	}
	names := map[string]bool{}
	mapped := map[uuid.UUID]bool{}
	ids := map[uuid.UUID]bool{}
	for i := range specs {
		spec := &specs[i]
		path := "columns[" + strconv.Itoa(i) + "]"
		spec.Name = strings.TrimSpace(spec.Name)
		if spec.Name == "" || utf8.RuneCountInString(spec.Name) > MaxNameLength {
			problems = append(problems, apperror.FieldError{Field: path + ".name", Message: "el nombre es obligatorio y admite hasta 80 caracteres"})
		} else if key := strings.ToLower(spec.Name); names[key] {
			problems = append(problems, apperror.FieldError{Field: path + ".name", Message: "el nombre de la columna está repetido"})
		} else {
			names[key] = true
		}
		if spec.ID != uuid.Nil {
			if ids[spec.ID] {
				problems = append(problems, apperror.FieldError{Field: path + ".id", Message: "la columna está repetida"})
			}
			ids[spec.ID] = true
		}
		if spec.MinIssues != nil && *spec.MinIssues < 0 {
			problems = append(problems, apperror.FieldError{Field: path + ".min_issues", Message: "no puede ser negativo"})
		}
		if spec.MaxIssues != nil && *spec.MaxIssues < 1 {
			problems = append(problems, apperror.FieldError{Field: path + ".max_issues", Message: "debe ser al menos 1"})
		}
		if spec.MinIssues != nil && spec.MaxIssues != nil && *spec.MinIssues > *spec.MaxIssues {
			problems = append(problems, apperror.FieldError{Field: path + ".min_issues", Message: "no puede superar el máximo"})
		}
		if !columnColors[spec.Color] {
			problems = append(problems, apperror.FieldError{Field: path + ".color", Message: "color no válido"})
		}
		for _, statusID := range spec.StatusIDs {
			switch {
			case !workflow[statusID]:
				problems = append(problems, apperror.FieldError{Field: path + ".status_ids", Message: "el estado " + statusID.String() + " no pertenece al flujo del proyecto"})
			case mapped[statusID]:
				problems = append(problems, apperror.FieldError{Field: path + ".status_ids", Message: "el estado " + statusID.String() + " ya está en otra columna"})
			}
			mapped[statusID] = true
		}
	}
	if len(problems) > 0 {
		return invalid(ErrInvalidColumns, problems...)
	}
	return nil
}

// UnmappedStatuses returns the workflow statuses no column maps, in the order given.
func UnmappedStatuses(workflowOrder []uuid.UUID, columns []Column) []uuid.UUID {
	mapped := map[uuid.UUID]bool{}
	for _, column := range columns {
		for _, id := range column.StatusIDs {
			mapped[id] = true
		}
	}
	var out []uuid.UUID
	for _, id := range workflowOrder {
		if !mapped[id] {
			out = append(out, id)
		}
	}
	return out
}

// ValidateQueries checks the names and the query text of quick filters (kind "quick_filters") or swimlanes
// (kind "swimlanes") against the minimal query grammar.
func ValidateQueries(kind string, names, queries []string) error {
	var problems []apperror.FieldError
	limit := MaxQuickFilters
	if kind == "swimlanes" {
		limit = MaxSwimlanes
	}
	if len(names) > limit {
		return invalid(ErrInvalidQuery, apperror.FieldError{Field: kind, Message: "se superó el máximo de elementos"})
	}
	seen := map[string]bool{}
	for i := range names {
		path := kind + "[" + strconv.Itoa(i) + "]"
		name := strings.TrimSpace(names[i])
		if name == "" || utf8.RuneCountInString(name) > MaxFilterNameLength {
			problems = append(problems, apperror.FieldError{Field: path + ".name", Message: "el nombre es obligatorio y admite hasta 60 caracteres"})
		} else if seen[strings.ToLower(name)] {
			problems = append(problems, apperror.FieldError{Field: path + ".name", Message: "el nombre está repetido"})
		}
		seen[strings.ToLower(name)] = true
		query := strings.TrimSpace(queries[i])
		if utf8.RuneCountInString(query) > MaxQueryLength {
			problems = append(problems, apperror.FieldError{Field: path + ".query", Message: "la consulta admite hasta 500 caracteres"})
		} else if _, err := ParseQuery(query); err != nil || query == "" {
			message := "la consulta es obligatoria"
			if err != nil {
				message = err.Error()
			}
			problems = append(problems, apperror.FieldError{Field: path + ".query", Message: message})
		}
	}
	if len(problems) > 0 {
		return invalid(ErrInvalidQuery, problems...)
	}
	return nil
}
