package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

// noTxManager ejecuta fn sin transaccion. Es el valor por defecto del servicio; el composition root
// inyecta el TxManager real con WithTxManager.
type noTxManager struct{}

func (noTxManager) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type TicketService struct {
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
}

func NewTicketService(repo port.Repository, directory port.UserDirectory, repositories ...any) *TicketService {
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
	return &TicketService{tx: noTxManager{}, repo: repo, directory: directory, collaboration: collaboration, issueLinks: issueLinks, workflows: workflows, projectRoles: projectRoles, authorizer: authorizer, watchers: watchers, assigneeWatch: assigneeWatch, savedFilters: savedFilters, backlog: backlog}
}

var _ port.Service = (*TicketService)(nil)

// WithTxManager fija el gestor de transacciones de las escrituras multiples (asignar y desasignar).
// Sin el, las escrituras se ejecutan sin transaccion.
func (s *TicketService) WithTxManager(tx database.TxManager) *TicketService {
	if tx == nil {
		tx = noTxManager{}
	}
	s.tx = tx
	return s
}

func (s *TicketService) Create(ctx context.Context, in port.CreateInput) (*domain.Ticket, error) {
	ticket, err := domain.NewTicket(in.Title, in.Description, in.Priority, in.RequesterID)
	if err != nil {
		return nil, err
	}
	ticket.ProjectID = domain.DefaultProjectID
	if in.ProjectID != nil {
		ticket.ProjectID = *in.ProjectID
	}
	if err := s.requireIssueCreate(ctx, ticket.ProjectID, in.RequesterID, in.GlobalRole); err != nil {
		return nil, err
	}
	if in.IssueType != "" {
		if err := ticket.UpdateMetadata(in.IssueType, in.ParentID, in.DueDate, in.StoryPoints); err != nil {
			return nil, err
		}
	}

	if err := s.repo.Save(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *TicketService) Get(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) AuthorizeIssueEdit(ctx context.Context, id, actorID uuid.UUID, globalRole string) error {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole)
}

func (s *TicketService) GetWatcherState(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.WatcherState, error) {
	if s.watchers == nil {
		return port.WatcherState{}, apperror.New(apperror.KindInternal, "watcher repository is unavailable")
	}
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.WatcherState{}, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return port.WatcherState{}, err
	}
	return s.watchers.State(ctx, issueID, actorID)
}

func (s *TicketService) WatchIssue(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.WatcherState, error) {
	if s.watchers == nil {
		return port.WatcherState{}, apperror.New(apperror.KindInternal, "watcher repository is unavailable")
	}
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.WatcherState{}, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return port.WatcherState{}, err
	}
	if err := s.watchers.Add(ctx, issueID, actorID); err != nil {
		return port.WatcherState{}, err
	}
	return s.watchers.State(ctx, issueID, actorID)
}

func (s *TicketService) UnwatchIssue(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.WatcherState, error) {
	if s.watchers == nil {
		return port.WatcherState{}, apperror.New(apperror.KindInternal, "watcher repository is unavailable")
	}
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.WatcherState{}, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return port.WatcherState{}, err
	}
	if err := s.watchers.Remove(ctx, issueID, actorID); err != nil {
		return port.WatcherState{}, err
	}
	return s.watchers.State(ctx, issueID, actorID)
}

