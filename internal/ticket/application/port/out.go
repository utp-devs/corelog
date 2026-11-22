package port

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type Repository interface {
	Save(ctx context.Context, t *domain.Ticket) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Ticket, error)
	FindAll(ctx context.Context, filter ListFilter) ([]*domain.Ticket, error)
	Update(ctx context.Context, t *domain.Ticket) error
}

type UserDirectory interface {
	TeamOf(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error)
}

type CollaborationRepository interface {
	FindComment(ctx context.Context, commentID uuid.UUID) (*domain.Comment, error)
	SaveComment(ctx context.Context, comment *domain.Comment) error
	FindComments(ctx context.Context, issueID uuid.UUID) ([]*domain.Comment, error)
	UpdateComment(ctx context.Context, comment *domain.Comment) error
	DeleteComment(ctx context.Context, comment *domain.Comment) error
	SaveActivity(ctx context.Context, activity *domain.Activity) error
	FindActivities(ctx context.Context, issueID uuid.UUID) ([]*domain.Activity, error)
	FindMentionIDs(ctx context.Context, issueID uuid.UUID, names []string) ([]uuid.UUID, error)
}

type IssueLinkRepository interface {
	Save(ctx context.Context, link *domain.IssueLink) error
	ListByIssue(ctx context.Context, issueID uuid.UUID) ([]*domain.IssueLink, error)
	Delete(ctx context.Context, issueID, linkID uuid.UUID) error
}

type WorkflowRepository interface {
	CanTransition(ctx context.Context, projectID uuid.UUID, fromStatus, toStatus domain.Status, actorRole string) (bool, error)
}

type AttachmentRepository interface {
	SaveAttachment(context.Context, *domain.Attachment) error
	FindAttachment(context.Context, uuid.UUID, uuid.UUID) (*domain.Attachment, error)
	ListAttachments(context.Context, uuid.UUID) ([]*domain.Attachment, error)
	DeleteAttachment(context.Context, uuid.UUID, uuid.UUID) error
}

type FileStorage interface {
	Save(context.Context, string, io.Reader) (int64, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

type ProjectRoleResolver interface {
	FindRole(ctx context.Context, projectID, userID uuid.UUID) (*string, error)
	ListProjectIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}
