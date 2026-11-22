package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

const (
	defaultVelocityLimit = 5
	maxVelocityLimit     = 20
)

// SprintService implementa los casos de uso de sprints. Los permisos se consultan al autorizador del
// proyecto: ver el proyecto para leer y el permiso de sprints para cambiar.
type SprintService struct {
	repo       port.Repository
	authorizer port.Authorizer
}

func NewSprintService(repo port.Repository, authorizer port.Authorizer) *SprintService {
	return &SprintService{repo: repo, authorizer: authorizer}
}

var _ port.Service = (*SprintService)(nil)

func (s *SprintService) Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, goal string, start, end *time.Time) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := domain.NewSprint(projectID, name, goal, start, end)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

func (s *SprintService) List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListByProject(ctx, projectID)
}

// Start activa un sprint planificado. Un proyecto solo puede tener un sprint activo a la vez.
func (s *SprintService) Start(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != domain.StatusPlanned {
		return nil, domain.ErrSprintNotPlanned
	}
	active, err := s.repo.HasActive(ctx, projectID, sprint.ID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrActiveSprintExists
	}
	if err := sprint.Start(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

// Close cierra un sprint activo. Las incidencias no cerradas van al backlog o a un sprint planificado
// del mismo proyecto, segun la opcion elegida.
func (s *SprintService) Close(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, input port.CloseInput) (*domain.Sprint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != domain.StatusActive {
		return nil, domain.ErrSprintNotActive
	}

	var target *uuid.UUID
	switch input.Unfinished {
	case "", port.UnfinishedToBacklog:
		target = nil
	case port.UnfinishedToSprint:
		if input.TargetSprintID == nil || *input.TargetSprintID == sprint.ID {
			return nil, domain.ErrInvalidCloseOption
		}
		next, err := s.findInProject(ctx, projectID, *input.TargetSprintID)
		if errors.Is(err, domain.ErrSprintNotFound) {
			return nil, domain.ErrInvalidTargetSprint
		}
		if err != nil {
			return nil, err
		}
		if next.Status != domain.StatusPlanned {
			return nil, domain.ErrInvalidTargetSprint
		}
		target = input.TargetSprintID
	default:
		return nil, domain.ErrInvalidCloseOption
	}

	if err := sprint.Close(); err != nil {
		return nil, err
	}
	if err := s.repo.MoveUnfinished(ctx, sprint.ID, target); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

func (s *SprintService) ListIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]port.IssueRef, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if _, err := s.findInProject(ctx, projectID, sprintID); err != nil {
		return nil, err
	}
	return s.repo.ListIssues(ctx, sprintID)
}

// AssignIssue pone una incidencia del proyecto en el sprint. Un sprint cerrado ya no cambia.
func (s *SprintService) AssignIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return err
	}
	if sprint.Status == domain.StatusClosed {
		return domain.ErrSprintClosed
	}
	return s.repo.SetIssueSprint(ctx, projectID, issueID, &sprintID)
}

// RemoveIssue quita una incidencia del sprint, que pasa a no tener sprint (backlog).
func (s *SprintService) RemoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error {
	if err := s.authorizer.Authorize(ctx, port.ActionManageSprints, actorID, globalRole, projectID); err != nil {
		return err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return err
	}
	if sprint.Status == domain.StatusClosed {
		return domain.ErrSprintClosed
	}
	return s.repo.RemoveIssueFromSprint(ctx, projectID, sprintID, issueID)
}

// Burndown calcula, dia a dia, el trabajo restante frente a la linea ideal que baja hasta cero.
// El trabajo restante cuenta las incidencias del sprint con puntos que no estaban cerradas ese dia.
func (s *SprintService) Burndown(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]port.BurndownPoint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	sprint, err := s.findInProject(ctx, projectID, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.StartDate == nil || sprint.EndDate == nil {
		return nil, domain.ErrSprintWithoutDates
	}
	issues, err := s.repo.ListIssues(ctx, sprintID)
	if err != nil {
		return nil, err
	}

	total := 0.0
	for _, issue := range issues {
		if issue.StoryPoints != nil {
			total += *issue.StoryPoints
		}
	}

	days := int(sprint.EndDate.Sub(*sprint.StartDate).Hours()/24) + 1
	points := make([]port.BurndownPoint, 0, days)
	for i := 0; i < days; i++ {
		day := sprint.StartDate.AddDate(0, 0, i)
		endOfDay := day.Add(24*time.Hour - time.Nanosecond)
		remaining := 0.0
		for _, issue := range issues {
			if issue.StoryPoints == nil {
				continue
			}
			if !isDoneBy(issue, endOfDay) {
				remaining += *issue.StoryPoints
			}
		}
		ideal := total
		if days > 1 {
			ideal = total * float64(days-1-i) / float64(days-1)
		}
		points = append(points, port.BurndownPoint{Date: day, Ideal: ideal, Remaining: remaining})
	}
	return points, nil
}

// isDoneBy indica si la incidencia estaba cerrada al final del dia indicado. Una incidencia cerrada
// sin fecha de cierre se considera cerrada desde el principio.
func isDoneBy(issue port.IssueRef, endOfDay time.Time) bool {
	if issue.Status != string(domain.StatusClosed) {
		return false
	}
	return issue.ClosedAt == nil || !issue.ClosedAt.After(endOfDay)
}

// Velocity devuelve los puntos completados de los ultimos sprints cerrados del proyecto.
func (s *SprintService) Velocity(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, limit int) ([]port.VelocityPoint, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultVelocityLimit
	}
	if limit > maxVelocityLimit {
		limit = maxVelocityLimit
	}
	return s.repo.Velocity(ctx, projectID, limit)
}

// findInProject carga un sprint y comprueba que pertenece al proyecto; si no, no existe para el llamador.
func (s *SprintService) findInProject(ctx context.Context, projectID, sprintID uuid.UUID) (*domain.Sprint, error) {
	sprint, err := s.repo.FindByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint == nil || sprint.ProjectID != projectID {
		return nil, domain.ErrSprintNotFound
	}
	return sprint, nil
}
