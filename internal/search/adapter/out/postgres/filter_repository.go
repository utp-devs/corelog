package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// FilterRepository stores saved filters, their shares and subscriptions.
type FilterRepository struct {
	db *sqlx.DB
}

var _ port.FilterRepository = (*FilterRepository)(nil)

// NewFilterRepository builds the repository.
func NewFilterRepository(db *sqlx.DB) *FilterRepository { return &FilterRepository{db: db} }

// filterViewSQL selects the filters with the access of one viewer ($1). A non-owner only gets a permission
// when they are a live member of the filter's organization and a share reaches them: directly, through a
// team, through the organization, or through a project they can browse.
const filterViewSQL = `
SELECT f.id, f.owner_id, f.organization_id, f.name, f.description, f.query_text, f.columns, f.sort,
       f.created_at, f.updated_at, ou.display_name AS owner_name,
       (f.owner_id = $1::uuid) AS is_owner,
       CASE WHEN EXISTS (SELECT 1 FROM organization_members m JOIN organizations o ON o.id = m.organization_id AND o.deleted_at IS NULL
                         WHERE m.organization_id = f.organization_id AND m.user_id = $1::uuid)
            THEN (SELECT CASE WHEN count(*) = 0 THEN '' WHEN bool_or(fs.permission = 'edit') THEN 'edit' ELSE 'view' END
                  FROM filter_shares fs WHERE fs.filter_id = f.id AND (
                       (fs.share_type = 'user' AND fs.target_id = $1::uuid)
                    OR (fs.share_type = 'team' AND fs.target_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id = $1::uuid))
                    OR (fs.share_type = 'organization' AND fs.target_id = f.organization_id)
                    OR (fs.share_type = 'project' AND EXISTS (
                            SELECT 1 FROM projects sp WHERE sp.id = fs.target_id AND sp.deleted_at IS NULL
                              AND sp.organization_id = f.organization_id AND ` + `%VISIBLE%` + `))))
            ELSE '' END AS shared_permission,
       EXISTS (SELECT 1 FROM favorites fav WHERE fav.user_id = $1::uuid AND fav.entity_type = 'filter' AND fav.entity_id = f.id) AS is_favorite,
       EXISTS (SELECT 1 FROM filter_subscriptions sub WHERE sub.filter_id = f.id AND sub.user_id = $1::uuid) AS has_subscription
FROM filters f
JOIN users ou ON ou.id = f.owner_id`

func viewSQL() string {
	return strings.ReplaceAll(filterViewSQL, "%VISIBLE%", visibleProjectSQL("sp", "$1::uuid", "FALSE"))
}

type filterRow struct {
	ID              uuid.UUID       `db:"id"`
	OwnerID         uuid.UUID       `db:"owner_id"`
	OrganizationID  uuid.UUID       `db:"organization_id"`
	Name            string          `db:"name"`
	Description     string          `db:"description"`
	QueryText       string          `db:"query_text"`
	Columns         json.RawMessage `db:"columns"`
	Sort            string          `db:"sort"`
	CreatedAt       time.Time       `db:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"`
	OwnerName       string          `db:"owner_name"`
	IsOwner         bool            `db:"is_owner"`
	SharedPerm      string          `db:"shared_permission"`
	IsFavorite      bool            `db:"is_favorite"`
	HasSubscription bool            `db:"has_subscription"`
	Total           int             `db:"total"`
}

