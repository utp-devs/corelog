// Package domain holds the automation rule model: triggers, conditions, actions, validation, smart values and the
// pure matching and loop rules. A rule is stored as three JSON documents, so the types carry JSON tags.
package domain

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Limits and safety constants.
const (
	MaxNameLen      = 120
	MaxConditions   = 10
	MaxActions      = 10
	MaxTextLen      = 2000
	MaxChainDepth   = 5
	MaxRunsPerHour  = 100
	MinEveryMinutes = 5
	MaxEveryMinutes = 7 * 24 * 60
	// MaxScheduledIssues caps the issues one scheduled run touches.
	MaxScheduledIssues = 50
	maxSmartValueLen   = 500
)

// Trigger types.
const (
	TriggerIssueCreated      = "issue_created"
	TriggerIssueUpdated      = "issue_updated"
	TriggerIssueTransitioned = "issue_transitioned"
	TriggerCommentAdded      = "comment_added"
	TriggerSprintStarted     = "sprint_started"
	TriggerSprintCompleted   = "sprint_completed"
	TriggerScheduled         = "scheduled"
)

// Condition types.
const (
	ConditionQuery     = "query"
	ConditionFieldEq   = "field_equals"
	ConditionFieldIn   = "field_in"
	ConditionActorRole = "actor_role"
	ConditionIssueType = "issue_type"
)

// Action types.
const (
	ActionTransition       = "transition_issue"
	ActionAssign           = "assign_issue"
	ActionSetField         = "set_field"
	ActionAddComment       = "add_comment"
	ActionAddLabel         = "add_label"
	ActionRemoveLabel      = "remove_label"
	ActionCreateSubtask    = "create_subtask"
	ActionSendWebhook      = "send_webhook"
	ActionSendNotification = "send_notification"
)

// Assignment and recipient selectors.
const (
	WhoUser       = "user"
	WhoReporter   = "reporter"
	WhoAssignee   = "assignee"
	WhoLead       = "lead"
	WhoRoundRobin = "round_robin"
)

// Run statuses.
const (
	RunSuccess = "success"
	RunSkipped = "skipped"
	RunFailed  = "failed"
)

// Errors.
var (
	ErrNotFound        = apperror.WithCode(apperror.KindNotFound, "automation.rule_not_found", "Regla de automatizacion no encontrada")
	ErrVersionConflict = apperror.WithCode(apperror.KindConflict, "automation.version_conflict", "La regla cambio mientras la editabas; recarga e intenta de nuevo")
)

func invalid(field, msg string) error {
	return apperror.WithCode(apperror.KindValidation, "automation.invalid_rule", "Regla no valida: "+msg).
		WithDetails(apperror.FieldError{Field: field, Message: msg})
}

// Trigger is the "when" part of a rule.
type Trigger struct {
	Type string `json:"type"`
	// Field restricts issue_updated to one changed field (summary, description, priority, assignee, labels, ...).
	Field string `json:"field,omitempty"`
	// From and To restrict issue_transitioned to status names.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// EveryMinutes and Query configure scheduled rules: the actions run for each issue the CoreQL query matches.
	EveryMinutes int    `json:"every_minutes,omitempty"`
	Query        string `json:"query,omitempty"`
}

// Condition is one "if" check; all conditions of a rule must hold.
type Condition struct {
	Type   string   `json:"type"`
	Query  string   `json:"query,omitempty"`
	Field  string   `json:"field,omitempty"`
	Value  string   `json:"value,omitempty"`
	Values []string `json:"values,omitempty"`
	Role   string   `json:"role,omitempty"`
}

