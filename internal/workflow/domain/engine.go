package domain

import (
	"sort"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// ResolutionAction says what a transition does to the resolution.
type ResolutionAction string

const (
	ResolutionNone  ResolutionAction = ""
	ResolutionSet   ResolutionAction = "set"
	ResolutionClear ResolutionAction = "clear"
)

// ResolutionChange is the planned resolution change. UseDefaultDone means "the organization's
// Done resolution" and the caller resolves the id.
type ResolutionChange struct {
	Action         ResolutionAction
	ID             string
	UseDefaultDone bool
}

// AssigneeChange is the planned assignee change; an empty UserID with Set unassigns.
type AssigneeChange struct {
	Set    bool
	UserID string
}

// FieldSet is one planned field write.
type FieldSet struct {
	Field string
	Value string
}

// Result is the plan produced by Evaluate. The caller applies it in one transaction.
type Result struct {
	TransitionID string
	FromStatusID string
	ToStatusID   string
	Resolution   ResolutionChange
	Assignee     AssigneeChange
	FieldSets    []FieldSet
	// Comment is the user's comment from the input (may be empty).
	Comment string
	// Comments are automatic comments added by post-functions.
	Comments []string
	Events   []string
}

func (r *Result) setField(field, value string) {
	for i := range r.FieldSets {
		if r.FieldSets[i].Field == field {
			r.FieldSets[i].Value = value
			return
		}
	}
	r.FieldSets = append(r.FieldSets, FieldSet{Field: field, Value: value})
}

func conditionsAllow(t Transition, actor ActorContext, issue IssueSnapshot) bool {
	for _, c := range t.Conditions {
		if !c.Allows(actor, issue) {
			return false
		}
	}
	return true
}

// screenFields lists the fields the transition asks for: RequiredFields plus required_field
// validators, deduplicated and in declaration order.
func screenFields(t Transition) []string {
	var out []string
	seen := map[string]bool{}
	add := func(f string) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	for _, f := range t.RequiredFields {
		add(f)
	}
	for _, v := range t.Validators {
		if rf, ok := v.(RequiredField); ok {
			add(rf.Field)
		}
	}
	return out
}

// Evaluate checks that the actor may run the transition on the issue and that the input
// satisfies it, and returns the planned changes. It never mutates its arguments.
//
// Errors: workflow.transition_not_allowed (unknown transition, wrong source status or failed
// condition) and workflow.validation_failed with one detail per failure.
func Evaluate(w Workflow, issue IssueSnapshot, actor ActorContext, transitionID string, input TransitionInput) (Result, error) {
	t, ok := w.Transition(transitionID)
	if !ok {
		return Result{}, notAllowed("la transición no existe en este flujo de trabajo")
	}
	if _, ok := w.Status(issue.StatusID); !ok || !t.AppliesFrom(issue.StatusID) {
		return Result{}, notAllowed("la transición no está disponible desde el estado actual")
	}
	if !conditionsAllow(t, actor, issue) {
		return Result{}, notAllowed("no tienes permiso para ejecutar esta transición")
	}
	target, ok := w.Status(t.To)
	if !ok {
		return Result{}, notAllowed("el estado de destino no existe en este flujo de trabajo")
	}

	var failures []apperror.FieldError
	for _, field := range t.RequiredFields {
		failures = append(failures, checkFieldPresent(field, issue, input)...)
	}
	for _, v := range t.Validators {
		failures = append(failures, v.Check(issue, input)...)
	}
	failures = dedupeFailures(failures)
	if len(failures) > 0 {
		return Result{}, apperror.WithCode(apperror.KindValidation, CodeValidationFailed, ErrValidationFailed.Message).WithDetails(failures...)
	}

	plan := Result{TransitionID: t.ID, FromStatusID: issue.StatusID, ToStatusID: t.To, Comment: input.Comment}

	// Values typed on the transition screen come first; post-functions may overwrite them.
	screen := screenFields(t)
	sort.Strings(screen)
	for _, field := range screen {
		if value, ok := input.Fields[field]; ok {
			plan.setField(field, value)
		}
	}
	if input.AssigneeID != "" {
		plan.Assignee = AssigneeChange{Set: true, UserID: input.AssigneeID}
	}
	for _, p := range t.PostFunctions {
		p.Apply(&plan, actor, issue)
	}

	switch {
	case input.ResolutionID != "":
		plan.Resolution = ResolutionChange{Action: ResolutionSet, ID: input.ResolutionID}
	case plan.Resolution.Action != ResolutionNone:
		// Set explicitly by a post-function.
	case target.Category == CategoryDone && issue.ResolutionID == "":
		plan.Resolution = ResolutionChange{Action: ResolutionSet, UseDefaultDone: true}
	case target.Category != CategoryDone && issue.ResolutionID != "":
		plan.Resolution = ResolutionChange{Action: ResolutionClear}
	}
	return plan, nil
}

func dedupeFailures(in []apperror.FieldError) []apperror.FieldError {
	seen := map[apperror.FieldError]bool{}
	var out []apperror.FieldError
	for _, f := range in {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// AvailableTransition is a transition the actor may run from the current status.
type AvailableTransition struct {
	TransitionID       string
	Name               string
	ToStatusID         string
	ToStatusName       string
	ToCategory         Category
	RequiredFields     []string
	RequiresComment    bool
	RequiresResolution bool
	RequiresAssignee   bool
}

// NeedsScreen reports whether the UI must ask for input before running the transition.
func (a AvailableTransition) NeedsScreen() bool {
	return len(a.RequiredFields) > 0 || a.RequiresComment || a.RequiresResolution || a.RequiresAssignee
}

// AvailableTransitions returns, in workflow order, the transitions that start at the issue's
// status and whose conditions the actor satisfies.
func AvailableTransitions(w Workflow, issue IssueSnapshot, actor ActorContext) []AvailableTransition {
	out := []AvailableTransition{}
	if _, ok := w.Status(issue.StatusID); !ok {
		return out
	}
	for _, t := range w.Transitions {
		if !t.AppliesFrom(issue.StatusID) || !conditionsAllow(t, actor, issue) {
			continue
		}
		target, ok := w.Status(t.To)
		if !ok {
			continue
		}
		item := AvailableTransition{
			TransitionID: t.ID, Name: t.Name, ToStatusID: t.To, ToStatusName: target.Name, ToCategory: target.Category,
			RequiredFields: append([]string{}, screenFields(t)...),
		}
		for _, v := range t.Validators {
			switch v.(type) {
			case CommentRequired:
				item.RequiresComment = true
			case ResolutionRequired:
				item.RequiresResolution = true
			case AssigneeSet:
				item.RequiresAssignee = true
			}
		}
		out = append(out, item)
	}
	return out
}
