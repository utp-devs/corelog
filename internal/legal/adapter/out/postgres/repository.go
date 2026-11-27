// Package postgres implements the legal repository on PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/legal/application/port"
	"github.com/jhonsferg/corelog/internal/legal/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Repository stores legal documents and consents.
type Repository struct {
	db *sqlx.DB
}

// NewRepository returns a repository bound to db; every method joins the transaction of the context.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

var _ port.Repository = (*Repository)(nil)

func (r *Repository) q(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

type documentRow struct {
	ID            uuid.UUID     `db:"id"`
	Slug          string        `db:"slug"`
	Version       int           `db:"version"`
	Title         string        `db:"title"`
	BodyMarkdown  string        `db:"body_markdown"`
	Summary       string        `db:"summary"`
	Required      bool          `db:"required"`
	EffectiveFrom time.Time     `db:"effective_from"`
	PublishedAt   time.Time     `db:"published_at"`
	CreatedBy     uuid.NullUUID `db:"created_by"`
}

func (d documentRow) toDomain() domain.Document {
	doc := domain.Document{
		ID: d.ID, Slug: domain.Slug(d.Slug), Version: d.Version, Title: d.Title, BodyMarkdown: d.BodyMarkdown,
		Summary: d.Summary, Required: d.Required, EffectiveFrom: d.EffectiveFrom.UTC(), PublishedAt: d.PublishedAt.UTC(),
	}
	if d.CreatedBy.Valid {
		id := d.CreatedBy.UUID
		doc.CreatedBy = &id
	}
	return doc
}

const (
	docColumns       = `id, slug, version, title, body_markdown, summary, required, effective_from, published_at, created_by`
	docColumnsNoBody = `id, slug, version, title, '' AS body_markdown, summary, required, effective_from, published_at, created_by`
)

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

func (r *Repository) ListCurrent(ctx context.Context, now time.Time) ([]domain.Document, error) {
	var rows []documentRow
	if err := sqlx.SelectContext(ctx, r.q(ctx), &rows, `
		SELECT DISTINCT ON (slug) `+docColumnsNoBody+`
		FROM legal_documents WHERE effective_from <= $1 ORDER BY slug, version DESC`, now); err != nil {
		return nil, internal("failed to list legal documents", err)
	}
	return toDomains(rows), nil
}

func (r *Repository) GetCurrent(ctx context.Context, slug domain.Slug, now time.Time) (*domain.Document, error) {
	var row documentRow
	err := sqlx.GetContext(ctx, r.q(ctx), &row, `
		SELECT `+docColumns+` FROM legal_documents WHERE slug = $1 AND effective_from <= $2
		ORDER BY version DESC LIMIT 1`, string(slug), now)
	return oneDocument(row, err)
}

func (r *Repository) GetVersion(ctx context.Context, slug domain.Slug, version int) (*domain.Document, error) {
	var row documentRow
	err := sqlx.GetContext(ctx, r.q(ctx), &row, `
		SELECT `+docColumns+` FROM legal_documents WHERE slug = $1 AND version = $2`, string(slug), version)
	return oneDocument(row, err)
}

func oneDocument(row documentRow, err error) (*domain.Document, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, internal("failed to read legal document", err)
	}
	d := row.toDomain()
	return &d, nil
}

func (r *Repository) ListAll(ctx context.Context) ([]domain.Document, error) {
	var rows []documentRow
	if err := sqlx.SelectContext(ctx, r.q(ctx), &rows, `
		SELECT `+docColumnsNoBody+` FROM legal_documents ORDER BY slug, version DESC`); err != nil {
		return nil, internal("failed to list legal documents", err)
	}
	return toDomains(rows), nil
}

func toDomains(rows []documentRow) []domain.Document {
	out := make([]domain.Document, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out
}

func (r *Repository) Publish(ctx context.Context, d *domain.Document, now time.Time) error {
	var createdBy any
	if d.CreatedBy != nil {
		createdBy = *d.CreatedBy
	}
	var out struct {
		ID      uuid.UUID `db:"id"`
		Version int       `db:"version"`
	}
	err := sqlx.GetContext(ctx, r.q(ctx), &out, `
		INSERT INTO legal_documents (slug, version, title, body_markdown, summary, required, effective_from, published_at, created_by)
		VALUES ($1, (SELECT COALESCE(MAX(version), 0) + 1 FROM legal_documents WHERE slug = $1), $2, $3, $4, $5, $6, $6, $7)
		RETURNING id, version`,
		string(d.Slug), d.Title, d.BodyMarkdown, d.Summary, d.Required, now, createdBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrPublishConflict
		}
		return internal("failed to publish legal document", err)
	}
	d.ID, d.Version, d.EffectiveFrom, d.PublishedAt = out.ID, out.Version, now, now
	return nil
}

