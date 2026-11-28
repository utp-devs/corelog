package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// GlobalIndex is the global search over projects, issues, people, teams and filters. Issues use the weighted
// search document (summary A, description B) with the key shortcut and a substring fallback; ranking is exact
// key, then text rank, then prefix match, then recency.
type GlobalIndex struct {
	db *sqlx.DB
}

var _ port.GlobalIndex = (*GlobalIndex)(nil)

// NewGlobalIndex builds the index.
func NewGlobalIndex(db *sqlx.DB) *GlobalIndex { return &GlobalIndex{db: db} }

// SearchAll runs one query per requested kind and returns the hits in the order of Kinds.
func (g *GlobalIndex) SearchAll(ctx context.Context, req port.GlobalRequest) ([]domain.GlobalHit, error) {
	terms := domain.SearchTerms(req.Query)
	var out []domain.GlobalHit
	for _, kind := range req.Kinds {
		var (
			hits []domain.GlobalHit
			err  error
		)
		switch kind {
		case domain.KindProject:
			hits, err = g.projects(ctx, req, terms)
		case domain.KindIssue:
			hits, err = g.issues(ctx, req, terms)
		case domain.KindPerson:
			hits, err = g.people(ctx, req, terms)
		case domain.KindTeam:
			hits, err = g.teams(ctx, req, terms)
		case domain.KindFilter:
			hits, err = g.filters(ctx, req, terms)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, hits...)
	}
	return out, nil
}

func fail(what string, err error) error {
	return apperror.Wrap(apperror.KindInternal, "failed to search "+what, err)
}

const markOptions = `'MaxWords=18,MinWords=8,MaxFragments=1,StartSel=' || chr(2) || ',StopSel=' || chr(3)`

func (g *GlobalIndex) issues(ctx context.Context, req port.GlobalRequest, terms []string) ([]domain.GlobalHit, error) {
	var rows []struct {
		ID      uuid.UUID `db:"id"`
		Key     string    `db:"key"`
		Summary string    `db:"summary"`
		Project string    `db:"project_name"`
		Snippet string    `db:"snippet"`
	}
	err := g.db.SelectContext(ctx, &rows, `
		WITH q AS (SELECT (websearch_to_tsquery('spanish', $3::text) || websearch_to_tsquery('simple', $3::text)) AS tsq)
		SELECT i.id, p.key || '-' || i.key_number::text AS key, i.summary, p.name AS project_name,
		       CASE WHEN i.search_vector @@ q.tsq AND i.description <> ''
		            THEN ts_headline('spanish', translate(i.description, chr(2) || chr(3), ''), q.tsq, `+markOptions+`)
		            ELSE '' END AS snippet
		FROM (
		  -- Candidates come from one indexed branch per match kind (full text, summary and description trigrams,
		  -- issue key) instead of an OR over the whole table, which could only be answered by reading every issue.
		  SELECT i2.id FROM issues i2
		  WHERE i2.deleted_at IS NULL AND i2.search_vector @@ (websearch_to_tsquery('spanish', $3::text) || websearch_to_tsquery('simple', $3::text))
		  UNION
		  SELECT i2.id FROM issues i2 WHERE i2.deleted_at IS NULL AND i2.summary ILIKE $4::text ESCAPE '\'
		  UNION
		  SELECT i2.id FROM issues i2 WHERE i2.deleted_at IS NULL AND i2.description ILIKE $4::text ESCAPE '\'
		  UNION
		  SELECT i2.id FROM projects p2 JOIN issues i2 ON i2.project_id = p2.id AND i2.deleted_at IS NULL
		  WHERE p2.deleted_at IS NULL AND lower(p2.key) = lower(split_part($3::text, '-', 1))
		    AND i2.key_number::text = split_part($3::text, '-', 2) AND split_part($3::text, '-', 3) = ''
		) cand
		JOIN issues i ON i.id = cand.id
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		CROSS JOIN q
		WHERE `+visibleProjectSQL("p", "$1::uuid", "$2::boolean")+`
		ORDER BY (lower(p.key || '-' || i.key_number::text) = lower($3::text)) DESC,
		         ts_rank_cd(i.search_vector, q.tsq) DESC,
		         (i.summary ILIKE $5::text ESCAPE '\') DESC,
		         i.updated_at DESC, i.id
		LIMIT $6::int`,
		req.Viewer.UserID, req.Viewer.PlatformAdmin, req.Query, likePattern(req.Query), prefixPattern(req.Query), req.Limit)
	if err != nil {
		return nil, fail("issues", err)
	}
	out := make([]domain.GlobalHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.GlobalHit{
			Hit:       domain.Hit{Kind: domain.KindIssue, ID: r.ID.String(), Title: r.Summary, Subtitle: r.Key, Link: "/issues/" + r.ID.String()},
			Key:       r.Key,
			Highlight: domain.HighlightHTML(r.Summary, terms),
			Snippet:   domain.SnippetHTML(r.Snippet),
		})
	}
	return out, nil
}

