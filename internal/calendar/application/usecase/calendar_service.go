package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
)

type CalendarService struct {
	repo       port.Repository
	authorizer port.Authorizer
}

func NewCalendarService(repo port.Repository, authorizer port.Authorizer) *CalendarService {
	return &CalendarService{repo: repo, authorizer: authorizer}
}

var _ port.Service = (*CalendarService)(nil)

// Events devuelve los hitos del proyecto entre from y to. El rango puede tener como maximo un ano.
func (s *CalendarService) Events(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, from, to time.Time) ([]port.Event, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if err := domain.ValidateRange(from, to); err != nil {
		return nil, err
	}
	return s.repo.Events(ctx, projectID, from, to)
}
