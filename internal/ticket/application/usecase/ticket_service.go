package usecase

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"

	authorization "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type TicketService struct {
	repo          port.Repository
	directory     port.UserDirectory
	collaboration port.CollaborationRepository
	issueLinks    port.IssueLinkRepository
	workflows     port.WorkflowRepository
	projectRoles  port.ProjectRoleResolver
	watchers      port.IssueWatcherRepository
	assigneeWatch port.AssigneeWatcherRepository
	savedFilters  port.SavedFilterRepository
	policy        authorization.Policy
}

func NewTicketService(repo port.Repository, directory port.UserDirectory, repositories ...any) *TicketService {
	var collaboration port.CollaborationRepository
	var issueLinks port.IssueLinkRepository
	var workflows port.WorkflowRepository
	var projectRoles port.ProjectRoleResolver
	var watchers port.IssueWatcherRepository
	var assigneeWatch port.AssigneeWatcherRepository
	var savedFilters port.SavedFilterRepository
	for _, repository := range repositories {
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
	return &TicketService{repo: repo, directory: directory, collaboration: collaboration, issueLinks: issueLinks, workflows: workflows, projectRoles: projectRoles, watchers: watchers, assigneeWatch: assigneeWatch, savedFilters: savedFilters}
}

var _ port.Service = (*TicketService)(nil)

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

func (s *TicketService) UpdateStatus(ctx context.Context, id, actorID uuid.UUID, globalRole string, status domain.Status) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIssueEdit(ctx, ticket.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}

	if s.workflows != nil {
		allowed, err := s.workflows.CanTransition(ctx, ticket.ProjectID, ticket.Status, status, globalRole)
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

	assignerTeam, err := s.directory.TeamOf(ctx, assignerID)
	if err != nil {
		return nil, err
	}

	assigneeTeam, err := s.directory.TeamOf(ctx, assigneeID)
	if err != nil {
		return nil, err
	}

	if assignerTeam == nil || assigneeTeam == nil || *assignerTeam != *assigneeTeam {
		return nil, domain.ErrCrossTeamAssignment
	}

	if err := ticket.AssignTo(assigneeID); err != nil {
		return nil, err
	}

	if s.assigneeWatch != nil {
		if err := s.assigneeWatch.UpdateAndWatchAssignee(ctx, ticket, assigneeID); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.Update(ctx, ticket); err != nil {
			return nil, err
		}
		if s.watchers != nil {
			if err := s.watchers.Add(ctx, id, assigneeID); err != nil {
				return nil, err
			}
		}
	}

	return ticket, nil
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
	role, err := s.projectRole(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if !s.policy.CanEditIssue(globalRole, role) {
		return apperror.New(apperror.KindForbidden, "forbidden issue operation")
	}
	return nil
}

func (s *TicketService) requireIssueView(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	role, err := s.projectRole(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if !s.policy.CanViewProject(globalRole, role) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

func (s *TicketService) requireIssueCreate(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	role, err := s.projectRole(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if !s.policy.CanCreateIssue(globalRole, role) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

func (s *TicketService) requireComment(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	role, err := s.projectRole(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if !s.policy.CanComment(globalRole, role) {
		return apperror.New(apperror.KindForbidden, "forbidden issue operation")
	}
	return nil
}

func (s *TicketService) projectRole(ctx context.Context, projectID, actorID uuid.UUID) (*authorization.ProjectRole, error) {
	if s.projectRoles == nil {
		return nil, nil
	}
	role, err := s.projectRoles.FindRole(ctx, projectID, actorID)
	if err != nil || role == nil {
		return nil, err
	}
	value := authorization.ProjectRole(*role)
	return &value, nil
}
