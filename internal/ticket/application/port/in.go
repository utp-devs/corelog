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

type CreateCommentInput struct {
	IssueID  uuid.UUID
	AuthorID uuid.UUID
	Body     string
}

type ListFilter struct {
	Status   domain.Status
	Priority domain.Priority
	Page     int
	PageSize int
}

type Service interface {
	Create(ctx context.Context, in CreateInput) (*domain.Ticket, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Ticket, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.Ticket, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, actorRole ...string) (*domain.Ticket, error)
	Assign(ctx context.Context, id uuid.UUID, assignerID uuid.UUID, assigneeID uuid.UUID) (*domain.Ticket, error)
	UpdateMetadata(ctx context.Context, id uuid.UUID, input UpdateMetadataInput) (*domain.Ticket, error)
	ListComments(ctx context.Context, issueID uuid.UUID) ([]*domain.Comment, error)
	CreateComment(ctx context.Context, in CreateCommentInput) (*domain.Comment, error)
	UpdateComment(ctx context.Context, commentID, actorID uuid.UUID, body string) (*domain.Comment, error)
	DeleteComment(ctx context.Context, commentID, actorID uuid.UUID) error
	ListActivities(ctx context.Context, issueID uuid.UUID) ([]*domain.Activity, error)
	ListIssueLinks(ctx context.Context, issueID uuid.UUID) ([]*domain.IssueLink, error)
	CreateIssueLink(ctx context.Context, sourceIssueID, targetIssueID, createdBy uuid.UUID, kind domain.IssueLinkKind) (*domain.IssueLink, error)
	DeleteIssueLink(ctx context.Context, linkID uuid.UUID) error
	UpdateRank(ctx context.Context, id uuid.UUID, rank string) (*domain.Ticket, error)
}
