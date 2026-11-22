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

// SearchIndex busca con ILIKE sobre proyectos e incidencias (ADR-009). La visibilidad replica la politica
// de proyecto: admin global o miembro del proyecto.
type SearchIndex struct {
	db *sqlx.DB
}

func NewSearchIndex(db *sqlx.DB) *SearchIndex {
	return &SearchIndex{db: db}
}

var _ port.Index = (*SearchIndex)(nil)

type projectHit struct {
	ID   uuid.UUID `db:"id"`
	Key  string    `db:"key"`
	Name string    `db:"name"`
}

type issueHit struct {
	ID    uuid.UUID `db:"id"`
	Title string    `db:"title"`
	Key   string    `db:"key"`
}

// likePattern envuelve la consulta en comodines y escapa los caracteres especiales de LIKE, para que
// buscar "100%" o "a_b" coincida literalmente.
func likePattern(query string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	return "%" + escaped + "%"
}

func (s *SearchIndex) Search(ctx context.Context, actorID uuid.UUID, admin bool, query string, limit int) ([]domain.Hit, error) {
	pattern := likePattern(query)

	var projects []projectHit
	if err := s.db.SelectContext(ctx, &projects, `
		SELECT p.id, p.key, p.name
		FROM projects p
		WHERE ($1 OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = $2))
		  AND (p.name ILIKE $3 ESCAPE '\' OR p.key ILIKE $3 ESCAPE '\')
		ORDER BY p.key
		LIMIT $4`, admin, actorID, pattern, limit); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to search projects", err)
	}

	var issues []issueHit
	if err := s.db.SelectContext(ctx, &issues, `
		SELECT i.id, i.title, p.key
		FROM issues i
		JOIN projects p ON p.id = i.project_id
		WHERE ($1 OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = i.project_id AND pm.user_id = $2))
		  AND (i.title ILIKE $3 ESCAPE '\' OR i.description ILIKE $3 ESCAPE '\')
		ORDER BY i.updated_at DESC
		LIMIT $4`, admin, actorID, pattern, limit); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to search issues", err)
	}

	hits := make([]domain.Hit, 0, len(projects)+len(issues))
	for _, project := range projects {
		hits = append(hits, domain.Hit{Kind: domain.KindProject, ID: project.ID.String(), Title: project.Name, Subtitle: project.Key, Link: "/projects/" + project.ID.String() + "/board"})
	}
	for _, issue := range issues {
		hits = append(hits, domain.Hit{Kind: domain.KindIssue, ID: issue.ID.String(), Title: issue.Title, Subtitle: issue.Key, Link: "/issues/" + issue.ID.String()})
	}
	return hits, nil
}
