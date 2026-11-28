package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

const (
	pgUniqueViolation = "23505"
	// Names of the unique indexes of issue_types that map to a specific domain error.
	constraintIssueTypeHierarchy = "uq_issue_types_org_hierarchy"
)

// CatalogRepository stores the catalogs of an organization and the project issue-type settings.
type CatalogRepository struct {
	db *sqlx.DB
}

var (
	_ port.CatalogStore = (*CatalogRepository)(nil)
	_ port.IssueCatalog = (*CatalogRepository)(nil)
)

func NewCatalogRepository(db *sqlx.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

// catalogSpec describes the table and the optional columns of one catalog kind. Table and column names come
// from this fixed map, never from user input.
type catalogSpec struct {
	table       string
	usageColumn string
	legacyType  bool
	hasDesc     bool
	hasIcon     bool
	hasLevel    bool
	hasLegacy   bool
}

var catalogSpecs = map[domain.CatalogKind]catalogSpec{
	domain.CatalogIssueType:  {table: "issue_types", usageColumn: "issue_type_id", legacyType: true, hasDesc: true, hasIcon: true, hasLevel: true, hasLegacy: true},
	domain.CatalogPriority:   {table: "priorities", usageColumn: "priority_id", hasIcon: true, hasLegacy: true},
	domain.CatalogResolution: {table: "resolutions", usageColumn: "resolution_id", hasDesc: true},
}

// selectList returns the columns of a catalog item. Columns a kind lacks are returned as neutral values.
func (sp catalogSpec) selectList(alias string) string {
	icon := "'' AS icon, '' AS color"
	if sp.hasIcon {
		icon = alias + ".icon, " + alias + ".color"
	}
	level := "0 AS hierarchy_level"
	if sp.hasLevel {
		level = alias + ".hierarchy_level"
	}
	description := "'' AS description"
	if sp.hasDesc {
		description = alias + ".description"
	}
	legacy := "'' AS legacy_key"
	if sp.hasLegacy {
		legacy = "COALESCE(" + alias + ".legacy_key, '') AS legacy_key"
	}
	return strings.Join([]string{
		alias + ".id", alias + ".organization_id", alias + ".name", description,
		icon, level, alias + ".position", alias + ".is_default", legacy,
		"(SELECT COUNT(*) FROM issues iu WHERE iu." + sp.usageColumn + " = " + alias + ".id)::int AS usage",
		alias + ".created_at", alias + ".updated_at",
	}, ", ")
}

type catalogRow struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	Icon           string    `db:"icon"`
	Color          string    `db:"color"`
	HierarchyLevel int       `db:"hierarchy_level"`
	Position       int       `db:"position"`
	IsDefault      bool      `db:"is_default"`
	LegacyKey      string    `db:"legacy_key"`
	Usage          int       `db:"usage"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

func (r catalogRow) toDomain(kind domain.CatalogKind) *domain.CatalogItem {
	return &domain.CatalogItem{
		ID: r.ID, OrganizationID: r.OrganizationID, Kind: kind, Name: r.Name, Description: r.Description,
		Icon: r.Icon, Color: r.Color, HierarchyLevel: r.HierarchyLevel, Position: r.Position,
		IsDefault: r.IsDefault, LegacyKey: r.LegacyKey, Usage: r.Usage, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// LockOrganization serialises catalog changes of one organization until the transaction ends.
func (r *CatalogRepository) LockOrganization(ctx context.Context, orgID uuid.UUID) error {
	var id uuid.UUID
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx,
		`SELECT id FROM organizations WHERE id = $1 FOR UPDATE`, orgID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.New(apperror.KindNotFound, "organization not found")
	}
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to lock organization catalogs", err)
	}
	return nil
}

func (r *CatalogRepository) ListItems(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind) ([]*domain.CatalogItem, error) {
	sp, err := specOf(kind)
	if err != nil {
		return nil, err
	}
	var rows []catalogRow
	query := "SELECT " + sp.selectList("t") + " FROM " + sp.table + " t WHERE t.organization_id = $1 AND t.deleted_at IS NULL ORDER BY t.position, t.name"
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, query, orgID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list catalog", err)
	}
	items := make([]*domain.CatalogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain(kind))
	}
	return items, nil
}

func (r *CatalogRepository) FindItem(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID) (*domain.CatalogItem, error) {
	sp, err := specOf(kind)
	if err != nil {
		return nil, err
	}
	var row catalogRow
	query := "SELECT " + sp.selectList("t") + " FROM " + sp.table + " t WHERE t.organization_id = $1 AND t.id = $2 AND t.deleted_at IS NULL"
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, query, orgID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCatalogNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find catalog item", err)
	}
	return row.toDomain(kind), nil
}

// InsertItem stores a new item and sets its id. Position, default flag and timestamps come from the item.
func (r *CatalogRepository) InsertItem(ctx context.Context, item *domain.CatalogItem) error {
	sp, err := specOf(item.Kind)
	if err != nil {
		return err
	}
	columns := []string{"organization_id", "name"}
	args := []any{item.OrganizationID, item.Name}
	if sp.hasDesc {
		columns = append(columns, "description")
		args = append(args, item.Description)
	}
	if sp.hasIcon {
		columns = append(columns, "icon", "color")
		args = append(args, item.Icon, item.Color)
	}
	if sp.hasLevel {
		columns = append(columns, "hierarchy_level")
		args = append(args, item.HierarchyLevel)
	}
	columns = append(columns, "position", "is_default")
	args = append(args, item.Position, item.IsDefault)
	if sp.hasLegacy && item.LegacyKey != "" {
		columns = append(columns, "legacy_key")
		args = append(args, item.LegacyKey)
	}
	columns = append(columns, "created_at", "updated_at")
	args = append(args, item.CreatedAt, item.UpdatedAt)

	placeholders := make([]string, len(args))
	for i := range args {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		sp.table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, query, args...).Scan(&item.ID); err != nil {
		return mapCatalogWriteError(err, "failed to create catalog item")
	}
	return nil
}

func (r *CatalogRepository) UpdateItem(ctx context.Context, item *domain.CatalogItem) error {
	sp, err := specOf(item.Kind)
	if err != nil {
		return err
	}
	sets := []string{"name = $1"}
	args := []any{item.Name}
	if sp.hasDesc {
		sets = append(sets, fmt.Sprintf("description = $%d", len(args)+1))
		args = append(args, item.Description)
	}
	if sp.hasIcon {
		sets = append(sets, fmt.Sprintf("icon = $%d", len(args)+1), fmt.Sprintf("color = $%d", len(args)+2))
		args = append(args, item.Icon, item.Color)
	}
	sets = append(sets, fmt.Sprintf("updated_at = $%d", len(args)+1))
	args = append(args, item.UpdatedAt)
	args = append(args, item.ID, item.OrganizationID)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d AND organization_id = $%d AND deleted_at IS NULL",
		sp.table, strings.Join(sets, ", "), len(args)-1, len(args))

	result, err := database.Executor(ctx, r.db).ExecContext(ctx, query, args...)
	if err != nil {
		return mapCatalogWriteError(err, "failed to update catalog item")
	}
	if n, err := result.RowsAffected(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update catalog item", err)
	} else if n == 0 {
		return domain.ErrCatalogNotFound
	}
	return nil
}

// DeleteItem logically deletes an item: the row stays so every issue that used it keeps its name for history and
// statistics. The delete is refused while any issue still uses the item, which is reported as in use.
func (r *CatalogRepository) DeleteItem(ctx context.Context, item *domain.CatalogItem) error {
	sp, err := specOf(item.Kind)
	if err != nil {
		return err
	}
	exec := database.Executor(ctx, r.db)
	var inUse bool
	if err := sqlx.GetContext(ctx, exec, &inUse, fmt.Sprintf(
		"SELECT EXISTS (SELECT 1 FROM issues WHERE %s = $1)", sp.usageColumn), item.ID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to check catalog item usage", err)
	}
	if inUse {
		return domain.ErrCatalogInUse
	}
	query := fmt.Sprintf("UPDATE %s SET deleted_at = now(), is_default = FALSE WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL", sp.table)
	result, err := exec.ExecContext(ctx, query, item.ID, item.OrganizationID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete catalog item", err)
	}
	if n, err := result.RowsAffected(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete catalog item", err)
	} else if n == 0 {
		return domain.ErrCatalogNotFound
	}
	return nil
}

// SetDefaultItem clears the default flag of the kind and sets it on id. Two statements keep the partial unique
// index happy: the old default must be cleared before the new one is set.
func (r *CatalogRepository) SetDefaultItem(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID) error {
	sp, err := specOf(kind)
	if err != nil {
		return err
	}
	exec := database.Executor(ctx, r.db)
	if _, err := exec.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET is_default = FALSE WHERE organization_id = $1 AND is_default AND id <> $2", sp.table),
		orgID, id); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to clear default catalog item", err)
	}
	if _, err := exec.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET is_default = TRUE WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL", sp.table), orgID, id); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to set default catalog item", err)
	}
	return nil
}

func (r *CatalogRepository) SetPositions(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind, ids []uuid.UUID) error {
	sp, err := specOf(kind)
	if err != nil {
		return err
	}
	exec := database.Executor(ctx, r.db)
	query := fmt.Sprintf("UPDATE %s SET position = $1 WHERE id = $2 AND organization_id = $3", sp.table)
	for i, id := range ids {
		if _, err := exec.ExecContext(ctx, query, i, id, orgID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to reorder catalog", err)
		}
	}
	return nil
}

// MoveIssues points every issue that uses from at target. The legacy enum column of the issue is updated to
// the legacy value of the target so old readers stay consistent.
func (r *CatalogRepository) MoveIssues(ctx context.Context, from, target *domain.CatalogItem) (int64, error) {
	exec := database.Executor(ctx, r.db)
	var (
		query string
		args  []any
	)
	switch from.Kind {
	case domain.CatalogIssueType:
		query = "UPDATE issues SET issue_type_id = $1, issue_type = $2 WHERE issue_type_id = $3"
		args = []any{target.ID, string(domain.LegacyIssueTypeValue(target)), from.ID}
	case domain.CatalogPriority:
		query = "UPDATE issues SET priority_id = $1, priority = $2 WHERE priority_id = $3"
		args = []any{target.ID, string(domain.LegacyPriorityValue(target)), from.ID}
	case domain.CatalogResolution:
		query = "UPDATE issues SET resolution_id = $1 WHERE resolution_id = $2"
		args = []any{target.ID, from.ID}
	default:
		return 0, fmt.Errorf("unknown catalog kind %q", from.Kind)
	}
	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to reassign issues", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to reassign issues", err)
	}
	return n, nil
}

func (r *CatalogRepository) ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx,
		`SELECT organization_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID).Scan(&orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, apperror.New(apperror.KindNotFound, "project not found")
	}
	if err != nil {
		return uuid.Nil, apperror.Wrap(apperror.KindInternal, "failed to find project organization", err)
	}
	return orgID, nil
}

