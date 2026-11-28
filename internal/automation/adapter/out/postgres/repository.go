// Package postgres stores automation rules and runs.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/automation/application/port"
	"github.com/jhonsferg/corelog/internal/automation/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Repository is the PostgreSQL adapter of port.Repository.
type Repository struct{ db *sqlx.DB }

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository bound to db; it joins the transaction of the context when there is one.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ext(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

type ruleRow struct {
	ID         uuid.UUID  `db:"id"`
	OrgID      uuid.UUID  `db:"organization_id"`
	ProjectID  *uuid.UUID `db:"project_id"`
	Name       string     `db:"name"`
	Enabled    bool       `db:"enabled"`
	Trigger    []byte     `db:"trigger"`
	Conditions []byte     `db:"conditions"`
	Actions    []byte     `db:"actions"`
	CreatedBy  *uuid.UUID `db:"created_by"`
	Version    int        `db:"version"`
	RunCount   int64      `db:"run_count"`
	LastRunAt  *time.Time `db:"last_run_at"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

const ruleColumns = `id, organization_id, project_id, name, enabled, trigger, conditions, actions, created_by, version,
	run_count, last_run_at, created_at, updated_at`

func (r ruleRow) rule() (domain.Rule, error) {
	out := domain.Rule{ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID, Name: r.Name, Enabled: r.Enabled, CreatedBy: r.CreatedBy,
		Version: r.Version, RunCount: r.RunCount, LastRunAt: r.LastRunAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if err := json.Unmarshal(r.Trigger, &out.Trigger); err != nil {
		return out, internal("regla corrupta: disparador", err)
	}
	if err := json.Unmarshal(r.Conditions, &out.Conditions); err != nil {
		return out, internal("regla corrupta: condiciones", err)
	}
	if err := json.Unmarshal(r.Actions, &out.Actions); err != nil {
		return out, internal("regla corrupta: acciones", err)
	}
	if out.Conditions == nil {
		out.Conditions = []domain.Condition{}
	}
	return out, nil
}

func rules(rows []ruleRow) ([]domain.Rule, error) {
	out := make([]domain.Rule, 0, len(rows))
	for _, row := range rows {
		rule, err := row.rule()
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, nil
}

func documents(rule *domain.Rule) (trig, conds, acts string, err error) {
	// nil slices would be stored as JSON null, which the array checks of the table reject.
	if rule.Conditions == nil {
		rule.Conditions = []domain.Condition{}
	}
	if rule.Actions == nil {
		rule.Actions = []domain.Action{}
	}
	t, err := json.Marshal(rule.Trigger)
	if err != nil {
		return "", "", "", err
	}
	c, err := json.Marshal(rule.Conditions)
	if err != nil {
		return "", "", "", err
	}
	a, err := json.Marshal(rule.Actions)
	if err != nil {
		return "", "", "", err
	}
	return string(t), string(c), string(a), nil
}

// Create inserts a rule and fills its id.
func (r *Repository) Create(ctx context.Context, rule *domain.Rule) error {
	t, c, a, err := documents(rule)
	if err != nil {
		return internal("no se pudo codificar la regla", err)
	}
	err = sqlx.GetContext(ctx, r.ext(ctx), &rule.ID, `
		INSERT INTO automation_rules (organization_id, project_id, name, enabled, trigger, conditions, actions, created_by,
		                              version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, 1, $9, $9) RETURNING id`,
		rule.OrgID, rule.ProjectID, rule.Name, rule.Enabled, t, c, a, rule.CreatedBy, rule.CreatedAt)
	if err != nil {
		return internal("no se pudo guardar la regla", err)
	}
	return nil
}

// Get returns a rule by id.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Rule, error) {
	var row ruleRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `SELECT `+ruleColumns+` FROM automation_rules WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Rule{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Rule{}, internal("no se pudo leer la regla", err)
	}
	return row.rule()
}

// ListByProject returns the rules of a project by creation order.
func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.Rule, error) {
	var rows []ruleRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows,
		`SELECT `+ruleColumns+` FROM automation_rules WHERE project_id = $1 ORDER BY created_at, id`, projectID); err != nil {
		return nil, internal("no se pudieron listar las reglas", err)
	}
	return rules(rows)
}

// Update stores the definition guarded by the version.
func (r *Repository) Update(ctx context.Context, rule *domain.Rule) error {
	t, c, a, err := documents(rule)
	if err != nil {
		return internal("no se pudo codificar la regla", err)
	}
	var version int
	err = sqlx.GetContext(ctx, r.ext(ctx), &version, `
		UPDATE automation_rules SET name = $3, enabled = $4, trigger = $5::jsonb, conditions = $6::jsonb, actions = $7::jsonb,
		       version = version + 1, updated_at = $8
		WHERE id = $1 AND version = $2 RETURNING version`,
		rule.ID, rule.Version, rule.Name, rule.Enabled, t, c, a, rule.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrVersionConflict
	}
	if err != nil {
		return internal("no se pudo actualizar la regla", err)
	}
	rule.Version = version
	return nil
}

