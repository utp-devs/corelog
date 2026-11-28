package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var (
	ErrInvalidNotification  = apperror.WithCode(apperror.KindValidation, "notification.invalid", "invalid notification")
	ErrNotificationNotFound = apperror.WithCode(apperror.KindNotFound, "notification.not_found", "notification not found")
)

// Notification avisa a una persona de algo que le afecta. Es solo in-app; el correo queda como puerto
// futuro.
type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Kind      string
	Message   string
	Link      string
	ReadAt    *time.Time
	CreatedAt time.Time
	// Data is optional structured detail for the interface (for example the actions of a security alert).
	Data map[string]any
}

// New valida una notificacion nueva. El id lo asigna la base de datos.
func New(userID uuid.UUID, kind, message, link string) (*Notification, error) {
	kind = strings.TrimSpace(kind)
	message = strings.TrimSpace(message)
	link = strings.TrimSpace(link)
	if userID == uuid.Nil || kind == "" || message == "" {
		return nil, ErrInvalidNotification
	}
	if utf8.RuneCountInString(message) > 300 || utf8.RuneCountInString(link) > 300 || utf8.RuneCountInString(kind) > 40 {
		return nil, ErrInvalidNotification
	}
	return &Notification{UserID: userID, Kind: kind, Message: message, Link: link, CreatedAt: time.Now().UTC()}, nil
}

// MarkRead marca la notificacion como leida una sola vez.
func (n *Notification) MarkRead(at time.Time) {
	if n.ReadAt == nil {
		n.ReadAt = &at
	}
}
