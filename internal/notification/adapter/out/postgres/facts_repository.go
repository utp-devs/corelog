package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// FactsRepository answers the recipient questions of the engine with narrow read queries. It reads the shared
// tables directly so the notification hexagon does not import other modules.
type FactsRepository struct {
	db *sqlx.DB
}

// NewFactsRepository returns the repository.
func NewFactsRepository(db *sqlx.DB) *FactsRepository { return &FactsRepository{db: db} }

var _ port.FactsSource = (*FactsRepository)(nil)

func (r *FactsRepository) ext(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

func internal(message string, err error) error {
	return apperror.Wrap(apperror.KindInternal, message, err)
}

// jsonIDs encodes ids for use with jsonb_array_elements_text, which keeps the queries free of array drivers.
func jsonIDs(ids []uuid.UUID) string {
	list := make([]string, 0, len(ids))
	for _, id := range ids {
		list = append(list, id.String())
	}
	raw, _ := json.Marshal(list)
	return string(raw)
}

func (r *FactsRepository) Issue(ctx context.Context, issueID uuid.UUID) (*port.IssueFacts, error) {
	var row struct {
		ID         uuid.UUID  `db:"id"`
		OrgID      uuid.UUID  `db:"org_id"`
		ProjectID  uuid.UUID  `db:"project_id"`
		Key        string     `db:"issue_key"`
		Summary    string     `db:"summary"`
		ReporterID uuid.UUID  `db:"reporter_id"`
		AssigneeID *uuid.UUID `db:"assignee_id"`
		LeadID     uuid.UUID  `db:"lead_id"`
	}
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `
		SELECT i.id, p.organization_id AS org_id, i.project_id, p.key || '-' || i.key_number::text AS issue_key, i.summary,
		       i.reporter_id, i.assignee_id, p.lead_id
		FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.id = $1 AND i.deleted_at IS NULL`, issueID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internal("no se pudo leer la incidencia", err)
	}
	facts := &port.IssueFacts{
		IssueID: row.ID, OrgID: row.OrgID, ProjectID: row.ProjectID, Key: row.Key, Summary: row.Summary,
		ReporterID: row.ReporterID, ProjectLeadID: row.LeadID,
	}
	if row.AssigneeID != nil {
		facts.AssigneeID = *row.AssigneeID
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &facts.WatcherIDs, `SELECT user_id FROM watchers WHERE issue_id = $1 ORDER BY user_id`, issueID); err != nil {
		return nil, internal("no se pudieron leer los observadores", err)
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &facts.ComponentLeadIDs, `
		SELECT DISTINCT c.lead_id FROM issue_components ic JOIN components c ON c.id = ic.component_id
		WHERE ic.issue_id = $1 AND c.lead_id IS NOT NULL ORDER BY c.lead_id`, issueID); err != nil {
		return nil, internal("no se pudieron leer los responsables de componente", err)
	}
	return facts, nil
}

func (r *FactsRepository) Project(ctx context.Context, projectID uuid.UUID) (*port.ProjectFacts, error) {
	var row struct {
		ID   uuid.UUID `db:"id"`
		Org  uuid.UUID `db:"organization_id"`
		Name string    `db:"name"`
		Lead uuid.UUID `db:"lead_id"`
	}
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `SELECT id, organization_id, name, lead_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internal("no se pudo leer el proyecto", err)
	}
	return &port.ProjectFacts{ProjectID: row.ID, OrgID: row.Org, Name: row.Name, LeadID: row.Lead}, nil
}

func (r *FactsRepository) TeamMembers(ctx context.Context, teamIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	var rows []struct {
		TeamID uuid.UUID `db:"team_id"`
		UserID uuid.UUID `db:"user_id"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT team_id, user_id FROM team_members
		WHERE team_id IN (SELECT jsonb_array_elements_text($1::jsonb)::uuid) ORDER BY team_id, user_id`, jsonIDs(teamIDs)); err != nil {
		return nil, internal("no se pudieron leer los miembros del equipo", err)
	}
	out := map[uuid.UUID][]uuid.UUID{}
	for _, row := range rows {
		out[row.TeamID] = append(out[row.TeamID], row.UserID)
	}
	return out, nil
}

func (r *FactsRepository) RoleMembers(ctx context.Context, projectID uuid.UUID, roles []string) (map[string][]uuid.UUID, error) {
	encoded, _ := json.Marshal(roles)
	var rows []struct {
		Role   string    `db:"role"`
		UserID uuid.UUID `db:"user_id"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT role, user_id FROM project_members
		WHERE project_id = $1 AND role IN (SELECT jsonb_array_elements_text($2::jsonb)) ORDER BY role, user_id`, projectID, string(encoded)); err != nil {
		return nil, internal("no se pudieron leer los miembros del proyecto", err)
	}
	out := map[string][]uuid.UUID{}
	for _, row := range rows {
		out[row.Role] = append(out[row.Role], row.UserID)
	}
	return out, nil
}

func (r *FactsRepository) Contacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]port.Contact, error) {
	var rows []struct {
		ID          uuid.UUID `db:"id"`
		DisplayName string    `db:"display_name"`
		Name        string    `db:"name"`
		Email       string    `db:"email"`
		Active      bool      `db:"active"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, display_name, name, email, deactivated_at IS NULL AS active FROM users
		WHERE id IN (SELECT jsonb_array_elements_text($1::jsonb)::uuid)`, jsonIDs(ids)); err != nil {
		return nil, internal("no se pudieron leer los usuarios", err)
	}
	out := make(map[uuid.UUID]port.Contact, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.DisplayName)
		if name == "" {
			name = strings.TrimSpace(row.Name)
		}
		out[row.ID] = port.Contact{ID: row.ID, Name: name, Email: row.Email, Active: row.Active}
	}
	return out, nil
}
