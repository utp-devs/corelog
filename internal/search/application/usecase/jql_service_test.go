package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type fakeSearcher struct {
	last port.IssueQuery
	page port.IssuePage
	err  error
	n    int
}

func (f *fakeSearcher) SearchIssues(_ context.Context, q port.IssueQuery) (port.IssuePage, error) {
	f.last = q
	f.n++
	return f.page, f.err
}

type fakeCustom struct {
	fields []port.CustomFieldDef
	err    error
}

func (f fakeCustom) CustomFields(context.Context, port.Viewer) ([]port.CustomFieldDef, error) {
	return f.fields, f.err
}

type fakeViewers struct {
	tz  string
	err error
}

func (f fakeViewers) Timezone(context.Context, uuid.UUID) (string, error) { return f.tz, f.err }

type fakeValues struct {
	field, prefix string
	limit         int
	out           []port.ValueSuggestion
	err           error
}

func (f *fakeValues) SuggestValues(_ context.Context, _ port.Viewer, field, prefix string, limit int) ([]port.ValueSuggestion, error) {
	f.field, f.prefix, f.limit = field, prefix, limit
	return f.out, f.err
}

var viewerA = port.Viewer{UserID: uuid.MustParse("11111111-1111-4111-8111-111111111111")}

func newJQL(s *fakeSearcher, custom port.CustomFieldSource, viewers port.ViewerDirectory) *JQLService {
	return NewJQLService(s, custom, viewers, clock.NewFixed(time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)))
}

func problemsOf(t *testing.T, err error) []domain.Problem {
	t.Helper()
	var qe *domain.QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("not a query error: %v", err)
	}
	return qe.Problems
}

func TestJQLSearchRunsValidatedQuery(t *testing.T) {
	s := &fakeSearcher{page: port.IssuePage{Items: []port.IssueRecord{{Key: "A-1"}}, Total: 1, HasTotal: true}}
	svc := newJQL(s, nil, fakeViewers{tz: "America/Bogota"})
	proj := uuid.New()
	out, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "project = a and status=done order by created", ProjectID: &proj, Page: 2, PageSize: 10, WithTotal: true})
	if err != nil || out.PageNumber != 2 || out.PageSize != 10 || len(out.Page.Items) != 1 || out.Normalized == "" {
		t.Fatalf("%v %+v", err, out)
	}
	if s.last.Viewer.Timezone != "America/Bogota" || s.last.ProjectID == nil || *s.last.ProjectID != proj || s.last.Page != 2 || !s.last.WithTotal || s.last.Now.IsZero() {
		t.Fatalf("query handed to the searcher: %+v", s.last)
	}
	// The viewer's own time zone wins over the directory.
	svc.Search(context.Background(), port.SearchInput{Viewer: port.Viewer{UserID: viewerA.UserID, Timezone: "UTC"}})
	if s.last.Viewer.Timezone != "UTC" {
		t.Fatalf("explicit zone replaced: %q", s.last.Viewer.Timezone)
	}
	// Paging defaults and clamping.
	out, _ = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA})
	if out.PageNumber != 1 || out.PageSize != DefaultSearchPageSize || len(out.Warnings) != 0 {
		t.Fatalf("defaults: %+v", out)
	}
	out, _ = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, PageSize: 5000, Page: -3})
	if out.PageSize != MaxSearchPageSize || out.PageNumber != 1 || len(out.Warnings) != 1 || out.Warnings[0].Code != "search.page_size_clamped" {
		t.Fatalf("clamp: %+v", out)
	}
}

func TestJQLOrderOverride(t *testing.T) {
	s := &fakeSearcher{}
	svc := newJQL(s, nil, nil)
	out, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "labels = a ORDER BY created", Order: "key DESC, updated"})
	if err != nil || len(out.Warnings) != 1 || out.Warnings[0].Code != "search.order_overridden" {
		t.Fatalf("%v %+v", err, out)
	}
	if got := s.last.Query.OrderBy; len(got) != 2 || got[0].Field.Name != "key" || !got[0].Desc {
		t.Fatalf("override not applied: %+v", got)
	}
	// Without an ORDER BY in the query there is no warning.
	out, _ = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "labels = a", Order: "key"})
	if len(out.Warnings) != 0 {
		t.Fatalf("warnings: %+v", out.Warnings)
	}
	for _, bad := range []string{"key DESC garbage", "labels = a", "labels", "nosuch"} {
		if _, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Order: bad}); apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("order %q accepted: %v", bad, err)
		}
	}
	if _, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Order: strings.Repeat("a", 201)}); !errors.Is(err, domain.ErrCoreQLTooLong) {
		t.Fatalf("long order: %v", err)
	}
}