func (r *Repository) ActiveConsents(ctx context.Context, userID uuid.UUID) ([]domain.Accepted, error) {
	var rows []struct {
		Slug       string    `db:"document_slug"`
		Version    int       `db:"document_version"`
		AcceptedAt time.Time `db:"accepted_at"`
	}
	if err := sqlx.SelectContext(ctx, r.q(ctx), &rows, `
		SELECT document_slug, document_version, accepted_at FROM user_consents
		WHERE user_id = $1 AND revoked_at IS NULL ORDER BY document_slug, document_version`, userID); err != nil {
		return nil, internal("failed to list consents", err)
	}
	out := make([]domain.Accepted, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Accepted{Slug: domain.Slug(row.Slug), Version: row.Version, AcceptedAt: row.AcceptedAt.UTC()})
	}
	return out, nil
}

func (r *Repository) SaveConsent(ctx context.Context, c domain.Consent) (bool, error) {
	var ip, ua any
	if c.IP != "" {
		ip = c.IP
	}
	if c.UserAgent != "" {
		ua = c.UserAgent
	}
	res, err := r.q(ctx).ExecContext(ctx, `
		INSERT INTO user_consents (user_id, document_slug, document_version, accepted_at, ip, user_agent, source)
		VALUES ($1, $2, $3, $4, $5::inet, $6, $7)
		ON CONFLICT (user_id, document_slug, document_version) WHERE revoked_at IS NULL DO NOTHING`,
		c.UserID, string(c.DocumentSlug), c.DocumentVersion, c.AcceptedAt, ip, ua, string(c.Source))
	if err != nil {
		return false, internal("failed to save consent", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, internal("failed to save consent", err)
	}
	return n > 0, nil
}

func (r *Repository) RevokeConsent(ctx context.Context, userID uuid.UUID, slug domain.Slug, at time.Time) (bool, error) {
	res, err := r.q(ctx).ExecContext(ctx, `
		UPDATE user_consents SET revoked_at = $3 WHERE user_id = $1 AND document_slug = $2 AND revoked_at IS NULL`,
		userID, string(slug), at)
	if err != nil {
		return false, internal("failed to revoke consent", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, internal("failed to revoke consent", err)
	}
	return n > 0, nil
}

func (r *Repository) ListConsents(ctx context.Context, userID *uuid.UUID, limit int) ([]domain.Consent, error) {
	var rows []struct {
		ID         uuid.UUID      `db:"id"`
		UserID     uuid.UUID      `db:"user_id"`
		Slug       string         `db:"document_slug"`
		Version    int            `db:"document_version"`
		AcceptedAt time.Time      `db:"accepted_at"`
		IP         sql.NullString `db:"ip"`
		UserAgent  sql.NullString `db:"user_agent"`
		Source     string         `db:"source"`
		RevokedAt  *time.Time     `db:"revoked_at"`
	}
	var filter any
	if userID != nil {
		filter = *userID
	}
	if err := sqlx.SelectContext(ctx, r.q(ctx), &rows, `
		SELECT id, user_id, document_slug, document_version, accepted_at, host(ip) AS ip, user_agent, source, revoked_at
		FROM user_consents WHERE ($1::uuid IS NULL OR user_id = $1)
		ORDER BY accepted_at DESC, id LIMIT $2`, filter, limit); err != nil {
		return nil, internal("failed to list consents", err)
	}
	out := make([]domain.Consent, 0, len(rows))
	for _, row := range rows {
		c := domain.Consent{
			ID: row.ID, UserID: row.UserID, DocumentSlug: domain.Slug(row.Slug), DocumentVersion: row.Version,
			AcceptedAt: row.AcceptedAt.UTC(), IP: row.IP.String, UserAgent: row.UserAgent.String, Source: domain.Source(row.Source),
		}
		if row.RevokedAt != nil {
			t := row.RevokedAt.UTC()
			c.RevokedAt = &t
		}
		out = append(out, c)
	}
	return out, nil
}
