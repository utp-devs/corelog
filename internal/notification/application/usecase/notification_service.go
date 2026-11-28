package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

const listLimit = 50

type NotificationService struct {
	repo   port.Repository
	engine *Engine
}

// WithEngine routes the notices of known kinds (assignment, mention, reply) through the notification scheme.
func (s *NotificationService) WithEngine(engine *Engine) *NotificationService {
	s.engine = engine
	return s
}

func NewNotificationService(repo port.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

var _ port.Service = (*NotificationService)(nil)

// Notify guarda un aviso para el usuario indicado. Si el usuario desactivo los avisos en la aplicacion,
// el aviso se descarta sin error: quien notifica no debe saber la preferencia del destinatario.
func (s *NotificationService) Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error {
	if s.engine != nil {
		if handled, err := s.engine.NotifyLegacy(ctx, userID, kind, message, link); handled {
			return err
		}
	}
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
	if prefs.Email == "" {
		prefs.Email = domain.EmailImmediate
	}
	if !prefs.Email.IsValid() {
		return domain.Preferences{}, apperror.WithCode(apperror.KindValidation, "notification.preferences_invalid", "El modo de correo debe ser immediate, daily u off")
	}
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
