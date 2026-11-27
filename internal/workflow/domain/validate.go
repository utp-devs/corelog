package domain

import (
	"bytes"
	"fmt"
)

// expandFrom returns the source status ids of t that exist in the workflow.
func (w Workflow) expandFrom(t Transition) []string {
	var out []string
	if len(t.From) == 0 {
		for _, s := range w.Statuses {
			if s.ID != t.To {
				out = append(out, s.ID)
			}
		}
		return out
	}
	for _, id := range t.From {
		if _, ok := w.Status(id); ok {
			out = append(out, id)
		}
	}
	return out
}

// Validate returns every structural problem of the workflow, with JSON-path locations.
// An empty result means the workflow is valid.
func (w Workflow) Validate() []Issue {
	var issues []Issue
	add := func(path, code, msg string) { issues = append(issues, Issue{Path: path, Code: code, Message: msg}) }

	ids := map[string]bool{}
	names := map[string]bool{}
	hasDone := false
	for i, s := range w.Statuses {
		path := fmt.Sprintf("statuses[%d]", i)
		switch {
		case s.ID == "":
			add(path+".id", IssueRequired, "el identificador del estado es obligatorio")
		case ids[s.ID]:
			add(path+".id", IssueDuplicateID, "el identificador del estado está repetido")
		}
		ids[s.ID] = true
		key := normalizeName(s.Name)
		switch {
		case key == "":
			add(path+".name", IssueRequired, "el nombre del estado es obligatorio")
		case names[key]:
			add(path+".name", IssueDuplicateStatusName, "ya existe un estado con ese nombre")
		}
		names[key] = true
		if !s.Category.IsValid() {
			add(path+".category", IssueInvalidCategory, "la categoría del estado no es válida")
		}
		if s.Category == CategoryDone {
			hasDone = true
		}
	}
	if !hasDone {
		add("statuses", IssueNoDoneStatus, "el flujo debe tener al menos un estado de categoría \"done\"")
	}

	initialOK := false
	switch {
	case w.Initial == "":
		add("initial", IssueNoInitialStatus, "el flujo debe tener exactamente un estado inicial")
	case !ids[w.Initial]:
		add("initial", IssueUnknownInitialStatus, "el estado inicial no existe en el flujo")
	default:
		initialOK = true
	}

	transitionIDs := map[string]bool{}
	perSource := map[string]map[string]bool{}
	adjacency := map[string][]string{}
	for i, t := range w.Transitions {
		path := fmt.Sprintf("transitions[%d]", i)
		switch {
		case t.ID == "":
			add(path+".id", IssueRequired, "el identificador de la transición es obligatorio")
		case transitionIDs[t.ID]:
			add(path+".id", IssueDuplicateID, "el identificador de la transición está repetido")
		}
		transitionIDs[t.ID] = true
		name := normalizeName(t.Name)
		if name == "" {
			add(path+".name", IssueRequired, "el nombre de la transición es obligatorio")
		}
		toOK := ids[t.To]
		if !toOK {
			add(path+".to", IssueUnknownStatus, "el estado de destino no existe en el flujo")
		}
		for j, from := range t.From {
			switch {
			case !ids[from]:
				add(fmt.Sprintf("%s.from[%d]", path, j), IssueUnknownStatus, "el estado de origen no existe en el flujo")
			case from == t.To:
				add(fmt.Sprintf("%s.from[%d]", path, j), IssueSelfTransition, "una transición no puede ir de un estado a sí mismo")
			}
		}
		for j, f := range t.RequiredFields {
			if !validFieldName(f) {
				add(fmt.Sprintf("%s.required_fields[%d]", path, j), IssueInvalidField, "el nombre del campo no es válido")
			}
		}
		for j, c := range t.Conditions {
			if c == nil {
				add(fmt.Sprintf("%s.conditions[%d]", path, j), IssueInvalidRule, "la condición está vacía")
			} else if err := c.Validate(); err != nil {
				add(fmt.Sprintf("%s.conditions[%d]", path, j), IssueInvalidRule, c.Type()+": "+err.Error())
			}
		}
		for j, v := range t.Validators {
			if v == nil {
				add(fmt.Sprintf("%s.validators[%d]", path, j), IssueInvalidRule, "el validador está vacío")
			} else if err := v.Validate(); err != nil {
				add(fmt.Sprintf("%s.validators[%d]", path, j), IssueInvalidRule, v.Type()+": "+err.Error())
			}
		}
		for j, p := range t.PostFunctions {
			if p == nil {
				add(fmt.Sprintf("%s.post_functions[%d]", path, j), IssueInvalidRule, "la post-función está vacía")
			} else if err := p.Validate(); err != nil {
				add(fmt.Sprintf("%s.post_functions[%d]", path, j), IssueInvalidRule, p.Type()+": "+err.Error())
			}
		}
		if !toOK {
			continue
		}
		for _, src := range w.expandFrom(t) {
			if src == t.To {
				continue
			}
			adjacency[src] = append(adjacency[src], t.To)
			if name == "" {
				continue
			}
			if perSource[src] == nil {
				perSource[src] = map[string]bool{}
			}
			if perSource[src][name] {
				add(path+".name", IssueDuplicateTransition, "ya existe una transición con ese nombre desde el mismo estado")
				break
			}
			perSource[src][name] = true
		}
	}

	if initialOK {
		reached := map[string]bool{w.Initial: true}
		queue := []string{w.Initial}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, next := range adjacency[cur] {
				if !reached[next] {
					reached[next] = true
					queue = append(queue, next)
				}
			}
		}
		for i, s := range w.Statuses {
			if s.ID != "" && !reached[s.ID] {
				add(fmt.Sprintf("statuses[%d]", i), IssueUnreachableStatus, "el estado no es alcanzable desde el estado inicial")
			}
		}
	}
	return issues
}

