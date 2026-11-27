package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// noTxManager ejecuta fn sin transaccion. Es el valor por defecto del servicio; el composition root
// inyecta el TxManager real con WithTxManager.
type noTxManager struct{}

func (noTxManager) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type IssueService struct {
	tx            database.TxManager
	backlog       port.BacklogRepository
	notifier      port.Notifier
	repo          port.Repository
	directory     port.UserDirectory
	collaboration port.CollaborationRepository
	issueLinks    port.IssueLinkRepository
	workflows     port.WorkflowRepository
	projectRoles  port.ProjectRoleResolver
	watchers      port.IssueWatcherRepository
	assigneeWatch port.AssigneeWatcherRepository
	savedFilters  port.SavedFilterRepository
	authorizer    port.ProjectAuthorizer
	catalog       port.IssueCatalog
	customScope   port.CustomFieldScopeReader
	browse        port.ProjectBrowseChecker

	clock          clock.Clock
	events         events.Publisher
	perms          port.IssuePermissionChecker
	transitions    port.TransitionProvider
	requiredFields port.RequiredFieldReader
	defaults       port.ProjectDefaults
	viewReader     port.ViewReader
	hierarchy      port.HierarchyReader
	lifecycle      port.IssueLifecycle
	activityBatch  port.ActivityBatchWriter
	votes          port.VoteRepository
	children       port.ChildReader
}

func NewIssueService(repo port.Repository, directory port.UserDirectory, repositories ...any) *IssueService {
	var collaboration port.CollaborationRepository
	var issueLinks port.IssueLinkRepository
	var workflows port.WorkflowRepository
	var projectRoles port.ProjectRoleResolver
	var authorizer port.ProjectAuthorizer
	var watchers port.IssueWatcherRepository
	var assigneeWatch port.AssigneeWatcherRepository
	var savedFilters port.SavedFilterRepository
	var backlog port.BacklogRepository
	for _, repository := range repositories {
		if typed, ok := repository.(port.ProjectAuthorizer); ok {
			authorizer = typed
		}
		// El backlog va fuera del switch: un repositorio puede cumplir un case anterior y el switch se detiene.
		if typed, ok := repository.(port.BacklogRepository); ok {
			backlog = typed
		}
		switch typed := repository.(type) {
		case port.CollaborationRepository:
			collaboration = typed
		case port.IssueLinkRepository:
			issueLinks = typed
		case port.WorkflowRepository:
			workflows = typed
		case port.ProjectRoleResolver:
			projectRoles = typed
		case port.IssueWatcherRepository:
			watchers = typed
		case port.AssigneeWatcherRepository:
			assigneeWatch = typed
		case port.SavedFilterRepository:
			savedFilters = typed
		}
	}
	service := &IssueService{tx: noTxManager{}, clock: clock.RealClock{}, repo: repo, directory: directory, collaboration: collaboration, issueLinks: issueLinks, workflows: workflows, projectRoles: projectRoles, authorizer: authorizer, watchers: watchers, assigneeWatch: assigneeWatch, savedFilters: savedFilters, backlog: backlog}
	// The optional ports may be implemented by the repository or by any of the other adapters.
	for _, candidate := range append([]any{repo}, repositories...) {
		if typed, ok := candidate.(port.ViewReader); ok {
			service.viewReader = typed
		}
		if typed, ok := candidate.(port.HierarchyReader); ok {
			service.hierarchy = typed
		}
		if typed, ok := candidate.(port.IssueLifecycle); ok {
			service.lifecycle = typed
		}
		if typed, ok := candidate.(port.ProjectDefaults); ok {
			service.defaults = typed
		}
		if typed, ok := candidate.(port.ActivityBatchWriter); ok {
			service.activityBatch = typed
		}
		if typed, ok := candidate.(port.VoteRepository); ok {
			service.votes = typed
		}
		if typed, ok := candidate.(port.ChildReader); ok {
			service.children = typed
		}
	}
	return service
}

var _ port.Service = (*IssueService)(nil)

// WithTxManager fija el gestor de transacciones de las escrituras multiples (asignar y desasignar).
// Sin el, las escrituras se ejecutan sin transaccion.
func (s *IssueService) WithTxManager(tx database.TxManager) *IssueService {
	if tx == nil {
		tx = noTxManager{}
	}
	s.tx = tx
	return s
}

// WithCatalog fija el lector de catalogos. Sin el, las incidencias usan solo los valores legacy.
func (s *IssueService) WithCatalog(catalog port.IssueCatalog) *IssueService {
	s.catalog = catalog
	return s
}

