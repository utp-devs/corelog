package coreql

import (
	"reflect"
	"strconv"
	"strings"
)

// Precedence levels used by the printer. Higher binds tighter.
const (
	precOr = iota + 1
	precAnd
	precNot
	precAtom
)

var stringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// String returns the normalized form of the query: keywords in upper case,
// single spaces, strings in double quotes, and only the parentheses needed to
// keep the precedence. Parse(q.String()) yields a query Equal to q.
func (q *Query) String() string {
	if q == nil {
		return ""
	}
	var parts []string
	if q.Expr != nil {
		parts = append(parts, printExpr(q.Expr))
	}
	if len(q.OrderBy) > 0 {
		items := make([]string, len(q.OrderBy))
		for i, o := range q.OrderBy {
			items[i] = o.Field.Name
			if o.Desc {
				items[i] += " DESC"
			}
		}
		parts = append(parts, "ORDER BY "+strings.Join(items, ", "))
	}
	return strings.Join(parts, " ")
}

func exprPrec(e Expr) int {
	switch e.(type) {
	case *Or:
		return precOr
	case *And:
		return precAnd
	case *Not:
		return precNot
	}
	return precAtom
}

func printChild(e Expr, minPrec int) string {
	s := printExpr(e)
	if exprPrec(e) < minPrec {
		return "(" + s + ")"
	}
	return s
}

func printExpr(e Expr) string {
	switch e := e.(type) {
	case *Or:
		// Left-associative: the right operand needs parentheses at equal precedence.
		return printChild(e.Left, precOr) + " OR " + printChild(e.Right, precAnd)
	case *And:
		return printChild(e.Left, precAnd) + " AND " + printChild(e.Right, precNot)
	case *Not:
		return "NOT " + printChild(e.Expr, precNot)
	case *Compare:
		return e.Field.Name + " " + e.Op.String() + " " + printValue(e.Value)
	case *In:
		op := " IN ("
		if e.Not {
			op = " NOT IN ("
		}
		vals := make([]string, len(e.Values))
		for i, v := range e.Values {
			vals[i] = printValue(v)
		}
		return e.Field.Name + op + strings.Join(vals, ", ") + ")"
	case *IsEmpty:
		if e.Not {
			return e.Field.Name + " IS NOT EMPTY"
		}
		return e.Field.Name + " IS EMPTY"
	}
	return ""
}

func printValue(v Value) string {
	switch v := v.(type) {
	case *String:
		return `"` + stringEscaper.Replace(v.Text) + `"`
	case *Number:
		return v.Text
	case *Word:
		return v.Text
	case *RelativeDate:
		return strconv.Itoa(v.Amount) + v.Unit
	case *Func:
		args := make([]string, len(v.Args))
		for i, a := range v.Args {
			args[i] = printValue(a)
		}
		return v.Name + "(" + strings.Join(args, ", ") + ")"
	}
	return ""
}

// Equal reports whether two queries have the same structure, ignoring source spans.
func Equal(a, b *Query) bool {
	if a == nil || b == nil {
		return a == b
	}
	return equalValue(reflect.ValueOf(a), reflect.ValueOf(b))
}

var spanType = reflect.TypeOf(Span{})

func equalValue(a, b reflect.Value) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Ptr, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return equalValue(a.Elem(), b.Elem())
	case reflect.Struct:
		if a.Type() == spanType {
			return true
		}
		for i := 0; i < a.NumField(); i++ {
			if !equalValue(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !equalValue(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	}
	return a.Interface() == b.Interface()
}
