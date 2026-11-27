package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// SchemeRepository guarda los esquemas de permisos editados en la tabla project_permissions.
type SchemeRepository struct {
	db *sqlx.DB
	tx *database.SQLTxManager
}

func NewSchemeRepository(db *sqlx.DB) *SchemeRepository {
	return &SchemeRepository{db: db, tx: database.NewTxManager(db)}
}

var _ port.SchemeRepository = (*SchemeRepository)(nil)

// Load devuelve el esquema guardado del proyecto, o nil si no tiene filas propias.
func (r *SchemeRepository) Load(ctx context.Context, projectID uuid.UUID) (*domain.Scheme, error) {
	rows, err := database.Executor(ctx, r.db).QueryContext(ctx,
		`SELECT permission, holder FROM project_permissions WHERE project_id = $1 ORDER BY permission, holder`,
		projectID)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load permission scheme", err)
	}
	defer rows.Close()

	index := map[domain.Permission]int{}
	var grants []domain.Grant
	for rows.Next() {
		var permission, holder string
		if err := rows.Scan(&permission, &holder); err != nil {
			return nil, apperror.Wrap(apperror.KindInternal, "failed to read permission scheme", err)
		}
		p := domain.Permission(permission)
		i, ok := index[p]
		if !ok {
			i = len(grants)
			index[p] = i
			grants = append(grants, domain.Grant{Permission: p})
		}
		grants[i].Holders = append(grants[i].Holders, domain.Holder(holder))
	}
	if err := rows.Err(); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read permission scheme", err)
	}
	if len(grants) == 0 {
		return nil, nil
	}
	scheme, err := domain.NewScheme(grants)
	if err != nil {
		return nil, fmt.Errorf("stored permission scheme of project %s is invalid: %w", projectID, err)
	}
	return &scheme, nil
}

// Replace sustituye todas las filas del proyecto por las del esquema, en una sola transaccion.
func (r *SchemeRepository) Replace(ctx context.Context, projectID uuid.UUID, scheme domain.Scheme) error {
	return r.tx.InTx(ctx, func(ctx context.Context) error {
		exec := database.Executor(ctx, r.db)
		if _, err := exec.ExecContext(ctx, `DELETE FROM project_permissions WHERE project_id = $1`, projectID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to replace permission scheme", err)
		}
		var values []string
		var args []any
		for _, g := range scheme.Grants() {
			for _, h := range g.Holders {
				args = append(args, projectID, string(g.Permission), string(h))
				n := len(args)
				values = append(values, fmt.Sprintf("($%d, $%d, $%d)", n-2, n-1, n))
			}
		}
		if len(values) == 0 {
			return nil
		}
		query := `INSERT INTO project_permissions (project_id, permission, holder) VALUES ` + strings.Join(values, ", ")
		if _, err := exec.ExecContext(ctx, query, args...); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to replace permission scheme", err)
		}
		return nil
	})
}

// Reset borra las filas del proyecto, que vuelve al esquema por defecto.
func (r *SchemeRepository) Reset(ctx context.Context, projectID uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `DELETE FROM project_permissions WHERE project_id = $1`, projectID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to reset permission scheme", err)
	}
	return nil
}
