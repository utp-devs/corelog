package postgres

import (
	"context"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/search/application/port"
)

// prefixPattern escapes the LIKE wildcards of s and anchors it at the start.
func prefixPattern(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%"
}

// viewerOrgsSQL selects the live organizations of the viewer ($1 = user id, $2 = platform administrator).
const viewerOrgsSQL = `(SELECT o.id FROM organizations o WHERE o.deleted_at IS NULL AND ($2::boolean OR EXISTS (
    SELECT 1 FROM organization_members om WHERE om.organization_id = o.id AND om.user_id = $1::uuid)))`

// suggestQueries holds the value queries per field. Parameters: $1 viewer, $2 platform admin, $3 prefix
// pattern (anchored, ILIKE), $4 limit. Each query returns value, label and detail. Project-scoped catalogs
// go through the browse rule so a suggestion never reveals a project the viewer cannot see.
var suggestQueries = map[string]string{
	"assignee": userSuggestSQL, "reporter": userSuggestSQL, "creator": userSuggestSQL, "watcher": userSuggestSQL,
	"status": `SELECT DISTINCT ON (lower(s.name)) s.name AS value, s.name AS label, s.category AS detail
		FROM statuses s WHERE s.organization_id IN ` + viewerOrgsSQL + ` AND s.name ILIKE $3 ESCAPE '\'
		ORDER BY lower(s.name) LIMIT $4`,
	"priority": `SELECT DISTINCT ON (lower(x.name)) x.name AS value, x.name AS label, '' AS detail
		FROM priorities x WHERE x.deleted_at IS NULL AND x.organization_id IN ` + viewerOrgsSQL + ` AND x.name ILIKE $3 ESCAPE '\'
		ORDER BY lower(x.name) LIMIT $4`,
	"issuetype": issueTypeSuggestSQL, "type": issueTypeSuggestSQL,
	"resolution": `SELECT DISTINCT ON (lower(x.name)) x.name AS value, x.name AS label, '' AS detail
		FROM resolutions x WHERE x.deleted_at IS NULL AND x.organization_id IN ` + viewerOrgsSQL + ` AND x.name ILIKE $3 ESCAPE '\'
		ORDER BY lower(x.name) LIMIT $4`,
	"project": `SELECT p.key AS value, p.name AS label, p.key AS detail FROM projects p
		WHERE p.deleted_at IS NULL AND ` + visibleProjectSQL("p", "$1::uuid", "$2::boolean") + `
		AND (p.key ILIKE $3 ESCAPE '\' OR p.name ILIKE $3 ESCAPE '\') ORDER BY p.key LIMIT $4`,
	"labels": `SELECT DISTINCT ON (lower(l.name)) l.name AS value, l.name AS label, '' AS detail
		FROM labels l JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL AND l.deleted_at IS NULL
		WHERE ` + visibleProjectSQL("p", "$1::uuid", "$2::boolean") + ` AND l.name ILIKE $3 ESCAPE '\'
		ORDER BY lower(l.name) LIMIT $4`,
	"component": `SELECT DISTINCT ON (lower(c.name)) c.name AS value, c.name AS label, '' AS detail
		FROM components c JOIN projects p ON p.id = c.project_id AND p.deleted_at IS NULL AND c.deleted_at IS NULL
		WHERE ` + visibleProjectSQL("p", "$1::uuid", "$2::boolean") + ` AND c.name ILIKE $3 ESCAPE '\'
		ORDER BY lower(c.name) LIMIT $4`,
	"sprint": `SELECT s.name AS value, s.name AS label, p.key AS detail
		FROM sprints s JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL AND s.deleted_at IS NULL
		WHERE ` + visibleProjectSQL("p", "$1::uuid", "$2::boolean") + ` AND s.name ILIKE $3 ESCAPE '\'
		ORDER BY s.status = 'active' DESC, s.created_at DESC LIMIT $4`,
	"key": issueSuggestSQL, "epic": issueSuggestSQL, "parent": issueSuggestSQL,
	"fixversion": versionSuggestSQL, "affectedversion": versionSuggestSQL,
}

