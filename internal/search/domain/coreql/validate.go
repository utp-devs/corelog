package coreql

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidationError is a semantic error found by Validate.
type ValidationError struct {
	Pos         int
	Len         int
	Message     string   // Spanish
	Suggestions []string // close names, when the problem is an unknown name
}

// Validate checks a parsed query against a field catalog and a function
// registry. Nil arguments default to SystemCatalog and DefaultFunctions.
func Validate(q *Query, cat FieldCatalog, funcs FunctionRegistry) []ValidationError {
	if q == nil {
		return nil
	}
	if cat == nil {
		cat = SystemCatalog()
	}
	if funcs == nil {
		funcs = DefaultFunctions()
	}
	v := &validator{cat: cat, funcs: funcs}
	if q.Expr != nil {
		v.expr(q.Expr)
	}
	for _, o := range q.OrderBy {
		v.order(o)
	}
	return v.errs
}

type validator struct {
	cat   FieldCatalog
	funcs FunctionRegistry
	errs  []ValidationError
}

func (v *validator) add(s Span, msg string, suggestions []string) {
	v.errs = append(v.errs, ValidationError{Pos: s.Pos, Len: s.Len, Message: msg, Suggestions: suggestions})
}

func (v *validator) expr(e Expr) {
	switch e := e.(type) {
	case *And:
		v.expr(e.Left)
		v.expr(e.Right)
	case *Or:
		v.expr(e.Left)
		v.expr(e.Right)
	case *Not:
		v.expr(e.Expr)
	case *Compare:
		def, ok := v.field(e.Field)
		if ok && !hasOp(def.allowedOps(), e.Op) {
			v.add(e.OpSpan, fmt.Sprintf("el operador %s no es válido para el campo %s (tipo %s)", e.Op, def.Name, def.Type), nil)
		}
		v.value(e.Value, defPtr(def, ok))
	case *In:
		def, ok := v.field(e.Field)
		if ok && !hasOp(def.allowedOps(), OpEq) {
			v.add(e.Span, fmt.Sprintf("el campo %s (tipo %s) no admite IN", def.Name, def.Type), nil)
		}
		for _, val := range e.Values {
			v.value(val, defPtr(def, ok))
		}
	case *IsEmpty:
		v.field(e.Field)
	}
}

func (v *validator) field(f Field) (FieldDef, bool) {
	def, ok := v.cat.Lookup(f.Name)
	if !ok {
		v.add(f.Span, fmt.Sprintf("campo desconocido '%s'", f.Name), suggestNames(f.Name, v.cat.Names()))
	}
	return def, ok
}

func (v *validator) order(o *Order) {
	def, ok := v.field(o.Field)
	if ok && !def.Sortable {
		v.add(o.Field.Span, fmt.Sprintf("no se puede ordenar por el campo %s", def.Name), nil)
	}
}

// value checks one value. def is nil when the field is unknown; in that case
// only the value's own structure (function existence and arity) is checked.
func (v *validator) value(val Value, def *FieldDef) {
	switch x := val.(type) {
	case *Func:
		v.function(x, def)
	case *RelativeDate:
		if def != nil && !isDateType(def.Type) {
			v.add(x.Span, "las fechas relativas solo aplican a campos de fecha", nil)
		}
	case *Number:
		if def != nil && !numberAllowed(def.Type) {
			v.add(x.Span, fmt.Sprintf("se esperaba un valor de tipo %s, no un número", def.Type), nil)
		}
	case *String:
		v.literalText(x.Span, x.Text, def)
	case *Word:
		v.literalText(x.Span, x.Text, def)
	}
}

func (v *validator) literalText(s Span, text string, def *FieldDef) {
	if def == nil {
		return
	}
	switch def.Type {
	case TypeNumber:
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			v.add(s, fmt.Sprintf("se esperaba un número en el campo %s", def.Name), nil)
		}
	case TypeBool:
		if text != "true" && text != "false" {
			v.add(s, "se esperaba true o false", nil)
		}
	case TypeDate:
		if !parsesAs(text, "2006-01-02") {
			v.add(s, "se esperaba una fecha YYYY-MM-DD o una fecha relativa", nil)
		}
	case TypeDateTime:
		if !parsesAs(text, "2006-01-02 15:04", "2006-01-02") {
			v.add(s, "se esperaba una fecha y hora YYYY-MM-DD HH:MM", nil)
		}
	}
}

func (v *validator) function(f *Func, def *FieldDef) {
	spec, ok := v.funcs.Lookup(f.Name)
	if !ok {
		v.add(f.Span, fmt.Sprintf("función desconocida '%s'", f.Name), suggestNames(f.Name, v.funcs.Names()))
		return
	}
	n := len(f.Args)
	if n < spec.MinArgs || n > spec.MaxArgs {
		v.add(f.Span, fmt.Sprintf("la función %s acepta %s argumentos, se recibieron %d", spec.Name, arityText(spec), n), nil)
	}
	for _, a := range f.Args {
		switch a.(type) {
		case *Func:
			v.add(a.Location(), "no se admiten funciones anidadas", nil)
		case *RelativeDate:
			if spec.Arg != ArgRelDate {
				v.add(a.Location(), fmt.Sprintf("la función %s espera texto como argumento", spec.Name), nil)
			}
		case *String, *Word:
			if spec.Arg == ArgRelDate {
				v.add(a.Location(), fmt.Sprintf("la función %s espera una fecha relativa (por ejemplo -1d)", spec.Name), nil)
			}
		default:
			v.add(a.Location(), fmt.Sprintf("argumento no válido en la función %s", spec.Name), nil)
		}
	}
	if def != nil && !compatible(def.Type, spec.Returns) {
		v.add(f.Span, fmt.Sprintf("la función %s devuelve %s y no es compatible con el campo %s (tipo %s)",
			spec.Name, spec.Returns, def.Name, def.Type), nil)
	}
}

func arityText(s FuncSpec) string {
	switch {
	case s.MinArgs == s.MaxArgs:
		return strconv.Itoa(s.MinArgs)
	}
	return fmt.Sprintf("entre %d y %d", s.MinArgs, s.MaxArgs)
}

func defPtr(d FieldDef, ok bool) *FieldDef {
	if !ok {
		return nil
	}
	return &d
}

func hasOp(ops []Op, op Op) bool {
	for _, o := range ops {
		if o == op {
			return true
		}
	}
	return false
}

func isDateType(t FieldType) bool { return t == TypeDate || t == TypeDateTime }

func numberAllowed(t FieldType) bool {
	switch t {
	case TypeNumber, TypeText, TypeLabels, TypeVersion, TypeComponent, TypeSprint,
		TypeEnum, TypeStatus, TypePriority, TypeResolution, TypeIssueType, TypeProject:
		return true
	}
	return false
}

func parsesAs(text string, layouts ...string) bool {
	for _, l := range layouts {
		if _, err := time.Parse(l, text); err == nil {
			return true
		}
	}
	return false
}

// suggestNames returns up to five names within Levenshtein distance 2 of input,
// closest first. Comparison ignores case.
func suggestNames(input string, names []string) []string {
	type cand struct {
		name string
		dist int
	}
	var cands []cand
	for _, n := range names {
		if d := levenshtein(input, n); d <= 2 {
			cands = append(cands, cand{n, d})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].name < cands[j].name
	})
	var out []string
	for i, c := range cands {
		if i == 5 {
			break
		}
		out = append(out, c.name)
	}
	return out
}

// levenshtein computes the edit distance between a and b, ignoring case.
func levenshtein(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
