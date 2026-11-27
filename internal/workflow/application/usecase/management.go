package usecase

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// Authorization actions and organization roles used by the administration use cases.
const (
	actionViewProject    = "view_project"
	actionManageWorkflow = "manage_workflow"
	orgRoleGuest         = "guest"
	orgRoleAdmin         = "admin"
)

// Management is the administration use case set: reading and replacing the workflow of a project, applying
// templates, moving issues and managing the status catalog of an organization. Writes run in one transaction.
type Management struct {
	repo  port.ManagementRepository
	authz port.Authorizer
	orgs  port.OrgRoles
	tx    database.TxManager
	clock clock.Clock
}

// NewManagement builds the use cases. A nil clock means the system clock.
func NewManagement(repo port.ManagementRepository, authz port.Authorizer, orgs port.OrgRoles, tx database.TxManager, c clock.Clock) *Management {
	if c == nil {
		c = clock.RealClock{}
	}
	return &Management{repo: repo, authz: authz, orgs: orgs, tx: tx, clock: c}
}

var _ port.Admin = (*Management)(nil)

// project loads the project and checks the action on it. Missing projects return port.ErrProjectNotFound.
func (m *Management) project(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, action string) (port.ProjectRef, error) {
	ref, err := m.repo.ProjectContext(ctx, projectID)
	if err != nil {
		return port.ProjectRef{}, err
	}
	if err := m.authz.Authorize(ctx, action, actorID, globalRole, projectID); err != nil {
		return port.ProjectRef{}, err
	}
	return ref, nil
}

// GetWorkflow returns the stored workflow of a project with issue counts. Anyone who can browse the project may read it.
func (m *Management) GetWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (port.WorkflowView, error) {
	ref, err := m.project(ctx, actorID, globalRole, projectID, actionViewProject)
	if err != nil {
		return port.WorkflowView{}, err
	}
	return m.view(ctx, ref)
}

func (m *Management) view(ctx context.Context, ref port.ProjectRef) (port.WorkflowView, error) {
	record, err := m.repo.FindWorkflow(ctx, ref.ID)
	if err != nil {
		return port.WorkflowView{}, err
	}
	counts, err := m.repo.IssueCounts(ctx, ref.ID)
	if err != nil {
		return port.WorkflowView{}, err
	}
	return buildView(record, counts, ref.Template), nil
}

func buildView(record port.Record, counts map[uuid.UUID]int, template string) port.WorkflowView {
	w := record.Workflow
	view := port.WorkflowView{
		InitialStatusID:   parseID(w.Initial),
		Transitions:       w.Transitions,
		Version:           record.Version,
		UpdatedAt:         record.UpdatedAt,
		IsTemplateDefault: isTemplateDefault(w, template),
	}
	for i, s := range w.Statuses {
		id := parseID(s.ID)
		view.Statuses = append(view.Statuses, port.StatusView{ID: id, Name: s.Name, Category: s.Category, Position: i, IssueCount: counts[id]})
	}
	return view
}

