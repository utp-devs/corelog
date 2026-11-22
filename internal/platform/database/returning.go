package database

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// InsertReturningID ejecuta un INSERT con parametros nombrados que termina en
// "RETURNING id" y devuelve el identificador generado por la base de datos.
// Si la sentencia no devuelve filas (por ejemplo ON CONFLICT DO NOTHING),
// devuelve sql.ErrNoRows.
func InsertReturningID(ctx context.Context, q sqlx.ExtContext, query string, arg interface{}) (uuid.UUID, error) {
	rows, err := sqlx.NamedQueryContext(ctx, q, query, arg)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return uuid.Nil, err
		}
		return uuid.Nil, sql.ErrNoRows
	}
	var id uuid.UUID
	if err := rows.Scan(&id); err != nil {
		return uuid.Nil, err
	}
	if err := rows.Close(); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
