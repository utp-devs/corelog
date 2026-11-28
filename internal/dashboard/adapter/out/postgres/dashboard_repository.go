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

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Repository stores dashboards, gadgets and shares.
// querier is what the repositories need of the database handle; *sqlx.DB satisfies it.
type querier interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

type Repository struct {
	db querier
}

var _ port.Repository = (*Repository)(nil)

// NewRepository builds the repository.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

// visibleProjectSQL is the browse rule of a project row aliased as alias: the platform administrator sees every
// live project; anyone else needs a live organization membership and then either an owner or admin role, an
// explicit project role, a team grant, or the member role on a project that is not private.
func visibleProjectSQL(alias, viewer, admin string) string {
	return "(" + admin + " OR EXISTS (SELECT 1 FROM organization_members vom" +
		" JOIN organizations vo ON vo.id = vom.organization_id AND vo.deleted_at IS NULL" +
		" WHERE vom.organization_id = " + alias + ".organization_id AND vom.user_id = " + viewer +
		" AND (vom.role IN ('owner', 'admin')" +
		" OR EXISTS (SELECT 1 FROM project_members vpm WHERE vpm.project_id = " + alias + ".id AND vpm.user_id = " + viewer + ")" +
		" OR EXISTS (SELECT 1 FROM project_team_roles vptr JOIN team_members vtm ON vtm.team_id = vptr.team_id" +
		" WHERE vptr.project_id = " + alias + ".id AND vtm.user_id = " + viewer + ")" +
		" OR (vom.role = 'member' AND " + alias + ".visibility <> 'private'))))"
}

// dashboardViewSQL selects dashboards with the access of the viewer ($1): the owner, a live member of the
// organization reached by a share (directly, through a team, the organization or a project they can browse), or
// an organization owner or admin (read only). The WHERE clause is added by the callers on the wrapped query.
var dashboardViewSQL = `
SELECT * FROM (
SELECT d.id, d.owner_id, d.organization_id, d.name, d.description, d.layout, d.is_default, d.created_at, d.updated_at,
       ou.display_name AS owner_name,
       (d.owner_id = $1::uuid) AS is_owner,
       CASE WHEN m.role IS NOT NULL
            THEN (SELECT CASE WHEN count(*) = 0 THEN '' WHEN bool_or(ds.permission = 'edit') THEN 'edit' ELSE 'view' END
                  FROM dashboard_shares ds WHERE ds.dashboard_id = d.id AND (
                       (ds.share_type = 'user' AND ds.target_id = $1::uuid)
                    OR (ds.share_type = 'team' AND ds.target_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id = $1::uuid))
                    OR (ds.share_type = 'organization' AND ds.target_id = d.organization_id)
                    OR (ds.share_type = 'project' AND EXISTS (
                            SELECT 1 FROM projects sp WHERE sp.id = ds.target_id AND sp.deleted_at IS NULL
                              AND sp.organization_id = d.organization_id AND ` + visibleProjectSQL("sp", "$1::uuid", "FALSE") + `))))
            ELSE '' END AS shared_permission,
       COALESCE(m.role IN ('owner', 'admin'), FALSE) AS is_org_admin,
       EXISTS (SELECT 1 FROM favorites fav WHERE fav.user_id = $1::uuid AND fav.entity_type = 'dashboard' AND fav.entity_id = d.id) AS is_favorite,
       EXISTS (SELECT 1 FROM dashboard_shares any_share WHERE any_share.dashboard_id = d.id) AS has_shares
FROM dashboards d
JOIN users ou ON ou.id = d.owner_id
LEFT JOIN (SELECT om.organization_id, om.role FROM organization_members om
           JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
           WHERE om.user_id = $1::uuid) m ON m.organization_id = d.organization_id
) v WHERE (v.is_owner OR v.shared_permission <> '' OR v.is_org_admin)`

type dashboardRow struct {
	ID             uuid.UUID       `db:"id"`
	OwnerID        uuid.UUID       `db:"owner_id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	Name           string          `db:"name"`
	Description    string          `db:"description"`
	Layout         json.RawMessage `db:"layout"`
	IsDefault      bool            `db:"is_default"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	OwnerName      string          `db:"owner_name"`
	IsOwner        bool            `db:"is_owner"`
	SharedPerm     string          `db:"shared_permission"`
	IsOrgAdmin     bool            `db:"is_org_admin"`
	IsFavorite     bool            `db:"is_favorite"`
	HasShares      bool            `db:"has_shares"`
	Total          int             `db:"total"`
}

func layoutOf(raw json.RawMessage) domain.Layout {
	var l struct {
		Template string `json:"template"`
	}
	if err := json.Unmarshal(raw, &l); err != nil || !domain.Layout(l.Template).Valid() {
		return domain.Layout2
	}
	return domain.Layout(l.Template)
}

