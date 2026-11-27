package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Rule is the common shape of conditions, validators and post-functions.
type Rule interface {
	// Type is the stable tag used in JSON.
	Type() string
	// Validate checks the parameters of the rule.
	Validate() error
}

// Condition decides whether an actor may see and run a transition.
type Condition interface {
	Rule
	Allows(actor ActorContext, issue IssueSnapshot) bool
}

// Validator checks the transition input; it returns one failure per problem.
type Validator interface {
	Rule
	Check(issue IssueSnapshot, input TransitionInput) []apperror.FieldError
}

// PostFunction contributes planned changes to the transition result.
type PostFunction interface {
	Rule
	Apply(plan *Result, actor ActorContext, issue IssueSnapshot)
}

// ---- Conditions ----

// UserInRole allows actors whose project role is one of Roles.
type UserInRole struct {
	Roles []string `json:"roles"`
}

func (UserInRole) Type() string { return "user_in_role" }
func (c UserInRole) Validate() error {
	if len(c.Roles) == 0 {
		return errors.New("indica al menos un rol")
	}
	for _, role := range c.Roles {
		if role != RoleLead && role != RoleContributor && role != RoleViewer {
			return fmt.Errorf("rol desconocido %q", role)
		}
	}
	return nil
}
func (c UserInRole) Allows(actor ActorContext, _ IssueSnapshot) bool {
	for _, role := range c.Roles {
		if role == actor.Role {
			return true
		}
	}
	return false
}

// IsAssignee allows only the current assignee.
type IsAssignee struct{}

func (IsAssignee) Type() string    { return "is_assignee" }
func (IsAssignee) Validate() error { return nil }
func (IsAssignee) Allows(actor ActorContext, issue IssueSnapshot) bool {
	return actor.UserID != "" && actor.UserID == issue.AssigneeID
}

// IsReporter allows only the reporter.
type IsReporter struct{}

func (IsReporter) Type() string    { return "is_reporter" }
func (IsReporter) Validate() error { return nil }
func (IsReporter) Allows(actor ActorContext, issue IssueSnapshot) bool {
	return actor.UserID != "" && actor.UserID == issue.ReporterID
}

var permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Permission allows actors holding Permission.
type Permission struct {
	Permission string `json:"permission"`
}

func (Permission) Type() string { return "permission" }
func (c Permission) Validate() error {
	if !permissionPattern.MatchString(c.Permission) {
		return errors.New("el permiso no es válido")
	}
	return nil
}
func (c Permission) Allows(actor ActorContext, _ IssueSnapshot) bool {
	return actor.HasPermission(c.Permission)
}

// FieldEquals allows the transition only when the issue field equals Value.
type FieldEquals struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

func (FieldEquals) Type() string { return "field_equals" }
func (c FieldEquals) Validate() error {
	if !validFieldName(c.Field) {
		return errors.New("el nombre del campo no es válido")
	}
	return nil
}
func (c FieldEquals) Allows(_ ActorContext, issue IssueSnapshot) bool {
	value, ok := issue.Fields[c.Field]
	return ok && value == c.Value
}

// ---- Validators ----

// RequiredField requires a non-empty value for Field, from the input or the issue.
type RequiredField struct {
	Field string `json:"field"`
}

func (RequiredField) Type() string { return "required_field" }
func (v RequiredField) Validate() error {
	if !validFieldName(v.Field) {
		return errors.New("el nombre del campo no es válido")
	}
	return nil
}
func (v RequiredField) Check(issue IssueSnapshot, input TransitionInput) []apperror.FieldError {
	return checkFieldPresent(v.Field, issue, input)
}

func checkFieldPresent(field string, issue IssueSnapshot, input TransitionInput) []apperror.FieldError {
	value, ok := input.Fields[field]
	if !ok {
		value = issue.Fields[field]
	}
	if strings.TrimSpace(value) != "" {
		return nil
	}
	return []apperror.FieldError{{Field: "fields." + field, Message: "el campo es obligatorio para esta transición"}}
}

// CommentRequired requires a non-blank comment in the input.
type CommentRequired struct{}

func (CommentRequired) Type() string    { return "comment_required" }
func (CommentRequired) Validate() error { return nil }
func (CommentRequired) Check(_ IssueSnapshot, input TransitionInput) []apperror.FieldError {
	if strings.TrimSpace(input.Comment) != "" {
		return nil
	}
	return []apperror.FieldError{{Field: "comment", Message: "el comentario es obligatorio para esta transición"}}
}

// ResolutionRequired requires a resolution in the input or already on the issue.
type ResolutionRequired struct{}

func (ResolutionRequired) Type() string    { return "resolution_required" }
func (ResolutionRequired) Validate() error { return nil }
func (ResolutionRequired) Check(issue IssueSnapshot, input TransitionInput) []apperror.FieldError {
	if input.ResolutionID != "" || issue.ResolutionID != "" {
		return nil
	}
	return []apperror.FieldError{{Field: "resolution_id", Message: "la resolución es obligatoria para esta transición"}}
}