// SetEnabled toggles a rule and returns it.
func (r *Repository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool, at time.Time) (domain.Rule, error) {
	var row ruleRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `
		UPDATE automation_rules SET enabled = $2, updated_at = $3 WHERE id = $1 RETURNING `+ruleColumns, id, enabled, at)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Rule{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Rule{}, internal("no se pudo actualizar la regla", err)
	}
	return row.rule()
}

// Delete removes a rule; runs cascade.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.ext(ctx).ExecContext(ctx, `DELETE FROM automation_rules WHERE id = $1`, id)
	if err != nil {
		return internal("no se pudo eliminar la regla", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ProjectOrg returns the organization of a live project.
func (r *Repository) ProjectOrg(ctx context.Context, projectID uuid.UUID) (uuid.UUID, bool, error) {
	var org uuid.UUID
	err := sqlx.GetContext(ctx, r.ext(ctx), &org, `SELECT organization_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, internal("no se pudo leer el proyecto", err)
	}
	return org, true, nil
}

// ActiveFor returns the enabled rules of the project and the organization-wide ones.
func (r *Repository) ActiveFor(ctx context.Context, orgID, projectID uuid.UUID) ([]domain.Rule, error) {
	var rows []ruleRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `SELECT `+ruleColumns+` FROM automation_rules
		WHERE organization_id = $1 AND enabled AND (project_id = $2 OR project_id IS NULL) ORDER BY created_at, id`, orgID, projectID)
	if err != nil {
		return nil, internal("no se pudieron cargar las reglas activas", err)
	}
	return rules(rows)
}

// ActiveScheduled returns the enabled scheduled rules.
func (r *Repository) ActiveScheduled(ctx context.Context) ([]domain.Rule, error) {
	var rows []ruleRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `SELECT `+ruleColumns+` FROM automation_rules
		WHERE enabled AND trigger->>'type' = 'scheduled' ORDER BY created_at, id`)
	if err != nil {
		return nil, internal("no se pudieron cargar las reglas programadas", err)
	}
	return rules(rows)
}

// RecordRun stores a run and, for executed ones, updates the counters of the rule.
func (r *Repository) RecordRun(ctx context.Context, run *domain.Run) error {
	ex := r.ext(ctx)
	err := sqlx.GetContext(ctx, ex, &run.ID, `
		INSERT INTO automation_runs (rule_id, event_id, issue_id, status, detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		run.RuleID, run.EventID, run.IssueID, run.Status, run.Detail, run.CreatedAt)
	if err != nil {
		return internal("no se pudo registrar la ejecucion", err)
	}
	if run.Status == domain.RunSkipped {
		return nil
	}
	if _, err := ex.ExecContext(ctx,
		`UPDATE automation_rules SET run_count = run_count + 1, last_run_at = $2 WHERE id = $1`, run.RuleID, run.CreatedAt); err != nil {
		return internal("no se pudo actualizar la regla", err)
	}
	return nil
}

// TouchScheduled sets last_run_at.
func (r *Repository) TouchScheduled(ctx context.Context, id uuid.UUID, at time.Time) error {
	if _, err := r.ext(ctx).ExecContext(ctx, `UPDATE automation_rules SET last_run_at = $2 WHERE id = $1`, id, at); err != nil {
		return internal("no se pudo actualizar la regla", err)
	}
	return nil
}

// CountRunsSince counts executed runs since the instant.
func (r *Repository) CountRunsSince(ctx context.Context, ruleID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.ext(ctx), &n, `
		SELECT count(*) FROM automation_runs WHERE rule_id = $1 AND created_at >= $2 AND status <> 'skipped'`, ruleID, since)
	if err != nil {
		return 0, internal("no se pudieron contar las ejecuciones", err)
	}
	return n, nil
}

// ListRuns returns the latest runs of a rule.
func (r *Repository) ListRuns(ctx context.Context, ruleID uuid.UUID, status string, limit int) ([]domain.Run, error) {
	var rows []struct {
		ID        uuid.UUID  `db:"id"`
		RuleID    uuid.UUID  `db:"rule_id"`
		EventID   *int64     `db:"event_id"`
		IssueID   *uuid.UUID `db:"issue_id"`
		Status    string     `db:"status"`
		Detail    string     `db:"detail"`
		CreatedAt time.Time  `db:"created_at"`
	}
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, rule_id, event_id, issue_id, status, detail, created_at FROM automation_runs
		WHERE rule_id = $1 AND ($2 = '' OR status = $2) ORDER BY created_at DESC, id LIMIT $3`, ruleID, status, limit)
	if err != nil {
		return nil, internal("no se pudieron listar las ejecuciones", err)
	}
	out := make([]domain.Run, len(rows))
	for i, row := range rows {
		out[i] = domain.Run{ID: row.ID, RuleID: row.RuleID, EventID: row.EventID, IssueID: row.IssueID, Status: row.Status,
			Detail: row.Detail, CreatedAt: row.CreatedAt}
	}
	return out, nil
}
