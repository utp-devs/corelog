package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/application/usecase"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type fakeNotes struct{ hits []domain.GlobalHit }

func (f fakeNotes) SearchNotes(_ context.Context, _ uuid.UUID, query string, _ int) ([]domain.GlobalHit, error) {
	return f.hits, nil
}

func (e *searchEnv) global(user uuid.UUID, q string, kinds ...domain.Kind) []domain.GlobalHit {
	e.t.Helper()
	svc := usecase.NewGlobalService(postgres.NewGlobalIndex(e.db), fakeNotes{hits: []domain.GlobalHit{{Hit: domain.Hit{Kind: domain.KindNote, ID: "n1", Title: "nota"}}}})
	hits, err := svc.SearchAll(e.ctx, e.viewer(user), q, kinds, 10)
	if err != nil {
		e.t.Fatalf("global %q: %v", q, err)
	}
	return hits
}

func titles(hits []domain.GlobalHit) string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = string(h.Kind) + ":" + h.Title
	}
	return strings.Join(out, "|")
}

func TestGlobalSearchIssuesRankingAndKeys(t *testing.T) {
	e := newSearchEnv(t)
	issues := func(q string) []domain.GlobalHit { return e.global(e.alice, q, domain.KindIssue) }

	// A key is an exact shortcut and comes first, whatever its text.
	hits := issues("pub-3")
	if len(hits) == 0 || hits[0].Key != "PUB-3" || hits[0].Subtitle != "PUB-3" || hits[0].Link != "/issues/"+hits[0].ID {
		t.Fatalf("key shortcut: %+v", hits)
	}
	// Exact key beats text rank: a summary that matches the words ranks below the key.
	e.exec(`UPDATE issues SET summary = 'Revisar pub-3 antes del release' WHERE id = (SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.key = 'PUB' AND i.key_number = 2)`)
	hits = issues("pub-3")
	if hits[0].Key != "PUB-3" || len(hits) < 2 {
		t.Fatalf("exact key must rank first: %+v", hits)
	}
	// Full text: stems and the weighted document (summary A before description B).
	if hits := issues("documentando"); len(hits) != 1 || hits[0].Key != "PUB-3" {
		t.Fatalf("stemming: %+v", hits)
	}
	e.exec(`UPDATE issues SET description = 'login login login' WHERE id = (SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.key = 'PUB' AND i.key_number = 5)`)
	hits = issues("login")
	if len(hits) < 2 || hits[0].Key != "PUB-1" {
		t.Fatalf("a summary match must outrank a description match: %s", titles(hits))
	}
	// Partial words still match through the substring fallback.
	if hits := issues("documen"); len(hits) != 1 || hits[0].Key != "PUB-3" {
		t.Fatalf("partial word: %+v", hits)
	}
	// The Spanish stemmer folds accents, so a word typed without them still matches.
	if hits := issues("movil"); len(hits) != 1 || hits[0].Key != "PUB-1" {
		t.Fatalf("accent-insensitive stem: %+v", hits)
	}
	// Wildcards are literal.
	if hits := issues("%%"); len(hits) != 0 {
		t.Fatalf("wildcard matched: %+v", hits)
	}
	// Limits and visibility: bob never sees the private project, dave only his organization.
	if hits := e.global(e.bob, "secreto", domain.KindIssue); len(hits) != 0 {
		t.Fatalf("bob sees a private issue: %+v", hits)
	}
	if hits := e.global(e.alice, "secreto", domain.KindIssue); len(hits) != 1 || hits[0].Key != "PRIV-1" {
		t.Fatalf("alice: %+v", hits)
	}
	if hits := e.global(e.dave, "login", domain.KindIssue); len(hits) != 1 || hits[0].Key != "OTH-1" {
		t.Fatalf("dave: %+v", hits)
	}
	if hits := e.global(e.outsider, "login", domain.KindIssue); len(hits) != 0 {
		t.Fatalf("outsider: %+v", hits)
	}
	// Soft-deleted issues never match.
	if hits := e.global(e.admin, "eliminada", domain.KindIssue); len(hits) != 0 {
		t.Fatalf("deleted issue found: %+v", hits)
	}
}

