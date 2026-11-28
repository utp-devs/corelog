// Package postgres stores webhooks and deliveries and describes events for payloads.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

// Repository is the PostgreSQL adapter of port.Repository.
type Repository struct{ db *sqlx.DB }

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository bound to db; it joins the transaction of the context when there is one.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ext(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

type webhookRow struct {
	ID          uuid.UUID         `db:"id"`
	OrgID       uuid.UUID         `db:"organization_id"`
	ProjectID   *uuid.UUID        `db:"project_id"`
	Name        string            `db:"name"`
	URL         string            `db:"url"`
	Secret      []byte            `db:"secret_encrypted"`
	Events      database.TextList `db:"events"`
	FilterQuery *string           `db:"filter_query"`
	Enabled     bool              `db:"enabled"`
	CreatedBy   *uuid.UUID        `db:"created_by"`
	CreatedAt   time.Time         `db:"created_at"`
	UpdatedAt   time.Time         `db:"updated_at"`
}

const webhookColumns = `id, organization_id, project_id, name, url, secret_encrypted, to_jsonb(events) AS events,
	filter_query, enabled, created_by, created_at, updated_at`

func (w webhookRow) stored() port.Stored {
	return port.Stored{SecretEncrypted: w.Secret, Webhook: domain.Webhook{
		ID: w.ID, OrgID: w.OrgID, ProjectID: w.ProjectID, Name: w.Name, URL: w.URL, Events: w.Events,
		FilterQuery: w.FilterQuery, Enabled: w.Enabled, CreatedBy: w.CreatedBy, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}}
}

// Create inserts a webhook and fills its id.
func (r *Repository) Create(ctx context.Context, w *port.Stored) error {
	err := sqlx.GetContext(ctx, r.ext(ctx), &w.ID, `
		INSERT INTO webhooks (organization_id, project_id, name, url, secret_encrypted, events, filter_query, enabled,
		                      created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, ARRAY(SELECT jsonb_array_elements_text($6::jsonb)), $7, $8, $9, $10, $10)
		RETURNING id`,
		w.OrgID, w.ProjectID, w.Name, w.URL, w.SecretEncrypted, database.TextList(w.Events), w.FilterQuery, w.Enabled,
		w.CreatedBy, w.CreatedAt)
	if err != nil {
		return internal("no se pudo guardar el webhook", err)
	}
	return nil
}

// Get returns a webhook of the organization.
func (r *Repository) Get(ctx context.Context, orgID, id uuid.UUID) (port.Stored, error) {
	var row webhookRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row,
		`SELECT `+webhookColumns+` FROM webhooks WHERE id = $1 AND organization_id = $2`, id, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return port.Stored{}, domain.ErrNotFound
	}
	if err != nil {
		return port.Stored{}, internal("no se pudo leer el webhook", err)
	}
	return row.stored(), nil
}

// StoredByID returns a webhook by id only (delivery worker).
func (r *Repository) StoredByID(ctx context.Context, id uuid.UUID) (port.Stored, error) {
	var row webhookRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `SELECT `+webhookColumns+` FROM webhooks WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return port.Stored{}, domain.ErrNotFound
	}
	if err != nil {
		return port.Stored{}, internal("no se pudo leer el webhook", err)
	}
	return row.stored(), nil
}

// List returns the webhooks of an organization, optionally of one project.
func (r *Repository) List(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) ([]domain.Webhook, error) {
	var rows []webhookRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `SELECT `+webhookColumns+` FROM webhooks
		WHERE organization_id = $1 AND ($2::uuid IS NULL OR project_id = $2) ORDER BY created_at, id`, orgID, projectID)
	if err != nil {
		return nil, internal("no se pudieron listar los webhooks", err)
	}
	out := make([]domain.Webhook, len(rows))
	for i, row := range rows {
		out[i] = row.stored().Webhook
	}
	return out, nil
}

// Update stores the editable fields.
func (r *Repository) Update(ctx context.Context, w domain.Webhook) error {
	res, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE webhooks SET name = $3, url = $4, events = ARRAY(SELECT jsonb_array_elements_text($5::jsonb)),
		       filter_query = $6, enabled = $7, updated_at = $8
		WHERE id = $1 AND organization_id = $2`,
		w.ID, w.OrgID, w.Name, w.URL, database.TextList(w.Events), w.FilterQuery, w.Enabled, w.UpdatedAt)
	return affected(res, err, "no se pudo actualizar el webhook")
}