func (s *IssueService) Create(ctx context.Context, in port.CreateInput) (*domain.Issue, error) {
	priority := in.Priority
	if priority == "" {
		// The catalog default replaces this placeholder below; without a catalog it is the legacy default.
		priority = domain.PriorityMedium
	}
	issue, err := domain.NewIssue(in.Summary, in.Description, priority, in.ReporterID)
	if err != nil {
		return nil, err
	}
	if issue.Summary, err = domain.NormalizeSummary(issue.Summary); err != nil {
		return nil, err
	}
	if issue.Description, err = domain.NormalizeDescription(issue.Description); err != nil {
		return nil, err
	}
	issue.ProjectID = domain.DefaultProjectID
	if in.ProjectID != nil {
		issue.ProjectID = *in.ProjectID
	}
	if err := s.requireIssueCreate(ctx, issue.ProjectID, in.ReporterID, in.GlobalRole); err != nil {
		return nil, err
	}
	if s.catalog != nil {
		if err := s.applyCreateCatalog(ctx, issue, in); err != nil {
			return nil, err
		}
	} else if in.IssueType != "" {
		if err := issue.UpdateMetadata(in.IssueType, in.ParentID, in.DueDate, in.StoryPoints); err != nil {
			return nil, err
		}
	}
	if err := s.applyCustom(ctx, issue, true, uuid.Nil, in.Custom); err != nil {
		return nil, err
	}
	return s.finishCreate(ctx, issue, in)
}

// ResolveKey resolves a human key among the issues the actor can see. One visible candidate is returned,
// none is not found and several are ambiguous. A project the actor cannot view counts as missing.
func (s *IssueService) ResolveKey(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, ref string) (uuid.UUID, error) {
	projectKey, number, ok := domain.ParseKey(ref)
	if !ok {
		return uuid.Nil, domain.ErrInvalidIssueRef
	}
	candidates, err := s.repo.FindKeyCandidates(ctx, projectKey, number, orgID, actorID, globalRole == "admin")
	if err != nil {
		return uuid.Nil, err
	}
	var visible []*domain.Issue
	for _, candidate := range candidates {
		err := s.requireIssueView(ctx, candidate.ProjectID, actorID, globalRole)
		switch {
		case err == nil:
			visible = append(visible, candidate)
		case apperror.KindOf(err) == apperror.KindInternal:
			return uuid.Nil, err
		}
	}
	switch len(visible) {
	case 0:
		return uuid.Nil, domain.ErrIssueNotFound
	case 1:
		return visible[0].ID, nil
	default:
		return uuid.Nil, domain.ErrKeyAmbiguous
	}
}

func (s *IssueService) Get(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Issue, error) {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	return issue, nil
}

func (s *IssueService) AuthorizeIssueEdit(ctx context.Context, id, actorID uuid.UUID, globalRole string) error {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole)
}

func (s *IssueService) GetWatcherState(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.WatcherState, error) {
	if s.watchers == nil {
		return port.WatcherState{}, apperror.New(apperror.KindInternal, "watcher repository is unavailable")
	}
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.WatcherState{}, err
	}
	if err := s.requireIssueView(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return port.WatcherState{}, err
	}
	return s.watchers.State(ctx, issueID, actorID)
}

func (s *IssueService) WatchIssue(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.WatcherState, error) {
	if s.watchers == nil {
		return port.WatcherState{}, apperror.New(apperror.KindInternal, "watcher repository is unavailable")
	}
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.WatcherState{}, err
	}
	if err := s.requireIssueView(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return port.WatcherState{}, err
	}
	if err := s.watchers.Add(ctx, issueID, actorID); err != nil {
		return port.WatcherState{}, err
	}
	return s.watchers.State(ctx, issueID, actorID)
}

func (s *IssueService) UnwatchIssue(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.WatcherState, error) {
	if s.watchers == nil {
		return port.WatcherState{}, apperror.New(apperror.KindInternal, "watcher repository is unavailable")
	}
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.WatcherState{}, err
	}
	if err := s.requireIssueView(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return port.WatcherState{}, err
	}
	if err := s.watchers.Remove(ctx, issueID, actorID); err != nil {
		return port.WatcherState{}, err
	}
	return s.watchers.State(ctx, issueID, actorID)
}

