package usecase

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// newStatusPrefix marks the ids of statuses that do not exist yet while a definition is planned. Real ids
// replace them just before the workflow is stored.
const newStatusPrefix = "new:"

// Stable issue codes of the planning step that the domain does not define.
const (
	issueStatusNotInOrg   = "workflow.status_not_in_org"
	issueCategoryMismatch = "workflow.category_mismatch"
	issueMappingInvalid   = "workflow.invalid_status_mapping"
	issueTransitionGone   = "workflow.transition_removed"
	issueStatusGone       = "workflow.status_removed"
	issueMigrated         = "workflow.issues_migrated"
	issueInitialChanged   = "workflow.initial_changed"
	issueCategoryChanged  = "workflow.status_category_changed"
	codeWorkflowInvalid   = "workflow.invalid"
	messageWorkflowBad    = "el flujo de trabajo no es válido"
)

// migration moves the issues of a removed status to the target status of the new workflow.
type migration struct {
	from  port.OrgStatus
	to    port.OrgStatus
	count int
}

// planned is a requested definition resolved against the organization catalog.
type planned struct {
	// workflow uses real ids for organization statuses and newStatusPrefix placeholders for new ones.
	workflow   domain.Workflow
	creates    []port.StatusInput
	issues     []domain.Issue
	warnings   []domain.Issue
	mapping    []port.MappingRequirement
	migrations []migration
	diff       domain.WorkflowDiff
}

// planDefinition resolves statuses and transitions, validates the result, diffs it against the stored workflow
// and works out the status mapping that removed statuses with issues need. It never writes.
func planDefinition(def port.Definition, current domain.Workflow, org []port.OrgStatus, counts map[uuid.UUID]int) planned {
	var p planned
	addIssue := func(path, code, message string) {
		p.issues = append(p.issues, domain.Issue{Path: path, Code: code, Message: message})
	}
	byID := make(map[uuid.UUID]port.OrgStatus, len(org))
	byName := make(map[string]port.OrgStatus, len(org))
	for _, s := range org {
		byID[s.ID] = s
		byName[strings.ToLower(s.Name)] = s
	}

	refs := make(map[string]string, len(def.Statuses))
	statuses := make([]domain.Status, 0, len(def.Statuses))
	for i, in := range def.Statuses {
		path := fmt.Sprintf("statuses[%d]", i)
		ref := strings.TrimSpace(in.Ref)
		switch {
		case ref == "":
			addIssue(path+".ref", domain.IssueRequired, port.ErrStatusRefInvalid.Message)
		case refs[ref] != "":
			addIssue(path+".ref", domain.IssueDuplicateID, port.ErrStatusRefInvalid.Message)
		}
		status := resolveStatus(&p, path, ref, in, byID, byName, addIssue)
		refs[ref] = status.ID
		statuses = append(statuses, status)
	}

	initial := def.InitialRef
	if id, ok := refs[def.InitialRef]; ok {
		initial = id
	}
	transitions := make([]domain.Transition, 0, len(def.Transitions))
	for _, t := range def.Transitions {
		nt := t
		nt.From = mapRefs(t.From, refs)
		nt.To = mapRef(t.To, refs)
		transitions = append(transitions, nt)
	}

	w := domain.Workflow{Initial: initial, Statuses: statuses, Transitions: transitions}
	p.issues = append(p.issues, w.Validate()...)
	p.workflow = w
	p.diff = domain.Diff(current, w)
	p.warnings = append(p.warnings, warningsOf(p.diff, initial != current.Initial)...)
	p.planMappings(def, byID, counts, addIssue)
	return p
}

// resolveStatus maps one requested status to an organization status (by id or name) or plans its creation.
func resolveStatus(p *planned, path, ref string, in port.StatusInput, byID map[uuid.UUID]port.OrgStatus, byName map[string]port.OrgStatus, addIssue func(string, string, string)) domain.Status {
	if in.ID != uuid.Nil {
		org, ok := byID[in.ID]
		if !ok {
			addIssue(path+".id", issueStatusNotInOrg, port.ErrStatusNotInOrg.Message)
			return domain.Status{ID: newStatusPrefix + ref, Name: in.Name, Category: in.Category}
		}
		return existingStatus(path, org, in.Category, addIssue)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		addIssue(path+".name", domain.IssueRequired, "el nombre del estado es obligatorio")
		return domain.Status{ID: newStatusPrefix + ref, Category: in.Category}
	}
	if org, ok := byName[strings.ToLower(name)]; ok {
		return existingStatus(path, org, in.Category, addIssue)
	}
	if utf8.RuneCountInString(name) > maxStatusNameLength {
		addIssue(path+".name", "workflow.status_name_invalid", port.ErrStatusNameInvalid.Message)
	}
	if !in.Category.IsValid() {
		addIssue(path+".category", domain.IssueInvalidCategory, port.ErrStatusCategory.Message)
	}
	p.creates = append(p.creates, port.StatusInput{Ref: ref, Name: name, Category: in.Category})
	return domain.Status{ID: newStatusPrefix + ref, Name: name, Category: in.Category}
}

