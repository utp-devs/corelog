// Package postgres implements the note repository and the read-only link access over PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Repository stores notes. Tags travel as JSON in both directions so that the text[] column needs no
// array codec: writes expand the JSON array in SQL and reads serialize the array with to_json.
type Repository struct {
	db *sqlx.DB
}

var _ port.Repository = (*Repository)(nil)

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// noteColumns lists the columns read for a note. Keep in sync with noteRow.
const noteColumns = `id, owner_id, title, body, color, pinned, to_json(tags)::text AS tags_json,
	issue_id, project_id, created_at, updated_at`

type noteRow struct {
	ID        uuid.UUID  `db:"id"`
	OwnerID   uuid.UUID  `db:"owner_id"`
	Title     string     `db:"title"`
	Body      string     `db:"body"`
	Color     string     `db:"color"`
	Pinned    bool       `db:"pinned"`
	TagsJSON  string     `db:"tags_json"`
	IssueID   *uuid.UUID `db:"issue_id"`
	ProjectID *uuid.UUID `db:"project_id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

func (r noteRow) toDomain() (domain.Note, error) {
	var tags []string
	if err := json.Unmarshal([]byte(r.TagsJSON), &tags); err != nil {
		return domain.Note{}, apperror.Wrap(apperror.KindInternal, "failed to read note tags", err)
	}
	if tags == nil {
		tags = []string{}
	}
	return domain.Note{
		ID: r.ID, OwnerID: r.OwnerID, Title: r.Title, Body: r.Body, Color: r.Color, Pinned: r.Pinned,
		Tags: tags, IssueID: r.IssueID, ProjectID: r.ProjectID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}

// tagsJSON serializes tags as a JSON array; a nil slice becomes [].
func tagsJSON(tags []string) (string, error) {
	if tags == nil {
		tags = []string{}
	}
	raw, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("note: encode tags: %w", err)
	}
	return string(raw), nil
}

// nullableUUID turns an optional identifier into a SQL value (nil for NULL).
func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func (r *Repository) Insert(ctx context.Context, n domain.Note) (domain.Note, error) {
	tags, err := tagsJSON(n.Tags)
	if err != nil {
		return domain.Note{}, err
	}
	var id uuid.UUID
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &id, `
		INSERT INTO notes (owner_id, title, body, color, pinned, tags, issue_id, project_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, ARRAY(SELECT jsonb_array_elements_text($6::jsonb)), $7, $8, $9, $10)
		RETURNING id`,
		n.OwnerID.String(), n.Title, n.Body, n.Color, n.Pinned, tags,
		nullableUUID(n.IssueID), nullableUUID(n.ProjectID), n.CreatedAt, n.UpdatedAt,
	); err != nil {
		return domain.Note{}, apperror.Wrap(apperror.KindInternal, "failed to create note", err)
	}
	n.ID = id
	return n, nil
}

func (r *Repository) Update(ctx context.Context, n domain.Note) error {
	tags, err := tagsJSON(n.Tags)
	if err != nil {
		return err
	}
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE notes
		SET title = $3, body = $4, color = $5, pinned = $6,
		    tags = ARRAY(SELECT jsonb_array_elements_text($7::jsonb)),
		    issue_id = $8, project_id = $9, updated_at = $10
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		n.ID.String(), n.OwnerID.String(), n.Title, n.Body, n.Color, n.Pinned, tags,
		nullableUUID(n.IssueID), nullableUUID(n.ProjectID), n.UpdatedAt,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update note", err)
	}
	return requireRow(res)
}

func (r *Repository) Get(ctx context.Context, ownerID, id uuid.UUID) (domain.Note, error) {
	var row noteRow
	err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `
		SELECT `+noteColumns+`
		FROM notes
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`, id.String(), ownerID.String())
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, apperror.Wrap(apperror.KindInternal, "failed to load note", err)
	}
	return row.toDomain()
}

func (r *Repository) List(ctx context.Context, ownerID uuid.UUID, q port.ListQuery) ([]domain.Note, error) {
	args := []any{ownerID.String()}
	placeholder := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where := []string{"owner_id = $1", "deleted_at IS NULL"}
	if q.Query != "" {
		p := placeholder(likePattern(q.Query))
		where = append(where, fmt.Sprintf("(title ILIKE %s ESCAPE '\\' OR body ILIKE %s ESCAPE '\\')", p, p))
	}
	if q.Tag != "" {
		where = append(where, fmt.Sprintf("%s = ANY(tags)", placeholder(q.Tag)))
	}
	if q.Pinned != nil {
		where = append(where, fmt.Sprintf("pinned = %s", placeholder(*q.Pinned)))
	}
	if q.IssueID != nil {
		where = append(where, fmt.Sprintf("issue_id = %s::uuid", placeholder(q.IssueID.String())))
	}
	if q.ProjectID != nil {
		where = append(where, fmt.Sprintf("project_id = %s::uuid", placeholder(q.ProjectID.String())))
	}
	if q.After != nil {
		where = append(where, fmt.Sprintf("(pinned, updated_at, id) < (%s::boolean, %s::timestamptz, %s::uuid)",
			placeholder(q.After.Pinned), placeholder(q.After.UpdatedAt.UTC()), placeholder(q.After.ID.String())))
	}
	args = append(args, q.Limit+1)
	query := `SELECT ` + noteColumns + ` FROM notes WHERE ` + strings.Join(where, " AND ") +
		fmt.Sprintf(` ORDER BY pinned DESC, updated_at DESC, id DESC LIMIT $%d`, len(args))

	var rows []noteRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list notes", err)
	}
	notes := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		n, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (r *Repository) SoftDelete(ctx context.Context, ownerID, id uuid.UUID, at time.Time) error {
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE notes SET deleted_at = $3
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`, id.String(), ownerID.String(), at.UTC())
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete note", err)
	}
	return requireRow(res)
}

func (r *Repository) ListTags(ctx context.Context, ownerID uuid.UUID) ([]port.TagCount, error) {
	var tags []port.TagCount
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &tags, `
		SELECT t.tag, COUNT(*) AS count
		FROM notes n
		CROSS JOIN LATERAL unnest(n.tags) AS t(tag)
		WHERE n.owner_id = $1 AND n.deleted_at IS NULL
		GROUP BY t.tag
		ORDER BY count DESC, t.tag ASC`, ownerID.String()); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list note tags", err)
	}
	if tags == nil {
		tags = []port.TagCount{}
	}
	return tags, nil
}

func (r *Repository) SearchOwn(ctx context.Context, ownerID uuid.UUID, query string, limit int) ([]port.SearchHit, error) {
	var hits []struct {
		ID        uuid.UUID `db:"id"`
		Title     string    `db:"title"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &hits, `
		SELECT id, title, updated_at
		FROM notes
		WHERE owner_id = $1 AND deleted_at IS NULL
		  AND (title ILIKE $2 ESCAPE '\' OR body ILIKE $2 ESCAPE '\')
		ORDER BY updated_at DESC, id DESC
		LIMIT $3`, ownerID.String(), likePattern(query), limit); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to search notes", err)
	}
	out := make([]port.SearchHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, port.SearchHit{ID: h.ID, Title: h.Title, UpdatedAt: h.UpdatedAt})
	}
	return out, nil
}

// likePattern wraps the term in wildcards and escapes LIKE metacharacters so that "100%" or "a_b" match literally.
func likePattern(term string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
	return "%" + escaped + "%"
}

// requireRow maps a write that touched no live row to domain.ErrNotFound.
func requireRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to read affected rows", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
