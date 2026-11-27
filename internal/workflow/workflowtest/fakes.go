// Package workflowtest holds in-memory doubles of the workflow ports for the use case and HTTP tests. The
// doubles follow the contract of the PostgreSQL adapter: version checks, name uniqueness per organization,
// refusals of statuses in use and issue moves that write a history row and an activity per issue.
package workflowtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// Repo is an in-memory port.ManagementRepository.
type Repo struct {
	Projects    map[uuid.UUID]port.ProjectRef
	Workflows   map[uuid.UUID]port.Record
	Statuses    []port.OrgStatus
	Issues      map[uuid.UUID]Issue
	HistoryRefs map[uuid.UUID]int
	Activities  int
	// Calls counts the writes that reached the repository; tests use it to prove a refused request wrote nothing.
	Writes int
}

// Issue is an issue of a project in a status.
type Issue struct {
	ProjectID uuid.UUID
	StatusID  uuid.UUID
}

var _ port.ManagementRepository = (*Repo)(nil)

// NewRepo returns an empty repository.
func NewRepo() *Repo {
	return &Repo{
		Projects:    map[uuid.UUID]port.ProjectRef{},
		Workflows:   map[uuid.UUID]port.Record{},
		Issues:      map[uuid.UUID]Issue{},
		HistoryRefs: map[uuid.UUID]int{},
	}
}

// SeedProject creates a project in the organization with the statuses and workflow of the template. Status
// names are shared with existing statuses of the organization, as the provisioning function does.
func (r *Repo) SeedProject(orgID uuid.UUID, template string) uuid.UUID {
	tpl, err := domain.TemplateWorkflow(template)
	if err != nil {
		panic(err)
	}
	projectID := uuid.New()
	r.Projects[projectID] = port.ProjectRef{ID: projectID, OrganizationID: orgID, Template: template}
	ids := map[string]string{}
	w := domain.Workflow{}
	for i, s := range tpl.Statuses {
		status := r.ensureStatus(orgID, s.Name, s.Category, i)
		ids[s.ID] = status.ID.String()
		w.Statuses = append(w.Statuses, domain.Status{ID: status.ID.String(), Name: status.Name, Category: status.Category})
	}
	w.Initial = ids[tpl.Initial]
	for _, t := range tpl.Transitions {
		nt := t
		nt.From = nil
		for _, from := range t.From {
			nt.From = append(nt.From, ids[from])
		}
		nt.To = ids[t.To]
		w.Transitions = append(w.Transitions, nt)
	}
	r.Workflows[projectID] = port.Record{ProjectID: projectID, Workflow: w, Version: 1, UpdatedAt: time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)}
	return projectID
}

// ensureStatus returns the organization status with the name, creating it when missing.
func (r *Repo) ensureStatus(orgID uuid.UUID, name string, category domain.Category, position int) port.OrgStatus {
	for _, s := range r.Statuses {
		if s.OrganizationID == orgID && strings.EqualFold(s.Name, name) {
			return s
		}
	}
	s := port.OrgStatus{ID: uuid.New(), OrganizationID: orgID, Name: name, Category: category, Position: position}
	r.Statuses = append(r.Statuses, s)
	return s
}

// AddIssues creates n issues of the project in the status.
func (r *Repo) AddIssues(projectID, statusID uuid.UUID, n int) {
	for i := 0; i < n; i++ {
		r.Issues[uuid.New()] = Issue{ProjectID: projectID, StatusID: statusID}
	}
}

// StatusByName returns the organization status with the name (case insensitive).
func (r *Repo) StatusByName(orgID uuid.UUID, name string) (port.OrgStatus, bool) {
	for _, s := range r.Statuses {
		if s.OrganizationID == orgID && strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return port.OrgStatus{}, false
}

// IssueStatus returns the status of the issue shown by the fake.
func (r *Repo) IssueStatus(projectID uuid.UUID) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	for _, issue := range r.Issues {
		if issue.ProjectID == projectID {
			out[issue.StatusID]++
		}
	}
	return out
}

