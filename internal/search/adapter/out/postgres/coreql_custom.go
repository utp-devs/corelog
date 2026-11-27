package postgres

import (
	"strings"

	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// customMatch matches a custom field. Values live in issues.custom keyed by the field id; every organization
// that defines a field with the name contributes one id, and an issue matches through any of them.
func (b *builder) customMatch(f resolved, op coreql.Op, v coreql.Value) string {
	parts := make([]string, 0, len(f.custom.IDs))
	for _, id := range f.custom.IDs {
		ph := b.bindText(id.String())
		parts = append(parts, b.customOne(f, ph, op, v))
		if b.err != nil {
			return "FALSE"
		}
	}
	return anyOf(parts)
}

// optionIDs selects the ids of the select options of the field whose label or id equals the value.
func (b *builder) optionIDs(fieldPh, valuePh string) string {
	return "SELECT o ->> 'id' FROM custom_fields cfo, jsonb_array_elements(cfo.options) o WHERE cfo.id = " + fieldPh +
		"::uuid AND (lower(o ->> 'label') = lower(" + valuePh + ") OR o ->> 'id' = " + valuePh + ")"
}

func arrayElements(fieldPh string) string {
	return "jsonb_array_elements_text(CASE WHEN jsonb_typeof(i.custom -> " + fieldPh + ") = 'array' THEN i.custom -> " + fieldPh + " ELSE '[]'::jsonb END) e(v)"
}

func (b *builder) customOne(f resolved, fieldPh string, op coreql.Op, v coreql.Value) string {
	val := "(i.custom ->> " + fieldPh + ")"
	switch f.custom.Kind {
	case "text", "paragraph", "url":
		t, ok := textOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		if op == coreql.OpContains {
			return val + " ILIKE " + b.bindText(likePattern(t)) + " ESCAPE '\\'"
		}
		return "lower(" + val + ") = lower(" + b.bindText(t) + ")"
	case "number":
		n, ok := b.numberOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		return "(CASE WHEN jsonb_typeof(i.custom -> " + fieldPh + ") = 'number' THEN " + val + "::numeric END) " +
			sqlCompareOp(op) + " " + b.bindText(n) + "::numeric"
	case "date":
		return b.timestampCompare(val, mTextDate, op, v, f)
	case "datetime":
		return b.timestampCompare(val, mTextTS, op, v, f)
	case "select":
		t, ok := textOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		return val + " IN (" + b.optionIDs(fieldPh, b.bindText(t)) + ")"
	case "multiselect":
		t, ok := textOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		return "EXISTS (SELECT 1 FROM " + arrayElements(fieldPh) + " WHERE e.v IN (" + b.optionIDs(fieldPh, b.bindText(t)) + "))"
	case "checkbox":
		t, ok := textOf(v)
		t = strings.ToLower(t)
		if !ok || (t != "true" && t != "false") {
			return b.badValue(v, f)
		}
		return "COALESCE(" + val + ", 'false') = " + b.bindText(t)
	case "user":
		return b.userMatch(val, true, v, f)
	case "labels":
		t, ok := textOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		return "EXISTS (SELECT 1 FROM " + arrayElements(fieldPh) + " WHERE lower(e.v) = lower(" + b.bindText(t) + "))"
	}
	b.fail(v.Location(), domain.CodeInvalidQuery, "tipo de campo personalizado no soportado en %s", f.name)
	return "FALSE"
}
