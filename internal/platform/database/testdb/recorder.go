package testdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Statement is one SQL statement sent through a Recorder, with the arguments it was sent with.
type Statement struct {
	Query string
	Args  []any
}

// Recorder is a second pool on the schema of a migrated test database that records every statement it runs.
// Tests build the object under test on Recorder.DB, run a call, and read the statements that call issued: the
// count proves there is no N+1 pattern and the texts and arguments can be replayed under EXPLAIN.
type Recorder struct {
	mu   sync.Mutex
	list []Statement
	db   *sqlx.DB
}

// NewRecorder opens the recording pool. The test is skipped when CORELOG_TEST_DATABASE_URL is not set.
func NewRecorder(t testing.TB, migrated *sqlx.DB) *Recorder {
	t.Helper()
	dsn := os.Getenv(envDatabaseURL)
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL integration tests", envDatabaseURL)
	}
	var schema string
	if err := migrated.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatalf("testdb: read schema: %v", err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("testdb: parse dsn: %v", err)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["search_path"] = schema

	r := &Recorder{}
	pool := sql.OpenDB(recordingConnector{inner: stdlib.GetConnector(*cfg), rec: r})
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = pool.Close() })
	r.db = sqlx.NewDb(pool, "pgx")
	return r
}

// DB is the pool whose statements are recorded.
func (r *Recorder) DB() *sqlx.DB { return r.db }

// Reset forgets the statements recorded so far.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = nil
}

// Count is the number of statements recorded since the last Reset.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.list)
}

// Statements returns a copy of the statements recorded since the last Reset.
func (r *Recorder) Statements() []Statement {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Statement(nil), r.list...)
}

func (r *Recorder) add(query string, args []driver.NamedValue) {
	values := make([]any, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	r.mu.Lock()
	r.list = append(r.list, Statement{Query: query, Args: values})
	r.mu.Unlock()
}

type recordingConnector struct {
	inner driver.Connector
	rec   *Recorder
}

func (c recordingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &recordingConn{Conn: conn, rec: c.rec}, nil
}

func (c recordingConnector) Driver() driver.Driver { return c.inner.Driver() }

// recordingConn records QueryContext and ExecContext calls; the pgx driver runs every statement through them.
type recordingConn struct {
	driver.Conn
	rec *Recorder
}

func (c *recordingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.rec.add(query, args)
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, fmt.Errorf("testdb: connection cannot query directly")
	}
	return q.QueryContext(ctx, query, args)
}

func (c *recordingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.rec.add(query, args)
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, fmt.Errorf("testdb: connection cannot exec directly")
	}
	return e.ExecContext(ctx, query, args)
}

// CheckNamedValue forwards to the pgx connection so argument conversion is unchanged.
func (c *recordingConn) CheckNamedValue(nv *driver.NamedValue) error {
	if v, ok := c.Conn.(driver.NamedValueChecker); ok {
		return v.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// BeginTx keeps the isolation level and the read-only flag of the caller.
func (c *recordingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	return c.Conn.Begin() //nolint:staticcheck // fallback for drivers without ConnBeginTx
}

// PrepareContext forwards statement preparation.
func (c *recordingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, query)
	}
	return c.Conn.Prepare(query)
}

// Ping forwards the liveness check of the pool.
func (c *recordingConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

// ResetSession forwards the reset between uses of a pooled connection.
func (c *recordingConn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

// IsValid forwards the validity check of a pooled connection.
func (c *recordingConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}
