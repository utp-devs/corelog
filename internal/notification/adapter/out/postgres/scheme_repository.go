package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// SchemeRepository stores notification schemes as one JSON document per scope.
type SchemeRepository struct {
	db *sqlx.DB
}

// NewSchemeRepository returns the repository.
func NewSchemeRepository(db *sqlx.DB) *SchemeRepository { return &SchemeRepository{db: db} }

var _ port.SchemeRepository = (*SchemeRepository)(nil)

type recipientJSON struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Role string `json:"role,omitempty"`
}

type ruleJSON struct {
	Event      string          `json:"event"`
	Recipients []recipientJSON `json:"recipients"`
	Channels   []string        `json:"channels"`
}

func encodeRules(s domain.Scheme) ([]byte, error) {
	rules := make([]ruleJSON, 0, len(s.Rules))
	for _, r := range s.Rules {
		item := ruleJSON{Event: string(r.Event), Recipients: []recipientJSON{}, Channels: []string{}}
		for _, rec := range r.Recipients {
			j := recipientJSON{Type: string(rec.Type), Role: rec.Role}
			if rec.ID != uuid.Nil {
				j.ID = rec.ID.String()
			}
			item.Recipients = append(item.Recipients, j)
		}
		for _, c := range r.Channels {
			item.Channels = append(item.Channels, string(c))
		}
		rules = append(rules, item)
	}
	return json.Marshal(rules)
}

func decodeRules(raw []byte) (domain.Scheme, error) {
	var rules []ruleJSON
	if err := json.Unmarshal(raw, &rules); err != nil {
		return domain.Scheme{}, err
	}
	out := domain.Scheme{Rules: make([]domain.Rule, 0, len(rules))}
	for _, r := range rules {
		rule := domain.Rule{Event: domain.Event(r.Event), Recipients: []domain.Recipient{}, Channels: []domain.Channel{}}
		for _, rec := range r.Recipients {
			item := domain.Recipient{Type: domain.RecipientType(rec.Type), Role: rec.Role}
			if rec.ID != "" {
				id, err := uuid.Parse(rec.ID)
				if err != nil {
					return domain.Scheme{}, err
				}
				item.ID = id
			}
			rule.Recipients = append(rule.Recipients, item)
		}
		for _, c := range r.Channels {
			rule.Channels = append(rule.Channels, domain.Channel(c))
		}
		out.Rules = append(out.Rules, rule)
	}
	return out, nil
}

// scopeClause selects the row of a scope: the project scheme, or the organization default (project NULL).
func scopeArgs(orgID uuid.UUID, projectID *uuid.UUID) (string, []any) {
	if projectID != nil {
		return `project_id = $1`, []any{*projectID}
	}
	return `organization_id = $1 AND project_id IS NULL`, []any{orgID}
}

func (r *SchemeRepository) Get(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) (*port.StoredScheme, error) {
	where, args := scopeArgs(orgID, projectID)
	var row struct {
		Rules     []byte    `db:"rules"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `SELECT rules, updated_at FROM notification_schemes WHERE `+where, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "no se pudo leer el esquema de notificaciones", err)
	}
	scheme, err := decodeRules(row.Rules)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "esquema de notificaciones danado", err)
	}
	return &port.StoredScheme{Scheme: scheme, UpdatedAt: row.UpdatedAt}, nil
}

func (r *SchemeRepository) Save(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, scheme domain.Scheme, updatedBy uuid.UUID, now time.Time) (port.StoredScheme, error) {
	raw, err := encodeRules(scheme)
	if err != nil {
		return port.StoredScheme{}, apperror.Wrap(apperror.KindInternal, "no se pudo guardar el esquema de notificaciones", err)
	}
	conflict := `ON CONFLICT (organization_id) WHERE project_id IS NULL`
	if projectID != nil {
		conflict = `ON CONFLICT (project_id) WHERE project_id IS NOT NULL`
	}
	_, err = database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO notification_schemes (organization_id, project_id, rules, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3::jsonb, $4, $5, $5) `+conflict+`
		DO UPDATE SET rules = EXCLUDED.rules, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at`,
		orgID, projectID, string(raw), updatedBy, now)
	if err != nil {
		return port.StoredScheme{}, apperror.Wrap(apperror.KindInternal, "no se pudo guardar el esquema de notificaciones", err)
	}
	return port.StoredScheme{Scheme: scheme, UpdatedAt: now}, nil
}

func (r *SchemeRepository) Delete(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) (bool, error) {
	where, args := scopeArgs(orgID, projectID)
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, `DELETE FROM notification_schemes WHERE `+where, args...)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "no se pudo restablecer el esquema de notificaciones", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *SchemeRepository) ProjectOrg(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var org uuid.UUID
	err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &org, `SELECT organization_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, apperror.Wrap(apperror.KindInternal, "no se pudo leer el proyecto", err)
	}
	return org, nil
}