func (row filterRow) view() port.FilterView {
	cols := []string{}
	_ = json.Unmarshal(row.Columns, &cols)
	return port.FilterView{
		Filter: domain.Filter{
			ID: row.ID, OwnerID: row.OwnerID, OrganizationID: row.OrganizationID, Name: row.Name,
			Description: row.Description, QueryText: row.QueryText, Columns: cols, Sort: row.Sort,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		},
		OwnerName:       row.OwnerName,
		Access:          domain.ResolveAccess(row.IsOwner, domain.Permission(row.SharedPerm)),
		IsFavorite:      row.IsFavorite,
		HasSubscription: row.HasSubscription,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// OrganizationFor resolves the organization of a new filter.
func (r *FilterRepository) OrganizationFor(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.GetContext(ctx, &id, `
		SELECT om.organization_id FROM organization_members om
		JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
		WHERE om.user_id = $1::uuid AND ($2::uuid IS NULL OR om.organization_id = $2::uuid)
		ORDER BY om.joined_at, om.organization_id LIMIT 1`, userID, nullUUID(requested))
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, apperror.WithCode(apperror.KindValidation, domain.CodeFilterInvalid, "no perteneces a ninguna organización")
	}
	if err != nil {
		return uuid.Nil, apperror.Wrap(apperror.KindInternal, "failed to resolve the filter organization", err)
	}
	return id, nil
}

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func columnsJSON(cols []string) []byte {
	if cols == nil {
		cols = []string{}
	}
	raw, _ := json.Marshal(cols)
	return raw
}

// Create inserts a filter.
func (r *FilterRepository) Create(ctx context.Context, ownerID, orgID uuid.UUID, f domain.FilterFields, now time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.GetContext(ctx, &id, `
		INSERT INTO filters (owner_id, organization_id, name, description, query_text, columns, sort, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $8) RETURNING id`,
		ownerID, orgID, f.Name, f.Description, f.QueryText, string(columnsJSON(f.Columns)), f.Sort, now)
	if isUniqueViolation(err) {
		return uuid.Nil, domain.ErrFilterNameTaken
	}
	if err != nil {
		return uuid.Nil, apperror.Wrap(apperror.KindInternal, "failed to create the filter", err)
	}
	return id, nil
}

// Get returns the filter with the viewer's access, or not-found when the viewer cannot see it.
func (r *FilterRepository) Get(ctx context.Context, viewerID, id uuid.UUID) (*port.FilterView, error) {
	var row filterRow
	err := r.db.GetContext(ctx, &row, `SELECT x.*, 0 AS total FROM (`+viewSQL()+` WHERE f.id = $2::uuid) x WHERE x.is_owner OR x.shared_permission <> ''`, viewerID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrFilterNotFound
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read the filter", err)
	}
	v := row.view()
	return &v, nil
}

// List returns a page of the visible filters and the total.
func (r *FilterRepository) List(ctx context.Context, viewerID uuid.UUID, q port.FilterListQuery) ([]port.FilterView, int, error) {
	cond := "x.is_owner OR x.shared_permission <> ''"
	switch q.Scope {
	case port.ScopeMine:
		cond = "x.is_owner"
	case port.ScopeShared:
		cond = "NOT x.is_owner AND x.shared_permission <> ''"
	case port.ScopeFavorite:
		cond = "x.is_favorite AND (x.is_owner OR x.shared_permission <> '')"
	}
	var rows []filterRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT x.*, count(*) OVER () AS total FROM (`+viewSQL()+`) x
		WHERE (`+cond+`) AND ($2::text = '' OR x.name ILIKE $3 ESCAPE '\')
		ORDER BY lower(x.name), x.id LIMIT $4 OFFSET $5`,
		viewerID, q.Text, likePattern(q.Text), q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.KindInternal, "failed to list filters", err)
	}
	out := make([]port.FilterView, 0, len(rows))
	total := 0
	for _, row := range rows {
		out = append(out, row.view())
		total = row.Total
	}
	return out, total, nil
}

// Update rewrites the editable fields.
func (r *FilterRepository) Update(ctx context.Context, id uuid.UUID, f domain.FilterFields, now time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE filters SET name = $2, description = $3, query_text = $4, columns = $5::jsonb, sort = $6, updated_at = $7
		WHERE id = $1`, id, f.Name, f.Description, f.QueryText, string(columnsJSON(f.Columns)), f.Sort, now)
	if isUniqueViolation(err) {
		return domain.ErrFilterNameTaken
	}
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update the filter", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrFilterNotFound
	}
	return nil
}

// Delete removes a filter; shares and subscriptions go with it. Favorites of the filter are cleaned too.
func (r *FilterRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete the filter", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `DELETE FROM filters WHERE id = $1`, id)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete the filter", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrFilterNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM favorites WHERE entity_type = 'filter' AND entity_id = $1`, id); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete the filter", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM recent_items WHERE entity_type = 'filter' AND entity_id = $1`, id); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete the filter", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete the filter", err)
	}
	return nil
}

// Shares lists the shares of a filter with the name of each target.
func (r *FilterRepository) Shares(ctx context.Context, id uuid.UUID) ([]port.ShareView, error) {
	var rows []struct {
		Type       string    `db:"share_type"`
		TargetID   uuid.UUID `db:"target_id"`
		Permission string    `db:"permission"`
		Name       string    `db:"target_name"`
	}
	err := r.db.SelectContext(ctx, &rows, `
		SELECT fs.share_type, fs.target_id, fs.permission,
		       COALESCE(CASE fs.share_type
		           WHEN 'user' THEN (SELECT u.display_name FROM users u WHERE u.id = fs.target_id)
		           WHEN 'team' THEN (SELECT t.name FROM teams t WHERE t.id = fs.target_id)
		           WHEN 'project' THEN (SELECT p.name FROM projects p WHERE p.id = fs.target_id)
		           WHEN 'organization' THEN (SELECT o.name FROM organizations o WHERE o.id = fs.target_id)
		       END, '') AS target_name
		FROM filter_shares fs WHERE fs.filter_id = $1
		ORDER BY fs.share_type, target_name, fs.target_id`, id)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list filter shares", err)
	}
	out := make([]port.ShareView, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.ShareView{
			Share:      domain.Share{Type: domain.ShareType(row.Type), TargetID: row.TargetID, Permission: domain.Permission(row.Permission)},
			TargetName: row.Name,
		})
	}
	return out, nil
}