func (s *TicketService) CreateSavedFilter(ctx context.Context, ownerID uuid.UUID, name string, criteria domain.SavedFilterCriteria) (*domain.SavedFilter, error) {
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

func (s *TicketService) ListSavedFilters(ctx context.Context, ownerID uuid.UUID) ([]*domain.SavedFilter, error) {
	if s.savedFilters == nil {
		return nil, apperror.New(apperror.KindInternal, "saved filter repository is unavailable")
	}
	return s.savedFilters.ListSavedFilters(ctx, ownerID)
}

func (s *TicketService) DeleteSavedFilter(ctx context.Context, ownerID, filterID uuid.UUID) error {
	if s.savedFilters == nil {
		return apperror.New(apperror.KindInternal, "saved filter repository is unavailable")
	}
	return s.savedFilters.DeleteSavedFilter(ctx, ownerID, filterID)
}

func (s *TicketService) List(ctx context.Context, actorID uuid.UUID, globalRole string, filter port.ListFilter) (port.ListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	empty := port.ListResult{Items: []*domain.Ticket{}, Page: filter.Page, PageSize: filter.PageSize}
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
func (s *TicketService) UserNames(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	return s.directory.NamesOf(ctx, userIDs)
}

func (s *TicketService) UpdateStatus(ctx context.Context, id, actorID uuid.UUID, globalRole string, status domain.Status) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}

	if s.workflows != nil {
		transitionRole, err := s.transitionRole(ctx, ticket.ProjectID, actorID, globalRole)
		if err != nil {
			return nil, err
		}
		allowed, err := s.workflows.CanTransition(ctx, ticket.ProjectID, ticket.Status, status, transitionRole)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, domain.ErrIllegalTransition
		}
	}

	if err := ticket.ChangeStatus(status); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *TicketService) Assign(ctx context.Context, id, assignerID, assigneeID uuid.UUID, globalRole string) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, assignerID, globalRole); err != nil {
		return nil, err
	}

	sameTeam, err := s.directory.SharesTeam(ctx, assignerID, assigneeID)
	if err != nil {
		return nil, err
	}
	if !sameTeam {
		return nil, domain.ErrCrossTeamAssignment
	}

	previous := ticket.AssigneeID
	if err := ticket.AssignTo(assigneeID); err != nil {
		return nil, err
	}

	// La asignacion, el seguimiento y el historial son una sola unidad: si el historial falla, la asignacion se deshace.
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if s.assigneeWatch != nil {
			if err := s.assigneeWatch.UpdateAndWatchAssignee(ctx, ticket, assigneeID); err != nil {
				return err
			}
		} else {
			if err := s.repo.Update(ctx, ticket); err != nil {
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

	s.notifyAssignment(ctx, ticket, assignerID, assigneeID)
	return ticket, nil
}

// Unassign quita la persona asignada con el mismo permiso que asignar (edit_issue). Si la incidencia ya
// no tiene asignada, devuelve el estado actual sin escribir nada: la operacion es idempotente.
// La desasignacion y su entrada de historial se escriben en una sola transaccion.
func (s *TicketService) Unassign(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if ticket.AssigneeID == nil {
		return ticket, nil
	}

	previous := *ticket.AssigneeID
	ticket.ClearAssignee()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, ticket); err != nil {
			return err
		}
		return s.recordAssigneeChange(ctx, id, actorID, &previous, nil)
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

// recordAssigneeChange guarda la entrada de historial de un cambio de persona asignada. No escribe nada si
// la persona no cambia o si no hay repositorio de colaboracion.
func (s *TicketService) recordAssigneeChange(ctx context.Context, issueID, actorID uuid.UUID, previous, next *uuid.UUID) error {
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

func (s *TicketService) UpdateMetadata(ctx context.Context, id, actorID uuid.UUID, globalRole string, input port.UpdateMetadataInput) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if err := ticket.UpdateMetadata(input.IssueType, input.ParentID, input.DueDate, input.StoryPoints); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) UpdateDetails(ctx context.Context, id, actorID uuid.UUID, globalRole string, input port.UpdateDetailsInput) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if err := ticket.UpdateDetails(input.Title, input.Description, input.Priority); err != nil {
		return nil, err
	}
	if err := ticket.UpdateMetadata(input.IssueType, input.ParentID, input.DueDate, input.StoryPoints); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
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

func (s *TicketService) ListComments(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.Comment, error) {
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.collaboration == nil {
		return []*domain.Comment{}, nil
	}
	return s.collaboration.FindComments(ctx, issueID)
}

func (s *TicketService) CreateComment(ctx context.Context, in port.CreateCommentInput) (*domain.Comment, error) {
	if s.collaboration == nil {
		return nil, domain.ErrInvalidComment
	}
	ticket, err := s.repo.FindByID(ctx, in.IssueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireComment(ctx, ticket.ProjectID, in.AuthorID, in.GlobalRole); err != nil {
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

func (s *TicketService) UpdateComment(ctx context.Context, commentID, actorID uuid.UUID, globalRole, body string) (*domain.Comment, error) {
	if s.collaboration == nil {
		return nil, domain.ErrInvalidComment
	}
	existing, err := s.collaboration.FindComment(ctx, commentID)
	if err != nil {
		return nil, err
	}
	ticket, err := s.repo.FindByID(ctx, existing.IssueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireComment(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
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

func (s *TicketService) DeleteComment(ctx context.Context, commentID, actorID uuid.UUID, globalRole string) error {
	if s.collaboration == nil {
		return domain.ErrInvalidComment
	}
	existing, err := s.collaboration.FindComment(ctx, commentID)
	if err != nil {
		return err
	}
	ticket, err := s.repo.FindByID(ctx, existing.IssueID)
	if err != nil {
		return err
	}
	if err := s.requireComment(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	comment := &domain.Comment{ID: commentID, AuthorID: actorID}
	comment.Delete()
	return s.collaboration.DeleteComment(ctx, comment)
}

func (s *TicketService) ListActivities(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.Activity, error) {
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.collaboration == nil {
		return []*domain.Activity{}, nil
	}
	return s.collaboration.FindActivities(ctx, issueID)
}

func (s *TicketService) ListIssueLinks(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.IssueLink, error) {
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueView(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.issueLinks == nil {
		return []*domain.IssueLink{}, nil
	}
	return s.issueLinks.ListByIssue(ctx, issueID)
}

func (s *TicketService) CreateIssueLink(ctx context.Context, sourceIssueID, targetIssueID, createdBy uuid.UUID, globalRole string, kind domain.IssueLinkKind) (*domain.IssueLink, error) {
	if s.issueLinks == nil {
		return nil, domain.ErrInvalidIssueLink
	}
	source, err := s.repo.FindByID(ctx, sourceIssueID)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.FindByID(ctx, targetIssueID)
	if err != nil {
		return nil, err
	}
	if source.ProjectID != target.ProjectID {
		return nil, domain.ErrInvalidIssueLink
	}
	if err := s.requireIssueEdit(ctx, source.ProjectID, createdBy, globalRole); err != nil {
		return nil, err
	}
	link, err := domain.NewIssueLink(sourceIssueID, targetIssueID, createdBy, kind)
	if err != nil {
		return nil, err
	}
	if err := s.issueLinks.Save(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

func (s *TicketService) DeleteIssueLink(ctx context.Context, issueID, linkID, actorID uuid.UUID, globalRole string) error {
	if s.issueLinks == nil {
		return domain.ErrInvalidIssueLink
	}
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	return s.issueLinks.Delete(ctx, issueID, linkID)
}

func (s *TicketService) UpdateRank(ctx context.Context, id, actorID uuid.UUID, globalRole, rank string) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if err := ticket.UpdateRank(rank); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) requireIssueEdit(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "edit_issue", projectID, actorID, globalRole)
}

func (s *TicketService) requireIssueView(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "view_project", projectID, actorID, globalRole)
}

func (s *TicketService) requireIssueCreate(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "create_issue", projectID, actorID, globalRole)
}

func (s *TicketService) requireComment(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	return s.authorize(ctx, "comment", projectID, actorID, globalRole)
}

// authorize delega la decision de acceso al autorizador del proyecto. Sin autorizador se deniega.
func (s *TicketService) authorize(ctx context.Context, action string, projectID, actorID uuid.UUID, globalRole string) error {
	if s.authorizer == nil {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return s.authorizer.Authorize(ctx, action, actorID, globalRole, projectID)
}

// transitionRole devuelve el rol con el que el flujo evalua una transicion: admin global, o el rol
// del actor en el proyecto (ADR-003). Un no miembro obtiene cadena vacia, que el flujo nunca permite.
func (s *TicketService) transitionRole(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) (string, error) {
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
func (s *TicketService) DefaultWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (*port.Workflow, error) {
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
func (s *TicketService) UpdateDefaultWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, transitions []port.WorkflowTransition) (*port.Workflow, error) {
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
func (s *TicketService) MoveIssueToStatus(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID uuid.UUID, status domain.Status) error {
	ticket, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return err
	}
	if ticket.ProjectID != projectID {
		return domain.ErrTicketNotFound
	}
	_, err = s.UpdateStatus(ctx, issueID, actorID, globalRole, status)
	return err
}

// Backlog devuelve las incidencias del proyecto en el orden de su rank. Lo puede leer quien puede ver el proyecto.
func (s *TicketService) Backlog(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.Ticket, error) {
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
func (s *TicketService) MoveInBacklog(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID uuid.UUID, position int) ([]*domain.Ticket, error) {
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
		return nil, domain.ErrTicketNotFound
	}
	if position < 0 || position >= len(items) {
		return nil, apperror.New(apperror.KindValidation, "invalid backlog position")
	}

	moved := items[from]
	rest := make([]*domain.Ticket, 0, len(items)-1)
	for i, item := range items {
		if i != from {
			rest = append(rest, item)
		}
	}
	reordered := make([]*domain.Ticket, 0, len(items))
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
func (s *TicketService) WithNotifier(notifier port.Notifier) *TicketService {
	s.notifier = notifier
	return s
}

// notifyAssignment avisa a la persona asignada. Es best-effort: un fallo al avisar no deshace la asignacion.
func (s *TicketService) notifyAssignment(ctx context.Context, ticket *domain.Ticket, assignerID, assigneeID uuid.UUID) {
	if s.notifier == nil || assignerID == assigneeID {
		return
	}
	_ = s.notifier.Notify(ctx, assigneeID, "issue_assigned", "Te asignaron la incidencia: "+ticket.Title, "/issues/"+ticket.ID.String())
}