// SubtasksDone requires every subtask to be in a done status.
type SubtasksDone struct{}

func (SubtasksDone) Type() string    { return "subtasks_done" }
func (SubtasksDone) Validate() error { return nil }
func (SubtasksDone) Check(issue IssueSnapshot, _ TransitionInput) []apperror.FieldError {
	if issue.OpenSubtasks == 0 {
		return nil
	}
	return []apperror.FieldError{{Field: "subtasks", Message: "todas las subtareas deben estar completadas"}}
}

// AssigneeSet requires an assignee on the issue or in the input.
type AssigneeSet struct{}

func (AssigneeSet) Type() string    { return "assignee_set" }
func (AssigneeSet) Validate() error { return nil }
func (AssigneeSet) Check(issue IssueSnapshot, input TransitionInput) []apperror.FieldError {
	if issue.AssigneeID != "" || input.AssigneeID != "" {
		return nil
	}
	return []apperror.FieldError{{Field: "assignee_id", Message: "la incidencia debe tener un responsable"}}
}

// ---- Post-functions ----

// DoneResolution is the SetResolution value that means "the organization's Done resolution".
const DoneResolution = "done"

// Assignment targets of the Assign post-function.
const (
	AssignReporter    = "reporter"
	AssignCurrentUser = "current_user"
	AssignLead        = "lead"
	AssignUnassigned  = "unassigned"
)

// SetResolution sets the resolution to a catalog id, or to the default Done resolution.
type SetResolution struct {
	Resolution string `json:"resolution"`
}

func (SetResolution) Type() string { return "set_resolution" }
func (p SetResolution) Validate() error {
	if strings.TrimSpace(p.Resolution) == "" {
		return errors.New("indica una resolución o \"done\"")
	}
	return nil
}
func (p SetResolution) Apply(plan *Result, _ ActorContext, _ IssueSnapshot) {
	if p.Resolution == DoneResolution {
		plan.Resolution = ResolutionChange{Action: ResolutionSet, UseDefaultDone: true}
		return
	}
	plan.Resolution = ResolutionChange{Action: ResolutionSet, ID: p.Resolution}
}

// ClearResolution removes the resolution.
type ClearResolution struct{}

func (ClearResolution) Type() string    { return "clear_resolution" }
func (ClearResolution) Validate() error { return nil }
func (ClearResolution) Apply(plan *Result, _ ActorContext, _ IssueSnapshot) {
	plan.Resolution = ResolutionChange{Action: ResolutionClear}
}

// Assign changes the assignee. Targets that cannot be resolved (for example a missing
// reporter) leave the assignee untouched.
type Assign struct {
	To string `json:"to"`
}

func (Assign) Type() string { return "assign" }
func (p Assign) Validate() error {
	switch p.To {
	case AssignReporter, AssignCurrentUser, AssignLead, AssignUnassigned:
		return nil
	}
	return fmt.Errorf("destino de asignación desconocido %q", p.To)
}
func (p Assign) Apply(plan *Result, actor ActorContext, issue IssueSnapshot) {
	var user string
	switch p.To {
	case AssignReporter:
		user = issue.ReporterID
	case AssignCurrentUser:
		user = actor.UserID
	case AssignLead:
		user = actor.LeadID
	case AssignUnassigned:
		plan.Assignee = AssigneeChange{Set: true}
		return
	}
	if user != "" {
		plan.Assignee = AssigneeChange{Set: true, UserID: user}
	}
}

// SetField sets a field to a fixed value.
type SetField struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

func (SetField) Type() string { return "set_field" }
func (p SetField) Validate() error {
	if !validFieldName(p.Field) {
		return errors.New("el nombre del campo no es válido")
	}
	return nil
}
func (p SetField) Apply(plan *Result, _ ActorContext, _ IssueSnapshot) {
	plan.setField(p.Field, p.Value)
}

// MaxCommentLength bounds the text of AddComment.
const MaxCommentLength = 2000

// AddComment appends an automatic comment.
type AddComment struct {
	Text string `json:"text"`
}

func (AddComment) Type() string { return "add_comment" }
func (p AddComment) Validate() error {
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return errors.New("el texto del comentario es obligatorio")
	}
	if len([]rune(text)) > MaxCommentLength {
		return fmt.Errorf("el comentario supera los %d caracteres", MaxCommentLength)
	}
	return nil
}
func (p AddComment) Apply(plan *Result, _ ActorContext, _ IssueSnapshot) {
	plan.Comments = append(plan.Comments, p.Text)
}

var eventPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

// Emit publishes a named domain event.
type Emit struct {
	Event string `json:"event"`
}

func (Emit) Type() string { return "emit" }
func (p Emit) Validate() error {
	if !eventPattern.MatchString(p.Event) {
		return errors.New("el nombre del evento no es válido")
	}
	return nil
}
func (p Emit) Apply(plan *Result, _ ActorContext, _ IssueSnapshot) {
	plan.Events = append(plan.Events, p.Event)
}
