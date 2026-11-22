package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
)

const listLimit = 50

type NotificationService struct {
	repo port.Repository
}

func NewNotificationService(repo port.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

var _ port.Service = (*NotificationService)(nil)

// Notify guarda un aviso para el usuario indicado.
func (s *NotificationService) Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error {
	notification, err := domain.New(userID, kind, message, link)
	if err != nil {
		return err
	}
	return s.repo.Save(ctx, notification)
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID) ([]*domain.Notification, error) {
	return s.repo.ListByUser(ctx, userID, listLimit)
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// MarkRead marca una notificacion del usuario. Una notificacion ajena responde no encontrada.
func (s *NotificationService) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}
