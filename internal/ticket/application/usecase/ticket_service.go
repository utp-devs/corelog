package usecase

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type TicketService struct {
	repo          port.Repository
	directory     port.UserDirectory
	collaboration port.CollaborationRepository
	issueLinks    port.IssueLinkRepository
}

func NewTicketService(repo port.Repository, directory port.UserDirectory, repositories ...any) *TicketService {
	var collaboration port.CollaborationRepository
	var issueLinks port.IssueLinkRepository
	for _, repository := range repositories {
		switch typed := repository.(type) {
		case port.CollaborationRepository:
			collaboration = typed
		case port.IssueLinkRepository:
			issueLinks = typed
		}
	}
	return &TicketService{repo: repo, directory: directory, collaboration: collaboration, issueLinks: issueLinks}
}

var _ port.Service = (*TicketService)(nil)

func (s *TicketService) Create(ctx context.Context, in port.CreateInput) (*domain.Ticket, error) {
	ticket, err := domain.NewTicket(in.Title, in.Description, in.Priority, in.RequesterID)
	if err != nil {
		return nil, err
	}
	ticket.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
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

func (s *TicketService) Get(ctx context.Context, id uuid.UUID) (*domain.Ticket, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *TicketService) List(ctx context.Context, filter port.ListFilter) ([]*domain.Ticket, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return s.repo.FindAll(ctx, filter)
}

func (s *TicketService) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := ticket.ChangeStatus(status); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *TicketService) Assign(ctx context.Context, id uuid.UUID, assignerID uuid.UUID, assigneeID uuid.UUID) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
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

	if err := s.repo.Update(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *TicketService) UpdateMetadata(ctx context.Context, id uuid.UUID, input port.UpdateMetadataInput) (*domain.Ticket, error) {
	ticket, err := s.repo.FindByID(ctx, id)
	if err != nil {
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

func (s *TicketService) ListComments(ctx context.Context, issueID uuid.UUID) ([]*domain.Comment, error) {
	if s.collaboration == nil {
		return []*domain.Comment{}, nil
	}
	return s.collaboration.FindComments(ctx, issueID)
}

func (s *TicketService) CreateComment(ctx context.Context, in port.CreateCommentInput) (*domain.Comment, error) {
	if s.collaboration == nil {
		return nil, domain.ErrInvalidComment
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

func (s *TicketService) UpdateComment(ctx context.Context, commentID, actorID uuid.UUID, body string) (*domain.Comment, error) {
	if s.collaboration == nil {
		return nil, domain.ErrInvalidComment
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

func (s *TicketService) DeleteComment(ctx context.Context, commentID, actorID uuid.UUID) error {
	if s.collaboration == nil {
		return domain.ErrInvalidComment
	}
	comment := &domain.Comment{ID: commentID, AuthorID: actorID}
	comment.Delete()
	return s.collaboration.DeleteComment(ctx, comment)
}

func (s *TicketService) ListActivities(ctx context.Context, issueID uuid.UUID) ([]*domain.Activity, error) {
	if s.collaboration == nil {
		return []*domain.Activity{}, nil
	}
	return s.collaboration.FindActivities(ctx, issueID)
}

func (s *TicketService) ListIssueLinks(ctx context.Context, issueID uuid.UUID) ([]*domain.IssueLink, error) {
	if s.issueLinks == nil {
		return []*domain.IssueLink{}, nil
	}
	return s.issueLinks.ListByIssue(ctx, issueID)
}

func (s *TicketService) CreateIssueLink(ctx context.Context, sourceIssueID, targetIssueID, createdBy uuid.UUID, kind domain.IssueLinkKind) (*domain.IssueLink, error) {
	if s.issueLinks == nil {
		return nil, domain.ErrInvalidIssueLink
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

func (s *TicketService) DeleteIssueLink(ctx context.Context, linkID uuid.UUID) error {
	if s.issueLinks == nil {
		return domain.ErrInvalidIssueLink
	}
	return s.issueLinks.Delete(ctx, linkID)
}