func (s *IssueService) CreateSavedFilter(ctx context.Context, ownerID uuid.UUID, name string, criteria domain.SavedFilterCriteria) (*domain.SavedFilter, error) {
	if s.savedFilters == nil {
		return nil, apperror.New(apperror.KindInternal, "saved filter repository is unavailable")
	}
	filter, err := domain.NewSavedFilter(ownerID, name, criteria)
	if err != nil {
		return nil, err
	}
	if err := s.savedFilters.SaveSavedFilter(ctx, filter); err != nil {
		return nil, err
	}
	return filter, nil
}

func (s *IssueService) ListSavedFilters(ctx context.Context, ownerID uuid.UUID) ([]*domain.SavedFilter, error) {
	if s.savedFilters == nil {
		return nil, apperror.New(apperror.KindInternal, "saved filter repository is unavailable")
	}
	return s.savedFilters.ListSavedFilters(ctx, ownerID)
}

func (s *IssueService) DeleteSavedFilter(ctx context.Context, ownerID, filterID uuid.UUID) error {
	if s.savedFilters == nil {
		return apperror.New(apperror.KindInternal, "saved filter repository is unavailable")
	}
	return s.savedFilters.DeleteSavedFilter(ctx, ownerID, filterID)
}

func (s *IssueService) List(ctx context.Context, actorID uuid.UUID, globalRole string, filter port.ListFilter) (port.ListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	empty := port.ListResult{Items: []*domain.Issue{}, Page: filter.Page, PageSize: filter.PageSize}
	if globalRole != "admin" {
		if s.projectRoles == nil {
			return empty, nil
		}
		projectIDs, err := s.projectRoles.ListProjectIDs(ctx, actorID)
		if err != nil {
			return port.ListResult{}, err
		}
		if len(projectIDs) == 0 {
			return empty, nil
		}
		filter.ProjectIDs = projectIDs
	}
	items, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return port.ListResult{}, err
	}
	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return port.ListResult{}, err
	}
	return port.ListResult{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// UserNames devuelve el nombre visible de cada usuario indicado.
func (s *IssueService) UserNames(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	return s.directory.NamesOf(ctx, userIDs)
}

func (s *IssueService) UpdateStatus(ctx context.Context, id, actorID uuid.UUID, globalRole string, status domain.Status) (*domain.Issue, error) {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}

	if s.workflows != nil {
		transitionRole, err := s.transitionRole(ctx, issue.ProjectID, actorID, globalRole)
		if err != nil {
			return nil, err
		}
		allowed, err := s.workflows.CanTransition(ctx, issue.ProjectID, issue.Status, status, transitionRole)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, domain.ErrIllegalTransition
		}
	}

	if err := issue.ChangeStatus(status); err != nil {
		return nil, err
	}
	if err := s.applyResolution(ctx, issue); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, issue); err != nil {
		return nil, err
	}

	return issue, nil
}

func (s *IssueService) Assign(ctx context.Context, id, assignerID, assigneeID uuid.UUID, globalRole string) (*domain.Issue, error) {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, issue.ProjectID, assignerID, globalRole); err != nil {
		return nil, err
	}

	sameTeam, err := s.directory.SharesTeam(ctx, assignerID, assigneeID)
	if err != nil {
		return nil, err
	}
	if !sameTeam {
		return nil, domain.ErrCrossTeamAssignment
	}

	previous := issue.AssigneeID
	if err := issue.AssignTo(assigneeID); err != nil {
		return nil, err
	}

	// La asignacion, el seguimiento y el historial son una sola unidad: si el historial falla, la asignacion se deshace.
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if s.assigneeWatch != nil {
			if err := s.assigneeWatch.UpdateAndWatchAssignee(ctx, issue, assigneeID); err != nil {
				return err
			}
		} else {
			if err := s.repo.Update(ctx, issue); err != nil {
				return err
			}
			if s.watchers != nil {
				if err := s.watchers.Add(ctx, id, assigneeID); err != nil {
					return err
				}
			}
		}
		return s.recordAssigneeChange(ctx, id, assignerID, previous, &assigneeID)
	})
	if err != nil {
		return nil, err
	}

	s.notifyAssignment(ctx, issue, assignerID, assigneeID)
	return issue, nil
}

// Unassign quita la persona asignada con el mismo permiso que asignar (edit_issue). Si la incidencia ya
// no tiene asignada, devuelve el estado actual sin escribir nada: la operacion es idempotente.
// La desasignacion y su entrada de historial se escriben en una sola transaccion.
func (s *IssueService) Unassign(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Issue, error) {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if issue.AssigneeID == nil {
		return issue, nil
	}

	previous := *issue.AssigneeID
	issue.ClearAssignee()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, issue); err != nil {
			return err
		}
		return s.recordAssigneeChange(ctx, id, actorID, &previous, nil)
	})
	if err != nil {
		return nil, err
	}
	return issue, nil
}

