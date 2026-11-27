package postgres

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// Limits that keep a query cheap to build and to run.
const (
	maxClauses      = 100
	maxValues       = 500
	maxExprDepth    = 40
	maxPageSize     = 100
	maxResultOffset = 10000
)

// BuiltQuery is the SQL of a query. Every user value is a bind parameter; identifiers come only from code.
type BuiltQuery struct {
	SelectSQL  string
	SelectArgs []any
	CountSQL   string
	CountArgs  []any
}

// BuildIssueQuery translates a validated query into parameterized SQL over issues. The visibility rule and
// the optional project scope are applied outside the user expression, so an OR in the query cannot escape
// them. versions may be nil, in which case the version fields are reported as unavailable.
func BuildIssueQuery(q port.IssueQuery, versions VersionResolver) (*BuiltQuery, error) {
	if q.Query == nil {
		q.Query = &coreql.Query{}
	}
	loc := time.UTC
	if q.Viewer.Timezone != "" {
		if l, err := time.LoadLocation(q.Viewer.Timezone); err == nil {
			loc = l
		}
	}
	now := q.Now
	if now.IsZero() {
		now = time.Now()
	}
	b := &builder{q: q, versions: versions, loc: loc, now: now.In(loc)}

	viewer := b.bind(q.Viewer.UserID, "uuid")
	admin := b.bind(q.Viewer.PlatformAdmin, "boolean")

	from := " FROM issues i JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL"
	where := " WHERE i.deleted_at IS NULL AND " + visibleProjectSQL("p", viewer, admin)
	if q.ProjectID != nil {
		where += " AND p.id = " + b.bind(*q.ProjectID, "uuid")
	}
	if q.Query.Expr != nil {
		expr := b.expr(q.Query.Expr, 0)
		if b.err != nil {
			return nil, b.err
		}
		where += " AND (" + expr + ")"
	}
	whereArgs := len(b.args)

	order := b.orderBy(q.Query.OrderBy)
	if b.err != nil {
		return nil, b.err
	}

	size := q.PageSize
	if size <= 0 || size > maxPageSize {
		size = maxPageSize
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * size
	if offset >= maxResultOffset {
		return nil, domain.ErrPageOutOfRange
	}
	limitPh := b.bind(size, "int")
	offsetPh := b.bind(offset, "int")

	out := &BuiltQuery{
		SelectSQL:  "SELECT i.id" + from + where + " ORDER BY " + order + " LIMIT " + limitPh + " OFFSET " + offsetPh,
		SelectArgs: b.args,
		CountSQL:   "SELECT count(*)" + from + where,
		CountArgs:  b.args[:whereArgs],
	}
	return out, nil
}

// visibleProjectSQL is the browse rule of a project row aliased as alias: the platform administrator sees
// every live project; anyone else needs a live organization membership and then either an owner or admin
// role, an explicit project role, a team grant, or the member role on a project that is not private. It is the
// SQL form of the project authorizer's effective role.
func visibleProjectSQL(alias, viewer, admin string) string {
	return "(" + admin + " OR EXISTS (SELECT 1 FROM organization_members vom" +
		" JOIN organizations vo ON vo.id = vom.organization_id AND vo.deleted_at IS NULL" +
		" WHERE vom.organization_id = " + alias + ".organization_id AND vom.user_id = " + viewer +
		" AND (vom.role IN ('owner', 'admin')" +
		" OR EXISTS (SELECT 1 FROM project_members vpm WHERE vpm.project_id = " + alias + ".id AND vpm.user_id = " + viewer + ")" +
		" OR EXISTS (SELECT 1 FROM project_team_roles vptr JOIN team_members vtm ON vtm.team_id = vptr.team_id" +
		" WHERE vptr.project_id = " + alias + ".id AND vtm.user_id = " + viewer + ")" +
		" OR (vom.role = 'member' AND " + alias + ".visibility <> 'private'))))"
}

type builder struct {
	q        port.IssueQuery
	versions VersionResolver
	loc      *time.Location
	now      time.Time
	args     []any
	err      error
	clauses  int
	values   int
}

// bind adds a parameter and returns its placeholder with an explicit cast, so the server never has to guess
// the parameter type.
func (b *builder) bind(v any, cast string) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args)) + "::" + cast
}

// bindText binds a text parameter.
func (b *builder) bindText(s string) string { return b.bind(s, "text") }

