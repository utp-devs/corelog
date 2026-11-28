package postgres

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

type fieldClass int

const (
	cProject fieldClass = iota + 1
	cKey
	cIssueType
	cStatus
	cStatusCategory
	cResolution
	cPriority
	cUser
	cLabels
	cComponent
	cFixVersion
	cAffectedVersion
	cSprint
	cEpic
	cParent
	cText
	cComment
	cFullText
	cDateTime
	cDate
	cNumber
	cWatcher
	cFlagged
	cUnavailable
	cCustom
)

// resolved is a field reference resolved to its SQL handling. col is a constant column expression chosen in
// code, never user text.
type resolved struct {
	class  fieldClass
	col    string
	name   string
	custom *port.CustomFieldDef
}

var systemFields = map[string]resolved{
	"project":           {class: cProject},
	"key":               {class: cKey},
	"issuetype":         {class: cIssueType},
	"type":              {class: cIssueType},
	"status":            {class: cStatus},
	"statuscategory":    {class: cStatusCategory},
	"resolution":        {class: cResolution},
	"priority":          {class: cPriority},
	"assignee":          {class: cUser, col: "i.assignee_id"},
	"reporter":          {class: cUser, col: "i.reporter_id"},
	"creator":           {class: cUser, col: "i.reporter_id"},
	"labels":            {class: cLabels},
	"component":         {class: cComponent},
	"fixversion":        {class: cFixVersion},
	"affectedversion":   {class: cAffectedVersion},
	"sprint":            {class: cSprint},
	"epic":              {class: cEpic},
	"parent":            {class: cParent},
	"summary":           {class: cText, col: "i.summary"},
	"description":       {class: cText, col: "i.description"},
	"comment":           {class: cComment},
	"text":              {class: cFullText},
	"created":           {class: cDateTime, col: "i.created_at"},
	"updated":           {class: cDateTime, col: "i.updated_at"},
	"resolved":          {class: cDateTime, col: "i.resolved_at"},
	"due":               {class: cDate, col: "i.due_date"},
	"start":             {class: cDate, col: "i.start_date"},
	"votes":             {class: cNumber, col: "i.vote_count"},
	"watcher":           {class: cWatcher},
	"flagged":           {class: cFlagged},
	"storypoints":       {class: cNumber, col: "i.story_points"},
	"originalestimate":  {class: cNumber, col: "i.original_estimate_seconds"},
	"remainingestimate": {class: cNumber, col: "i.remaining_estimate_seconds"},
	"timespent":         {class: cUnavailable},
}

func (b *builder) field(f coreql.Field) (resolved, bool) {
	lower := strings.ToLower(f.Name)
	if !strings.HasPrefix(lower, "cf[") {
		if sf, ok := systemFields[lower]; ok {
			sf.name = f.Name
			return sf, true
		}
	}
	if def, ok := b.q.Custom[port.CustomRef(f.Name)]; ok {
		return resolved{class: cCustom, name: def.Name, custom: &def}, true
	}
	b.fail(f.Span, domain.CodeUnknownField, "campo desconocido '%s'", f.Name)
	return resolved{}, false
}

// textOf returns the text of a string, bare word or number value.
func textOf(v coreql.Value) (string, bool) {
	switch x := v.(type) {
	case *coreql.String:
		return x.Text, true
	case *coreql.Word:
		return x.Text, true
	case *coreql.Number:
		return x.Text, true
	}
	return "", false
}

func funcName(v coreql.Value) (string, *coreql.Func, bool) {
	if f, ok := v.(*coreql.Func); ok {
		return strings.ToLower(f.Name), f, true
	}
	return "", nil, false
}

func (b *builder) badValue(v coreql.Value, f resolved) string {
	b.fail(v.Location(), domain.CodeInvalidQuery, "valor no admitido para el campo %s", f.name)
	return "FALSE"
}

// match builds the positive predicate of op over the values (OR across values). Negations wrap it.
func (b *builder) match(f resolved, op coreql.Op, vals []coreql.Value, span coreql.Span) string {
	if f.class == cUnavailable {
		b.fail(span, domain.CodeFieldUnavailable, "el campo %s todavía no está disponible en las búsquedas", f.name)
		return "FALSE"
	}
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, b.matchOne(f, op, v, span))
		if b.err != nil {
			return "FALSE"
		}
	}
	return anyOf(parts)
}

