package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// VersionLinkRepository reads and writes the version and component links of issues. It joins the transaction of
// the context when there is one.
type VersionLinkRepository struct {
	db *sqlx.DB
}

// NewVersionLinkRepository builds the repository.
func NewVersionLinkRepository(db *sqlx.DB) *VersionLinkRepository {
	return &VersionLinkRepository{db: db}
}

var _ port.VersionLinkRepository = (*VersionLinkRepository)(nil)

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// ResolveVersions returns the versions among ids that belong to the project.
func (r *VersionLinkRepository) ResolveVersions(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.VersionRef, error) {
	out := []domain.VersionRef{}
	if len(ids) == 0 {
		return out, nil
	}
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &out, `
		SELECT id, name, status FROM versions WHERE project_id = $1 AND id = ANY($2::uuid[])`,
		projectID, uuidStrings(ids)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to resolve versions", err)
	}
	return out, nil
}

// ResolveComponents returns the components among ids that belong to the project.
func (r *VersionLinkRepository) ResolveComponents(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.LinkedRef, error) {
	out := []domain.LinkedRef{}
	if len(ids) == 0 {
		return out, nil
	}
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &out, `
		SELECT id, name FROM components WHERE project_id = $1 AND id = ANY($2::uuid[])`,
		projectID, uuidStrings(ids)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to resolve components", err)
	}
	return out, nil
}

// ComponentDefaults returns the assignee policy of the components among ids, ordered by name.
func (r *VersionLinkRepository) ComponentDefaults(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.ComponentDefault, error) {
	out := []domain.ComponentDefault{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []componentDefaultRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT id, name, default_assignee_policy AS policy, lead_id
		FROM components WHERE project_id = $1 AND id = ANY($2::uuid[])
		ORDER BY name ASC, id ASC`,
		projectID, uuidStrings(ids)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read component defaults", err)
	}
	for _, row := range rows {
		item := domain.ComponentDefault{ID: row.ID, Name: row.Name, Policy: row.Policy}
		if row.LeadID.Valid {
			lead := row.LeadID.UUID
			item.LeadID = &lead
		}
		out = append(out, item)
	}
	return out, nil
}

type componentDefaultRow struct {
	ID     uuid.UUID     `db:"id"`
	Name   string        `db:"name"`
	Policy string        `db:"policy"`
	LeadID uuid.NullUUID `db:"lead_id"`
}

type versionLinkRow struct {
	IssueID uuid.UUID `db:"issue_id"`
	Kind    string    `db:"kind"`
	ID      uuid.UUID `db:"id"`
	Name    string    `db:"name"`
}

// LoadLinks returns the links of every issue in issueIDs with one query per kind.
func (r *VersionLinkRepository) LoadLinks(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]domain.IssueLinks, error) {
	out := make(map[uuid.UUID]domain.IssueLinks, len(issueIDs))
	if len(issueIDs) == 0 {
		return out, nil
	}
	ids := uuidStrings(issueIDs)
	exec := database.Executor(ctx, r.db)

	var versions []versionLinkRow
	if err := sqlx.SelectContext(ctx, exec, &versions, `
		SELECT iv.issue_id, iv.kind, v.id, v.name
		FROM issue_versions iv
		JOIN versions v ON v.id = iv.version_id
		WHERE iv.issue_id = ANY($1::uuid[])
		ORDER BY v.name ASC, v.id ASC`, ids); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read issue versions", err)
	}
	for _, row := range versions {
		links := out[row.IssueID]
		ref := domain.LinkedRef{ID: row.ID, Name: row.Name}
		if row.Kind == domain.VersionLinkFix {
			links.FixVersions = append(links.FixVersions, ref)
		} else {
			links.AffectsVersions = append(links.AffectsVersions, ref)
		}
		out[row.IssueID] = links
	}

	var components []versionLinkRow
	if err := sqlx.SelectContext(ctx, exec, &components, `
		SELECT ic.issue_id, '' AS kind, c.id, c.name
		FROM issue_components ic
		JOIN components c ON c.id = ic.component_id
		WHERE ic.issue_id = ANY($1::uuid[])
		ORDER BY c.name ASC, c.id ASC`, ids); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read issue components", err)
	}
	for _, row := range components {
		links := out[row.IssueID]
		links.Components = append(links.Components, domain.LinkedRef{ID: row.ID, Name: row.Name})
		out[row.IssueID] = links
	}
	return out, nil
}

// ReplaceIssueVersions sets the versions of one kind of an issue.
func (r *VersionLinkRepository) ReplaceIssueVersions(ctx context.Context, issueID uuid.UUID, kind string, versionIDs []uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	if _, err := exec.ExecContext(ctx, `DELETE FROM issue_versions WHERE issue_id = $1 AND kind = $2`, issueID, kind); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to clear issue versions", err)
	}
	if len(versionIDs) == 0 {
		return nil
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_versions (issue_id, version_id, kind)
		SELECT $1, version_id, $3 FROM unnest($2::uuid[]) AS version_id
		ON CONFLICT DO NOTHING`, issueID, uuidStrings(versionIDs), kind); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save issue versions", err)
	}
	return nil
}

// ReplaceIssueComponents sets the components of an issue.
func (r *VersionLinkRepository) ReplaceIssueComponents(ctx context.Context, issueID uuid.UUID, componentIDs []uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	if _, err := exec.ExecContext(ctx, `DELETE FROM issue_components WHERE issue_id = $1`, issueID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to clear issue components", err)
	}
	if len(componentIDs) == 0 {
		return nil
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_components (issue_id, component_id)
		SELECT $1, component_id FROM unnest($2::uuid[]) AS component_id
		ON CONFLICT DO NOTHING`, issueID, uuidStrings(componentIDs)); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save issue components", err)
	}
	return nil
}
