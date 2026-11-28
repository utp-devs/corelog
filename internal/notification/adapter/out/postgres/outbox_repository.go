package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// OutboxRepository is the email queue on PostgreSQL.
type OutboxRepository struct {
	db *sqlx.DB
}

// NewOutboxRepository returns the repository.
func NewOutboxRepository(db *sqlx.DB) *OutboxRepository { return &OutboxRepository{db: db} }

var _ port.Outbox = (*OutboxRepository)(nil)

type outboxRow struct {
	ID            uuid.UUID  `db:"id"`
	UserID        *uuid.UUID `db:"user_id"`
	To            string     `db:"to_email"`
	Kind          string     `db:"kind"`
	Subject       string     `db:"subject"`
	Text          string     `db:"body_text"`
	HTML          string     `db:"body_html"`
	Status        string     `db:"status"`
	Attempts      int        `db:"attempts"`
	NextAttemptAt time.Time  `db:"next_attempt_at"`
	LastError     string     `db:"last_error"`
	CreatedAt     time.Time  `db:"created_at"`
	SentAt        *time.Time `db:"sent_at"`
}

const outboxColumns = `id, user_id, to_email, kind, subject, body_text, body_html, status, attempts, next_attempt_at, last_error, created_at, sent_at`

func (r outboxRow) entry() port.OutboxEntry {
	return port.OutboxEntry{
		ID: r.ID, UserID: r.UserID, To: r.To, Kind: r.Kind, Subject: r.Subject, Text: r.Text, HTML: r.HTML, Status: r.Status,
		Attempts: r.Attempts, NextAttemptAt: r.NextAttemptAt, LastError: r.LastError, CreatedAt: r.CreatedAt, SentAt: r.SentAt,
	}
}

func (r *OutboxRepository) ext(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

func (r *OutboxRepository) Enqueue(ctx context.Context, msg port.EmailMessage, now time.Time) (bool, error) {
	var user *uuid.UUID
	if msg.UserID != uuid.Nil {
		user = &msg.UserID
	}
	var key *string
	if msg.IdempotencyKey != "" {
		key = &msg.IdempotencyKey
	}
	at := msg.NotBefore
	if at.IsZero() {
		at = now
	}
	res, err := r.ext(ctx).ExecContext(ctx, `
		INSERT INTO email_outbox (user_id, to_email, kind, subject, body_text, body_html, next_attempt_at, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`,
		user, msg.To, msg.Kind, msg.Subject, msg.Text, msg.HTML, at, key, now)
	if err != nil {
		return false, internal("no se pudo encolar el correo", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ClaimDue leases due messages in one statement: the rows are locked with SKIP LOCKED, so two workers never
// pick the same message, and their next attempt moves past the lease so a crash only delays them.
func (r *OutboxRepository) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]port.OutboxEntry, error) {
	var rows []outboxRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		UPDATE email_outbox SET next_attempt_at = $2, updated_at = $3
		WHERE id IN (
			SELECT id FROM email_outbox WHERE status = 'pending' AND next_attempt_at <= $3
			ORDER BY next_attempt_at, id LIMIT $1 FOR UPDATE SKIP LOCKED)
		RETURNING `+outboxColumns, limit, now.Add(lease), now)
	if err != nil {
		return nil, internal("no se pudo reservar correo pendiente", err)
	}
	out := make([]port.OutboxEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.entry())
	}
	return out, nil
}

func (r *OutboxRepository) MarkSent(ctx context.Context, id uuid.UUID, now time.Time) error {
	if _, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE email_outbox SET status = 'sent', sent_at = $2, updated_at = $2, last_error = '' WHERE id = $1`, id, now); err != nil {
		return internal("no se pudo marcar el correo como enviado", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, next *time.Time, lastError string, now time.Time) error {
	status, at := port.OutboxPending, now
	if next == nil {
		status = port.OutboxDead
	} else {
		at = *next
	}
	if _, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE email_outbox SET status = $2, attempts = $3, next_attempt_at = $4, last_error = $5, updated_at = $6 WHERE id = $1`,
		id, status, attempts, at, lastError, now); err != nil {
		return internal("no se pudo registrar el fallo del correo", err)
	}
	return nil
}

func (r *OutboxRepository) List(ctx context.Context, status string, limit, offset int) ([]port.OutboxEntry, int, error) {
	var total int
	if err := sqlx.GetContext(ctx, r.ext(ctx), &total, `SELECT count(*) FROM email_outbox WHERE ($1::text = '' OR status = $1::text)`, status); err != nil {
		return nil, 0, internal("no se pudo contar la cola de correo", err)
	}
	var rows []outboxRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT `+outboxColumns+` FROM email_outbox WHERE ($1::text = '' OR status = $1::text)
		ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, status, limit, offset); err != nil {
		return nil, 0, internal("no se pudo leer la cola de correo", err)
	}
	out := make([]port.OutboxEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.entry())
	}
	return out, total, nil
}

func (r *OutboxRepository) Retry(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	res, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE email_outbox SET status = 'pending', attempts = 0, next_attempt_at = $2, updated_at = $2
		WHERE id = $1 AND status = 'dead'`, id, now)
	if err != nil {
		return false, internal("no se pudo reintentar el correo", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