// UpdateSecret replaces the sealed secret.
func (r *Repository) UpdateSecret(ctx context.Context, orgID, id uuid.UUID, sealed []byte, at time.Time) error {
	res, err := r.ext(ctx).ExecContext(ctx,
		`UPDATE webhooks SET secret_encrypted = $3, updated_at = $4 WHERE id = $1 AND organization_id = $2`, id, orgID, sealed, at)
	return affected(res, err, "no se pudo rotar el secreto")
}

// Delete removes a webhook; deliveries cascade.
func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res, err := r.ext(ctx).ExecContext(ctx, `DELETE FROM webhooks WHERE id = $1 AND organization_id = $2`, id, orgID)
	return affected(res, err, "no se pudo eliminar el webhook")
}

func affected(res sql.Result, err error, msg string) error {
	if err != nil {
		return internal(msg, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ProjectInOrg reports whether a live project belongs to the organization.
func (r *Repository) ProjectInOrg(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	var ok bool
	err := sqlx.GetContext(ctx, r.ext(ctx), &ok, `SELECT EXISTS (
		SELECT 1 FROM projects WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL)`, projectID, orgID)
	if err != nil {
		return false, internal("no se pudo comprobar el proyecto", err)
	}
	return ok, nil
}

// Matching returns the enabled webhooks that apply to the project and subscribe to the event.
func (r *Repository) Matching(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, eventType string) ([]domain.Webhook, error) {
	var rows []webhookRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `SELECT `+webhookColumns+` FROM webhooks
		WHERE organization_id = $1 AND enabled
		  AND (project_id IS NULL OR project_id = $2)
		  AND ('*' = ANY(events) OR $3 = ANY(events))
		ORDER BY created_at, id`, orgID, projectID, eventType)
	if err != nil {
		return nil, internal("no se pudieron buscar los webhooks", err)
	}
	out := make([]domain.Webhook, len(rows))
	for i, row := range rows {
		out[i] = row.stored().Webhook
	}
	return out, nil
}

type deliveryRow struct {
	ID              uuid.UUID  `db:"id"`
	WebhookID       uuid.UUID  `db:"webhook_id"`
	EventID         *int64     `db:"event_id"`
	EventType       string     `db:"event_type"`
	Payload         []byte     `db:"payload"`
	Status          string     `db:"status"`
	Attempts        int        `db:"attempts"`
	NextAttemptAt   *time.Time `db:"next_attempt_at"`
	ResponseStatus  *int       `db:"response_status"`
	ResponseExcerpt string     `db:"response_excerpt"`
	DurationMS      int        `db:"duration_ms"`
	CreatedAt       time.Time  `db:"created_at"`
}

const deliveryColumns = `id, webhook_id, event_id, event_type, payload, status, attempts, next_attempt_at,
	response_status, response_excerpt, duration_ms, created_at`

func (d deliveryRow) delivery() domain.Delivery {
	return domain.Delivery{ID: d.ID, WebhookID: d.WebhookID, EventID: d.EventID, EventType: d.EventType, Payload: d.Payload,
		Status: domain.DeliveryStatus(d.Status), Attempts: d.Attempts, NextAttemptAt: d.NextAttemptAt,
		ResponseStatus: d.ResponseStatus, ResponseExcerpt: d.ResponseExcerpt, DurationMS: d.DurationMS, CreatedAt: d.CreatedAt}
}

// InsertDelivery stores a delivery; a repeated (webhook, event) pair is ignored.
func (r *Repository) InsertDelivery(ctx context.Context, d *domain.Delivery) (bool, error) {
	err := sqlx.GetContext(ctx, r.ext(ctx), &d.ID, `
		INSERT INTO webhook_deliveries (webhook_id, event_id, event_type, payload, status, attempts, next_attempt_at, created_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, 0, $6, $7)
		ON CONFLICT (webhook_id, event_id) WHERE event_id IS NOT NULL DO NOTHING
		RETURNING id`,
		d.WebhookID, d.EventID, d.EventType, string(d.Payload), string(d.Status), d.NextAttemptAt, d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, internal("no se pudo guardar la entrega", err)
	}
	return true, nil
}

// ListDeliveries returns the latest deliveries of a webhook.
func (r *Repository) ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]domain.Delivery, error) {
	var rows []deliveryRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `SELECT `+deliveryColumns+` FROM webhook_deliveries
		WHERE webhook_id = $1 ORDER BY created_at DESC, id LIMIT $2`, webhookID, limit)
	if err != nil {
		return nil, internal("no se pudieron listar las entregas", err)
	}
	out := make([]domain.Delivery, len(rows))
	for i, row := range rows {
		out[i] = row.delivery()
	}
	return out, nil
}

