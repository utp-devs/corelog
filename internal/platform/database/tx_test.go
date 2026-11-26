package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

// newTxTestTable creates a uniquely named table in the connected schema and
// drops it on cleanup. The table is a regular table because a temp table is
// bound to one session and would not be visible across pooled connections.
func newTxTestTable(t *testing.T, db *sqlx.DB) string {
	t.Helper()
	name := fmt.Sprintf("tx_test_%d", time.Now().UnixNano())
	if _, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY)", name)); err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + name)
	})
	return name
}

func countRows(t *testing.T, db *sqlx.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Get(&n, "SELECT count(*) FROM "+table); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func insertRow(ctx context.Context, db *sqlx.DB, table string, id int) error {
	_, err := Executor(ctx, db).ExecContext(ctx, "INSERT INTO "+table+" (id) VALUES ($1)", id)
	return err
}

func TestTxManager(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CORELOG_TEST_DATABASE_URL not set")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()

	t.Run("commit persists writes when fn returns nil", func(t *testing.T) {
		table := newTxTestTable(t, db)
		mgr := NewTxManager(db)

		err := mgr.InTx(ctx, func(ctx context.Context) error {
			return insertRow(ctx, db, table, 1)
		})
		if err != nil {
			t.Fatalf("InTx: %v", err)
		}
		if n := countRows(t, db, table); n != 1 {
			t.Errorf("rows = %d, want 1", n)
		}
	})

	t.Run("rollback discards writes and returns fn error", func(t *testing.T) {
		table := newTxTestTable(t, db)
		mgr := NewTxManager(db)
		boom := errors.New("boom")

		err := mgr.InTx(ctx, func(ctx context.Context) error {
			if err := insertRow(ctx, db, table, 1); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
		if n := countRows(t, db, table); n != 0 {
			t.Errorf("rows = %d, want 0", n)
		}
	})

	t.Run("panic rolls back and re-panics", func(t *testing.T) {
		table := newTxTestTable(t, db)
		mgr := NewTxManager(db)

		func() {
			defer func() {
				if r := recover(); r != "kaboom" {
					t.Errorf("recovered %v, want kaboom", r)
				}
			}()
			_ = mgr.InTx(ctx, func(ctx context.Context) error {
				if err := insertRow(ctx, db, table, 1); err != nil {
					return err
				}
				panic("kaboom")
			})
			t.Error("expected panic to propagate")
		}()

		if n := countRows(t, db, table); n != 0 {
			t.Errorf("rows = %d, want 0 after panic", n)
		}
	})

	t.Run("nested InTx reuses the outer transaction", func(t *testing.T) {
		table := newTxTestTable(t, db)
		mgr := NewTxManager(db)

		err := mgr.InTx(ctx, func(outer context.Context) error {
			outerTx, ok := TxFromContext(outer)
			if !ok {
				return errors.New("outer context has no transaction")
			}
			if err := insertRow(outer, db, table, 1); err != nil {
				return err
			}
			return mgr.InTx(outer, func(inner context.Context) error {
				innerTx, ok := TxFromContext(inner)
				if !ok || innerTx != outerTx {
					return errors.New("nested InTx did not reuse the outer transaction")
				}
				return insertRow(inner, db, table, 2)
			})
		})
		if err != nil {
			t.Fatalf("InTx: %v", err)
		}
		if n := countRows(t, db, table); n != 2 {
			t.Errorf("rows = %d, want 2", n)
		}
	})

	t.Run("nested failure rolls back the whole outer transaction", func(t *testing.T) {
		table := newTxTestTable(t, db)
		mgr := NewTxManager(db)
		inner := errors.New("inner failed")

		err := mgr.InTx(ctx, func(outer context.Context) error {
			if err := insertRow(outer, db, table, 1); err != nil {
				return err
			}
			return mgr.InTx(outer, func(context.Context) error { return inner })
		})
		if !errors.Is(err, inner) {
			t.Fatalf("err = %v, want inner", err)
		}
		if n := countRows(t, db, table); n != 0 {
			t.Errorf("rows = %d, want 0", n)
		}
	})
}

func TestTxContextHelpers(t *testing.T) {
	t.Run("TxFromContext is false on a plain context", func(t *testing.T) {
		if _, ok := TxFromContext(context.Background()); ok {
			t.Error("expected no transaction")
		}
	})

	t.Run("Executor falls back to db without transaction", func(t *testing.T) {
		db := &sqlx.DB{}
		if got := Executor(context.Background(), db); got != sqlx.ExtContext(db) {
			t.Errorf("Executor returned %T, want the db", got)
		}
	})

	t.Run("WithTx stores tx retrievable by TxFromContext and Executor", func(t *testing.T) {
		tx := &sqlx.Tx{}
		ctx := WithTx(context.Background(), tx)
		got, ok := TxFromContext(ctx)
		if !ok || got != sqlx.ExtContext(tx) {
			t.Fatalf("TxFromContext = %v, %v", got, ok)
		}
		if Executor(ctx, &sqlx.DB{}) != sqlx.ExtContext(tx) {
			t.Error("Executor did not prefer the transaction")
		}
	})
}
