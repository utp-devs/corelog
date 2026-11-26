// Package coreql implements CoreQL, the JQL-compatible query language used by
// issue search. The package is pure: lexer, parser, AST, validator, printer
// and autocomplete suggestions. It performs no I/O and has no dependencies
// outside the standard library. SQL generation lives in the application layer.
package coreql

// Span locates a node in the input. Pos and Len are byte offsets.
type Span struct {
	Pos int
	Len int
}

// Location returns the span itself. It exists so that every node satisfies Node.
func (s Span) Location() Span { return s }

// Node is implemented by every AST node.
type Node interface {
	Location() Span
}

// Expr is a boolean expression node.
type Expr interface {
	Node
	exprNode()
}

// Value is a right-hand-side value node.
type Value interface {
	Node
	valueNode()
}

// Query is a whole CoreQL query. Expr is nil for an empty filter.
type Query struct {
	Span
	Expr    Expr
	OrderBy []*Order
}

// And is a conjunction. AND binds tighter than OR.
type And struct {
	Span
	Left, Right Expr
}

// Or is a disjunction.
type Or struct {
	Span
	Left, Right Expr
}

// Not is a negation of a single term.
type Not struct {
	Span
	Expr Expr
}

// Field is a field reference: a system or custom field name, or cf[<key>].
// Name is the raw source text.
type Field struct {
	Span
	Name string
}

// Op is a comparison operator.
type Op int

const (
	OpNone Op = iota
	OpEq
	OpNe
	OpGt
	OpGe
	OpLt
	OpLe
	OpContains
	OpNotContains
)

func opText(op Op) string {
	switch op {
	case OpEq:
		return "="
	case OpNe:
		return "!="
	case OpGt:
		return ">"
	case OpGe:
		return ">="
	case OpLt:
		return "<"
	case OpLe:
		return "<="
	case OpContains:
		return "~"
	case OpNotContains:
		return "!~"
	}
	return ""
}

// String returns the operator's source spelling.
func (o Op) String() string { return opText(o) }

// Compare is `field op value`. OpSpan locates the operator for diagnostics.
type Compare struct {
	Span
	Field  Field
	Op     Op
	OpSpan Span
	Value  Value
}

// In is `field [NOT] IN (value, ...)`.
type In struct {
	Span
	Field  Field
	Not    bool
	Values []Value
}

// IsEmpty is `field IS [NOT] EMPTY`. The synonym NULL parses to the same node.
type IsEmpty struct {
	Span
	Field Field
	Not   bool
}

// Order is one entry of the ORDER BY list.
type Order struct {
	Span
	Field Field
	Desc  bool
}

// String is a quoted string value. Text is unescaped.
type String struct {
	Span
	Text string
}

// Number is an integer or decimal value. Text is the source spelling.
type Number struct {
	Span
	Text string
}

// Word is a bare word value such as Done, CORE or CORE-12.
type Word struct {
	Span
	Text string
}

// RelativeDate is a date offset such as -7d or 1M. Unit is one of m, h, d, w, M, y.
type RelativeDate struct {
	Span
	Amount int
	Unit   string
}

// Func is a function call such as startOfDay(-1d) or membersOf("team").
type Func struct {
	Span
	Name string
	Args []Value
}

func (*And) exprNode()     {}
func (*Or) exprNode()      {}
func (*Not) exprNode()     {}
func (*Compare) exprNode() {}
func (*In) exprNode()      {}
func (*IsEmpty) exprNode() {}

func (*String) valueNode()       {}
func (*Number) valueNode()       {}
func (*Word) valueNode()         {}
func (*RelativeDate) valueNode() {}
func (*Func) valueNode()         {}

// join returns the span from the start of a to the end of b.
func join(a, b Span) Span {
	return Span{Pos: a.Pos, Len: b.Pos + b.Len - a.Pos}
}

// spanOf returns the span covered by a token.
func spanOf(t Token) Span {
	return Span{Pos: t.Pos, Len: t.Len}
}