func layoutJSON(l domain.Layout) []byte {
	raw, _ := json.Marshal(map[string]string{"template": string(l)})
	return raw
}

func (r dashboardRow) view() port.DashboardView {
	perm := domain.Permission(r.SharedPerm)
	access := domain.ResolveAccess(r.IsOwner, perm)
	if access == domain.AccessNone && r.IsOrgAdmin {
		access = domain.AccessView
	}
	return port.DashboardView{
		Dashboard: domain.Dashboard{
			ID: r.ID, OwnerID: r.OwnerID, OrganizationID: r.OrganizationID, Name: r.Name, Description: r.Description,
			Layout: layoutOf(r.Layout), IsDefault: r.IsDefault, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		},
		OwnerName: r.OwnerName, Access: access, IsFavorite: r.IsFavorite, Shared: r.HasShares,
	}
}

type gadgetRow struct {
	ID          uuid.UUID       `db:"id"`
	DashboardID uuid.UUID       `db:"dashboard_id"`
	Type        string          `db:"type"`
	Title       string          `db:"title"`
	Config      json.RawMessage `db:"config"`
	Position    int             `db:"position"`
	ColumnIndex int             `db:"column_index"`
}

func (r *Repository) gadgetsOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.Gadget, error) {
	out := map[uuid.UUID][]domain.Gadget{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []gadgetRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, dashboard_id, type, title, config, position, column_index FROM dashboard_gadgets
		WHERE dashboard_id = ANY($1::uuid[]) ORDER BY dashboard_id, column_index, position, created_at, id`, uuidArray(ids)); err != nil {
		return nil, internal("failed to read the gadgets", err)
	}
	for _, g := range rows {
		out[g.DashboardID] = append(out[g.DashboardID], domain.Gadget{ID: g.ID, Type: g.Type, Title: g.Title, Config: g.Config, Position: g.Position, ColumnIndex: g.ColumnIndex})
	}
	return out, nil
}

func uuidArray(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// OrganizationFor resolves the organization of a new dashboard.
func (r *Repository) OrganizationFor(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error) {
	var req any
	if requested != uuid.Nil {
		req = requested
	}
	var id uuid.UUID
	err := r.db.GetContext(ctx, &id, `
		SELECT om.organization_id FROM organization_members om
		JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
		WHERE om.user_id = $1::uuid AND ($2::uuid IS NULL OR om.organization_id = $2::uuid)
		ORDER BY om.joined_at, om.organization_id LIMIT 1`, userID, req)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, domain.ErrNoOrganization
	}
	if err != nil {
		return uuid.Nil, internal("failed to resolve the dashboard organization", err)
	}
	return id, nil
}

// List returns the dashboards the viewer can see, newest first.
func (r *Repository) List(ctx context.Context, viewer port.Viewer, q port.ListQuery) ([]port.DashboardView, int, error) {
	scope := q.Scope
	if scope == "" {
		scope = "all"
	}
	var rows []dashboardRow
	err := r.db.SelectContext(ctx, &rows, `SELECT w.*, count(*) OVER() AS total FROM (`+dashboardViewSQL+`
		  AND ($2 = 'all' OR ($2 = 'mine' AND v.is_owner) OR ($2 = 'shared' AND NOT v.is_owner))
		  AND ($3 = '' OR v.name ILIKE '%' || $3 || '%')) w
		ORDER BY w.is_owner DESC, w.is_default DESC, w.updated_at DESC, w.id
		LIMIT $4 OFFSET $5`, viewer.UserID, scope, escapeLike(q.Query), q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return nil, 0, internal("failed to list the dashboards", err)
	}
	total := 0
	ids := make([]uuid.UUID, len(rows))
	views := make([]port.DashboardView, len(rows))
	for i, row := range rows {
		total = row.Total
		ids[i] = row.ID
		views[i] = row.view()
	}
	gadgets, err := r.gadgetsOf(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range views {
		views[i].Gadgets = gadgets[views[i].ID]
	}
	return views, total, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}

// Get returns one visible dashboard with its gadgets.
func (r *Repository) Get(ctx context.Context, viewer port.Viewer, id uuid.UUID) (*port.DashboardView, error) {
	var row dashboardRow
	err := r.db.GetContext(ctx, &row, `SELECT w.*, 0 AS total FROM (`+dashboardViewSQL+` AND v.id = $2) w`, viewer.UserID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, internal("failed to read the dashboard", err)
	}
	return r.withGadgets(ctx, row)
}

func (r *Repository) withGadgets(ctx context.Context, row dashboardRow) (*port.DashboardView, error) {
	view := row.view()
	gadgets, err := r.gadgetsOf(ctx, []uuid.UUID{row.ID})
	if err != nil {
		return nil, err
	}
	view.Gadgets = gadgets[row.ID]
	return &view, nil
}

// FindDefault returns the default dashboard of the viewer.
func (r *Repository) FindDefault(ctx context.Context, viewer port.Viewer) (*port.DashboardView, error) {
	var row dashboardRow
	err := r.db.GetContext(ctx, &row, `SELECT w.*, 0 AS total FROM (`+dashboardViewSQL+` AND v.is_owner AND v.is_default) w`, viewer.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internal("failed to read the default dashboard", err)
	}
	return r.withGadgets(ctx, row)
}

func mapWriteError(err error, msg string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		switch pg.ConstraintName {
		case "uq_dashboards_owner_name":
			return domain.ErrNameTaken
		case "uq_dashboards_owner_default":
			return apperror.WithCode(apperror.KindConflict, "dashboard.default_conflict", "otra petición cambió el panel principal; inténtalo de nuevo")
		}
	}
	return internal(msg, err)
}

func (r *Repository) insertGadgets(ctx context.Context, tx *sqlx.Tx, dashboardID uuid.UUID, gadgets []domain.Gadget, at time.Time) ([]domain.Gadget, error) {
	out := make([]domain.Gadget, len(gadgets))
	for i, g := range gadgets {
		if g.ID == uuid.Nil {
			g.ID = uuid.New()
		}
		config := g.Config
		if len(config) == 0 {
			config = json.RawMessage(`{}`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dashboard_gadgets (id, dashboard_id, type, title, config, position, column_index, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $8)`,
			g.ID, dashboardID, g.Type, g.Title, []byte(config), g.Position, g.ColumnIndex, at); err != nil {
			return nil, internal("failed to store a gadget", err)
		}
		out[i] = g
	}
	return out, nil
}