func (b *builder) matchOne(f resolved, op coreql.Op, v coreql.Value, span coreql.Span) string {
	switch f.class {
	case cProject:
		return b.projectMatch(f, v)
	case cKey:
		return b.keyMatch(f, op, v)
	case cIssueType:
		return b.refMatch(f, v, "issue_types", "it", "i.issue_type_id", true)
	case cPriority:
		return b.refMatch(f, v, "priorities", "pr", "i.priority_id", true)
	case cStatus:
		return b.refMatch(f, v, "statuses", "st", "i.status_id", false)
	case cResolution:
		if t, ok := textOf(v); ok && isUnresolved(t) {
			return "i.resolution_id IS NULL"
		}
		return b.refMatch(f, v, "resolutions", "rs", "i.resolution_id", false)
	case cStatusCategory:
		return b.categoryMatch(f, v)
	case cUser:
		return b.userMatch(f.col, false, v, f)
	case cWatcher:
		return "EXISTS (SELECT 1 FROM watchers w WHERE w.issue_id = i.id AND " + b.userMatch("w.user_id", false, v, f) + ")"
	case cLabels:
		t, ok := textOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		return "EXISTS (SELECT 1 FROM issue_labels il JOIN labels l ON l.id = il.label_id AND l.deleted_at IS NULL WHERE il.issue_id = i.id AND lower(l.name) = lower(" + b.bindText(t) + "))"
	case cComponent:
		t, ok := textOf(v)
		if !ok {
			return b.badValue(v, f)
		}
		return "EXISTS (SELECT 1 FROM issue_components ic JOIN components c ON c.id = ic.component_id AND c.deleted_at IS NULL WHERE ic.issue_id = i.id AND lower(c.name) = lower(" + b.bindText(t) + "))"
	case cFixVersion:
		return b.versionMatch(f, port.VersionFix, v)
	case cAffectedVersion:
		return b.versionMatch(f, port.VersionAffects, v)
	case cSprint:
		return b.sprintMatch(f, v)
	case cEpic:
		return b.epicMatch(f, v, true)
	case cParent:
		return b.epicMatch(f, v, false)
	case cText:
		return b.textMatch(f, op, v)
	case cComment:
		return b.commentMatch(f, v)
	case cFullText:
		return b.fullTextMatch(f, v)
	case cDateTime:
		return b.timestampCompare(f.col, mTimestamptz, op, v, f)
	case cDate:
		return b.timestampCompare(f.col, mDate, op, v, f)
	case cNumber:
		return b.numberCompare(f.col, op, v, f)
	case cFlagged:
		t, ok := textOf(v)
		if !ok || (t != "true" && t != "false") {
			return b.badValue(v, f)
		}
		return "i.flagged = " + b.bind(t == "true", "boolean")
	case cCustom:
		return b.customMatch(f, op, v)
	}
	b.fail(span, domain.CodeUnknownField, "campo desconocido '%s'", f.name)
	return "FALSE"
}

func isUnresolved(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "unresolved", "sin resolver", "sin resolucion", "sin resolución":
		return true
	}
	return false
}

func (b *builder) projectMatch(f resolved, v coreql.Value) string {
	if name, _, ok := funcName(v); ok {
		if name != "currentproject" {
			return b.badValue(v, f)
		}
		if b.q.ProjectID == nil {
			b.fail(v.Location(), domain.CodeScopeRequired, "currentProject() solo se puede usar dentro de un proyecto")
			return "FALSE"
		}
		return "p.id = " + b.bind(*b.q.ProjectID, "uuid")
	}
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	ph := b.bindText(t)
	return "(lower(p.key) = lower(" + ph + ") OR lower(p.name) = lower(" + ph + ") OR p.id::text = lower(" + ph + "))"
}