// ProjectIssueTypes returns the configured types, or every type of the organization when the project has no
// configuration.
func (r *CatalogRepository) ProjectIssueTypes(ctx context.Context, projectID uuid.UUID) ([]*domain.CatalogItem, bool, error) {
	exec := database.Executor(ctx, r.db)
	var configured bool
	if err := exec.QueryRowxContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM project_issue_types WHERE project_id = $1)`, projectID).Scan(&configured); err != nil {
		return nil, false, apperror.Wrap(apperror.KindInternal, "failed to read project issue types", err)
	}
	sp := catalogSpecs[domain.CatalogIssueType]
	var rows []catalogRow
	var query string
	if configured {
		query = "SELECT " + sp.selectList("t") + " FROM project_issue_types pit JOIN issue_types t ON t.id = pit.issue_type_id " +
			"WHERE pit.project_id = $1 AND t.deleted_at IS NULL ORDER BY pit.position, t.name"
	} else {
		query = "SELECT " + sp.selectList("t") + " FROM projects p JOIN issue_types t ON t.organization_id = p.organization_id " +
			"WHERE p.id = $1 AND t.deleted_at IS NULL ORDER BY t.position, t.name"
	}
	if err := sqlx.SelectContext(ctx, exec, &rows, query, projectID); err != nil {
		return nil, false, apperror.Wrap(apperror.KindInternal, "failed to read project issue types", err)
	}
	items := make([]*domain.CatalogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain(domain.CatalogIssueType))
	}
	return items, configured, nil
}

func (r *CatalogRepository) ReplaceProjectIssueTypes(ctx context.Context, projectID uuid.UUID, issueTypeIDs []uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	if _, err := exec.ExecContext(ctx, `DELETE FROM project_issue_types WHERE project_id = $1`, projectID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to replace project issue types", err)
	}
	for i, id := range issueTypeIDs {
		if _, err := exec.ExecContext(ctx,
			`INSERT INTO project_issue_types (project_id, issue_type_id, position) VALUES ($1, $2, $3)`,
			projectID, id, i); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to replace project issue types", err)
		}
	}
	return nil
}

// Select resolves an item for an issue of the project. Default issue types are restricted to the types the
// project enables, and the first enabled standard type wins when the organization default is not enabled.
func (r *CatalogRepository) Select(ctx context.Context, projectID uuid.UUID, kind domain.CatalogKind, sel port.CatalogSelector) (*domain.CatalogItem, error) {
	sp, err := specOf(kind)
	if err != nil {
		return nil, err
	}
	var (
		where []string
		args  = []any{projectID}
		order = "t.position"
	)
	switch {
	case sel.ID != uuid.Nil:
		args = append(args, sel.ID)
		where = append(where, fmt.Sprintf("t.id = $%d", len(args)))
	case sel.LegacyKey != "":
		if !sp.hasLegacy {
			return nil, domain.ErrCatalogNotFound
		}
		args = append(args, sel.LegacyKey)
		where = append(where, fmt.Sprintf("t.legacy_key = $%d", len(args)))
	case sel.Default:
		where = append(where, "t.is_default")
		if kind == domain.CatalogIssueType {
			where = []string{"t.hierarchy_level = 0", enabledClause}
			order = "t.is_default DESC, t.position"
		}
	default:
		return nil, domain.ErrCatalogNotFound
	}
	query := "SELECT " + sp.selectList("t") + " FROM projects p JOIN " + sp.table + " t ON t.organization_id = p.organization_id " +
		"WHERE p.id = $1 AND t.deleted_at IS NULL AND " + strings.Join(where, " AND ") + " ORDER BY " + order + " LIMIT 1"
	var row catalogRow
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCatalogNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to select catalog item", err)
	}
	return row.toDomain(kind), nil
}

// enabledClause keeps the issue types a project uses: all of them when the project has no configuration.
const enabledClause = `(NOT EXISTS (SELECT 1 FROM project_issue_types x WHERE x.project_id = $1)
	OR t.id IN (SELECT y.issue_type_id FROM project_issue_types y WHERE y.project_id = $1))`

func (r *CatalogRepository) IssueTypeEnabled(ctx context.Context, projectID, issueTypeID uuid.UUID) (bool, error) {
	var enabled bool
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT NOT EXISTS (SELECT 1 FROM project_issue_types WHERE project_id = $1)
		    OR EXISTS (SELECT 1 FROM project_issue_types WHERE project_id = $1 AND issue_type_id = $2)`,
		projectID, issueTypeID).Scan(&enabled)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check issue type", err)
	}
	return enabled, nil
}