func TestGlobalSearchHighlightsAreEscaped(t *testing.T) {
	e := newSearchEnv(t)
	e.exec(`UPDATE issues SET summary = $1, description = $2 WHERE id = (SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.key = 'PUB' AND i.key_number = 1)`,
		`<script>alert("login")</script> & login`, "texto con <b>login</b> y \x02marca falsa\x03 en la descripcion de login")
	hits := e.global(e.alice, "login", domain.KindIssue)
	var h domain.GlobalHit
	for _, c := range hits {
		if c.Key == "PUB-1" {
			h = c
		}
	}
	if h.Key == "" {
		t.Fatalf("not found: %+v", hits)
	}
	if strings.Contains(h.Highlight, "<script>") || !strings.Contains(h.Highlight, "&lt;script&gt;") || !strings.Contains(h.Highlight, "<mark>login</mark>") {
		t.Fatalf("highlight: %s", h.Highlight)
	}
	if strings.Contains(h.Snippet, "<b>") || !strings.Contains(h.Snippet, "<mark>") {
		t.Fatalf("snippet: %s", h.Snippet)
	}
	if strings.Count(h.Snippet, "<mark>") != strings.Count(h.Snippet, "</mark>") || strings.ContainsAny(h.Snippet, "\x02\x03") {
		t.Fatalf("unbalanced marks: %s", h.Snippet)
	}
}

func TestGlobalSearchOtherKinds(t *testing.T) {
	f := newFilterEnv(t)
	e := f.searchEnv
	// Projects: exact key first, then prefix.
	hits := e.global(e.alice, "pri", domain.KindProject)
	if titles(hits) != "project:Privado" || hits[0].Key != "PRIV" {
		t.Fatalf("projects: %s", titles(hits))
	}
	hits = e.global(e.alice, "pub", domain.KindProject)
	if len(hits) != 1 || hits[0].Link != "/projects/"+hits[0].ID+"/board" {
		t.Fatalf("project hit: %+v", hits)
	}
	if hits := e.global(e.bob, "priv", domain.KindProject); len(hits) != 0 {
		t.Fatalf("bob sees a private project: %+v", hits)
	}
	// People of the same organizations only; deactivated accounts are skipped.
	if hits := e.global(e.alice, "bob", domain.KindPerson); titles(hits) != "person:bob" || hits[0].Subtitle != "bob@example.test" {
		t.Fatalf("people: %+v", hits)
	}
	if hits := e.global(e.alice, "dave", domain.KindPerson); len(hits) != 0 {
		t.Fatalf("person of another organization: %+v", hits)
	}
	e.exec(`UPDATE users SET deactivated_at = now() WHERE id = $1`, e.bob)
	if hits := e.global(e.alice, "bob", domain.KindPerson); len(hits) != 0 {
		t.Fatalf("deactivated person: %+v", hits)
	}
	// Teams are scoped to the organization.
	if hits := e.global(e.alice, "back", domain.KindTeam); titles(hits) != "team:Backend" {
		t.Fatalf("teams: %+v", hits)
	}
	if hits := e.global(e.dave, "back", domain.KindTeam); len(hits) != 0 {
		t.Fatalf("team of another organization: %+v", hits)
	}
	// Filters honor sharing.
	id := f.create(e.alice, "Mis incidencias criticas", "priority = Máxima")
	if hits := e.global(e.alice, "criticas", domain.KindFilter); len(hits) != 1 || hits[0].Subtitle != "priority = Máxima" || hits[0].Link != "/filters/"+id.String() {
		t.Fatalf("filters: %+v", hits)
	}
	if hits := e.global(e.bob, "criticas", domain.KindFilter); len(hits) != 0 {
		t.Fatalf("unshared filter found: %+v", hits)
	}
	f.share(e.alice, id, domain.Share{Type: domain.ShareUser, TargetID: e.bob})
	if hits := e.global(e.bob, "criticas", domain.KindFilter); len(hits) != 1 {
		t.Fatalf("shared filter: %+v", hits)
	}
	// Notes come from their provider; the default kinds stay project and issue.
	if hits := e.global(e.alice, "nota", domain.KindNote); titles(hits) != "note:nota" {
		t.Fatalf("notes: %+v", hits)
	}
	for _, h := range e.global(e.alice, "pub") {
		if h.Kind != domain.KindProject && h.Kind != domain.KindIssue {
			t.Fatalf("default kinds leaked %s", h.Kind)
		}
	}
}

func TestGlobalSearchRejectsBadQueries(t *testing.T) {
	e := newSearchEnv(t)
	svc := usecase.NewGlobalService(postgres.NewGlobalIndex(e.db), nil)
	if _, err := svc.SearchAll(e.ctx, e.viewer(e.alice), "a", nil, 0); err == nil {
		t.Fatal("a one character query must be rejected")
	}
	if _, err := svc.SearchAll(e.ctx, e.viewer(e.alice), strings.Repeat("x", 101), nil, 0); err == nil {
		t.Fatal("a long query must be rejected")
	}
	hits, err := svc.SearchAll(e.ctx, e.viewer(e.alice), "pub", []domain.Kind{domain.KindNote}, 0)
	if err != nil || len(hits) != 0 {
		t.Fatalf("notes without a provider: %v %v", hits, err)
	}
	var _ port.GlobalService = svc
}