// Action is one "then" step.
type Action struct {
	Type string `json:"type"`
	// Status is the target status name of transition_issue.
	Status string `json:"status,omitempty"`
	// Who selects the user of assign_issue (user, reporter, lead, round_robin) and the recipient of
	// send_notification (user, assignee, reporter, lead).
	Who    string `json:"who,omitempty"`
	UserID string `json:"user_id,omitempty"`
	TeamID string `json:"team_id,omitempty"`
	Field  string `json:"field,omitempty"`
	Value  string `json:"value,omitempty"`
	// Text is the comment, subtask summary or notification message; it may use smart values.
	Text  string `json:"text,omitempty"`
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Rule is a stored automation rule.
type Rule struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	ProjectID  *uuid.UUID
	Name       string
	Enabled    bool
	Trigger    Trigger
	Conditions []Condition
	Actions    []Action
	CreatedBy  *uuid.UUID
	Version    int
	RunCount   int64
	LastRunAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Run is one execution record of a rule.
type Run struct {
	ID        uuid.UUID
	RuleID    uuid.UUID
	EventID   *int64
	IssueID   *uuid.UUID
	Status    string
	Detail    string
	CreatedAt time.Time
}

// Smart values: the closed set of placeholders texts may use.
var smartValues = map[string]bool{
	"issue.key": true, "issue.summary": true, "issue.status": true, "issue.priority": true, "issue.type": true,
	"actor.name": true, "project.key": true, "project.name": true, "trigger.from": true, "trigger.to": true,
	"rule.name": true,
}

var placeholder = regexp.MustCompile(`\{\{\s*([a-zA-Z_.]+)\s*\}\}`)

// SmartValueNames returns the allowed placeholder names.
func SmartValueNames() []string {
	out := make([]string, 0, len(smartValues))
	for k := range smartValues {
		out = append(out, k)
	}
	return out
}

// checkTemplate rejects unknown placeholders, stray braces and over-long texts.
func checkTemplate(field, text string, required bool) error {
	if required && strings.TrimSpace(text) == "" {
		return invalid(field, "el texto es obligatorio")
	}
	if utf8.RuneCountInString(text) > MaxTextLen {
		return invalid(field, "el texto admite hasta 2000 caracteres")
	}
	for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
		if !smartValues[m[1]] {
			return invalid(field, "valor inteligente desconocido: {{"+m[1]+"}}")
		}
	}
	rest := placeholder.ReplaceAllString(text, "")
	if strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
		return invalid(field, "llaves dobles sin cerrar o sin valor inteligente valido")
	}
	return nil
}

// Render substitutes smart values with the given facts. Values are HTML-escaped and trimmed; the result is not
// evaluated or expanded again, so a value that contains "{{...}}" stays literal.
func Render(text string, facts map[string]string) string {
	return placeholder.ReplaceAllStringFunc(text, func(m string) string {
		name := placeholder.FindStringSubmatch(m)[1]
		v, ok := facts[name]
		if !ok || !smartValues[name] {
			return ""
		}
		if utf8.RuneCountInString(v) > maxSmartValueLen {
			v = string([]rune(v)[:maxSmartValueLen])
		}
		return html.EscapeString(v)
	})
}

