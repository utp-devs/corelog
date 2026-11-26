package coreql

import "strings"

// SuggestionKind classifies an autocomplete entry.
type SuggestionKind string

const (
	SuggestField    SuggestionKind = "field"
	SuggestOperator SuggestionKind = "operator"
	SuggestKeyword  SuggestionKind = "keyword"
	SuggestFunction SuggestionKind = "function"
)

// Suggestion is one autocomplete entry. Label is also the text to insert.
type Suggestion struct {
	Label  string
	Kind   SuggestionKind
	Detail string
}

// Suggest returns completions for the cursor. cursor is a rune offset into
// input (the UI counts characters). A nil catalog means SystemCatalog.
// Matching of the partially typed word is a case-insensitive prefix match.
func Suggest(input string, cursor int, cat FieldCatalog) []Suggestion {
	if cat == nil {
		cat = SystemCatalog()
	}
	cb := ByteOffset(input, cursor)
	toks := tokenize(input)
	prefix := ""
	var ctx []Token
	found := false
	for i, t := range toks {
		if t.Kind == TokEOF {
			break
		}
		if t.Pos < cb && cb <= t.Pos+t.Len {
			if !isPrefixable(t.Kind) {
				return nil
			}
			prefix = input[t.Pos:cb]
			ctx = toks[:i]
			found = true
			break
		}
		if t.Pos >= cb {
			ctx = toks[:i]
			found = true
			break
		}
	}
	if !found {
		ctx = toks[:len(toks)-1]
	}
	lp := strings.ToLower(prefix)
	var out []Suggestion
	for _, s := range suggestionsFor(ctx, cat) {
		if strings.HasPrefix(strings.ToLower(s.Label), lp) {
			out = append(out, s)
		}
	}
	return out
}

func isPrefixable(k TokenKind) bool {
	switch k {
	case TokWord, TokAnd, TokOr, TokNot, TokIn, TokIs, TokEmpty, TokNull, TokOrder, TokBy, TokAsc, TokDesc:
		return true
	}
	return false
}

// suggestionsFor returns the candidates for the position after the tokens in ctx.
func suggestionsFor(ctx []Token, cat FieldCatalog) []Suggestion {
	n := len(ctx)
	for _, t := range ctx {
		if t.Kind == TokOrder {
			return orderSuggestions(ctx, cat)
		}
	}
	if n == 0 {
		return append(fieldSuggestions(cat), keyword("NOT"), keyword("ORDER BY"))
	}
	last := ctx[n-1]
	var prev Token
	hasPrev := n >= 2
	if hasPrev {
		prev = ctx[n-2]
	}
	switch last.Kind {
	case TokLParen, TokAnd, TokOr:
		return append(fieldSuggestions(cat), keyword("NOT"))
	case TokNot:
		if hasPrev && fieldStart(ctx, n-2) {
			return []Suggestion{keyword("IN")}
		}
		return fieldSuggestions(cat)
	case TokWord, TokCustomField:
		if fieldStart(ctx, n-1) {
			return operatorSuggestions(last.Text, cat)
		}
		if hasPrev && isValueKind(prev.Kind) {
			return valueEndSuggestions()
		}
	case TokOp:
		if hasPrev && (prev.Kind == TokWord || prev.Kind == TokCustomField) {
			return functionSuggestions(prev.Text, cat)
		}
	case TokIs:
		return []Suggestion{keyword("EMPTY"), keyword("NULL"), keyword("NOT")}
	case TokString, TokNumber, TokRelDate, TokRParen, TokEmpty, TokNull:
		return valueEndSuggestions()
	}
	return nil
}

// fieldStart reports whether ctx[i] is a field name in a position where a
// clause starts (beginning of query, after '(', AND, OR or NOT).
func fieldStart(ctx []Token, i int) bool {
	if i < 0 || (ctx[i].Kind != TokWord && ctx[i].Kind != TokCustomField) {
		return false
	}
	if i == 0 {
		return true
	}
	switch ctx[i-1].Kind {
	case TokLParen, TokAnd, TokOr, TokNot:
		return true
	}
	return false
}

func isValueKind(k TokenKind) bool {
	switch k {
	case TokOp, TokString, TokNumber, TokRelDate, TokRParen:
		return true
	}
	return false
}

// orderSuggestions handles the ORDER BY list.
func orderSuggestions(ctx []Token, cat FieldCatalog) []Suggestion {
	last := ctx[len(ctx)-1]
	switch last.Kind {
	case TokBy, TokComma:
		var out []Suggestion
		for _, name := range cat.Names() {
			if def, ok := cat.Lookup(name); ok && def.Sortable {
				out = append(out, Suggestion{Label: name, Kind: SuggestField, Detail: def.Type.String()})
			}
		}
		return out
	case TokWord, TokCustomField:
		return []Suggestion{keyword("ASC"), keyword("DESC")}
	}
	return nil
}

func fieldSuggestions(cat FieldCatalog) []Suggestion {
	var out []Suggestion
	for _, name := range cat.Names() {
		def, _ := cat.Lookup(name)
		out = append(out, Suggestion{Label: name, Kind: SuggestField, Detail: def.Type.String()})
	}
	return out
}

func operatorSuggestions(fieldRaw string, cat FieldCatalog) []Suggestion {
	var out []Suggestion
	if def, ok := cat.Lookup(fieldRaw); ok {
		for _, op := range def.allowedOps() {
			out = append(out, Suggestion{Label: op.String(), Kind: SuggestOperator, Detail: "operador"})
		}
	}
	return append(out, keyword("IN"), keyword("IS"), keyword("NOT IN"))
}

// functionSuggestions proposes the functions whose return type fits the field.
func functionSuggestions(fieldRaw string, cat FieldCatalog) []Suggestion {
	def, ok := cat.Lookup(fieldRaw)
	if !ok {
		return nil
	}
	var out []Suggestion
	for _, name := range DefaultFunctions().Names() {
		spec, _ := DefaultFunctions().Lookup(name)
		if compatible(def.Type, spec.Returns) {
			out = append(out, Suggestion{Label: name + "()", Kind: SuggestFunction, Detail: spec.Returns.String()})
		}
	}
	return out
}

func valueEndSuggestions() []Suggestion {
	return []Suggestion{keyword("AND"), keyword("OR"), keyword("ORDER BY")}
}

func keyword(label string) Suggestion {
	return Suggestion{Label: label, Kind: SuggestKeyword}
}