// SeedDefaults inserts the default catalogs of one organization. Existing rows are kept.
func (r *CatalogRepository) SeedDefaults(ctx context.Context, orgID uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	for _, seed := range defaultCatalogSeeds {
		if err := insertSeedRows(ctx, exec, orgID, seed); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to seed default catalogs", err)
		}
	}
	return nil
}

// specOf returns the spec of a kind or an error for an unknown kind.
func specOf(kind domain.CatalogKind) (catalogSpec, error) {
	sp, ok := catalogSpecs[kind]
	if !ok {
		return catalogSpec{}, fmt.Errorf("unknown catalog kind %q", kind)
	}
	return sp, nil
}

// mapCatalogWriteError maps unique violations to the catalog conflicts and hides the rest.
func mapCatalogWriteError(err error, message string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		if pgErr.ConstraintName == constraintIssueTypeHierarchy {
			return domain.ErrCatalogHierarchyTaken
		}
		return domain.ErrCatalogNameTaken
	}
	return apperror.Wrap(apperror.KindInternal, message, err)
}

// seedRow is one default catalog item. Columns a kind lacks are left empty.
type seedRow struct {
	name, icon, color, legacyKey string
	level, position              int
	isDefault                    bool
}

type seedSet struct {
	kind domain.CatalogKind
	rows []seedRow
}