// GetDelivery returns one delivery of a webhook.
func (r *Repository) GetDelivery(ctx context.Context, webhookID, id uuid.UUID) (domain.Delivery, error) {
	var row deliveryRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row,
		`SELECT `+deliveryColumns+` FROM webhook_deliveries WHERE id = $1 AND webhook_id = $2`, id, webhookID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Delivery{}, domain.ErrDeliveryNotFound
	}
	if err != nil {
		return domain.Delivery{}, internal("no se pudo leer la entrega", err)
	}
	return row.delivery(), nil
}

// ClaimDue selects due deliveries of enabled webhooks with row locks that skip rows other workers hold, and
// moves their next attempt forward by the lease.
func (r *Repository) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]port.DueDelivery, error) {
	var rows []struct {
		deliveryRow
		URL    string `db:"url"`
		Secret []byte `db:"secret_encrypted"`
	}
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		WITH due AS (
			SELECT d.id FROM webhook_deliveries d JOIN webhooks w ON w.id = d.webhook_id
			WHERE d.status IN ('pending', 'failed') AND d.next_attempt_at IS NOT NULL AND d.next_attempt_at <= $1 AND w.enabled
			ORDER BY d.next_attempt_at LIMIT $2 FOR UPDATE OF d SKIP LOCKED
		), claimed AS (
			UPDATE webhook_deliveries d SET next_attempt_at = $3 FROM due WHERE d.id = due.id
			RETURNING d.id, d.webhook_id, d.event_id, d.event_type, d.payload, d.status, d.attempts, d.next_attempt_at,
			          d.response_status, d.response_excerpt, d.duration_ms, d.created_at
		)
		SELECT c.*, w.url, w.secret_encrypted FROM claimed c JOIN webhooks w ON w.id = c.webhook_id ORDER BY c.created_at`,
		now, limit, now.Add(lease))
	if err != nil {
		return nil, internal("no se pudieron reservar las entregas", err)
	}
	out := make([]port.DueDelivery, len(rows))
	for i, row := range rows {
		out[i] = port.DueDelivery{Delivery: row.delivery(), URL: row.URL, SecretEncrypted: row.Secret}
	}
	return out, nil
}

// RecordResult stores the outcome of an attempt.
func (r *Repository) RecordResult(ctx context.Context, id uuid.UUID, res port.Result) error {
	_, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE webhook_deliveries SET status = $2, attempts = $3, next_attempt_at = $4, response_status = $5,
		       response_excerpt = $6, duration_ms = $7 WHERE id = $1`,
		id, string(res.Status), res.Attempts, res.NextAttemptAt, res.ResponseStatus, res.Excerpt, res.DurationMS)
	if err != nil {
		return internal("no se pudo registrar la entrega", err)
	}
	return nil
}
