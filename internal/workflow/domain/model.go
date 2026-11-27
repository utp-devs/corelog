package domain

import (
	"regexp"
	"strings"
)

// Category groups statuses for reporting and board logic.
type Category string

const (
	CategoryTodo       Category = "todo"
	CategoryInProgress Category = "in_progress"
	CategoryDone       Category = "done"
)

// IsValid reports whether c is a known category.
func (c Category) IsValid() bool {
	return c == CategoryTodo || c == CategoryInProgress || c == CategoryDone
}

// Project roles a condition can test.
const (
	RoleLead        = "lead"
	RoleContributor = "contributor"
	RoleViewer      = "viewer"
)

// Status is one node of a workflow.
type Status struct {
	ID       string
	Name     string
	Category Category
}

// Transition is one edge of a workflow. An empty From means "from any status" (every status
// except To).
type Transition struct {
	ID             string
	Name           string
	From           []string
	To             string
	Conditions     []Condition
	Validators     []Validator
	PostFunctions  []PostFunction
	RequiredFields []string
}

// AppliesFrom reports whether the transition can start from statusID.
func (t Transition) AppliesFrom(statusID string) bool {
	if len(t.From) == 0 {
		return statusID != t.To
	}
	for _, from := range t.From {
		if from == statusID {
			return true
		}
	}
	return false
}

// Workflow is the set of statuses and transitions of one project.
type Workflow struct {
	Statuses    []Status
	Initial     string
	Transitions []Transition
}

// Status returns the status with the given id.
func (w Workflow) Status(id string) (Status, bool) {
	for _, status := range w.Statuses {
		if status.ID == id {
			return status, true
		}
	}
	return Status{}, false
}

// Transition returns the transition with the given id.
func (w Workflow) Transition(id string) (Transition, bool) {
	for _, transition := range w.Transitions {
		if transition.ID == id {
			return transition, true
		}
	}
	return Transition{}, false
}

// ActorContext describes who runs a transition.
type ActorContext struct {
	UserID      string
	Role        string
	LeadID      string
	Permissions []string
}

// HasPermission reports whether the actor holds the permission.
func (a ActorContext) HasPermission(permission string) bool {
	for _, held := range a.Permissions {
		if held == permission {
			return true
		}
	}
	return false
}

// IssueSnapshot is the read-only view of an issue the engine needs.
type IssueSnapshot struct {
	StatusID     string
	AssigneeID   string
	ReporterID   string
	ResolutionID string
	// OpenSubtasks counts children that are not in a done status.
	OpenSubtasks int
	// Fields holds custom or standard field values by key.
	Fields map[string]string
}

// TransitionInput is what the caller supplies when running a transition.
type TransitionInput struct {
	Fields       map[string]string
	Comment      string
	ResolutionID string
	AssigneeID   string
}

var fieldNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// reservedFields have dedicated rules and cannot be used as generic field names.
var reservedFields = map[string]bool{"status": true, "assignee": true, "reporter": true, "resolution": true}

func validFieldName(name string) bool {
	return fieldNamePattern.MatchString(name) && !reservedFields[name]
}

func normalizeName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