// defaultCatalogSeeds are the catalogs every organization starts with.
var defaultCatalogSeeds = []seedSet{
	{domain.CatalogIssueType, []seedRow{
		{name: "Épica", icon: "epic", color: "purple", level: domain.HierarchyEpic, position: 0, legacyKey: "epic"},
		{name: "Historia", icon: "story", color: "green", position: 1, legacyKey: "story"},
		{name: "Tarea", icon: "task", color: "blue", position: 2, isDefault: true, legacyKey: "task"},
		{name: "Error", icon: "bug", color: "red", position: 3, legacyKey: "bug"},
		{name: "Subtarea", icon: "subtask", color: "teal", level: domain.HierarchySubtask, position: 4, legacyKey: "subtask"},
	}},
	{domain.CatalogPriority, []seedRow{
		{name: "Máxima", icon: "chevrons-up", color: "red", position: 0, legacyKey: "critical"},
		{name: "Alta", icon: "arrow-up", color: "orange", position: 1, legacyKey: "high"},
		{name: "Media", icon: "minus", color: "amber", position: 2, isDefault: true, legacyKey: "medium"},
		{name: "Baja", icon: "arrow-down", color: "blue", position: 3, legacyKey: "low"},
		{name: "Mínima", icon: "chevrons-down", color: "gray", position: 4, legacyKey: "low"},
	}},
	{domain.CatalogResolution, []seedRow{
		{name: "Hecho", position: 0, isDefault: true},
		{name: "Descartado", position: 1},
		{name: "Duplicado", position: 2},
		{name: "No se reproduce", position: 3},
		{name: "No se hará", position: 4},
	}},
}

func insertSeedRows(ctx context.Context, exec sqlx.ExtContext, orgID uuid.UUID, seed seedSet) error {
	sp := catalogSpecs[seed.kind]
	for _, row := range seed.rows {
		columns := []string{"organization_id", "name"}
		args := []any{orgID, row.name}
		if sp.hasDesc {
			columns = append(columns, "description")
			args = append(args, "")
		}
		if sp.hasIcon {
			columns = append(columns, "icon", "color")
			args = append(args, row.icon, row.color)
		}
		if sp.hasLevel {
			columns = append(columns, "hierarchy_level")
			args = append(args, row.level)
		}
		columns = append(columns, "position", "is_default")
		args = append(args, row.position, row.isDefault)
		if sp.hasLegacy {
			columns = append(columns, "legacy_key")
			args = append(args, row.legacyKey)
		}
		placeholders := make([]string, len(args))
		for i := range args {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
			sp.table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		if _, err := exec.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}
