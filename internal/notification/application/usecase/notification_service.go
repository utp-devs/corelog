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

// Notify guarda un aviso para el usuario indicado. Si el usuario desactivo los avisos en la aplicacion,
// el aviso se descarta sin error: quien notifica no debe saber la preferencia del destinatario.
func (s *NotificationService) Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error {
	notification, err := domain.New(userID, kind, message, link)
	if err != nil {
		return err
	}
	prefs, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		return err
	}
	if !prefs.InApp {
		return nil
	}
	return s.repo.Save(ctx, notification)
}

func (s *NotificationService) GetPreferences(ctx context.Context, userID uuid.UUID) (domain.Preferences, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, userID uuid.UUID, prefs domain.Preferences) (domain.Preferences, error) {
	if err := s.repo.SavePreferences(ctx, userID, prefs); err != nil {
		return domain.Preferences{}, err
	}
	return prefs, nil
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
