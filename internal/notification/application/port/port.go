package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// Repository persiste notificaciones. Todas las operaciones de lectura y marca se limitan al usuario dueño.
type Repository interface {
	Save(ctx context.Context, notification *domain.Notification) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Notification, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	GetPreferences(ctx context.Context, userID uuid.UUID) (domain.Preferences, error)
	SavePreferences(ctx context.Context, userID uuid.UUID, prefs domain.Preferences) error
}

// Service es el caso de uso de notificaciones. Notify lo usan otros modulos (puerto Notifier).
type Service interface {
	Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error
	List(ctx context.Context, userID uuid.UUID) ([]*domain.Notification, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	GetPreferences(ctx context.Context, userID uuid.UUID) (domain.Preferences, error)
	UpdatePreferences(ctx context.Context, userID uuid.UUID, prefs domain.Preferences) (domain.Preferences, error)
}