// recordAssigneeChange guarda la entrada de historial de un cambio de persona asignada. No escribe nada si
// la persona no cambia o si no hay repositorio de colaboracion.
func (s *IssueService) recordAssigneeChange(ctx context.Context, issueID, actorID uuid.UUID, previous, next *uuid.UUID) error {
	if s.collaboration == nil || sameAssignee(previous, next) {
		return nil
	}
	return s.collaboration.SaveActivity(ctx, domain.NewAssigneeActivity(issueID, actorID, previous, next))
}

func sameAssignee(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// UpdateMetadata is the legacy full-replace of type, parent, due date and story points. It runs through Patch,
// so it gets hierarchy rules, change tracking and the stale-free write of every other update.
func (s *IssueService) UpdateMetadata(ctx context.Context, id, actorID uuid.UUID, globalRole string, input port.UpdateMetadataInput) (*domain.Issue, error) {
	patch := port.PatchInput{
		ParentID:    optionalField(input.ParentID),
		DueDate:     optionalField(input.DueDate),
		StoryPoints: optionalField(input.StoryPoints),
	}
	setLegacyType(&patch, input.IssueType, input.IssueTypeID)
	result, err := s.Patch(ctx, id, actorID, globalRole, patch)
	if err != nil {
		return nil, err
	}
	return result.Issue, nil
}

// UpdateDetails is the legacy full-replace of the editable fields. It runs through Patch.
func (s *IssueService) UpdateDetails(ctx context.Context, id, actorID uuid.UUID, globalRole string, input port.UpdateDetailsInput) (*domain.Issue, error) {
	patch := port.PatchInput{
		Summary:     port.Some(input.Summary),
		Description: port.Some(input.Description),
		ParentID:    optionalField(input.ParentID),
		DueDate:     optionalField(input.DueDate),
		StoryPoints: optionalField(input.StoryPoints),
		Custom:      input.Custom,
	}
	if input.PriorityID != nil {
		patch.PriorityID = port.Some(*input.PriorityID)
	} else if input.Priority != "" {
		patch.Priority = port.Some(input.Priority)
	}
	setLegacyType(&patch, input.IssueType, input.IssueTypeID)
	result, err := s.Patch(ctx, id, actorID, globalRole, patch)
	if err != nil {
		return nil, err
	}
	return result.Issue, nil
}

// optionalField turns a legacy optional value into a set field: an absent value clears the field.
func optionalField[T any](v *T) port.Field[T] {
	return port.Field[T]{Set: true, Value: v}
}

func setLegacyType(patch *port.PatchInput, legacy domain.IssueType, id *uuid.UUID) {
	switch {
	case id != nil:
		patch.IssueTypeID = port.Some(*id)
	case legacy != "":
		patch.IssueType = port.Some(legacy)
	}
}

func extractMentionNames(body string) []string {
	re := regexp.MustCompile(`@([a-zA-Z0-9._-]+)`)
	matches := re.FindAllStringSubmatch(body, -1)
	names := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		name := strings.ToLower(strings.TrimSpace(match[1]))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func (s *IssueService) ListComments(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.Comment, error) {
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.collaboration == nil {
		return []*domain.Comment{}, nil
	}
	return s.collaboration.FindComments(ctx, issueID)
}

func (s *IssueService) CreateComment(ctx context.Context, in port.CreateCommentInput) (*domain.Comment, error) {
	if s.collaboration == nil {
		return nil, domain.ErrInvalidComment
	}
	issue, err := s.repo.FindByID(ctx, in.IssueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireComment(ctx, issue.ProjectID, in.AuthorID, in.GlobalRole); err != nil {
		return nil, err
	}
	comment, err := domain.NewComment(in.IssueID, in.AuthorID, in.Body)
	if err != nil {
		return nil, err
	}
	comment.Mentions, err = s.collaboration.FindMentionIDs(ctx, in.IssueID, extractMentionNames(in.Body))
	if err != nil {
		return nil, err
	}
	if err := s.collaboration.SaveComment(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *IssueService) UpdateComment(ctx context.Context, commentID, actorID uuid.UUID, globalRole, body string) (*domain.Comment, error) {
	if s.collaboration == nil {
		return nil, domain.ErrInvalidComment
	}
	existing, err := s.collaboration.FindComment(ctx, commentID)
	if err != nil {
		return nil, err
	}
	issue, err := s.repo.FindByID(ctx, existing.IssueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireComment(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	comment := &domain.Comment{ID: commentID, AuthorID: actorID}
	if err := comment.Update(body); err != nil {
		return nil, err
	}
	comment.Mentions, _ = s.collaboration.FindMentionIDs(ctx, uuid.Nil, extractMentionNames(body))
	if err := s.collaboration.UpdateComment(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *IssueService) DeleteComment(ctx context.Context, commentID, actorID uuid.UUID, globalRole string) error {
	if s.collaboration == nil {
		return domain.ErrInvalidComment
	}
	existing, err := s.collaboration.FindComment(ctx, commentID)
	if err != nil {
		return err
	}
	issue, err := s.repo.FindByID(ctx, existing.IssueID)
	if err != nil {
		return err
	}
	if err := s.requireComment(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	comment := &domain.Comment{ID: commentID, AuthorID: actorID}
	comment.Delete()
	return s.collaboration.DeleteComment(ctx, comment)
}

func (s *IssueService) ListActivities(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.Activity, error) {
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.collaboration == nil {
		return []*domain.Activity{}, nil
	}
	return s.collaboration.FindActivities(ctx, issueID)
}

func (s *IssueService) UpdateRank(ctx context.Context, id, actorID uuid.UUID, globalRole, rank string) (*domain.Issue, error) {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if err := issue.UpdateRank(rank); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, issue); err != nil {
		return nil, err
	}
	return issue, nil
}

func (s *IssueService) requireIssueEdit(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "edit_issue", projectID, actorID, globalRole)
}

func (s *IssueService) requireIssueView(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "view_project", projectID, actorID, globalRole)
}

func (s *IssueService) requireIssueCreate(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "create_issue", projectID, actorID, globalRole)
}

func (s *IssueService) requireComment(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "comment", projectID, actorID, globalRole)
}

// authorize delega la decision de acceso al autorizador del proyecto. Sin autorizador se deniega.
func (s *IssueService) authorize(ctx context.Context, action string, projectID, actorID uuid.UUID, globalRole string) error {
	if s.authorizer == nil {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return s.authorizer.Authorize(ctx, action, actorID, globalRole, projectID)
}

// transitionRole devuelve el rol con el que el flujo evalua una transicion: admin global, o el rol
// del actor en el proyecto (ADR-003). Un no miembro obtiene cadena vacia, que el flujo nunca permite.
func (s *IssueService) transitionRole(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) (string, error) {
	if globalRole == "admin" {
		return "admin", nil
	}
	if s.projectRoles == nil {
		return "", nil
	}
	role, err := s.projectRoles.FindRole(ctx, projectID, actorID)
	if err != nil {
		return "", err
	}
	if role == nil {
		return "", nil
	}
	return *role, nil
}

// DefaultWorkflow devuelve el flujo por defecto del proyecto. Lo puede leer quien puede ver el proyecto.
func (s *IssueService) DefaultWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (*port.Workflow, error) {
	if err := s.requireIssueView(ctx, projectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.workflows == nil {
		return nil, apperror.New(apperror.KindNotFound, "workflow not found")
	}
	workflow, err := s.workflows.DefaultWorkflow(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return nil, apperror.New(apperror.KindNotFound, "workflow not found")
	}
	return workflow, nil
}

// allowedWorkflowRoles son los roles de proyecto que pueden aparecer en una transicion.
var allowedWorkflowRoles = map[string]bool{"admin": true, "lead": true, "contributor": true, "viewer": true}

// UpdateDefaultWorkflow reemplaza las transiciones del flujo por defecto. Solo el lider del proyecto
// y el administrador global pueden editarlo. Valida estados, roles y duplicados antes de guardar.
func (s *IssueService) UpdateDefaultWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, transitions []port.WorkflowTransition) (*port.Workflow, error) {
	if err := s.authorize(ctx, port.ActionManageWorkflow, projectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.workflows == nil {
		return nil, apperror.New(apperror.KindNotFound, "workflow not found")
	}
	current, err := s.workflows.DefaultWorkflow(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, apperror.New(apperror.KindNotFound, "workflow not found")
	}
	seen := make(map[[2]domain.Status]bool, len(transitions))
	for _, transition := range transitions {
		if !transition.From.IsValid() || !transition.To.IsValid() {
			return nil, apperror.New(apperror.KindValidation, "invalid status in transition")
		}
		if transition.From == transition.To {
			return nil, apperror.New(apperror.KindValidation, "a transition must change the status")
		}
		key := [2]domain.Status{transition.From, transition.To}
		if seen[key] {
			return nil, apperror.New(apperror.KindValidation, "duplicated transition")
		}
		seen[key] = true
		if len(transition.Roles) == 0 {
			return nil, apperror.New(apperror.KindValidation, "a transition needs at least one role")
		}
		for _, role := range transition.Roles {
			if !allowedWorkflowRoles[role] {
				return nil, apperror.New(apperror.KindValidation, "invalid role in transition")
			}
		}
	}
	if err := s.workflows.ReplaceTransitions(ctx, current.ID, transitions); err != nil {
		return nil, err
	}
	return s.workflows.DefaultWorkflow(ctx, projectID)
}

// MoveIssueToStatus cambia el estado de una incidencia del proyecto indicado. Reutiliza UpdateStatus,
// asi que aplica permisos de edicion y reglas del flujo. Una incidencia de otro proyecto no existe aqui.
func (s *IssueService) MoveIssueToStatus(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID uuid.UUID, status domain.Status) error {
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return err
	}
	if issue.ProjectID != projectID {
		return domain.ErrIssueNotFound
	}
	_, err = s.UpdateStatus(ctx, issueID, actorID, globalRole, status)
	return err
}

// Backlog devuelve las incidencias del proyecto en el orden de su rank. Lo puede leer quien puede ver el proyecto.
func (s *IssueService) Backlog(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.Issue, error) {
	if err := s.requireIssueView(ctx, projectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.backlog == nil {
		return nil, apperror.New(apperror.KindInternal, "backlog is not available")
	}
	return s.backlog.ListBacklog(ctx, projectID)
}

// MoveInBacklog coloca una incidencia en una posicion del backlog (permiso move_cards). Renumera el rank de
// todas las incidencias del proyecto con el orden de la lista, asi el orden queda explicito y estable.
func (s *IssueService) MoveInBacklog(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID uuid.UUID, position int) ([]*domain.Issue, error) {
	if err := s.authorize(ctx, port.ActionMoveCards, projectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.backlog == nil {
		return nil, apperror.New(apperror.KindInternal, "backlog is not available")
	}
	items, err := s.backlog.ListBacklog(ctx, projectID)
	if err != nil {
		return nil, err
	}
	from := -1
	for i, item := range items {
		if item.ID == issueID {
			from = i
		}
	}
	if from < 0 {
		return nil, domain.ErrIssueNotFound
	}
	if position < 0 || position >= len(items) {
		return nil, apperror.New(apperror.KindValidation, "invalid backlog position")
	}

	moved := items[from]
	rest := make([]*domain.Issue, 0, len(items)-1)
	for i, item := range items {
		if i != from {
			rest = append(rest, item)
		}
	}
	reordered := make([]*domain.Issue, 0, len(items))
	reordered = append(reordered, rest[:position]...)
	reordered = append(reordered, moved)
	reordered = append(reordered, rest[position:]...)

	ranks := make(map[uuid.UUID]string)
	for i, item := range reordered {
		rank := fmt.Sprintf("%06d", i+1)
		if item.Rank != rank {
			ranks[item.ID] = rank
		}
	}
	if len(ranks) > 0 {
		if err := s.backlog.SaveRanks(ctx, projectID, ranks); err != nil {
			return nil, err
		}
	}
	for _, item := range reordered {
		if rank, ok := ranks[item.ID]; ok {
			if err := item.UpdateRank(rank); err != nil {
				return nil, err
			}
		}
	}
	return reordered, nil
}

// WithNotifier conecta el aviso in-app al asignar incidencias. Sin notificador, asignar no avisa.
func (s *IssueService) WithNotifier(notifier port.Notifier) *IssueService {
	s.notifier = notifier
	return s
}

// notifyAssignment avisa a la persona asignada. Es best-effort: un fallo al avisar no deshace la asignacion.
func (s *IssueService) notifyAssignment(ctx context.Context, issue *domain.Issue, assignerID, assigneeID uuid.UUID) {
	if s.notifier == nil || assignerID == assigneeID {
		return
	}
	_ = s.notifier.Notify(ctx, assigneeID, "issue_assigned", "Te asignaron la incidencia: "+issue.Summary, "/issues/"+issue.ID.String())
}
