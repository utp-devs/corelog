package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Deterministic inputs: a fixed namespace for UUIDv5-style ids, a fixed PRNG key
// and a constant base instant. The same scenario always produces the same rows.
var (
	idNamespace = uuid.MustParse("3f6c9a1e-2b7d-5c4a-8e0f-7d1b9c2a4e65")
	baseTime    = time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
)

const (
	day        = 24 * time.Hour
	seedKey    = 0x636f72656c6f67 // "corelog"
	onConflict = "ON CONFLICT DO NOTHING"

	// Keep every statement well under the 65535 bind-parameter limit of PostgreSQL.
	maxRowsPerStatement = 1000
	maxParamsPerStmt    = 60000
)

// PRNG stream identifiers, one per scenario, so scenarios do not share draws.
const (
	streamDemo  uint64 = 1
	streamScale uint64 = 2
)

func newRNG(stream uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seedKey, stream))
}

// idFor derives a stable UUID from a kind and natural key parts.
func idFor(kind string, parts ...string) uuid.UUID {
	return uuid.NewSHA1(idNamespace, []byte(kind+"|"+strings.Join(parts, "|")))
}

func userID(email string) uuid.UUID   { return idFor("user", email) }
func projectID(key string) uuid.UUID  { return idFor("project", key) }
func issueID(n int) uuid.UUID         { return idFor("issue", fmt.Sprint(n)) }
func daysFromBase(days int) time.Time { return baseTime.Add(time.Duration(days) * day) }
func ptr[T any](v T) *T               { return &v }

// opt converts an optional pointer into a SQL value (nil becomes NULL).
func opt[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// table holds the rows of one table, in insertion order.
type table struct {
	name   string
	cols   []string
	suffix string
	rows   [][]any
}

// add appends one row; values must match the column list.
func (t *table) add(vals ...any) {
	if len(vals) != len(t.cols) {
		panic(fmt.Sprintf("seed: table %s expects %d values, got %d", t.name, len(t.cols), len(vals)))
	}
	t.rows = append(t.rows, vals)
}

// dataset is an ordered set of tables. Tables are inserted in registration
// order, so parents must be registered before children.
type dataset struct {
	tables []*table
	byName map[string]*table
	// before holds statements that run right before the rows of a table are inserted; finally holds the
	// statements that run after every table. Both are idempotent.
	before  map[string][]string
	finally []string
}

func newDataset() *dataset {
	return &dataset{byName: map[string]*table{}, before: map[string][]string{}}
}

// table returns the named table, registering it with cols on first use.
func (d *dataset) table(name string, cols ...string) *table {
	if t, ok := d.byName[name]; ok {
		return t
	}
	t := &table{name: name, cols: cols}
	d.byName[name] = t
	d.tables = append(d.tables, t)
	return t
}

// runBefore registers a statement that runs before the rows of the named table are inserted.
func (d *dataset) runBefore(tableName, statement string) {
	d.before[tableName] = append(d.before[tableName], statement)
}

// runFinally registers a statement that runs after every table is inserted.
func (d *dataset) runFinally(statement string) {
	d.finally = append(d.finally, statement)
}

// ignoreDuplicates marks a table whose inserts may repeat a key harmlessly.
func (d *dataset) ignoreDuplicates(name string, cols ...string) *table {
	t := d.table(name, cols...)
	t.suffix = onConflict
	return t
}

// persist writes the dataset in one transaction, in chunks of multi-row INSERTs.
func persist(ctx context.Context, db *sqlx.DB, ds *dataset) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transaccion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	cache := map[catalogRef]uuid.UUID{}
	for _, t := range ds.tables {
		for _, statement := range ds.before[t.name] {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("preparar %s: %w", t.name, err)
			}
		}
		if err := insertRows(ctx, tx, t, cache); err != nil {
			return err
		}
	}
	for _, statement := range ds.finally {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("cerrar la siembra: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar siembra: %w", err)
	}
	return nil
}

func insertRows(ctx context.Context, tx *sqlx.Tx, t *table, cache map[catalogRef]uuid.UUID) error {
	if len(t.rows) == 0 {
		return nil
	}
	perStmt := min(maxRowsPerStatement, maxParamsPerStmt/len(t.cols))
	head := "INSERT INTO " + t.name + " (" + strings.Join(t.cols, ", ") + ") VALUES "
	for start := 0; start < len(t.rows); start += perStmt {
		end := min(start+perStmt, len(t.rows))
		var sb strings.Builder
		sb.WriteString(head)
		args := make([]any, 0, (end-start)*len(t.cols))
		for i, row := range t.rows[start:end] {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteByte('(')
			for j, v := range row {
				if j > 0 {
					sb.WriteString(", ")
				}
				if ref, ok := v.(catalogRef); ok {
					id, err := resolveCatalogRef(ctx, tx, cache, ref)
					if err != nil {
						return err
					}
					v = id
				}
				args = append(args, v)
				fmt.Fprintf(&sb, "$%d", len(args))
			}
			sb.WriteByte(')')
		}
		if t.suffix != "" {
			sb.WriteString(" " + t.suffix)
		}
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("insertar en %s: %w", t.name, err)
		}
	}
	return nil
}