// parseIssueKey splits "CORE-12" into its project key and number.
func parseIssueKey(s string) (string, int, bool) {
	s = strings.TrimSpace(s)
	i := strings.LastIndexByte(s, '-')
	if i <= 0 || i == len(s)-1 {
		return "", 0, false
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return s[:i], n, true
}

func (b *builder) keyMatch(f resolved, op coreql.Op, v coreql.Value) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	if op == coreql.OpContains {
		return "(p.key || '-' || i.key_number::text) ILIKE " + b.bindText(likePattern(t)) + " ESCAPE '\\'"
	}
	key, n, ok := parseIssueKey(t)
	if !ok {
		return "FALSE"
	}
	return "(lower(p.key) = lower(" + b.bindText(key) + ") AND i.key_number = " + b.bind(n, "int") + ")"
}

// refMatch matches a catalog row by name (case-insensitive) or, for issue types and priorities, by legacy key.
func (b *builder) refMatch(f resolved, v coreql.Value, table, alias, idExpr string, legacy bool) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	ph := b.bindText(t)
	cond := "lower(" + alias + ".name) = lower(" + ph + ")"
	if legacy {
		cond += " OR " + alias + ".legacy_key = lower(" + ph + ")"
	}
	return "EXISTS (SELECT 1 FROM " + table + " " + alias + " WHERE " + alias + ".id = " + idExpr + " AND (" + cond + "))"
}

// normalizeCategory maps the accepted spellings of a status category to its stored value.
func normalizeCategory(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "_", " "))) {
	case "todo", "to do", "por hacer", "open":
		return "todo", true
	case "in progress", "en curso", "en progreso":
		return "in_progress", true
	case "done", "hecho", "terminado", "closed":
		return "done", true
	}
	return "", false
}

func (b *builder) categoryMatch(f resolved, v coreql.Value) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	cat, ok := normalizeCategory(t)
	if !ok {
		b.fail(v.Location(), domain.CodeInvalidQuery, "categoría de estado no válida '%s'; usa todo, in_progress o done", t)
		return "FALSE"
	}
	return "corelog_issue_category(i.status_id, i.status) = " + b.bindText(cat)
}

// userMatch matches the user column col (a uuid column, or a text column when textCol is true) against a user
// reference: currentUser(), membersOf("team"), or an id, email or name.
func (b *builder) userMatch(col string, textCol bool, v coreql.Value, f resolved) string {
	idExpr, cast := "u.id", ""
	if textCol {
		idExpr, cast = "u.id::text", "::text"
	}
	if name, fn, ok := funcName(v); ok {
		switch name {
		case "currentuser":
			return col + " = " + b.bind(b.q.Viewer.UserID, "uuid") + cast
		case "membersof":
			if len(fn.Args) != 1 {
				return b.badValue(v, f)
			}
			team, ok := textOf(fn.Args[0])
			if !ok {
				return b.badValue(v, f)
			}
			sel := "SELECT tm.user_id" + cast + " FROM team_members tm JOIN teams t ON t.id = tm.team_id" +
				" WHERE t.deleted_at IS NULL AND lower(t.name) = lower(" + b.bindText(team) + ") AND t.organization_id = p.organization_id"
			return col + " IN (" + sel + ")"
		}
		return b.badValue(v, f)
	}
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	ph := b.bindText(t)
	return col + " IN (SELECT " + idExpr + " FROM users u WHERE lower(u.email) = lower(" + ph + ") OR lower(u.name) = lower(" + ph +
		") OR lower(u.display_name) = lower(" + ph + ") OR u.id::text = lower(" + ph + "))"
}

func (b *builder) versionMatch(f resolved, kind port.VersionKind, v coreql.Value) string {
	if b.versions == nil {
		b.fail(v.Location(), domain.CodeFieldUnavailable, "el campo %s todavía no está disponible en las búsquedas", f.name)
		return "FALSE"
	}
	spec := port.VersionSpec{}
	if name, _, ok := funcName(v); ok {
		spec.Function = name
	} else if t, ok := textOf(v); ok {
		spec.Name = t
	} else {
		return b.badValue(v, f)
	}
	frag, err := b.versions.Condition(kind, spec)
	if err != nil {
		if errors.Is(err, ErrVersionsUnavailable) {
			b.fail(v.Location(), domain.CodeFieldUnavailable, "el campo %s todavía no está disponible en las búsquedas", f.name)
		} else {
			b.fail(v.Location(), domain.CodeInvalidQuery, "%s", err.Error())
		}
		return "FALSE"
	}
	return b.fragment(frag, v.Location())
}