// Validate checks the whole definition of a rule and returns the first problem.
func (r Rule) Validate() error {
	name := strings.TrimSpace(r.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return invalid("name", "el nombre es obligatorio y admite hasta 120 caracteres")
	}
	if err := r.Trigger.Validate(); err != nil {
		return err
	}
	if len(r.Conditions) > MaxConditions {
		return invalid("conditions", "se admiten hasta 10 condiciones")
	}
	for i, c := range r.Conditions {
		if err := c.Validate(fmt.Sprintf("conditions[%d]", i)); err != nil {
			return err
		}
	}
	if len(r.Actions) == 0 || len(r.Actions) > MaxActions {
		return invalid("actions", "indica entre 1 y 10 acciones")
	}
	for i, a := range r.Actions {
		if err := a.Validate(fmt.Sprintf("actions[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks a trigger.
func (t Trigger) Validate() error {
	switch t.Type {
	case TriggerIssueCreated, TriggerCommentAdded, TriggerSprintStarted, TriggerSprintCompleted:
	case TriggerIssueUpdated:
	case TriggerIssueTransitioned:
	case TriggerScheduled:
		if t.EveryMinutes < MinEveryMinutes || t.EveryMinutes > MaxEveryMinutes {
			return invalid("trigger.every_minutes", "la frecuencia debe estar entre 5 minutos y 7 dias")
		}
		if strings.TrimSpace(t.Query) == "" {
			return invalid("trigger.query", "la consulta CoreQL es obligatoria en una regla programada")
		}
	default:
		return invalid("trigger.type", "disparador desconocido")
	}
	for _, v := range []string{t.Field, t.From, t.To, t.Query} {
		if utf8.RuneCountInString(v) > MaxTextLen {
			return invalid("trigger", "un valor del disparador es demasiado largo")
		}
	}
	return nil
}

// Validate checks a condition.
func (c Condition) Validate(field string) error {
	switch c.Type {
	case ConditionQuery:
		if strings.TrimSpace(c.Query) == "" || utf8.RuneCountInString(c.Query) > MaxTextLen {
			return invalid(field+".query", "la consulta CoreQL es obligatoria")
		}
	case ConditionFieldEq:
		if !comparableField(c.Field) || c.Value == "" {
			return invalid(field, "indica un campo comparable y un valor")
		}
	case ConditionFieldIn:
		if !comparableField(c.Field) || len(c.Values) == 0 || len(c.Values) > 20 {
			return invalid(field, "indica un campo comparable y entre 1 y 20 valores")
		}
	case ConditionActorRole:
		switch c.Role {
		case "lead", "contributor", "viewer", "org_admin":
		default:
			return invalid(field+".role", "rol desconocido")
		}
	case ConditionIssueType:
		if len(c.Values) == 0 || len(c.Values) > 10 {
			return invalid(field+".values", "indica entre 1 y 10 tipos de incidencia")
		}
	default:
		return invalid(field+".type", "condicion desconocida")
	}
	return nil
}

var comparableFields = map[string]bool{"status": true, "priority": true, "type": true, "summary": true, "assignee": true, "reporter": true, "status_category": true}

func comparableField(f string) bool { return comparableFields[f] }

// settableFields are the system fields set_field can write; custom fields use "custom:<field id>".
var settableFields = map[string]bool{"summary": true, "description": true, "priority": true, "story_points": true, "due_date": true}

// Validate checks an action.
func (a Action) Validate(field string) error {
	switch a.Type {
	case ActionTransition:
		if strings.TrimSpace(a.Status) == "" {
			return invalid(field+".status", "indica el estado de destino")
		}
	case ActionAssign:
		switch a.Who {
		case WhoReporter, WhoLead:
		case WhoUser:
			if _, err := uuid.Parse(a.UserID); err != nil {
				return invalid(field+".user_id", "indica un usuario valido")
			}
		case WhoRoundRobin:
			if _, err := uuid.Parse(a.TeamID); err != nil {
				return invalid(field+".team_id", "indica el equipo del reparto")
			}
		default:
			return invalid(field+".who", "asignacion desconocida")
		}
	case ActionSetField:
		if !settableFields[a.Field] && !strings.HasPrefix(a.Field, "custom:") {
			return invalid(field+".field", "campo no editable por automatizacion")
		}
		if strings.HasPrefix(a.Field, "custom:") {
			if _, err := uuid.Parse(strings.TrimPrefix(a.Field, "custom:")); err != nil {
				return invalid(field+".field", "identificador de campo personalizado no valido")
			}
		}
		return checkTemplate(field+".value", a.Value, false)
	case ActionAddComment, ActionCreateSubtask:
		return checkTemplate(field+".text", a.Text, true)
	case ActionAddLabel, ActionRemoveLabel:
		l := strings.TrimSpace(a.Label)
		if l == "" || utf8.RuneCountInString(l) > 50 {
			return invalid(field+".label", "la etiqueta es obligatoria (50 caracteres como maximo)")
		}
	case ActionSendWebhook:
		u, err := url.Parse(strings.TrimSpace(a.URL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return invalid(field+".url", "la URL debe ser http o https")
		}
	case ActionSendNotification:
		switch a.Who {
		case WhoAssignee, WhoReporter, WhoLead:
		case WhoUser:
			if _, err := uuid.Parse(a.UserID); err != nil {
				return invalid(field+".user_id", "indica un usuario valido")
			}
		default:
			return invalid(field+".who", "destinatario desconocido")
		}
		return checkTemplate(field+".text", a.Text, true)
	default:
		return invalid(field+".type", "accion desconocida")
	}
	return nil
}

// Facts describes an event for trigger matching.
type Facts struct {
	// Type is the domain event type (issue.created, issue.updated, ...).
	Type string
	// ChangedFields are the fields of an issue.updated event.
	ChangedFields []string
	// From and To are the status names of an issue.transitioned event.
	From, To string
}

// TriggerFor maps a domain event type to the trigger type it fires.
func TriggerFor(eventType string) (string, bool) {
	switch eventType {
	case "issue.created":
		return TriggerIssueCreated, true
	case "issue.updated":
		return TriggerIssueUpdated, true
	case "issue.transitioned":
		return TriggerIssueTransitioned, true
	case "comment.created":
		return TriggerCommentAdded, true
	case "sprint.started":
		return TriggerSprintStarted, true
	case "sprint.completed":
		return TriggerSprintCompleted, true
	}
	return "", false
}

// Matches reports whether the trigger fires for the event facts.
func (t Trigger) Matches(f Facts) bool {
	kind, ok := TriggerFor(f.Type)
	if !ok || kind != t.Type {
		return false
	}
	switch t.Type {
	case TriggerIssueUpdated:
		if t.Field == "" {
			return true
		}
		for _, c := range f.ChangedFields {
			if strings.EqualFold(c, t.Field) {
				return true
			}
		}
		return false
	case TriggerIssueTransitioned:
		return (t.From == "" || strings.EqualFold(t.From, f.From)) && (t.To == "" || strings.EqualFold(t.To, f.To))
	}
	return true
}

// Snapshot is the issue data conditions and smart values read.
type Snapshot struct {
	IssueID        uuid.UUID
	Key            string
	Summary        string
	Status         string
	StatusCategory string
	Priority       string
	IssueType      string
	AssigneeID     *uuid.UUID
	ReporterID     uuid.UUID
	ProjectKey     string
	ProjectName    string
	ProjectID      uuid.UUID
	Labels         []string
}

// Field returns the comparable value of a field.
func (s Snapshot) Field(name string) string {
	switch name {
	case "status":
		return s.Status
	case "status_category":
		return s.StatusCategory
	case "priority":
		return s.Priority
	case "type":
		return s.IssueType
	case "summary":
		return s.Summary
	case "reporter":
		return s.ReporterID.String()
	case "assignee":
		if s.AssigneeID == nil {
			return ""
		}
		return s.AssigneeID.String()
	}
	return ""
}

// Holds evaluates the static (non-query, non-role) conditions against the snapshot. Query and actor role
// conditions need ports and are evaluated by the engine.
func (c Condition) Holds(s Snapshot) bool {
	switch c.Type {
	case ConditionFieldEq:
		return strings.EqualFold(s.Field(c.Field), c.Value)
	case ConditionFieldIn:
		v := s.Field(c.Field)
		for _, want := range c.Values {
			if strings.EqualFold(v, want) {
				return true
			}
		}
		return false
	case ConditionIssueType:
		for _, want := range c.Values {
			if strings.EqualFold(s.IssueType, want) {
				return true
			}
		}
		return false
	}
	return false
}

// SmartFacts builds the placeholder values for rendering.
func SmartFacts(s Snapshot, actorName, ruleName string, f Facts) map[string]string {
	return map[string]string{
		"issue.key": s.Key, "issue.summary": s.Summary, "issue.status": s.Status, "issue.priority": s.Priority,
		"issue.type": s.IssueType, "actor.name": actorName, "project.key": s.ProjectKey, "project.name": s.ProjectName,
		"trigger.from": f.From, "trigger.to": f.To, "rule.name": ruleName,
	}
}

// Loop decides whether an event must be skipped for a rule. originRule is the automation rule that produced
// the event (uuid.Nil for human events) and depth the number of automation hops so far.
func Loop(ruleID, originRule uuid.UUID, depth int) (skip bool, reason string) {
	if depth >= MaxChainDepth {
		return true, "se alcanzo la profundidad maxima de encadenamiento"
	}
	if originRule != uuid.Nil && originRule == ruleID {
		return true, "el evento fue generado por esta misma regla"
	}
	return false, ""
}
