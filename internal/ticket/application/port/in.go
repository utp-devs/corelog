package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type CreateInput struct {
	Title       string
	Description string
	Priority    domain.Priority
	RequesterID uuid.UUID
	GlobalRole  string
	IssueType   domain.IssueType
	ParentID    *uuid.UUID
	DueDate     *time.Time
	StoryPoints *float64
}

type UpdateMetadataInput struct {
	IssueType   domain.IssueType
	ParentID    *uuid.UUID
	DueDate     *time.Time
	StoryPoints *float64
}

type UpdateDetailsInput struct {
	Title       string
	Description string
	Priority    domain.Priority
	IssueType   domain.IssueType
	ParentID    *uuid.UUID
	DueDate     *time.Time
	StoryPoints *float64
}

type CreateCommentInput struct {
	IssueID    uuid.UUID
	AuthorID   uuid.UUID
	GlobalRole string
	Body       string
}

type ListFilter struct {
	ProjectIDs []uuid.UUID
	Status     domain.Status
	Priority   domain.Priority
	Query      string
	IssueType  domain.IssueType
	LabelID    uuid.UUID
	AssigneeID *uuid.UUID
	DueFrom    *time.Time
	DueTo      *time.Time
	Page       int
	PageSize   int
}

type Service interface {
	Create(ctx context.Context, in CreateInput) (*domain.Ticket, error)
	Get(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Ticket, error)
	List(ctx context.Context, actorID uuid.UUID, globalRole string, filter ListFilter) ([]*domain.Ticket, error)
	UpdateStatus(ctx context.Context, id, actorID uuid.UUID, globalRole string, status domain.Status) (*domain.Ticket, error)
	Assign(ctx context.Context, id, assignerID, assigneeID uuid.UUID, globalRole string) (*domain.Ticket, error)
	UpdateMetadata(ctx context.Context, id, actorID uuid.UUID, globalRole string, input UpdateMetadataInput) (*domain.Ticket, error)
	UpdateDetails(ctx context.Context, id, actorID uuid.UUID, globalRole string, input UpdateDetailsInput) (*domain.Ticket, error)
	ListComments(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.Comment, error)
	CreateComment(ctx context.Context, in CreateCommentInput) (*domain.Comment, error)
	UpdateComment(ctx context.Context, commentID, actorID uuid.UUID, globalRole, body string) (*domain.Comment, error)
	DeleteComment(ctx context.Context, commentID, actorID uuid.UUID, globalRole string) error
	ListActivities(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.Activity, error)
	AuthorizeIssueEdit(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) error
	ListIssueLinks(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]*domain.IssueLink, error)
	CreateIssueLink(ctx context.Context, sourceIssueID, targetIssueID, createdBy uuid.UUID, globalRole string, kind domain.IssueLinkKind) (*domain.IssueLink, error)
	DeleteIssueLink(ctx context.Context, issueID, linkID, actorID uuid.UUID, globalRole string) error
	UpdateRank(ctx context.Context, id, actorID uuid.UUID, globalRole, rank string) (*domain.Ticket, error)
}