func (g *GlobalIndex) projects(ctx context.Context, req port.GlobalRequest, terms []string) ([]domain.GlobalHit, error) {
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Key  string    `db:"key"`
		Name string    `db:"name"`
	}
	err := g.db.SelectContext(ctx, &rows, `
		SELECT p.id, p.key, p.name FROM projects p
		WHERE p.deleted_at IS NULL AND `+visibleProjectSQL("p", "$1::uuid", "$2::boolean")+`
		  AND (p.name ILIKE $4::text ESCAPE '\' OR p.key ILIKE $4::text ESCAPE '\')
		ORDER BY (lower(p.key) = lower($3::text)) DESC, (p.key ILIKE $5::text ESCAPE '\') DESC,
		         (p.name ILIKE $5::text ESCAPE '\') DESC, lower(p.name), p.id
		LIMIT $6::int`,
		req.Viewer.UserID, req.Viewer.PlatformAdmin, req.Query, likePattern(req.Query), prefixPattern(req.Query), req.Limit)
	if err != nil {
		return nil, fail("projects", err)
	}
	out := make([]domain.GlobalHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.GlobalHit{
			Hit:       domain.Hit{Kind: domain.KindProject, ID: r.ID.String(), Title: r.Name, Subtitle: r.Key, Link: "/projects/" + r.ID.String() + "/board"},
			Key:       r.Key,
			Highlight: domain.HighlightHTML(r.Name, terms),
		})
	}
	return out, nil
}

func (g *GlobalIndex) people(ctx context.Context, req port.GlobalRequest, terms []string) ([]domain.GlobalHit, error) {
	var rows []struct {
		ID    uuid.UUID `db:"id"`
		Name  string    `db:"display_name"`
		Email string    `db:"email"`
	}
	err := g.db.SelectContext(ctx, &rows, `
		SELECT u.id, u.display_name, u.email FROM users u
		WHERE u.deactivated_at IS NULL
		  AND ($2::boolean OR EXISTS (
		        SELECT 1 FROM organization_members a JOIN organization_members b ON b.organization_id = a.organization_id
		        WHERE a.user_id = u.id AND b.user_id = $1::uuid))
		  AND (u.display_name ILIKE $4::text ESCAPE '\' OR u.name ILIKE $4::text ESCAPE '\' OR u.email ILIKE $4::text ESCAPE '\')
		ORDER BY (lower(u.email) = lower($3::text)) DESC, (u.display_name ILIKE $5::text ESCAPE '\') DESC,
		         lower(u.display_name), u.id
		LIMIT $6::int`,
		req.Viewer.UserID, req.Viewer.PlatformAdmin, req.Query, likePattern(req.Query), prefixPattern(req.Query), req.Limit)
	if err != nil {
		return nil, fail("people", err)
	}
	out := make([]domain.GlobalHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.GlobalHit{
			Hit:       domain.Hit{Kind: domain.KindPerson, ID: r.ID.String(), Title: r.Name, Subtitle: r.Email, Link: "/people/" + r.ID.String()},
			Highlight: domain.HighlightHTML(r.Name, terms),
		})
	}
	return out, nil
}

func (g *GlobalIndex) teams(ctx context.Context, req port.GlobalRequest, terms []string) ([]domain.GlobalHit, error) {
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Name string    `db:"name"`
	}
	err := g.db.SelectContext(ctx, &rows, `
		SELECT t.id, t.name FROM teams t
		WHERE t.deleted_at IS NULL AND t.organization_id IN `+viewerOrgsSQL+` AND t.name ILIKE $4::text ESCAPE '\'
		ORDER BY (lower(t.name) = lower($3::text)) DESC, (t.name ILIKE $5::text ESCAPE '\') DESC, lower(t.name), t.id
		LIMIT $6::int`,
		req.Viewer.UserID, req.Viewer.PlatformAdmin, req.Query, likePattern(req.Query), prefixPattern(req.Query), req.Limit)
	if err != nil {
		return nil, fail("teams", err)
	}
	out := make([]domain.GlobalHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.GlobalHit{
			Hit:       domain.Hit{Kind: domain.KindTeam, ID: r.ID.String(), Title: r.Name, Link: "/teams/" + r.ID.String()},
			Highlight: domain.HighlightHTML(r.Name, terms),
		})
	}
	return out, nil
}

func (g *GlobalIndex) filters(ctx context.Context, req port.GlobalRequest, terms []string) ([]domain.GlobalHit, error) {
	var rows []struct {
		ID    uuid.UUID `db:"id"`
		Name  string    `db:"name"`
		Query string    `db:"query_text"`
	}
	err := g.db.SelectContext(ctx, &rows, `
		SELECT x.id, x.name, x.query_text FROM (`+viewSQL()+`) x
		WHERE (x.is_owner OR x.shared_permission <> '') AND (x.name ILIKE $2::text ESCAPE '\' OR x.description ILIKE $2::text ESCAPE '\')
		ORDER BY (lower(x.name) = lower($3::text)) DESC, (x.name ILIKE $4::text ESCAPE '\') DESC, lower(x.name), x.id
		LIMIT $5::int`,
		req.Viewer.UserID, likePattern(req.Query), req.Query, prefixPattern(req.Query), req.Limit)
	if err != nil {
		return nil, fail("filters", err)
	}
	out := make([]domain.GlobalHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.GlobalHit{
			Hit:       domain.Hit{Kind: domain.KindFilter, ID: r.ID.String(), Title: r.Name, Subtitle: truncate(strings.TrimSpace(r.Query), 80), Link: "/filters/" + r.ID.String()},
			Highlight: domain.HighlightHTML(r.Name, terms),
		})
	}
	return out, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
