package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type componentRow struct {
	ID                    uuid.UUID     `db:"id"`
	ProjectID             uuid.UUID     `db:"project_id"`
	Name                  string        `db:"name"`
	Description           string        `db:"description"`
	LeadID                uuid.NullUUID `db:"lead_id"`
	LeadName              string        `db:"lead_name"`
	DefaultAssigneePolicy string        `db:"default_assignee_policy"`
	IssueCount            int           `db:"issue_count"`
	CreatedAt             time.Time     `db:"created_at"`
	UpdatedAt             time.Time     `db:"updated_at"`
}

// componentSelect reads the component with its lead name and the number of live issues it labels.
const componentSelect = `
	SELECT c.id, c.project_id, c.name, c.description, c.lead_id, COALESCE(u.display_name, u.name, '') AS lead_name,
	       c.default_assignee_policy,
	       (SELECT COUNT(*) FROM issue_components ic JOIN issues i ON i.id = ic.issue_id AND i.deleted_at IS NULL
	        WHERE ic.component_id = c.id)::int AS issue_count,
	       c.created_at, c.updated_at
	FROM components c
	LEFT JOIN users u ON u.id = c.lead_id`

const pgUniqueViolationCode = "23505"

func (r componentRow) toDomain() domain.Component {
	c := domain.Component{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, Description: r.Description,
		LeadName: r.LeadName, DefaultAssigneePolicy: r.DefaultAssigneePolicy, IssueCount: r.IssueCount,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.LeadID.Valid {
		lead := r.LeadID.UUID
		c.LeadID = &lead
	}
	return c
}

func mapComponentWriteError(err error, message string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
		return domain.ErrComponentNameTaken
	}
	return apperror.Wrap(apperror.KindInternal, message, err)
}

func (r *CatalogRepository) ListComponents(ctx context.Context, projectID uuid.UUID) ([]domain.Component, error) {
	var rows []componentRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows,
		componentSelect+` WHERE c.project_id = $1 AND c.deleted_at IS NULL ORDER BY c.name ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list components", err)
	}
	components := make([]domain.Component, 0, len(rows))
	for _, row := range rows {
		components = append(components, row.toDomain())
	}
	return components, nil
}

func (r *CatalogRepository) FindComponent(ctx context.Context, projectID, componentID uuid.UUID) (*domain.Component, error) {
	var row componentRow
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row,
		componentSelect+` WHERE c.project_id = $1 AND c.id = $2 AND c.deleted_at IS NULL`, projectID, componentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrComponentNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read component", err)
	}
	component := row.toDomain()
	return &component, nil
}

func (r *CatalogRepository) CreateComponent(ctx context.Context, component *domain.Component) error {
	var lead any
	if component.LeadID != nil {
		lead = *component.LeadID
	}
	id, err := database.InsertReturningID(ctx, database.Executor(ctx, r.db), `
		INSERT INTO components (project_id, name, description, lead_id, default_assignee_policy, created_at, updated_at)
		VALUES (:project_id, :name, :description, :lead_id, :policy, :created_at, :updated_at)
		RETURNING id`, map[string]interface{}{
		"project_id": component.ProjectID, "name": component.Name, "description": component.Description,
		"lead_id": lead, "policy": component.DefaultAssigneePolicy,
		"created_at": component.CreatedAt, "updated_at": component.UpdatedAt,
	})
	if err != nil {
		return mapComponentWriteError(err, "failed to create component")
	}
	component.ID = id
	return nil
}

func (r *CatalogRepository) UpdateComponent(ctx context.Context, component *domain.Component) error {
	var lead any
	if component.LeadID != nil {
		lead = *component.LeadID
	}
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE components SET name = $3, description = $4, lead_id = $5, default_assignee_policy = $6, updated_at = $7
		WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL`,
		component.ProjectID, component.ID, component.Name, component.Description, lead,
		component.DefaultAssigneePolicy, component.UpdatedAt); err != nil {
		return mapComponentWriteError(err, "failed to update component")
	}
	return nil
}

func (r *CatalogRepository) MoveComponentIssues(ctx context.Context, from, to uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO issue_components (issue_id, component_id)
		SELECT issue_id, $2 FROM issue_components WHERE component_id = $1
		ON CONFLICT DO NOTHING`, from, to); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to move component issues", err)
	}
	return nil
}

func (r *CatalogRepository) DeleteComponent(ctx context.Context, projectID, componentID uuid.UUID) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `UPDATE components SET deleted_at = now() WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL`, projectID, componentID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete component", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete component", err)
	}
	if affected == 0 {
		return domain.ErrComponentNotFound
	}
	return nil
}