// fragment binds the '?' placeholders of a fragment supplied by an optional port.
func (b *builder) fragment(frag SQLFragment, span coreql.Span) string {
	if strings.Count(frag.SQL, "?") != len(frag.Args) {
		b.fail(span, domain.CodeInvalidQuery, "fragmento de consulta inválido")
		return "FALSE"
	}
	var sb strings.Builder
	i := 0
	for _, r := range frag.SQL {
		if r == '?' {
			sb.WriteString(b.bindText(frag.Args[i]))
			i++
			continue
		}
		sb.WriteRune(r)
	}
	return "(" + sb.String() + ")"
}

func (b *builder) sprintMatch(f resolved, v coreql.Value) string {
	if name, _, ok := funcName(v); ok {
		status := map[string]string{"opensprints": "active", "closedsprints": "closed", "futuresprints": "planned"}[name]
		if status == "" {
			return b.badValue(v, f)
		}
		return "i.sprint_id IN (SELECT s.id FROM sprints s WHERE s.deleted_at IS NULL AND s.status = " + b.bindText(status) + ")"
	}
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	ph := b.bindText(t)
	return "i.sprint_id IN (SELECT s.id FROM sprints s WHERE s.deleted_at IS NULL AND (lower(s.name) = lower(" + ph + ") OR s.id::text = lower(" + ph + ")))"
}

// epicMatch matches the children of an issue (epic: also its grandchildren, as stories and their subtasks
// hang from the epic).
func (b *builder) epicMatch(f resolved, v coreql.Value, deep bool) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	sub := func() string {
		if id, err := uuid.Parse(t); err == nil {
			return "SELECT e.id FROM issues e WHERE e.deleted_at IS NULL AND e.id = " + b.bind(id, "uuid")
		}
		key, n, ok := parseIssueKey(t)
		if !ok {
			return "SELECT e.id FROM issues e WHERE FALSE"
		}
		return "SELECT e.id FROM issues e JOIN projects ep ON ep.id = e.project_id AND ep.deleted_at IS NULL" +
			" WHERE e.deleted_at IS NULL AND lower(ep.key) = lower(" + b.bindText(key) + ") AND e.key_number = " + b.bind(n, "int")
	}
	direct := "i.parent_id IN (" + sub() + ")"
	if !deep {
		return direct
	}
	return "(" + direct + " OR i.parent_id IN (SELECT c.id FROM issues c WHERE c.deleted_at IS NULL AND c.parent_id IN (" + sub() + ")))"
}

func (b *builder) textMatch(f resolved, op coreql.Op, v coreql.Value) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	if op == coreql.OpContains {
		return f.col + " ILIKE " + b.bindText(likePattern(t)) + " ESCAPE '\\'"
	}
	return "lower(" + f.col + ") = lower(" + b.bindText(t) + ")"
}

func (b *builder) commentMatch(f resolved, v coreql.Value) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	ph := b.bindText(t)
	return "EXISTS (SELECT 1 FROM comments c WHERE c.issue_id = i.id AND c.deleted_at IS NULL AND c.visibility = 'public' AND (" +
		"to_tsvector('spanish', c.body) @@ websearch_to_tsquery('spanish', " + ph + ") OR c.body ILIKE " + b.bindText(likePattern(t)) + " ESCAPE '\\'))"
}

// fullTextMatch searches summary, description and comments: the weighted document with both text search
// configurations, the comments through their own index, and a trigram-friendly substring match on the summary
// for partial words.
func (b *builder) fullTextMatch(f resolved, v coreql.Value) string {
	t, ok := textOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	ph := b.bindText(t)
	return "(i.search_vector @@ (websearch_to_tsquery('spanish', " + ph + ") || websearch_to_tsquery('simple', " + ph + "))" +
		" OR i.summary ILIKE " + b.bindText(likePattern(t)) + " ESCAPE '\\'" +
		" OR EXISTS (SELECT 1 FROM comments c WHERE c.issue_id = i.id AND c.deleted_at IS NULL AND c.visibility = 'public' AND to_tsvector('spanish', c.body) @@ websearch_to_tsquery('spanish', " + ph + ")))"
}