// ReplaceShares replaces every share of a filter. Every target must belong to the filter's organization.
func (r *FilterRepository) ReplaceShares(ctx context.Context, id, orgID uuid.UUID, shares []domain.Share, now time.Time) error {
	types := make([]string, len(shares))
	ids := make([]string, len(shares))
	perms := make([]string, len(shares))
	for i, s := range shares {
		types[i], ids[i], perms[i] = string(s.Type), s.TargetID.String(), string(s.Permission)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update filter shares", err)
	}
	defer func() { _ = tx.Rollback() }()
	var valid int
	if err := tx.GetContext(ctx, &valid, `
		SELECT count(*) FROM unnest($1::text[], $2::uuid[]) AS t(kind, target)
		WHERE (t.kind = 'user' AND EXISTS (SELECT 1 FROM organization_members m WHERE m.organization_id = $3::uuid AND m.user_id = t.target))
		   OR (t.kind = 'team' AND EXISTS (SELECT 1 FROM teams x WHERE x.id = t.target AND x.organization_id = $3::uuid))
		   OR (t.kind = 'project' AND EXISTS (SELECT 1 FROM projects x WHERE x.id = t.target AND x.organization_id = $3::uuid AND x.deleted_at IS NULL))
		   OR (t.kind = 'organization' AND t.target = $3::uuid)`, types, ids, orgID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to validate filter shares", err)
	}
	if valid != len(shares) {
		return apperror.WithCode(apperror.KindValidation, domain.CodeShareInvalid, "algún destino no pertenece a la organización del filtro")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM filter_shares WHERE filter_id = $1`, id); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update filter shares", err)
	}
	if len(shares) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO filter_shares (filter_id, share_type, target_id, permission, created_at)
			SELECT $1, t.kind, t.target, t.permission, $5 FROM unnest($2::text[], $3::uuid[], $4::text[]) AS t(kind, target, permission)`,
			id, types, ids, perms, now); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to update filter shares", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE filters SET updated_at = $2 WHERE id = $1`, id, now); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update filter shares", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update filter shares", err)
	}
	return nil
}

type subscriptionRow struct {
	ID         uuid.UUID       `db:"id"`
	FilterID   uuid.UUID       `db:"filter_id"`
	UserID     uuid.UUID       `db:"user_id"`
	Schedule   json.RawMessage `db:"schedule"`
	LastSentAt *time.Time      `db:"last_sent_at"`
	CreatedAt  time.Time       `db:"created_at"`
}

const subscriptionColumns = `id, filter_id, user_id, schedule, last_sent_at, created_at`

func (row subscriptionRow) subscription() domain.Subscription {
	var sch domain.Schedule
	_ = json.Unmarshal(row.Schedule, &sch)
	return domain.Subscription{ID: row.ID, FilterID: row.FilterID, UserID: row.UserID, Schedule: sch, LastSentAt: row.LastSentAt, CreatedAt: row.CreatedAt}
}

// GetSubscription returns the user's subscription to a filter, or nil when there is none.
func (r *FilterRepository) GetSubscription(ctx context.Context, filterID, userID uuid.UUID) (*domain.Subscription, error) {
	var row subscriptionRow
	err := r.db.GetContext(ctx, &row, `SELECT `+subscriptionColumns+` FROM filter_subscriptions WHERE filter_id = $1 AND user_id = $2`, filterID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read the subscription", err)
	}
	s := row.subscription()
	return &s, nil
}

// UpsertSubscription stores the schedule. Changing the schedule restarts the baseline, so the next delivery
// is the next scheduled slot.
func (r *FilterRepository) UpsertSubscription(ctx context.Context, filterID, userID uuid.UUID, schedule domain.Schedule, now time.Time) (domain.Subscription, error) {
	raw, _ := json.Marshal(schedule)
	var row subscriptionRow
	err := r.db.GetContext(ctx, &row, `
		INSERT INTO filter_subscriptions (filter_id, user_id, schedule, created_at)
		VALUES ($1, $2, $3::jsonb, $4)
		ON CONFLICT (filter_id, user_id) DO UPDATE SET schedule = EXCLUDED.schedule, created_at = EXCLUDED.created_at
		RETURNING `+subscriptionColumns, filterID, userID, string(raw), now)
	if err != nil {
		return domain.Subscription{}, apperror.Wrap(apperror.KindInternal, "failed to save the subscription", err)
	}
	return row.subscription(), nil
}

// DeleteSubscription removes the user's subscription.
func (r *FilterRepository) DeleteSubscription(ctx context.Context, filterID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM filter_subscriptions WHERE filter_id = $1 AND user_id = $2`, filterID, userID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete the subscription", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}

// ListSubscriptions pages through subscriptions ordered by id.
func (r *FilterRepository) ListSubscriptions(ctx context.Context, after uuid.UUID, limit int) ([]domain.Subscription, error) {
	var rows []subscriptionRow
	err := r.db.SelectContext(ctx, &rows, `SELECT `+subscriptionColumns+` FROM filter_subscriptions WHERE id > $1 ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list subscriptions", err)
	}
	out := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.subscription())
	}
	return out, nil
}

// MarkSubscriptionSent records the time of a delivery.
func (r *FilterRepository) MarkSubscriptionSent(ctx context.Context, id uuid.UUID, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE filter_subscriptions SET last_sent_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to record the delivery", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}