func TestJQLErrorsCarryPositions(t *testing.T) {
	svc := newJQL(&fakeSearcher{}, nil, nil)
	_, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "status = 'á' AND projct = X"})
	p := problemsOf(t, err)
	if len(p) != 1 || p[0].Pos != 17 || p[0].Len != 6 || p[0].Code != domain.CodeUnknownField || len(p[0].Suggestions) == 0 {
		t.Fatalf("%+v", p)
	}
	_, err = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "project ="})
	p = problemsOf(t, err)
	if p[0].Code != domain.CodeSyntaxError || len(p[0].Expected) == 0 {
		t.Fatalf("%+v", p)
	}
	for _, q := range []string{"status WAS done", "status changed"} {
		_, err = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: q})
		if p := problemsOf(t, err); p[0].Code != domain.CodeOperatorUnavailable {
			t.Fatalf("%q: %+v", q, p)
		}
	}
	if _, err = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: strings.Repeat("é", MaxQueryRunes+1)}); !errors.Is(err, domain.ErrCoreQLTooLong) {
		t.Fatalf("length is counted in characters: %v", err)
	}
	if _, err = svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: strings.Repeat("é", MaxQueryRunes)}); apperror.CodeOf(err) == domain.CodeQueryTooLong {
		t.Fatal("4000 characters are allowed")
	}
}

func TestJQLBuildErrorsAndInfraErrors(t *testing.T) {
	s := &fakeSearcher{err: &port.BuildError{Pos: 12, Len: 3, Code: domain.CodeFieldUnavailable, Message: "no disponible"}}
	svc := newJQL(s, nil, nil)
	_, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "labels = a AND key = A-1"})
	if p := problemsOf(t, err); p[0].Pos != 12 || p[0].Code != domain.CodeFieldUnavailable {
		t.Fatalf("%+v", p)
	}
	boom := errors.New("db down")
	s.err = boom
	if _, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA}); !errors.Is(err, boom) {
		t.Fatalf("infrastructure errors pass through: %v", err)
	}
	if _, err := newJQL(s, fakeCustom{err: boom}, nil).Search(context.Background(), port.SearchInput{Viewer: viewerA}); !errors.Is(err, boom) {
		t.Fatalf("custom field error: %v", err)
	}
	if _, err := newJQL(s, nil, fakeViewers{err: boom}).Search(context.Background(), port.SearchInput{Viewer: viewerA}); !errors.Is(err, boom) {
		t.Fatalf("viewer error: %v", err)
	}
}

func TestJQLCheckAndValidate(t *testing.T) {
	s := &fakeSearcher{}
	svc := newJQL(s, nil, nil)
	norm, err := svc.Check(context.Background(), viewerA, "project=a   AND status = done", "", nil)
	if err != nil || norm == "" || !s.last.DryRun {
		t.Fatalf("%v %q dry=%v", err, norm, s.last.DryRun)
	}
	rep, err := svc.Validate(context.Background(), viewerA, "project = a", nil)
	if err != nil || !rep.Valid || rep.Normalized == "" || len(rep.Problems) != 0 {
		t.Fatalf("%v %+v", err, rep)
	}
	rep, err = svc.Validate(context.Background(), viewerA, "projct = a AND stat = b", nil)
	if err != nil || rep.Valid || len(rep.Problems) != 2 {
		t.Fatalf("%v %+v", err, rep)
	}
	s.err = &port.BuildError{Code: domain.CodeScopeRequired, Message: "x"}
	rep, err = svc.Validate(context.Background(), viewerA, "project = currentProject()", nil)
	if err != nil || rep.Valid || rep.Problems[0].Code != domain.CodeScopeRequired {
		t.Fatalf("%v %+v", err, rep)
	}
	if _, err := svc.Validate(context.Background(), viewerA, strings.Repeat("a", 4001), nil); !errors.Is(err, domain.ErrCoreQLTooLong) {
		t.Fatalf("too long: %v", err)
	}
}

func TestCustomCatalog(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	cat := newCustomCatalog([]port.CustomFieldDef{
		{Name: "Cliente", Kind: "text", IDs: []uuid.UUID{a}},
		{Name: "cliente", Kind: "text", IDs: []uuid.UUID{b}},        // same name, same type: merged
		{Name: "Cliente", Kind: "number", IDs: []uuid.UUID{c}},      // same name, other type: ignored
		{Name: "Story points", Kind: "number", IDs: []uuid.UUID{c}}, // whitespace: only cf[...]
		{Name: "", Kind: "text", IDs: []uuid.UUID{c}},
		{Name: "Sin ids", Kind: "text"},
		{Name: "Fecha", Kind: "date", IDs: []uuid.UUID{a}}, {Name: "Hora", Kind: "datetime", IDs: []uuid.UUID{a}},
		{Name: "Sel", Kind: "select", IDs: []uuid.UUID{a}}, {Name: "Multi", Kind: "multiselect", IDs: []uuid.UUID{a}},
		{Name: "Caja", Kind: "checkbox", IDs: []uuid.UUID{a}}, {Name: "Quien", Kind: "user", IDs: []uuid.UUID{a}},
		{Name: "Tags", Kind: "labels", IDs: []uuid.UUID{a}}, {Name: "Parrafo", Kind: "paragraph", IDs: []uuid.UUID{a}},
	})
	if d, ok := cat.Lookup("CF[ cliente ]"); !ok || d.Type.String() != "text" {
		t.Fatalf("%v %v", d, ok)
	}
	if _, ok := cat.Lookup("CLIENTE"); !ok {
		t.Fatal("bare names resolve")
	}
	if _, ok := cat.Lookup("story points"); ok {
		t.Fatal("a name with spaces is not a bare field")
	}
	if d, ok := cat.Lookup("cf[story points]"); !ok || d.Type.String() != "number" || !d.Sortable {
		t.Fatalf("%v %v", d, ok)
	}
	if _, ok := cat.Lookup("cf[" + a.String() + "]"); !ok {
		t.Fatal("fields resolve by id")
	}
	if got := len(cat.refs()["cf[cliente]"].IDs); got != 2 {
		t.Fatalf("merged ids = %d", got)
	}
	types := map[string]string{"Fecha": "date", "Hora": "datetime", "Sel": "enum", "Multi": "enum", "Caja": "bool", "Quien": "user", "Tags": "labels", "Parrafo": "text"}
	for name, want := range types {
		if d, ok := cat.Lookup("cf[" + name + "]"); !ok || d.Type.String() != want {
			t.Errorf("%s: %v %v", name, d, ok)
		}
	}
	if d, _ := cat.Lookup("cf[multi]"); d.Sortable || !d.Multi {
		t.Fatalf("multiselect is multi and not sortable: %+v", d)
	}
	if names := cat.Names(); len(names) != 10 || names[0] != "cf[Cliente]" {
		t.Fatalf("names: %v", names)
	}
}