func parseID(text string) uuid.UUID {
	id, err := uuid.Parse(text)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// isTemplateDefault reports whether the stored workflow equals the default workflow of the template, with the
// statuses matched by name.
func isTemplateDefault(w domain.Workflow, template string) bool {
	tpl, err := domain.TemplateWorkflow(template)
	if err != nil || len(tpl.Statuses) != len(w.Statuses) || len(tpl.Transitions) != len(w.Transitions) {
		return false
	}
	byName := make(map[string]domain.Status, len(w.Statuses))
	for _, s := range w.Statuses {
		byName[strings.ToLower(s.Name)] = s
	}
	slugs := make(map[string]domain.Status, len(tpl.Statuses))
	want := domain.Workflow{}
	for _, s := range tpl.Statuses {
		stored, ok := byName[strings.ToLower(s.Name)]
		if !ok {
			return false
		}
		slugs[s.ID] = stored
		want.Statuses = append(want.Statuses, domain.Status{ID: stored.ID, Name: stored.Name, Category: s.Category})
	}
	want.Initial = slugs[tpl.Initial].ID
	for _, t := range tpl.Transitions {
		nt := t
		nt.From = nil
		for _, from := range t.From {
			nt.From = append(nt.From, slugs[from].ID)
		}
		nt.To = slugs[t.To].ID
		want.Transitions = append(want.Transitions, nt)
	}
	a, errA := domain.Encode(want)
	b, errB := domain.Encode(w)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

// ValidateWorkflow is a dry run: it reports the problems, the warnings and the mapping a replace would need.
func (m *Management) ValidateWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, def port.Definition) (port.ValidationReport, error) {
	ref, err := m.project(ctx, actorID, globalRole, projectID, actionManageWorkflow)
	if err != nil {
		return port.ValidationReport{}, err
	}
	p, err := m.plan(ctx, ref, def)
	if err != nil {
		return port.ValidationReport{}, err
	}
	return port.ValidationReport{
		Valid:           len(p.issues) == 0 && len(p.mapping) == 0,
		Errors:          p.issues,
		Warnings:        p.warnings,
		MappingRequired: p.mapping,
	}, nil
}

// plan loads what planDefinition needs from the repository.
func (m *Management) plan(ctx context.Context, ref port.ProjectRef, def port.Definition) (planned, error) {
	record, err := m.repo.FindWorkflow(ctx, ref.ID)
	if err != nil {
		return planned{}, err
	}
	org, err := m.repo.ListStatuses(ctx, ref.OrganizationID)
	if err != nil {
		return planned{}, err
	}
	counts, err := m.repo.IssueCounts(ctx, ref.ID)
	if err != nil {
		return planned{}, err
	}
	return planDefinition(def, record.Workflow, org, counts), nil
}

// ReplaceWorkflow validates and stores a new workflow for the project. It runs in one transaction that locks
// the workflow row, creates the new statuses, moves the issues of removed statuses and saves the definition.
func (m *Management) ReplaceWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, def port.Definition) (port.SaveResult, error) {
	ref, err := m.project(ctx, actorID, globalRole, projectID, actionManageWorkflow)
	if err != nil {
		return port.SaveResult{}, err
	}
	if def.ExpectedVersion < 1 {
		return port.SaveResult{}, port.ErrVersionRequired
	}
	return m.apply(ctx, actorID, ref, def)
}

// ResetWorkflow applies the default workflow of a template with the same rules as ReplaceWorkflow.
func (m *Management) ResetWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, template string, expectedVersion int, mapping map[uuid.UUID]uuid.UUID) (port.SaveResult, error) {
	ref, err := m.project(ctx, actorID, globalRole, projectID, actionManageWorkflow)
	if err != nil {
		return port.SaveResult{}, err
	}
	if expectedVersion < 1 {
		return port.SaveResult{}, port.ErrVersionRequired
	}
	tpl, err := domain.TemplateWorkflow(template)
	if err != nil {
		return port.SaveResult{}, err
	}
	def := port.Definition{ExpectedVersion: expectedVersion, InitialRef: tpl.Initial, Transitions: tpl.Transitions, Mapping: mapping}
	for _, s := range tpl.Statuses {
		def.Statuses = append(def.Statuses, port.StatusInput{Ref: s.ID, Name: s.Name, Category: s.Category})
	}
	return m.apply(ctx, actorID, ref, def)
}

// apply stores a planned definition. Every step runs in one transaction; a version mismatch or a planning
// failure rolls it back.
func (m *Management) apply(ctx context.Context, actorID uuid.UUID, ref port.ProjectRef, def port.Definition) (port.SaveResult, error) {
	var result port.SaveResult
	err := m.tx.InTx(ctx, func(ctx context.Context) error {
		version, err := m.lockWorkflow(ctx, ref.ID)
		if err != nil {
			return err
		}
		if version != def.ExpectedVersion {
			return port.ErrVersionConflict
		}
		p, err := m.plan(ctx, ref, def)
		if err != nil {
			return err
		}
		if len(p.issues) > 0 {
			return validationError(p.issues)
		}
		if len(p.mapping) > 0 {
			return &port.ValidationFailure{Err: port.ErrMappingRequired, Mapping: p.mapping}
		}

		ids := make(map[string]string, len(p.creates))
		for _, c := range p.creates {
			created, err := m.repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: ref.OrganizationID, Name: c.Name, Category: c.Category})
			if err != nil {
				return err
			}
			ids[newStatusPrefix+c.Ref] = created.ID.String()
		}

		now := m.clock.Now().UTC()
		warnings := append([]domain.Issue(nil), p.warnings...)
		for _, mg := range p.migrations {
			if _, err := m.repo.MigrateIssues(ctx, port.MigrationScope{OrganizationID: ref.OrganizationID, ProjectID: ref.ID}, mg.from, mg.to, actorID, now); err != nil {
				return err
			}
		}
		if _, err := m.repo.SaveWorkflow(ctx, ref.ID, replaceIDs(p.workflow, ids), def.ExpectedVersion, now); err != nil {
			return err
		}
		result.Warnings = warnings
		return nil
	})
	if err != nil {
		return port.SaveResult{}, err
	}
	view, err := m.view(ctx, ref)
	if err != nil {
		return port.SaveResult{}, err
	}
	result.Workflow = view
	return result, nil
}