func (b *builder) numberCompare(col string, op coreql.Op, v coreql.Value, f resolved) string {
	n, ok := b.numberOf(v)
	if !ok {
		return b.badValue(v, f)
	}
	return col + " " + sqlCompareOp(op) + " " + b.bindText(n) + "::numeric"
}

func (b *builder) numberOf(v coreql.Value) (string, bool) {
	t, ok := textOf(v)
	if !ok {
		return "", false
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return "", false
	}
	return strconv.FormatFloat(n, 'f', -1, 64), true
}

func sqlCompareOp(op coreql.Op) string {
	switch op {
	case coreql.OpGt:
		return ">"
	case coreql.OpGe:
		return ">="
	case coreql.OpLt:
		return "<"
	case coreql.OpLe:
		return "<="
	}
	return "="
}

// emptySQL is the predicate of `field IS EMPTY`.
func (b *builder) emptySQL(f resolved, span coreql.Span) string {
	switch f.class {
	case cUnavailable:
		b.fail(span, domain.CodeFieldUnavailable, "el campo %s todavía no está disponible en las búsquedas", f.name)
		return "FALSE"
	case cProject, cKey:
		return "FALSE"
	case cIssueType:
		return "i.issue_type_id IS NULL"
	case cPriority:
		return "i.priority_id IS NULL"
	case cStatus, cStatusCategory:
		return "i.status_id IS NULL"
	case cResolution:
		return "i.resolution_id IS NULL"
	case cUser, cDateTime, cDate, cNumber:
		return f.col + " IS NULL"
	case cText:
		return "(" + f.col + " IS NULL OR btrim(" + f.col + ") = '')"
	case cLabels:
		return "NOT EXISTS (SELECT 1 FROM issue_labels il WHERE il.issue_id = i.id)"
	case cComponent:
		return "NOT EXISTS (SELECT 1 FROM issue_components ic WHERE ic.issue_id = i.id)"
	case cWatcher:
		return "NOT EXISTS (SELECT 1 FROM watchers w WHERE w.issue_id = i.id)"
	case cComment:
		return "NOT EXISTS (SELECT 1 FROM comments c WHERE c.issue_id = i.id AND c.deleted_at IS NULL AND c.visibility = 'public')"
	case cSprint:
		return "i.sprint_id IS NULL"
	case cEpic, cParent:
		return "i.parent_id IS NULL"
	case cFullText, cFlagged:
		return "FALSE"
	case cFixVersion, cAffectedVersion:
		kind := port.VersionFix
		if f.class == cAffectedVersion {
			kind = port.VersionAffects
		}
		if b.versions == nil {
			b.fail(span, domain.CodeFieldUnavailable, "el campo %s todavía no está disponible en las búsquedas", f.name)
			return "FALSE"
		}
		frag, err := b.versions.Condition(kind, port.VersionSpec{Empty: true})
		if err != nil {
			b.fail(span, domain.CodeFieldUnavailable, "el campo %s todavía no está disponible en las búsquedas", f.name)
			return "FALSE"
		}
		return b.fragment(frag, span)
	case cCustom:
		parts := make([]string, 0, len(f.custom.IDs))
		for _, id := range f.custom.IDs {
			val := "(i.custom -> " + b.bindText(id.String()) + ")"
			parts = append(parts, "("+val+" IS NULL OR "+val+" IN ('null'::jsonb, '[]'::jsonb, '\"\"'::jsonb))")
		}
		return anyOfAnd(parts)
	}
	return "FALSE"
}

// anyOfAnd joins per-organization predicates of a custom field: a value is empty when it is empty in every
// definition (each issue holds the values of its own organization's field).
func anyOfAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return "TRUE"
	case 1:
		return parts[0]
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}