func existingStatus(path string, org port.OrgStatus, category domain.Category, addIssue func(string, string, string)) domain.Status {
	if category != "" && category != org.Category {
		addIssue(path+".category", issueCategoryMismatch, "la categoría no coincide con la del estado de la organización; cámbiala desde el catálogo de estados")
	}
	return domain.Status{ID: org.ID.String(), Name: org.Name, Category: org.Category}
}

// planMappings checks the mapping given for removed statuses and collects the ones that still need a target.
func (p *planned) planMappings(def port.Definition, byID map[uuid.UUID]port.OrgStatus, counts map[uuid.UUID]int, addIssue func(string, string, string)) {
	removed := make(map[string]domain.Status, len(p.diff.RemovedStatuses))
	for _, s := range p.diff.RemovedStatuses {
		removed[s.ID] = s
	}
	keys := make([]uuid.UUID, 0, len(def.Mapping))
	for key := range def.Mapping {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, key := range keys {
		path := "status_mapping." + key.String()
		if _, ok := removed[key.String()]; !ok {
			addIssue(path, issueMappingInvalid, "el mapeo solo puede indicar estados que se eliminan del flujo")
			continue
		}
		if _, ok := p.workflow.Status(def.Mapping[key].String()); !ok {
			addIssue(path, issueMappingInvalid, "el estado de destino no está en el flujo nuevo")
		}
	}

	for _, s := range p.diff.RemovedStatuses {
		id, err := uuid.Parse(s.ID)
		if err != nil {
			continue
		}
		count := counts[id]
		if count == 0 {
			p.warnings = append(p.warnings, domain.Issue{Path: "statuses", Code: issueStatusGone, Message: fmt.Sprintf("el estado %q se elimina del flujo", s.Name)})
			continue
		}
		target, ok := def.Mapping[id]
		if !ok {
			p.mapping = append(p.mapping, port.MappingRequirement{StatusID: id, Name: s.Name, IssueCount: count})
			continue
		}
		to, inFlow := p.workflow.Status(target.String())
		if !inFlow {
			continue
		}
		from := port.OrgStatus{ID: id, Name: s.Name, Category: s.Category}
		dest := port.OrgStatus{ID: target, Name: to.Name, Category: to.Category}
		p.migrations = append(p.migrations, migration{from: from, to: dest, count: count})
		p.warnings = append(p.warnings, domain.Issue{Path: "statuses", Code: issueMigrated, Message: fmt.Sprintf("%d incidencias pasan de %q a %q", count, s.Name, to.Name)})
	}
}

// warningsOf describes the changes of a diff that do not block the save.
func warningsOf(d domain.WorkflowDiff, initialChanged bool) []domain.Issue {
	var out []domain.Issue
	for _, t := range d.RemovedTransitions {
		out = append(out, domain.Issue{Path: "transitions", Code: issueTransitionGone, Message: fmt.Sprintf("la transición %q se elimina del flujo", t.Name)})
	}
	for _, c := range d.ChangedStatuses {
		if c.Old.Category != c.New.Category {
			out = append(out, domain.Issue{Path: "statuses", Code: issueCategoryChanged, Message: fmt.Sprintf("el estado %q cambia de categoría", c.New.Name)})
		}
	}
	if initialChanged {
		out = append(out, domain.Issue{Path: "initial_ref", Code: issueInitialChanged, Message: "cambia el estado inicial del flujo"})
	}
	return out
}

func mapRef(ref string, refs map[string]string) string {
	if id, ok := refs[ref]; ok {
		return id
	}
	return ref
}

func mapRefs(in []string, refs map[string]string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, ref := range in {
		out = append(out, mapRef(ref, refs))
	}
	return out
}

// replaceIDs rewrites placeholder status ids with the real ids of the statuses that were just created.
func replaceIDs(w domain.Workflow, ids map[string]string) domain.Workflow {
	out := domain.Workflow{Initial: mapRef(w.Initial, ids), Statuses: make([]domain.Status, 0, len(w.Statuses))}
	for _, s := range w.Statuses {
		s.ID = mapRef(s.ID, ids)
		out.Statuses = append(out.Statuses, s)
	}
	for _, t := range w.Transitions {
		t.From = mapRefs(t.From, ids)
		t.To = mapRef(t.To, ids)
		out.Transitions = append(out.Transitions, t)
	}
	return out
}

// validationError wraps issues in the workflow.invalid error.
func validationError(issues []domain.Issue) error {
	return &port.ValidationFailure{Err: apperror.WithCode(apperror.KindValidation, codeWorkflowInvalid, messageWorkflowBad), Issues: issues}
}