// lockWorkflow locks the workflow row and returns its version. A project without a row gets its template
// workflow first.
func (m *Management) lockWorkflow(ctx context.Context, projectID uuid.UUID) (int, error) {
	version, err := m.repo.LockWorkflow(ctx, projectID)
	if errors.Is(err, port.ErrWorkflowNotFound) {
		if _, perr := m.repo.FindWorkflow(ctx, projectID); perr != nil {
			return 0, perr
		}
		return m.repo.LockWorkflow(ctx, projectID)
	}
	return version, err
}

// MigrateIssues moves issues between statuses explicitly, for example before a status is removed. Both ends
// must belong to the organization and the target must belong to the project workflow.
func (m *Management) MigrateIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, mapping map[uuid.UUID]uuid.UUID) (port.MigrationResult, error) {
	ref, err := m.project(ctx, actorID, globalRole, projectID, actionManageWorkflow)
	if err != nil {
		return port.MigrationResult{}, err
	}
	if len(mapping) == 0 {
		return port.MigrationResult{}, &port.ValidationFailure{
			Err:    port.ErrInvalidMapping,
			Issues: []domain.Issue{{Path: "status_mapping", Code: "workflow.required", Message: "indica al menos un movimiento"}},
		}
	}
	record, err := m.repo.FindWorkflow(ctx, ref.ID)
	if err != nil {
		return port.MigrationResult{}, err
	}
	org, err := m.repo.ListStatuses(ctx, ref.OrganizationID)
	if err != nil {
		return port.MigrationResult{}, err
	}
	byID := make(map[uuid.UUID]port.OrgStatus, len(org))
	for _, s := range org {
		byID[s.ID] = s
	}

	keys := make([]uuid.UUID, 0, len(mapping))
	for from := range mapping {
		keys = append(keys, from)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	var issues []domain.Issue
	for _, from := range keys {
		to := mapping[from]
		path := "status_mapping." + from.String()
		if _, ok := byID[from]; !ok {
			issues = append(issues, domain.Issue{Path: path, Code: issueMappingInvalid, Message: port.ErrStatusNotInOrg.Message})
		}
		if _, ok := byID[to]; !ok {
			issues = append(issues, domain.Issue{Path: path, Code: issueMappingInvalid, Message: port.ErrStatusNotInOrg.Message})
		} else if _, inFlow := record.Workflow.Status(to.String()); !inFlow {
			issues = append(issues, domain.Issue{Path: path, Code: issueMappingInvalid, Message: "el estado de destino no está en el flujo del proyecto"})
		}
		if from == to {
			issues = append(issues, domain.Issue{Path: path, Code: issueMappingInvalid, Message: "el estado de origen y destino son el mismo"})
		}
	}
	if len(issues) > 0 {
		return port.MigrationResult{}, &port.ValidationFailure{Err: port.ErrInvalidMapping, Issues: issues}
	}

	var result port.MigrationResult
	err = m.tx.InTx(ctx, func(ctx context.Context) error {
		now := m.clock.Now().UTC()
		for _, from := range keys {
			to := mapping[from]
			count, err := m.repo.MigrateIssues(ctx, port.MigrationScope{OrganizationID: ref.OrganizationID, ProjectID: ref.ID}, byID[from], byID[to], actorID, now)
			if err != nil {
				return err
			}
			result.Total += count
			result.Moves = append(result.Moves, port.MoveCount{From: from, To: to, Count: count})
		}
		return nil
	})
	if err != nil {
		return port.MigrationResult{}, err
	}
	return result, nil
}

// ListTemplates returns the four template workflows in the request format, with Spanish labels.
func (m *Management) ListTemplates(_ context.Context) []port.TemplateView {
	var out []port.TemplateView
	for _, name := range domain.TemplateNames() {
		tpl, err := domain.TemplateWorkflow(name)
		if err != nil {
			continue
		}
		label, description := domain.TemplateLabel(name)
		def := port.Definition{InitialRef: tpl.Initial, Transitions: tpl.Transitions}
		for _, s := range tpl.Statuses {
			def.Statuses = append(def.Statuses, port.StatusInput{Ref: s.ID, Name: s.Name, Category: s.Category})
		}
		out = append(out, port.TemplateView{ID: name, Name: label, Description: description, Definition: def})
	}
	return out
}