func (b *builder) fail(span coreql.Span, code, format string, args ...any) {
	if b.err != nil {
		return
	}
	b.err = &port.BuildError{Pos: span.Pos, Len: span.Len, Code: code, Message: fmt.Sprintf(format, args...)}
}

func (b *builder) tooComplex(span coreql.Span) {
	b.fail(span, domain.CodeQueryTooComplex, "la consulta es demasiado compleja")
}

// expr compiles an expression. And and Or chains are flattened so that long chains do not deepen the recursion.
func (b *builder) expr(e coreql.Expr, depth int) string {
	if b.err != nil {
		return "FALSE"
	}
	if depth > maxExprDepth {
		b.tooComplex(e.Location())
		return "FALSE"
	}
	switch n := e.(type) {
	case *coreql.And:
		return b.chain(n, depth, " AND ", func(x coreql.Expr) (coreql.Expr, coreql.Expr, bool) {
			if a, ok := x.(*coreql.And); ok {
				return a.Left, a.Right, true
			}
			return nil, nil, false
		})
	case *coreql.Or:
		return b.chain(e, depth, " OR ", func(x coreql.Expr) (coreql.Expr, coreql.Expr, bool) {
			if o, ok := x.(*coreql.Or); ok {
				return o.Left, o.Right, true
			}
			return nil, nil, false
		})
	case *coreql.Not:
		return "NOT COALESCE(" + b.expr(n.Expr, depth+1) + ", FALSE)"
	case *coreql.Compare:
		return b.compare(n)
	case *coreql.In:
		return b.in(n)
	case *coreql.IsEmpty:
		return b.isEmpty(n)
	}
	b.fail(e.Location(), domain.CodeInvalidQuery, "expresión no soportada")
	return "FALSE"
}

// chain flattens a chain of the same operator into one parenthesized list, without recursion proportional to
// the chain length.
func (b *builder) chain(root coreql.Expr, depth int, sep string, split func(coreql.Expr) (coreql.Expr, coreql.Expr, bool)) string {
	var leaves []coreql.Expr
	stack := []coreql.Expr{root}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if l, r, ok := split(x); ok {
			stack = append(stack, r, l)
			continue
		}
		leaves = append(leaves, x)
	}
	parts := make([]string, 0, len(leaves))
	for _, leaf := range leaves {
		parts = append(parts, b.expr(leaf, depth+1))
	}
	return "(" + strings.Join(parts, sep) + ")"
}

func (b *builder) countClause(span coreql.Span, values int) {
	b.clauses++
	b.values += values
	if b.clauses > maxClauses || b.values > maxValues {
		b.tooComplex(span)
	}
}

func (b *builder) compare(c *coreql.Compare) string {
	b.countClause(c.Span, 1)
	f, ok := b.field(c.Field)
	if !ok {
		return "FALSE"
	}
	switch c.Op {
	case coreql.OpEq:
		return b.match(f, coreql.OpEq, []coreql.Value{c.Value}, c.Span)
	case coreql.OpNe:
		return negate(b.match(f, coreql.OpEq, []coreql.Value{c.Value}, c.Span))
	case coreql.OpContains:
		return b.match(f, coreql.OpContains, []coreql.Value{c.Value}, c.Span)
	case coreql.OpNotContains:
		return negate(b.match(f, coreql.OpContains, []coreql.Value{c.Value}, c.Span))
	default:
		return b.match(f, c.Op, []coreql.Value{c.Value}, c.Span)
	}
}

func (b *builder) in(c *coreql.In) string {
	b.countClause(c.Span, len(c.Values))
	f, ok := b.field(c.Field)
	if !ok {
		return "FALSE"
	}
	sql := b.match(f, coreql.OpEq, c.Values, c.Span)
	if c.Not {
		return negate(sql)
	}
	return sql
}

func (b *builder) isEmpty(c *coreql.IsEmpty) string {
	b.countClause(c.Span, 0)
	f, ok := b.field(c.Field)
	if !ok {
		return "FALSE"
	}
	sql := b.emptySQL(f, c.Span)
	if c.Not {
		return negate(sql)
	}
	return sql
}

// negate is the SQL negation that treats NULL as "no match", so that `field != x` includes issues where the
// field is empty.
func negate(sql string) string { return "NOT COALESCE(" + sql + ", FALSE)" }

func anyOf(parts []string) string {
	switch len(parts) {
	case 0:
		return "FALSE"
	case 1:
		return parts[0]
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}