// Create stores the dashboard and its gadgets.
func (r *Repository) Create(ctx context.Context, d *domain.Dashboard) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the dashboard transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	if d.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE dashboards SET is_default = FALSE WHERE owner_id = $1 AND is_default`, d.OwnerID); err != nil {
			return internal("failed to replace the default dashboard", err)
		}
	}
	err = tx.GetContext(ctx, &d.ID, `
		INSERT INTO dashboards (owner_id, organization_id, name, description, layout, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8) RETURNING id`,
		d.OwnerID, d.OrganizationID, d.Name, d.Description, layoutJSON(d.Layout), d.IsDefault, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return mapWriteError(err, "failed to create the dashboard")
	}
	if d.Gadgets, err = r.insertGadgets(ctx, tx, d.ID, d.Gadgets, d.CreatedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the dashboard", err)
	}
	return nil
}

// UpdateFields changes the dashboard fields.
func (r *Repository) UpdateFields(ctx context.Context, id uuid.UUID, p port.FieldsPatch, at time.Time) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the dashboard transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	var owner uuid.UUID
	if err := tx.GetContext(ctx, &owner, `SELECT owner_id FROM dashboards WHERE id = $1 FOR UPDATE`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return internal("failed to lock the dashboard", err)
	}
	if p.IsDefault != nil && *p.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE dashboards SET is_default = FALSE WHERE owner_id = $1 AND is_default AND id <> $2`, owner, id); err != nil {
			return internal("failed to replace the default dashboard", err)
		}
	}
	var layout any
	if p.Layout != nil {
		layout = layoutJSON(*p.Layout)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE dashboards SET name = COALESCE($2, name), description = COALESCE($3, description),
			layout = COALESCE($4::jsonb, layout), is_default = COALESCE($5, is_default), updated_at = $6
		WHERE id = $1`, id, p.Name, p.Description, layout, p.IsDefault, at); err != nil {
		return mapWriteError(err, "failed to update the dashboard")
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the dashboard", err)
	}
	return nil
}

// Delete removes a dashboard and the stars and recents that pointed at it.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the dashboard transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `DELETE FROM dashboards WHERE id = $1`, id)
	if err != nil {
		return internal("failed to delete the dashboard", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	for _, table := range []string{"favorites", "recent_items"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE entity_type = 'dashboard' AND entity_id = $1`, id); err != nil {
			return internal("failed to clean the references to the dashboard", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the deletion", err)
	}
	return nil
}