func (r *Repo) FindWorkflow(_ context.Context, projectID uuid.UUID) (port.Record, error) {
	rec, ok := r.Workflows[projectID]
	if !ok {
		return port.Record{}, port.ErrWorkflowNotFound
	}
	return rec, nil
}

func (r *Repo) SaveWorkflow(_ context.Context, projectID uuid.UUID, w domain.Workflow, expectedVersion int, at time.Time) (int, error) {
	rec, ok := r.Workflows[projectID]
	if !ok {
		return 0, port.ErrWorkflowNotFound
	}
	if rec.Version != expectedVersion {
		return 0, port.ErrVersionConflict
	}
	r.Writes++
	rec.Workflow = w
	rec.Version++
	rec.UpdatedAt = at
	r.Workflows[projectID] = rec
	return rec.Version, nil
}

func (r *Repo) ListStatuses(_ context.Context, orgID uuid.UUID) ([]port.OrgStatus, error) {
	var out []port.OrgStatus
	for _, s := range r.Statuses {
		if s.OrganizationID == orgID {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (r *Repo) CreateStatus(_ context.Context, s port.OrgStatus) (port.OrgStatus, error) {
	for _, existing := range r.Statuses {
		if existing.OrganizationID == s.OrganizationID && strings.EqualFold(existing.Name, s.Name) {
			return port.OrgStatus{}, port.ErrStatusNameTaken
		}
	}
	r.Writes++
	s.ID = uuid.New()
	s.Position = 0
	for _, existing := range r.Statuses {
		if existing.OrganizationID == s.OrganizationID && existing.Position >= s.Position {
			s.Position = existing.Position + 1
		}
	}
	r.Statuses = append(r.Statuses, s)
	return s, nil
}

func (r *Repo) UpdateStatus(_ context.Context, s port.OrgStatus) error {
	for i, existing := range r.Statuses {
		if existing.ID == s.ID && existing.OrganizationID == s.OrganizationID {
			r.Writes++
			r.Statuses[i] = s
			return nil
		}
	}
	return port.ErrStatusNotFound
}

func (r *Repo) DeleteStatus(_ context.Context, orgID, id uuid.UUID) error {
	for _, issue := range r.Issues {
		if issue.StatusID == id {
			return port.ErrStatusInUse
		}
	}
	for _, rec := range r.Workflows {
		if _, ok := rec.Workflow.Status(id.String()); ok && r.projectOrg(rec.ProjectID) == orgID {
			return port.ErrStatusInUse
		}
	}
	for i, s := range r.Statuses {
		if s.ID == id && s.OrganizationID == orgID {
			r.Writes++
			r.Statuses = append(r.Statuses[:i], r.Statuses[i+1:]...)
			return nil
		}
	}
	return port.ErrStatusNotFound
}

func (r *Repo) ColumnStatusIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) { return nil, nil }

func (r *Repo) projectOrg(projectID uuid.UUID) uuid.UUID {
	return r.Projects[projectID].OrganizationID
}

func (r *Repo) ProjectContext(_ context.Context, projectID uuid.UUID) (port.ProjectRef, error) {
	ref, ok := r.Projects[projectID]
	if !ok {
		return port.ProjectRef{}, port.ErrProjectNotFound
	}
	return ref, nil
}

func (r *Repo) LockWorkflow(_ context.Context, projectID uuid.UUID) (int, error) {
	rec, ok := r.Workflows[projectID]
	if !ok {
		return 0, port.ErrWorkflowNotFound
	}
	return rec.Version, nil
}

func (r *Repo) IssueCounts(_ context.Context, projectID uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	for _, issue := range r.Issues {
		if issue.ProjectID == projectID {
			out[issue.StatusID]++
		}
	}
	return out, nil
}

func (r *Repo) MigrateIssues(_ context.Context, scope port.MigrationScope, from, to port.OrgStatus, _ uuid.UUID, _ time.Time) (int, error) {
	moved := 0
	for id, issue := range r.Issues {
		if issue.StatusID != from.ID {
			continue
		}
		if scope.ProjectID != uuid.Nil && issue.ProjectID != scope.ProjectID {
			continue
		}
		if r.projectOrg(issue.ProjectID) != scope.OrganizationID {
			continue
		}
		issue.StatusID = to.ID
		r.Issues[id] = issue
		moved++
	}
	if moved > 0 {
		r.Writes++
		r.HistoryRefs[to.ID] += moved
		r.HistoryRefs[from.ID] += moved
		r.Activities += moved
	}
	return moved, nil
}

func (r *Repo) StatusUsage(_ context.Context, orgID, statusID uuid.UUID) (int, int, error) {
	issues := 0
	for _, issue := range r.Issues {
		if issue.StatusID == statusID && r.projectOrg(issue.ProjectID) == orgID {
			issues++
		}
	}
	return issues, r.HistoryRefs[statusID], nil
}

func (r *Repo) WorkflowsUsing(_ context.Context, orgID, statusID uuid.UUID) ([]port.Record, error) {
	var out []port.Record
	for _, rec := range r.Workflows {
		if r.projectOrg(rec.ProjectID) != orgID {
			continue
		}
		if _, ok := rec.Workflow.Status(statusID.String()); ok {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProjectID.String() < out[j].ProjectID.String() })
	return out, nil
}

func (r *Repo) RewriteStatusHistory(_ context.Context, from, to uuid.UUID) error {
	r.HistoryRefs[to] += r.HistoryRefs[from]
	delete(r.HistoryRefs, from)
	return nil
}

func (r *Repo) SetStatusPositions(_ context.Context, orgID uuid.UUID, ids []uuid.UUID) error {
	for position, id := range ids {
		for i, s := range r.Statuses {
			if s.ID == id && s.OrganizationID == orgID {
				r.Statuses[i].Position = position
			}
		}
	}
	return nil
}

func (r *Repo) RefreshLegacyStatus(context.Context, uuid.UUID, domain.Category, time.Time) error {
	return nil
}

// Authz is a fake port.Authorizer. Roles maps a user to "lead", "org_admin", "contributor", "viewer" or
// nothing (outsider). Archived projects refuse writes with project.archived.
type Authz struct {
	Roles    map[uuid.UUID]string
	Archived map[uuid.UUID]bool
}

func (a Authz) Authorize(_ context.Context, action string, actorID uuid.UUID, _ string, projectID uuid.UUID) error {
	role, ok := a.Roles[actorID]
	if !ok {
		return apperror.New(apperror.KindNotFound, "proyecto no encontrado")
	}
	if action == "view_project" {
		return nil
	}
	if role != "lead" && role != "org_admin" {
		return apperror.New(apperror.KindForbidden, "no tienes permiso para esta acción")
	}
	if a.Archived[projectID] {
		return apperror.WithCode(apperror.KindConflict, "project.archived", "el proyecto está archivado")
	}
	return nil
}

// OrgRoles is a fake port.OrgRoles keyed by organization and user. Roles are "owner", "admin", "member" or
// "guest".
type OrgRoles struct {
	Roles map[OrgUser]string
}

// OrgUser identifies a membership.
type OrgUser struct {
	OrgID  uuid.UUID
	UserID uuid.UUID
}

var orgRank = map[string]int{"guest": 1, "member": 2, "admin": 3, "owner": 4}

func (o OrgRoles) RequireOrgRole(_ context.Context, userID, orgID uuid.UUID, min string) error {
	role, ok := o.Roles[OrgUser{OrgID: orgID, UserID: userID}]
	if !ok {
		return apperror.New(apperror.KindNotFound, "organización no encontrada")
	}
	if orgRank[role] < orgRank[min] {
		return apperror.New(apperror.KindForbidden, "no tienes permiso para esta acción")
	}
	return nil
}

// Tx runs the function without a transaction; the fake repository has no rollback, so use cases must refuse
// before their first write.
type Tx struct{}

func (Tx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Describe formats a status list for test failure messages.
func Describe(list []port.OrgStatus) string {
	var parts []string
	for _, s := range list {
		parts = append(parts, fmt.Sprintf("%s(%s)", s.Name, s.Category))
	}
	return strings.Join(parts, ", ")
}
