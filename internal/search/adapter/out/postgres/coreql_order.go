package postgres

import (
	"strings"

	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// orderExpr returns the sort expression of a field. Only fields of the whitelist have one; the text of the
// expression is constant, with custom field ids bound as parameters.
func (b *builder) orderExpr(f resolved, span coreql.Span) (string, bool) {
	switch f.class {
	case cProject:
		return "p.key", true
	case cKey:
		return "p.key, i.key_number", true
	case cIssueType:
		return "(SELECT it.position FROM issue_types it WHERE it.id = i.issue_type_id)", true
	case cPriority:
		return "(SELECT pr.position FROM priorities pr WHERE pr.id = i.priority_id)", true
	case cStatus:
		return "(SELECT st.position FROM statuses st WHERE st.id = i.status_id)", true
	case cStatusCategory:
		return "(CASE corelog_issue_category(i.status_id, i.status) WHEN 'todo' THEN 0 WHEN 'in_progress' THEN 1 ELSE 2 END)", true
	case cResolution:
		return "(SELECT lower(rs.name) FROM resolutions rs WHERE rs.id = i.resolution_id)", true
	case cUser:
		return "(SELECT lower(u.display_name) FROM users u WHERE u.id = " + f.col + ")", true
	case cText:
		return "lower(" + f.col + ")", true
	case cDateTime, cDate, cNumber:
		return f.col, true
	case cCustom:
		ph := b.bindText(f.custom.IDs[0].String())
		val := "(i.custom ->> " + ph + ")"
		switch f.custom.Kind {
		case "text", "paragraph", "url", "select":
			return "lower(" + val + ")", true
		case "number":
			return "(CASE WHEN jsonb_typeof(i.custom -> " + ph + ") = 'number' THEN " + val + "::numeric END)", true
		case "date", "datetime", "checkbox":
			return val, true
		}
	}
	b.fail(span, domain.CodeInvalidQuery, "no se puede ordenar por el campo %s", f.name)
	return "", false
}

// orderBy builds the ORDER BY list. Entries are deduplicated, NULLs sort last, and the issue id breaks ties so
// that paging is stable. The default order is the creation date, newest first.
func (b *builder) orderBy(orders []*coreql.Order) string {
	if len(orders) == 0 {
		return "i.created_at DESC NULLS LAST, i.id ASC"
	}
	var parts []string
	seen := map[string]bool{}
	for _, o := range orders {
		f, ok := b.field(o.Field)
		if !ok {
			return ""
		}
		key := strings.ToLower(o.Field.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		expr, ok := b.orderExpr(f, o.Span)
		if !ok {
			return ""
		}
		dir := " ASC"
		if o.Desc {
			dir = " DESC"
		}
		if f.class == cKey {
			parts = append(parts, "p.key"+dir+" NULLS LAST", "i.key_number"+dir+" NULLS LAST")
			continue
		}
		parts = append(parts, expr+dir+" NULLS LAST")
	}
	return strings.Join(parts, ", ") + ", i.id ASC"
}
