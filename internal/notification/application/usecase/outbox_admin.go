package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Error codes of the outbox administration.
const (
	CodeOutboxStatusInvalid = "notification.outbox_status_invalid"
	CodeOutboxNotRetryable  = "notification.outbox_not_retryable"
)

// OutboxAdminService lists the email queue and requeues dead messages. The HTTP layer restricts it to platform
// administrators.
type OutboxAdminService struct {
	outbox port.Outbox
	clock  clock.Clock
}

var _ port.OutboxAdmin = (*OutboxAdminService)(nil)

// NewOutboxAdminService wires the service.
func NewOutboxAdminService(outbox port.Outbox, clk clock.Clock) *OutboxAdminService {
	return &OutboxAdminService{outbox: outbox, clock: clk}
}

// List returns messages, newest first, filtered by status (empty means every status).
func (s *OutboxAdminService) List(ctx context.Context, status string, limit, offset int) ([]port.OutboxEntry, int, error) {
	switch status {
	case "", port.OutboxPending, port.OutboxSent, port.OutboxDead:
	default:
		return nil, 0, apperror.WithCode(apperror.KindValidation, CodeOutboxStatusInvalid, "El estado debe ser pending, sent o dead")
	}
	return s.outbox.List(ctx, status, limit, offset)
}

// Retry requeues a dead message with a fresh attempt counter.
func (s *OutboxAdminService) Retry(ctx context.Context, id uuid.UUID) error {
	ok, err := s.outbox.Retry(ctx, id, s.clock.Now())
	if err != nil {
		return err
	}
	if !ok {
		return apperror.WithCode(apperror.KindConflict, CodeOutboxNotRetryable, "Solo se pueden reintentar los correos en estado dead")
	}
	return nil
}
