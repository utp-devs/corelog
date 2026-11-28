// Package postgres stores API tokens.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/apitoken/application/port"
	"github.com/jhonsferg/corelog/internal/apitoken/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Repository is the PostgreSQL adapter of port.Repository.
type Repository struct{ db *sqlx.DB }

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository bound to db; it joins the transaction of the context when there is one.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ext(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

type row struct {
	ID         uuid.UUID         `db:"id"`
	UserID     uuid.UUID         `db:"user_id"`
	Name       string            `db:"name"`
	Prefix     string            `db:"prefix"`
	Scopes     database.TextList `db:"scopes"`
	ExpiresAt  *time.Time        `db:"expires_at"`
	LastUsedAt *time.Time        `db:"last_used_at"`
	RevokedAt  *time.Time        `db:"revoked_at"`
	CreatedAt  time.Time         `db:"created_at"`
}

const columns = `id, user_id, name, prefix, to_jsonb(scopes) AS scopes, expires_at, last_used_at, revoked_at, created_at`

func (r row) token() domain.Token {
	scopes := make([]domain.Scope, len(r.Scopes))
	for i, s := range r.Scopes {
		scopes[i] = domain.Scope(s)
	}
	return domain.Token{ID: r.ID, UserID: r.UserID, Name: r.Name, Prefix: r.Prefix, Scopes: scopes,
		ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt, CreatedAt: r.CreatedAt}
}

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

// Insert stores the token and fills its id.
func (r *Repository) Insert(ctx context.Context, t *domain.Token, hash string) error {
	scopes := make(database.TextList, len(t.Scopes))
	for i, s := range t.Scopes {
		scopes[i] = string(s)
	}
	err := sqlx.GetContext(ctx, r.ext(ctx), &t.ID, `
		INSERT INTO api_tokens (user_id, name, token_hash, prefix, scopes, expires_at, created_at)
		VALUES ($1, $2, $3, $4, ARRAY(SELECT jsonb_array_elements_text($5::jsonb)), $6, $7) RETURNING id`,
		t.UserID, t.Name, hash, t.Prefix, scopes, t.ExpiresAt, t.CreatedAt)
	if err != nil {
		return internal("no se pudo guardar el token", err)
	}
	return nil
}

// CountActive counts live tokens of the user.
func (r *Repository) CountActive(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.ext(ctx), &n, `
		SELECT count(*) FROM api_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $2)`, userID, now)
	if err != nil {
		return 0, internal("no se pudieron contar los tokens", err)
	}
	return n, nil
}

// List returns the tokens of a user, newest first.
func (r *Repository) List(ctx context.Context, userID uuid.UUID) ([]domain.Token, error) {
	var rows []row
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows,
		`SELECT `+columns+` FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC, id`, userID); err != nil {
		return nil, internal("no se pudieron listar los tokens", err)
	}
	out := make([]domain.Token, len(rows))
	for i, rw := range rows {
		out[i] = rw.token()
	}
	return out, nil
}

// Revoke sets revoked_at once; a second call keeps the first instant.
func (r *Repository) Revoke(ctx context.Context, userID, id uuid.UUID, at time.Time) (domain.Token, error) {
	var rw row
	err := sqlx.GetContext(ctx, r.ext(ctx), &rw, `
		UPDATE api_tokens SET revoked_at = COALESCE(revoked_at, $3)
		WHERE id = $1 AND user_id = $2 RETURNING `+columns, id, userID, at)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Token{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Token{}, internal("no se pudo revocar el token", err)
	}
	return rw.token(), nil
}

// FindByHash returns the token, its owner and the stored hash.
func (r *Repository) FindByHash(ctx context.Context, hash string) (domain.Token, port.Owner, string, error) {
	var rw struct {
		ID          uuid.UUID         `db:"id"`
		UserID      uuid.UUID         `db:"user_id"`
		Name        string            `db:"name"`
		Prefix      string            `db:"prefix"`
		Scopes      database.TextList `db:"scopes"`
		ExpiresAt   *time.Time        `db:"expires_at"`
		LastUsedAt  *time.Time        `db:"last_used_at"`
		RevokedAt   *time.Time        `db:"revoked_at"`
		CreatedAt   time.Time         `db:"created_at"`
		Hash        string            `db:"token_hash"`
		Role        string            `db:"owner_role"`
		Deactivated *time.Time        `db:"deactivated_at"`
	}
	err := sqlx.GetContext(ctx, r.ext(ctx), &rw, `
		SELECT t.id, t.user_id, t.name, t.prefix, to_jsonb(t.scopes) AS scopes, t.expires_at, t.last_used_at,
		       t.revoked_at, t.created_at, t.token_hash, u.role AS owner_role, u.deactivated_at
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1`, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Token{}, port.Owner{}, "", domain.ErrNotFound
	}
	if err != nil {
		return domain.Token{}, port.Owner{}, "", internal("no se pudo buscar el token", err)
	}
	tok := row{ID: rw.ID, UserID: rw.UserID, Name: rw.Name, Prefix: rw.Prefix, Scopes: rw.Scopes, ExpiresAt: rw.ExpiresAt,
		LastUsedAt: rw.LastUsedAt, RevokedAt: rw.RevokedAt, CreatedAt: rw.CreatedAt}.token()
	return tok, port.Owner{Role: rw.Role, Deactivated: rw.Deactivated != nil}, rw.Hash, nil
}

// Touch updates last_used_at when it is missing or older than olderThan.
func (r *Repository) Touch(ctx context.Context, id uuid.UUID, at, olderThan time.Time) error {
	_, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE api_tokens SET last_used_at = $2 WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < $3)`,
		id, at, olderThan)
	if err != nil {
		return internal("no se pudo registrar el uso del token", err)
	}
	return nil
}