const userSuggestSQL = `SELECT u.email AS value, u.display_name AS label, u.email AS detail FROM users u
	WHERE ($2::boolean OR EXISTS (
	    SELECT 1 FROM organization_members a JOIN organization_members b ON b.organization_id = a.organization_id
	    WHERE a.user_id = u.id AND b.user_id = $1::uuid))
	  AND (u.display_name ILIKE $3 ESCAPE '\' OR u.email ILIKE $3 ESCAPE '\' OR u.name ILIKE $3 ESCAPE '\')
	ORDER BY lower(u.display_name) LIMIT $4`

const issueTypeSuggestSQL = `SELECT DISTINCT ON (lower(x.name)) x.name AS value, x.name AS label, '' AS detail
	FROM issue_types x WHERE x.deleted_at IS NULL AND x.organization_id IN ` + viewerOrgsSQL + ` AND x.name ILIKE $3 ESCAPE '\'
	ORDER BY lower(x.name) LIMIT $4`

const issueSuggestSQL = `SELECT p.key || '-' || i.key_number::text AS value, i.summary AS label, p.name AS detail
	FROM issues i JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
	WHERE i.deleted_at IS NULL AND ` + `%VISIBLE%` + `
	  AND ((p.key || '-' || i.key_number::text) ILIKE $3 ESCAPE '\' OR i.summary ILIKE $3 ESCAPE '\')
	ORDER BY i.updated_at DESC, i.id LIMIT $4`

const versionSuggestSQL = `SELECT DISTINCT ON (lower(v.name)) v.name AS value, v.name AS label, v.status AS detail
	FROM versions v JOIN projects p ON p.id = v.project_id AND p.deleted_at IS NULL AND v.deleted_at IS NULL
	WHERE ` + `%VISIBLE%` + ` AND v.name ILIKE $3 ESCAPE '\'
	ORDER BY lower(v.name) LIMIT $4`

var staticSuggestions = map[string][]port.ValueSuggestion{
	"statuscategory": {
		{Value: "todo", Label: "Por hacer"},
		{Value: "in_progress", Label: "En curso"},
		{Value: "done", Label: "Hecho"},
	},
	"flagged": {{Value: "true", Label: "Sí"}, {Value: "false", Label: "No"}},
}

// SuggestValues lists up to limit candidate values of a system field that start with prefix (or whose name
// does, for users and projects).
func (r *IssueSearchRepository) SuggestValues(ctx context.Context, viewer port.Viewer, field, prefix string, limit int) ([]port.ValueSuggestion, error) {
	field = strings.ToLower(field)
	if static, ok := staticSuggestions[field]; ok {
		out := make([]port.ValueSuggestion, 0, len(static))
		for _, s := range static {
			if strings.HasPrefix(strings.ToLower(s.Value), strings.ToLower(prefix)) {
				out = append(out, s)
			}
		}
		return out, nil
	}
	query, ok := suggestQueries[field]
	if !ok {
		return []port.ValueSuggestion{}, nil
	}
	if (field == "fixversion" || field == "affectedversion") && r.versions == nil {
		return []port.ValueSuggestion{}, nil
	}
	query = strings.ReplaceAll(query, "%VISIBLE%", visibleProjectSQL("p", "$1::uuid", "$2::boolean"))
	var rows []struct {
		Value  string `db:"value"`
		Label  string `db:"label"`
		Detail string `db:"detail"`
	}
	if err := r.db.SelectContext(ctx, &rows, query, viewer.UserID, viewer.PlatformAdmin, prefixPattern(prefix), limit); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to suggest values", err)
	}
	out := make([]port.ValueSuggestion, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.ValueSuggestion{Value: row.Value, Label: row.Label, Detail: row.Detail})
	}
	return out, nil
}
