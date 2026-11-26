package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

// statementCounter counts the statements executed through a connection. It is shared by every connection
// of the pool, so it sees the whole repository call.
type statementCounter struct{ n atomic.Int64 }

// countingConnector wraps the pgx connector so every connection counts the statements it runs.
type countingConnector struct {
	inner   driver.Connector
	counter *statementCounter
}

func (c countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: conn, counter: c.counter}, nil
}

func (c countingConnector) Driver() driver.Driver { return c.inner.Driver() }

// countingConn counts QueryContext and ExecContext calls. The repository uses only these paths (no
// prepared statements), so the count equals the number of SQL statements issued.
type countingConn struct {
	driver.Conn
	counter *statementCounter
}

func (c *countingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.counter.n.Add(1)
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, fmt.Errorf("countingConn: underlying connection cannot query directly")
	}
	return q.QueryContext(ctx, query, args)
}

func (c *countingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.counter.n.Add(1)
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, fmt.Errorf("countingConn: underlying connection cannot exec directly")
	}
	return e.ExecContext(ctx, query, args)
}

// CheckNamedValue forwards to the pgx connection so argument conversion is unchanged.
func (c *countingConn) CheckNamedValue(nv *driver.NamedValue) error {
	if v, ok := c.Conn.(driver.NamedValueChecker); ok {
		return v.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// newCountingTeamRepository returns a team repository whose statements are counted. It opens a second pool
// on the schema that testdb migrated, so the counted queries run against the same data.
func newCountingTeamRepository(t *testing.T, migrated *sqlx.DB) (*TeamRepository, *statementCounter) {
	t.Helper()
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CORELOG_TEST_DATABASE_URL is not set")
	}
	var schema string
	if err := migrated.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatalf("read schema: %v", err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["search_path"] = schema

	counter := &statementCounter{}
	pool := sql.OpenDB(countingConnector{inner: stdlib.GetConnector(*cfg), counter: counter})
	t.Cleanup(func() { _ = pool.Close() })
	return NewTeamRepository(sqlx.NewDb(pool, "pgx")), counter
}

// TestTeamRepository_ReadsIssueOneStatement proves that the team reads do not issue one query per team:
// a list of 10 teams with members, and a single-team read, each run exactly one SQL statement.
func TestTeamRepository_ReadsIssueOneStatement(t *testing.T) {
	db := testdb.New(t)
	repo := NewTeamRepository(db)
	ctx := context.Background()
	viewer := insertUser(t, repo, "count-viewer@example.test")
	org := insertOrg(t, repo, "org-count")

	var first *domain.Team
	for i := 0; i < 10; i++ {
		team := newTeam(t, org, viewer, fmt.Sprintf("Equipo conteo %02d", i))
		if err := repo.Save(ctx, team); err != nil {
			t.Fatalf("save team %d: %v", i, err)
		}
		if first == nil {
			first = team
		}
		// The viewer is the maintainer of the first team only. Team i gets i+1 other members.
		if i == 0 {
			m, _ := domain.NewMember(team.ID, viewer, domain.RoleMaintainer)
			if err := repo.AddMember(ctx, org, m); err != nil {
				t.Fatalf("add viewer: %v", err)
			}
		}
		for j := 0; j <= i; j++ {
			u := insertUser(t, repo, fmt.Sprintf("count-%d-%d@example.test", i, j))
			role := domain.RoleMember
			if j == 0 {
				role = domain.RoleMaintainer
			}
			m, _ := domain.NewMember(team.ID, u, role)
			if err := repo.AddMember(ctx, org, m); err != nil {
				t.Fatalf("add member: %v", err)
			}
		}
	}

	counted, counter := newCountingTeamRepository(t, db)

	counter.n.Store(0)
	list, err := counted.FindByOrganization(ctx, org, viewer)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := counter.n.Load(); got != 1 {
		t.Errorf("list of 10 teams issued %d statements, want 1", got)
	}
	if len(list) != 10 {
		t.Fatalf("list returned %d teams, want 10", len(list))
	}
	for _, team := range list {
		if team.MemberCount < 1 {
			t.Errorf("team %s: member_count = %d, want at least 1", team.Name, team.MemberCount)
		}
	}

	counter.n.Store(0)
	got, err := counted.FindByID(ctx, first.ID, viewer)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if n := counter.n.Load(); n != 1 {
		t.Errorf("single-team read issued %d statements, want 1", n)
	}
	if got.MemberCount != 2 || got.MyRole != domain.RoleMaintainer {
		t.Errorf("single-team read: member_count=%d my_role=%q, want 2 and maintainer", got.MemberCount, got.MyRole)
	}
}
