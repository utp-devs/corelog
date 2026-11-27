package port

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

type Repository interface {
	Save(ctx context.Context, t *domain.Issue) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Issue, error)
	FindAll(ctx context.Context, filter ListFilter) ([]*domain.Issue, error)
	Count(ctx context.Context, filter ListFilter) (int, error)
	Update(ctx context.Context, t *domain.Issue) error
	// FindKeyCandidates returns the issues numbered number in live projects with the given key that the user can
	// reach (organization member, project member or team grant; allOrgs skips that test for platform admins).
	// orgID, when not uuid.Nil, restricts the search to one organization. Visibility is applied by the caller.
	FindKeyCandidates(ctx context.Context, projectKey string, number int, orgID, userID uuid.UUID, allOrgs bool) ([]*domain.Issue, error)
}

type UserDirectory interface {
	// SharesTeam indica si los dos usuarios pertenecen a al menos un mismo equipo.
	SharesTeam(ctx context.Context, userA, userB uuid.UUID) (bool, error)
	NamesOf(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error)
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

type WorkflowRepository interface {
	CanTransition(ctx context.Context, projectID uuid.UUID, fromStatus, toStatus domain.Status, actorRole string) (bool, error)
	DefaultWorkflow(ctx context.Context, projectID uuid.UUID) (*Workflow, error)
	ReplaceTransitions(ctx context.Context, workflowID uuid.UUID, transitions []WorkflowTransition) error
}

// ActionManageWorkflow es la accion del autorizador para editar el flujo de un proyecto.
const ActionManageWorkflow = "manage_workflow"

// ActionMoveCards es la accion del autorizador para mover incidencias (columnas y backlog).
const ActionMoveCards = "move_cards"

// Workflow es el flujo por defecto de un proyecto con sus transiciones permitidas.
type Workflow struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Name        string
	Transitions []WorkflowTransition
}

// WorkflowTransition es un cambio de estado permitido y los roles de proyecto que lo pueden ejecutar.
type WorkflowTransition struct {
	From  domain.Status
	To    domain.Status
	Roles []string
}

type IssueWatcherRepository interface {
	State(context.Context, uuid.UUID, uuid.UUID) (WatcherState, error)
	Add(context.Context, uuid.UUID, uuid.UUID) error
	Remove(context.Context, uuid.UUID, uuid.UUID) error
}

type AssigneeWatcherRepository interface {
	UpdateAndWatchAssignee(context.Context, *domain.Issue, uuid.UUID) error
}

type SavedFilterRepository interface {
	SaveSavedFilter(context.Context, *domain.SavedFilter) error
	ListSavedFilters(context.Context, uuid.UUID) ([]*domain.SavedFilter, error)
	DeleteSavedFilter(context.Context, uuid.UUID, uuid.UUID) error
}

type WatcherState struct {
	IsWatching bool
	Count      int
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

// ProjectAuthorizer decide si un actor puede ejecutar una accion sobre un proyecto. Las acciones
// usan los nombres de la matriz de permisos (view_project, create_issue, edit_issue, comment).
type ProjectAuthorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

type ProjectRoleResolver interface {
	FindRole(ctx context.Context, projectID, userID uuid.UUID) (*string, error)
	ListProjectIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

// BacklogRepository ordena las incidencias de un proyecto por rank y guarda el nuevo orden.
type BacklogRepository interface {
	ListBacklog(ctx context.Context, projectID uuid.UUID) ([]*domain.Issue, error)
	SaveRanks(ctx context.Context, projectID uuid.UUID, ranks map[uuid.UUID]string) error
}

// Notifier avisa a una persona de un evento del modulo issue. Lo implementa el modulo notification.
type Notifier interface {
	Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error
}
