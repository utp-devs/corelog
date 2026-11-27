package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	noteport "github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search"
	searchpostgres "github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	searchport "github.com/jhonsferg/corelog/internal/search/application/port"
	searchdomain "github.com/jhonsferg/corelog/internal/search/domain"
)

// noteSearchSource adapts the notes provider to the global search: the notes of the caller only.
type noteSearchSource struct {
	notes noteport.SearchProvider
}

var _ searchport.NoteSource = noteSearchSource{}

func (s noteSearchSource) SearchNotes(ctx context.Context, userID uuid.UUID, query string, limit int) ([]searchdomain.GlobalHit, error) {
	hits, err := s.notes.SearchOwn(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}
	terms := searchdomain.SearchTerms(query)
	out := make([]searchdomain.GlobalHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, searchdomain.GlobalHit{
			Hit:       searchdomain.Hit{Kind: searchdomain.KindNote, ID: h.ID.String(), Title: h.Title, Link: "/notes/" + h.ID.String()},
			Highlight: searchdomain.HighlightHTML(h.Title, terms),
		})
	}
	return out, nil
}

// wireSearch enables the CoreQL issue search, the saved filters and the global search v2 on the search module.
func wireSearch(m *search.Module, db *sqlx.DB, notes noteport.SearchProvider) {
	repo := searchpostgres.NewIssueSearchRepository(db).WithVersions(searchpostgres.VersionTables{})
	m.WithIssueSearch(search.IssueSearchDeps{
		Searcher: repo, Custom: repo, Viewers: repo, Suggest: repo, Clock: clock.RealClock{},
	}).
		WithFilters(searchpostgres.NewFilterRepository(db), clock.RealClock{}).
		WithGlobal(searchpostgres.NewGlobalIndex(db), noteSearchSource{notes: notes})
}