// ReplaceGadgets swaps the gadget list atomically.
func (r *Repository) ReplaceGadgets(ctx context.Context, id uuid.UUID, layout *domain.Layout, gadgets []domain.Gadget, at time.Time) ([]domain.Gadget, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, internal("failed to begin the dashboard transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	var locked uuid.UUID
	if err := tx.GetContext(ctx, &locked, `SELECT id FROM dashboards WHERE id = $1 FOR UPDATE`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, internal("failed to lock the dashboard", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dashboard_gadgets WHERE dashboard_id = $1`, id); err != nil {
		return nil, internal("failed to clear the gadgets", err)
	}
	stored, err := r.insertGadgets(ctx, tx, id, gadgets, at)
	if err != nil {
		return nil, err
	}
	var layoutArg any
	if layout != nil {
		layoutArg = layoutJSON(*layout)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dashboards SET layout = COALESCE($2::jsonb, layout), updated_at = $3 WHERE id = $1`, id, layoutArg, at); err != nil {
		return nil, internal("failed to update the dashboard", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, internal("failed to commit the gadgets", err)
	}
	return stored, nil
}

type shareRow struct {
	Type       string    `db:"share_type"`
	TargetID   uuid.UUID `db:"target_id"`
	Permission string    `db:"permission"`
	Name       string    `db:"name"`
}

// Shares lists the shares of a dashboard with the name of each target.
func (r *Repository) Shares(ctx context.Context, id uuid.UUID) ([]port.ShareView, error) {
	var rows []shareRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT s.share_type, s.target_id, s.permission,
		       COALESCE(u.display_name, t.name, p.name, o.name, '') AS name
		FROM dashboard_shares s
		LEFT JOIN users u ON s.share_type = 'user' AND u.id = s.target_id
		LEFT JOIN teams t ON s.share_type = 'team' AND t.id = s.target_id
		LEFT JOIN projects p ON s.share_type = 'project' AND p.id = s.target_id
		LEFT JOIN organizations o ON s.share_type = 'organization' AND o.id = s.target_id
		WHERE s.dashboard_id = $1 ORDER BY s.share_type, lower(COALESCE(u.display_name, t.name, p.name, o.name, '')), s.target_id`, id)
	if err != nil {
		return nil, internal("failed to read the shares", err)
	}
	out := make([]port.ShareView, len(rows))
	for i, row := range rows {
		out[i] = port.ShareView{Share: domain.Share{Type: domain.ShareType(row.Type), TargetID: row.TargetID, Permission: domain.Permission(row.Permission)}, TargetName: row.Name}
	}
	return out, nil
}

// ReplaceShares swaps the shares after checking that every target belongs to the organization.
func (r *Repository) ReplaceShares(ctx context.Context, id, organizationID uuid.UUID, shares []domain.Share, at time.Time) error {
	types := make([]string, len(shares))
	targets := make([]uuid.UUID, len(shares))
	perms := make([]string, len(shares))
	for i, s := range shares {
		types[i], targets[i], perms[i] = string(s.Type), s.TargetID, string(s.Permission)
	}
	typeArr, permArr := textArray(types), textArray(perms)
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the dashboard transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	var valid int
	if err := tx.GetContext(ctx, &valid, `
		SELECT count(*) FROM unnest($1::text[], $2::uuid[]) AS s(t, tid)
		WHERE (t = 'user' AND EXISTS (SELECT 1 FROM organization_members om WHERE om.organization_id = $3 AND om.user_id = s.tid))
		   OR (t = 'team' AND EXISTS (SELECT 1 FROM teams tm WHERE tm.id = s.tid AND tm.organization_id = $3))
		   OR (t = 'project' AND EXISTS (SELECT 1 FROM projects p WHERE p.id = s.tid AND p.organization_id = $3 AND p.deleted_at IS NULL))
		   OR (t = 'organization' AND s.tid = $3)`, typeArr, uuidArray(targets), organizationID); err != nil {
		return internal("failed to validate the share targets", err)
	}
	if valid != len(shares) {
		return apperror.WithCode(apperror.KindValidation, domain.CodeInvalidShare, "alguna compartición apunta a un destino que no existe en la organización").
			WithDetails(apperror.FieldError{Field: "shares", Message: "todos los destinos deben existir en la organización del panel"})
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dashboard_shares WHERE dashboard_id = $1`, id); err != nil {
		return internal("failed to clear the shares", err)
	}
	if len(shares) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dashboard_shares (dashboard_id, share_type, target_id, permission, created_at)
			SELECT $1, s.t, s.tid, s.perm, $5 FROM unnest($2::text[], $3::uuid[], $4::text[]) AS s(t, tid, perm)`,
			id, typeArr, uuidArray(targets), permArr, at); err != nil {
			return internal("failed to store the shares", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dashboards SET updated_at = $2 WHERE id = $1`, id, at); err != nil {
		return internal("failed to update the dashboard", err)
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the shares", err)
	}
	return nil
}

func textArray(items []string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(it, `\`, `\\`), `"`, `\"`) + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
}