// Check returns nil for a valid workflow, otherwise a typed error with one detail per issue.
func (w Workflow) Check() error { return IssuesError(w.Validate()) }

// TransitionChange pairs the old and new definition of a transition with the same id.
type TransitionChange struct {
	Old Transition
	New Transition
}

// StatusChange pairs the old and new definition of a status with the same id.
type StatusChange struct {
	Old Status
	New Status
}

// WorkflowDiff lists what changed between two versions of a workflow.
type WorkflowDiff struct {
	RemovedStatuses    []Status
	AddedStatuses      []Status
	ChangedStatuses    []StatusChange
	RemovedTransitions []Transition
	AddedTransitions   []Transition
	ChangedTransitions []TransitionChange
	InitialChanged     bool
}

// RequiresMapping reports whether issues in a removed status need a status_mapping.
func (d WorkflowDiff) RequiresMapping() bool { return len(d.RemovedStatuses) > 0 }

// Diff compares two workflows by status and transition id. Results keep the order of the
// workflow they come from (removed from old, added and changed from new).
func Diff(old, updated Workflow) WorkflowDiff {
	var d WorkflowDiff
	d.InitialChanged = old.Initial != updated.Initial

	for _, s := range old.Statuses {
		if _, ok := updated.Status(s.ID); !ok {
			d.RemovedStatuses = append(d.RemovedStatuses, s)
		}
	}
	for _, s := range updated.Statuses {
		prev, ok := old.Status(s.ID)
		switch {
		case !ok:
			d.AddedStatuses = append(d.AddedStatuses, s)
		case prev != s:
			d.ChangedStatuses = append(d.ChangedStatuses, StatusChange{Old: prev, New: s})
		}
	}
	for _, t := range old.Transitions {
		if _, ok := updated.Transition(t.ID); !ok {
			d.RemovedTransitions = append(d.RemovedTransitions, t)
		}
	}
	for _, t := range updated.Transitions {
		prev, ok := old.Transition(t.ID)
		switch {
		case !ok:
			d.AddedTransitions = append(d.AddedTransitions, t)
		case !sameTransition(prev, t):
			d.ChangedTransitions = append(d.ChangedTransitions, TransitionChange{Old: prev, New: t})
		}
	}
	return d
}

func sameTransition(a, b Transition) bool {
	ea, errA := Encode(Workflow{Transitions: []Transition{a}})
	eb, errB := Encode(Workflow{Transitions: []Transition{b}})
	return errA == nil && errB == nil && bytes.Equal(ea, eb)
}
