package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// TxManager runs a unit of work inside a database transaction.
type TxManager interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txContextKey struct{}

// SQLTxManager implements TxManager on top of *sqlx.DB.
type SQLTxManager struct {
	db *sqlx.DB
}

// NewTxManager returns a TxManager bound to db.
func NewTxManager(db *sqlx.DB) *SQLTxManager {
	return &SQLTxManager{db: db}
}

// InTx runs fn in a transaction. The transaction is committed when fn returns
// nil, rolled back when fn returns an error, and rolled back before re-panicking
// when fn panics. A call made inside an existing transaction reuses it.
func (m *SQLTxManager) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := m.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(WithTx(ctx, tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("database: rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: commit: %w", err)
	}
	return nil
}

// WithTx stores tx in ctx so that repositories can use it through Executor.
func WithTx(ctx context.Context, tx sqlx.ExtContext) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext returns the transaction stored in ctx, if any.
func TxFromContext(ctx context.Context) (sqlx.ExtContext, bool) {
	tx, ok := ctx.Value(txContextKey{}).(sqlx.ExtContext)
	return tx, ok && tx != nil
}

// Executor returns the transaction bound to ctx, or db when there is none.
func Executor(ctx context.Context, db sqlx.ExtContext) sqlx.ExtContext {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return db
}