func TestJQLCustomFieldsInQueries(t *testing.T) {
	id := uuid.New()
	s := &fakeSearcher{}
	svc := newJQL(s, fakeCustom{fields: []port.CustomFieldDef{{Name: "Horas", Kind: "number", IDs: []uuid.UUID{id}}}}, nil)
	if _, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: "cf[Horas] > 3 ORDER BY cf[Horas] DESC"}); err != nil {
		t.Fatal(err)
	}
	if got := s.last.Custom["cf[horas]"]; len(got.IDs) != 1 || got.IDs[0] != id {
		t.Fatalf("custom refs: %+v", s.last.Custom)
	}
	if _, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: `cf[Horas] ~ "x"`}); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("operator not valid for a number field: %v", err)
	}
	if _, err := svc.Search(context.Background(), port.SearchInput{Viewer: viewerA, Query: `cf[Nada] = 1`}); apperror.CodeOf(err) != domain.CodeUnknownField {
		t.Fatalf("unknown custom field: %v", err)
	}
}

func TestJQLSuggest(t *testing.T) {
	vals := &fakeValues{out: []port.ValueSuggestion{{Value: "a@b.c", Label: "A"}}}
	svc := newJQL(&fakeSearcher{}, fakeCustom{fields: []port.CustomFieldDef{{Name: "Cliente", Kind: "text", IDs: []uuid.UUID{uuid.New()}}}}, nil).WithSuggestions(vals)
	out, err := svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Field: "Assignee", Prefix: " al ", Limit: 99})
	if err != nil || len(out.Values) != 1 || vals.field != "assignee" || vals.prefix != "al" || vals.limit != 10 {
		t.Fatalf("%v %+v %+v", err, out, vals)
	}
	if _, err = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Field: "status", Limit: 5}); err != nil || vals.limit != 5 {
		t.Fatalf("limit: %v %d", err, vals.limit)
	}
	if out, err = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Field: "cf[Cliente]"}); err != nil || len(out.Values) != 0 {
		t.Fatalf("custom field values are not suggested: %v %+v", err, out)
	}
	if _, err = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Field: "nope"}); apperror.CodeOf(err) != domain.CodeUnknownField {
		t.Fatalf("unknown field: %v", err)
	}
	vals.err = errors.New("db")
	if _, err = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Field: "status"}); err == nil {
		t.Fatal("source errors pass through")
	}
	// Without a source there are no values.
	if out, err = newJQL(&fakeSearcher{}, nil, nil).Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Field: "status"}); err != nil || len(out.Values) != 0 {
		t.Fatalf("%v %+v", err, out)
	}
	// Completions of the query text, with the cursor at the end or clamped.
	out, err = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Query: "assignee = cur", Cursor: -1})
	if err != nil || len(out.Completions) == 0 || out.Completions[0].Label != "currentUser()" {
		t.Fatalf("%v %+v", err, out)
	}
	if out, _ = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Query: "ass", Cursor: 999}); len(out.Completions) == 0 {
		t.Fatal("cursor beyond the end is clamped")
	}
	if out, _ = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Query: "", Cursor: 0}); len(out.Completions) == 0 || len(out.Completions) > 50 {
		t.Fatalf("field completions: %d", len(out.Completions))
	}
	if _, err = svc.Suggest(context.Background(), port.SuggestInput{Viewer: viewerA, Query: strings.Repeat("a", 4001)}); !errors.Is(err, domain.ErrCoreQLTooLong) {
		t.Fatalf("too long: %v", err)
	}
	if _, err = newJQL(&fakeSearcher{}, fakeCustom{err: errors.New("x")}, nil).Suggest(context.Background(), port.SuggestInput{Viewer: viewerA}); err == nil {
		t.Fatal("custom field error passes through")
	}
}
