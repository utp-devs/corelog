package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

const secret = "test-secret-with-enough-length-0123456789"

var (
	userID  = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	otherID = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	filtID  = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

type fakeLegacy struct{ hits []domain.Hit }

func (f fakeLegacy) Search(_ context.Context, _ uuid.UUID, _ string, _ string, _ int) ([]domain.Hit, error) {
	return f.hits, nil
}

type fakeIssues struct {
	search   func(port.SearchInput) (port.SearchOutput, error)
	validate func(string) (port.ValidationReport, error)
	suggest  func(port.SuggestInput) (port.SuggestOutput, error)
	lastIn   port.SearchInput
	lastSug  port.SuggestInput
}

func (f *fakeIssues) Search(_ context.Context, in port.SearchInput) (port.SearchOutput, error) {
	f.lastIn = in
	return f.search(in)
}

func (f *fakeIssues) Validate(_ context.Context, _ port.Viewer, text string, _ *uuid.UUID) (port.ValidationReport, error) {
	return f.validate(text)
}

func (f *fakeIssues) Suggest(_ context.Context, in port.SuggestInput) (port.SuggestOutput, error) {
	f.lastSug = in
	return f.suggest(in)
}

type fakeFilters struct {
	detail  *port.FilterDetail
	err     error
	views   []port.FilterView
	total   int
	shares  []port.ShareView
	sub     domain.Subscription
	run     port.SearchOutput
	lastQ   port.FilterListQuery
	lastNew domain.FilterFields
	lastOrg uuid.UUID
	patch   port.FilterPatch
	set     []domain.Share
}

func (f *fakeFilters) Create(_ context.Context, _ port.Viewer, org uuid.UUID, fields domain.FilterFields) (*port.FilterDetail, error) {
	f.lastNew, f.lastOrg = fields, org
	return f.detail, f.err
}
func (f *fakeFilters) Get(context.Context, uuid.UUID, uuid.UUID) (*port.FilterDetail, error) {
	return f.detail, f.err
}
func (f *fakeFilters) List(_ context.Context, _ uuid.UUID, q port.FilterListQuery) ([]port.FilterView, int, error) {
	f.lastQ = q
	return f.views, f.total, f.err
}
func (f *fakeFilters) Update(_ context.Context, _ port.Viewer, _ uuid.UUID, p port.FilterPatch) (*port.FilterDetail, error) {
	f.patch = p
	return f.detail, f.err
}
func (f *fakeFilters) Delete(context.Context, uuid.UUID, uuid.UUID) error { return f.err }
func (f *fakeFilters) SetShares(_ context.Context, _, _ uuid.UUID, s []domain.Share) ([]port.ShareView, error) {
	f.set = s
	return f.shares, f.err
}
func (f *fakeFilters) Run(context.Context, port.Viewer, uuid.UUID, int, int, bool) (port.SearchOutput, error) {
	return f.run, f.err
}
func (f *fakeFilters) Subscribe(_ context.Context, _ port.Viewer, _ uuid.UUID, s domain.Schedule) (domain.Subscription, error) {
	f.sub.Schedule = s
	return f.sub, f.err
}
func (f *fakeFilters) Unsubscribe(context.Context, uuid.UUID, uuid.UUID) error { return f.err }
func (f *fakeFilters) DueSubscriptions(context.Context) ([]port.DueSubscription, error) {
	return nil, f.err
}
func (f *fakeFilters) MarkDelivered(context.Context, uuid.UUID) error { return f.err }

type fakeGlobal struct {
	hits  []domain.GlobalHit
	kinds []domain.Kind
	err   error
}

func (f *fakeGlobal) SearchAll(_ context.Context, _ port.Viewer, _ string, kinds []domain.Kind, _ int) ([]domain.GlobalHit, error) {
	f.kinds = kinds
	return f.hits, f.err
}

func token(t *testing.T, role string, id uuid.UUID) string {
	t.Helper()
	tok, err := authtoken.Generate(secret, time.Hour, id, role)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func do(t *testing.T, h *Handler, method, path, body, tok string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { h.RegisterRoutes(r) })
	req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return out
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	e, _ := decode(t, rec)["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func sampleRecord() port.IssueRecord {
	assignee := uuid.New()
	name := "Ana"
	pts := 3.0
	due := "2026-12-01"
	parent := uuid.New()
	return port.IssueRecord{
		ID: uuid.New(), Key: "CORE-1", ProjectID: uuid.New(), ProjectKey: "CORE", Summary: "Resumen", Description: "d", LegacyStatus: "open",
		Status:     &port.IssueRef{ID: uuid.New(), Name: "Por hacer", Category: "todo"},
		IssueType:  &port.IssueRef{ID: uuid.New(), Name: "Error", Icon: "bug", Color: "red", HierarchyLevel: 0},
		Priority:   &port.IssueRef{ID: uuid.New(), Name: "Alta", Icon: "arrow-up", Color: "orange"},
		Resolution: &port.IssueRef{ID: uuid.New(), Name: "Hecho"}, AssigneeID: &assignee, AssigneeName: &name, ParentID: &parent,
		StoryPoints: &pts, DueDate: &due, Labels: []string{"bug"}, Custom: map[string]any{"x": 1}, ReporterID: uuid.New(), ReporterName: "Bob",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

func newHandler() (*Handler, *fakeIssues, *fakeFilters, *fakeGlobal) {
	fi := &fakeIssues{
		search: func(port.SearchInput) (port.SearchOutput, error) {
			return port.SearchOutput{
				Page:       port.IssuePage{Items: []port.IssueRecord{sampleRecord()}, Total: 7, HasTotal: true},
				PageNumber: 2, PageSize: 5, Normalized: "project = CORE", Warnings: []domain.Warning{{Code: "w", Message: "aviso"}},
			}, nil
		},
		validate: func(string) (port.ValidationReport, error) {
			return port.ValidationReport{Valid: true, Normalized: "x = 1"}, nil
		},
		suggest: func(port.SuggestInput) (port.SuggestOutput, error) {
			return port.SuggestOutput{
				Completions: []coreql.Suggestion{{Label: "status", Kind: coreql.SuggestField, Detail: "status"}},
				Values:      []port.ValueSuggestion{{Value: "a", Label: "A", Detail: "d"}},
			}, nil
		},
	}
	ff := &fakeFilters{}
	fg := &fakeGlobal{}
	h := NewHandler(fakeLegacy{hits: []domain.Hit{{Kind: domain.KindProject, ID: "p1", Title: "T", Subtitle: "K", Link: "/p"}}}, secret).
		WithIssueSearch(fi).WithFilters(ff).WithGlobal(fg)
	return h, fi, ff, fg
}

func TestAuthenticationIsRequiredEverywhere(t *testing.T) {
	h, _, _, _ := newHandler()
	id := filtID.String()
	routes := []struct{ method, path string }{
		{"GET", "/search?q=ab"}, {"GET", "/search/issues"}, {"POST", "/search/issues"}, {"POST", "/search/validate"}, {"GET", "/search/suggest"},
		{"GET", "/issues/search"}, {"POST", "/issues/search"}, {"GET", "/filters"}, {"POST", "/filters"}, {"GET", "/filters/" + id},
		{"PATCH", "/filters/" + id}, {"DELETE", "/filters/" + id}, {"PUT", "/filters/" + id + "/shares"}, {"GET", "/filters/" + id + "/issues"},
		{"GET", "/filters/" + id + "/subscription"}, {"POST", "/filters/" + id + "/subscription"}, {"DELETE", "/filters/" + id + "/subscription"},
	}
	for _, rt := range routes {
		if rec := do(t, h, rt.method, rt.path, "{}", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: %d", rt.method, rt.path, rec.Code)
		}
	}
}

func TestOptionalFeaturesAreAbsentUntilEnabled(t *testing.T) {
	h := NewHandler(fakeLegacy{}, secret)
	tok := token(t, "user", userID)
	for _, p := range []string{"/search/validate", "/search/suggest", "/issues/search", "/filters"} {
		if rec := do(t, h, "GET", p, "", tok); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s answered %d without its service", p, rec.Code)
		}
	}
	// The legacy global search keeps its shape.
	rec := do(t, NewHandler(fakeLegacy{hits: []domain.Hit{{Kind: domain.KindIssue, ID: "i", Title: "t", Subtitle: "s", Link: "/l"}}}, secret), "GET", "/search?q=ab", "", tok)
	data, _ := decode(t, rec)["data"].([]any)
	if rec.Code != 200 || len(data) != 1 || data[0].(map[string]any)["kind"] != "issue" {
		t.Fatalf("legacy search: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, NewHandler(fakeLegacy{}, secret), "GET", "/search?q=ab&limit=zz", "", tok); rec.Code != 422 {
		t.Fatalf("legacy bad limit: %d", rec.Code)
	}
}

func TestSearchIssuesShape(t *testing.T) {
	h, fi, _, _ := newHandler()
	tok := token(t, "user", userID)
	rec := do(t, h, "POST", "/issues/search", `{"jql":"project = CORE","order":"key DESC","page":2,"page_size":5,"with_total":false,"project_id":"`+otherID.String()+`"}`, tok)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if fi.lastIn.Query != "project = CORE" || fi.lastIn.Order != "key DESC" || fi.lastIn.Page != 2 || fi.lastIn.WithTotal || fi.lastIn.ProjectID == nil ||
		fi.lastIn.Viewer.UserID != userID || fi.lastIn.Viewer.PlatformAdmin {
		t.Fatalf("input: %+v", fi.lastIn)
	}
	body := decode(t, rec)
	meta := body["meta"].(map[string]any)
	if meta["total"].(float64) != 7 || meta["page"].(float64) != 2 || meta["page_size"].(float64) != 5 || meta["query"] != "project = CORE" || len(meta["warnings"].([]any)) != 1 {
		t.Fatalf("meta: %v", meta)
	}
	item := body["data"].([]any)[0].(map[string]any)
	for _, key := range []string{"id", "key", "project_id", "project_key", "summary", "description", "status", "workflow_status", "issue_type", "priority", "resolution",
		"start_date", "due_date", "story_points", "flagged", "vote_count", "rank", "reporter_id", "reporter_name", "assignee_id", "assignee_name", "labels", "custom", "created_at", "updated_at", "parent_id"} {
		if _, ok := item[key]; !ok {
			t.Errorf("issue lacks %q", key)
		}
	}
	if item["workflow_status"].(map[string]any)["category"] != "todo" || item["issue_type"].(map[string]any)["name"] != "Error" || item["labels"].([]any)[0] != "bug" {
		t.Fatalf("refs: %v", item)
	}
	// An admin token marks the viewer as platform admin.
	do(t, h, "GET", "/search/issues?jql=a&page=3&page_size=7&total=false&fields=summary&project_id="+otherID.String(), "", token(t, "admin", userID))
	if !fi.lastIn.Viewer.PlatformAdmin || fi.lastIn.Page != 3 || fi.lastIn.PageSize != 7 || fi.lastIn.WithTotal {
		t.Fatalf("get input: %+v", fi.lastIn)
	}
	// Total is on by default; fields project the response.
	rec = do(t, h, "GET", "/issues/search?jql=a&fields=summary,due_date", "", tok)
	got := decode(t, rec)["data"].([]any)[0].(map[string]any)
	if len(got) != 4 || got["summary"] == nil || got["id"] == nil || got["key"] == nil || got["due_date"] == nil || !fi.lastIn.WithTotal {
		t.Fatalf("projection: %v", got)
	}
	// Total is omitted when it was not computed.
	fi.search = func(port.SearchInput) (port.SearchOutput, error) {
		return port.SearchOutput{PageNumber: 1, PageSize: 50}, nil
	}
	rec = do(t, h, "GET", "/issues/search", "", tok)
	m := decode(t, rec)["meta"].(map[string]any)
	if _, has := m["total"]; !has || m["total"] != nil {
		t.Fatalf("meta without total: %v", m)
	}
	if data := decode(t, rec)["data"].([]any); len(data) != 0 {
		t.Fatalf("empty page: %v", data)
	}
}

func TestSearchIssuesRejectsBadRequests(t *testing.T) {
	h, fi, _, _ := newHandler()
	tok := token(t, "user", userID)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"unknown body field":  do(t, h, "POST", "/issues/search", `{"jql":"a","nope":1}`, tok),
		"trailing data":       do(t, h, "POST", "/issues/search", `{"jql":"a"} {}`, tok),
		"not json":            do(t, h, "POST", "/issues/search", `jql=a`, tok),
		"bad page":            do(t, h, "GET", "/issues/search?page=x", "", tok),
		"bad page size":       do(t, h, "GET", "/issues/search?page_size=x", "", tok),
		"bad project":         do(t, h, "GET", "/issues/search?project_id=zzz", "", tok),
		"bad project in body": do(t, h, "POST", "/issues/search", `{"project_id":"zzz"}`, tok),
	} {
		if rec.Code != 422 || errCode(t, rec) != "validation.failed" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	// Located problems travel in details.
	fi.search = func(port.SearchInput) (port.SearchOutput, error) {
		return port.SearchOutput{}, domain.NewQueryError([]domain.Problem{{Pos: 4, Len: 3, Code: domain.CodeUnknownField, Message: "campo desconocido 'abc'", Suggestions: []string{"abd"}}})
	}
	rec := do(t, h, "POST", "/issues/search", `{"jql":"x abc"}`, tok)
	body := decode(t, rec)["error"].(map[string]any)
	d := body["details"].([]any)[0].(map[string]any)
	if rec.Code != 422 || body["code"] != domain.CodeUnknownField || d["position"].(float64) != 4 || d["length"].(float64) != 3 || d["suggestions"].([]any)[0] != "abd" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	// A problem without suggestions still renders an array.
	fi.search = func(port.SearchInput) (port.SearchOutput, error) {
		return port.SearchOutput{}, domain.NewQueryError([]domain.Problem{{Code: domain.CodeSyntaxError, Message: "x", Expected: []string{"un campo"}}})
	}
	rec = do(t, h, "POST", "/issues/search", `{}`, tok)
	d = decode(t, rec)["error"].(map[string]any)["details"].([]any)[0].(map[string]any)
	if d["suggestions"] == nil || d["expected"].([]any)[0] != "un campo" {
		t.Fatalf("%s", rec.Body.String())
	}
	// Typed application errors keep their status and code.
	for err, want := range map[error]int{domain.ErrQueryTimeout: 422, domain.ErrPageOutOfRange: 422, apperror.New(apperror.KindInternal, "x"): 500} {
		e := err
		fi.search = func(port.SearchInput) (port.SearchOutput, error) { return port.SearchOutput{}, e }
		if rec := do(t, h, "POST", "/issues/search", `{}`, tok); rec.Code != want {
			t.Errorf("%v: %d", err, rec.Code)
		}
	}
	fi.search = func(port.SearchInput) (port.SearchOutput, error) { return port.SearchOutput{}, domain.ErrQueryTimeout }
	if rec := do(t, h, "GET", "/issues/search", "", tok); errCode(t, rec) != domain.CodeQueryTimeout {
		t.Fatalf("timeout code: %s", rec.Body.String())
	}
}

func TestValidateAndSuggestEndpoints(t *testing.T) {
	h, fi, _, _ := newHandler()
	tok := token(t, "user", userID)
	rec := do(t, h, "POST", "/search/validate", `{"jql":"x = 1","project_id":"`+otherID.String()+`"}`, tok)
	data := decode(t, rec)["data"].(map[string]any)
	if rec.Code != 200 || data["valid"] != true || data["query"] != "x = 1" || len(data["errors"].([]any)) != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	fi.validate = func(string) (port.ValidationReport, error) {
		return port.ValidationReport{Problems: []domain.Problem{{Pos: 1, Len: 2, Code: domain.CodeSyntaxError, Message: "m"}}}, nil
	}
	data = decode(t, do(t, h, "POST", "/search/validate", `{"jql":"x"}`, tok))["data"].(map[string]any)
	if data["valid"] != false || data["errors"].([]any)[0].(map[string]any)["position"].(float64) != 1 {
		t.Fatalf("%v", data)
	}
	for _, body := range []string{`{"jql":"x","other":1}`, `{"project_id":"zzz"}`} {
		if rec := do(t, h, "POST", "/search/validate", body, tok); rec.Code != 422 {
			t.Errorf("%s: %d", body, rec.Code)
		}
	}
	fi.validate = func(string) (port.ValidationReport, error) { return port.ValidationReport{}, domain.ErrCoreQLTooLong }
	if rec := do(t, h, "POST", "/search/validate", `{"jql":"x"}`, tok); rec.Code != 422 || errCode(t, rec) != domain.CodeQueryTooLong {
		t.Fatalf("too long: %d", rec.Code)
	}

	rec = do(t, h, "GET", "/search/suggest?field=status&q=en&limit=5&jql=a&cursor=3", "", tok)
	data = decode(t, rec)["data"].(map[string]any)
	if rec.Code != 200 || fi.lastSug.Field != "status" || fi.lastSug.Prefix != "en" || fi.lastSug.Limit != 5 || fi.lastSug.Cursor != 3 ||
		data["completions"].([]any)[0].(map[string]any)["kind"] != "field" || data["values"].([]any)[0].(map[string]any)["value"] != "a" {
		t.Fatalf("%d %s %+v", rec.Code, rec.Body.String(), fi.lastSug)
	}
	do(t, h, "GET", "/search/suggest?jql=ab", "", tok)
	if fi.lastSug.Cursor != -1 || fi.lastSug.Field != "" {
		t.Fatalf("defaults: %+v", fi.lastSug)
	}
	for _, q := range []string{"cursor=x", "limit=x"} {
		if rec := do(t, h, "GET", "/search/suggest?"+q, "", tok); rec.Code != 422 {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
	fi.suggest = func(port.SuggestInput) (port.SuggestOutput, error) {
		return port.SuggestOutput{}, apperror.WithCode(apperror.KindValidation, domain.CodeUnknownField, "campo")
	}
	if rec := do(t, h, "GET", "/search/suggest?field=zz", "", tok); rec.Code != 422 || errCode(t, rec) != domain.CodeUnknownField {
		t.Fatalf("unknown field: %d", rec.Code)
	}
}

func TestGlobalSearchV2Shape(t *testing.T) {
	h, _, _, fg := newHandler()
	tok := token(t, "user", userID)
	fg.hits = []domain.GlobalHit{
		{Hit: domain.Hit{Kind: domain.KindIssue, ID: "i1", Title: "T", Subtitle: "CORE-1", Link: "/issues/i1"}, Key: "CORE-1", Highlight: "<mark>T</mark>", Snippet: "s"},
		{Hit: domain.Hit{Kind: domain.KindPerson, ID: "u1", Title: "Ana"}, Highlight: "Ana"},
	}
	rec := do(t, h, "GET", "/search?q=ab&types=issue,person", "", tok)
	body := decode(t, rec)
	first := body["data"].([]any)[0].(map[string]any)
	counts := body["meta"].(map[string]any)["counts"].(map[string]any)
	if rec.Code != 200 || first["kind"] != "issue" || first["key"] != "CORE-1" || first["highlight"] != "<mark>T</mark>" || first["snippet"] != "s" ||
		first["subtitle"] != "CORE-1" || first["link"] != "/issues/i1" || counts["issue"].(float64) != 1 || counts["person"].(float64) != 1 {
		t.Fatalf("%s", rec.Body.String())
	}
	if len(fg.kinds) != 2 || fg.kinds[0] != domain.KindIssue || fg.kinds[1] != domain.KindPerson {
		t.Fatalf("kinds: %v", fg.kinds)
	}
	do(t, h, "GET", "/search?q=ab", "", tok)
	if len(fg.kinds) != 2 || fg.kinds[0] != domain.KindProject {
		t.Fatalf("default kinds: %v", fg.kinds)
	}
	fg.err = domain.ErrQueryTooShort
	if rec := do(t, h, "GET", "/search?q=a", "", tok); rec.Code != 422 || errCode(t, rec) != "search.query_too_short" {
		t.Fatalf("short query: %d %s", rec.Code, rec.Body.String())
	}
}

func filterDetail(access domain.Access) *port.FilterDetail {
	sub := domain.Subscription{ID: uuid.New(), FilterID: filtID, Schedule: domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9, Timezone: "UTC"}, CreatedAt: time.Now()}
	return &port.FilterDetail{
		FilterView: port.FilterView{
			Filter:    domain.Filter{ID: filtID, OwnerID: userID, OrganizationID: otherID, Name: "F", QueryText: "a = b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OwnerName: "Ana", Access: access, IsFavorite: true, HasSubscription: true,
		},
		Shares:       []port.ShareView{{Share: domain.Share{Type: domain.ShareUser, TargetID: otherID, Permission: domain.PermissionEdit}, TargetName: "Bob"}},
		Subscription: &sub,
	}
}

func TestFilterEndpoints(t *testing.T) {
	h, _, ff, _ := newHandler()
	tok := token(t, "user", userID)
	id := filtID.String()

	ff.detail = filterDetail(domain.AccessOwner)
	rec := do(t, h, "POST", "/filters", `{"name":"F","description":"d","query":"a = b","columns":["key"],"sort":"key","organization_id":"`+otherID.String()+`"}`, tok)
	body := decode(t, rec)["data"].(map[string]any)
	if rec.Code != 201 || ff.lastNew.Name != "F" || ff.lastNew.QueryText != "a = b" || ff.lastNew.Sort != "key" || ff.lastOrg != otherID ||
		body["access"] != "owner" || body["query"] != "a = b" || body["owner"].(map[string]any)["name"] != "Ana" || len(body["shares"].([]any)) != 1 ||
		body["subscription"].(map[string]any)["schedule"].(map[string]any)["frequency"] != "daily" || body["is_favorite"] != true || body["is_subscribed"] != true {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{`{"name":"F","nope":1}`, `{"name":"F","organization_id":"zzz"}`} {
		if rec := do(t, h, "POST", "/filters", bad, tok); rec.Code != 422 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}

	// Only the owner receives shares; editors and viewers get their access level.
	ff.detail = filterDetail(domain.AccessEdit)
	body = decode(t, do(t, h, "GET", "/filters/"+id, "", tok))["data"].(map[string]any)
	if body["access"] != "edit" || body["shares"] != nil {
		t.Fatalf("editor view: %v", body)
	}
	ff.detail = filterDetail(domain.AccessView)
	if body = decode(t, do(t, h, "GET", "/filters/"+id, "", tok))["data"].(map[string]any); body["access"] != "view" {
		t.Fatalf("viewer view: %v", body)
	}
	ff.detail.Subscription = nil
	if rec := do(t, h, "GET", "/filters/"+id, "", tok); decode(t, rec)["data"].(map[string]any)["subscription"] != nil {
		t.Fatalf("no subscription: %s", rec.Body.String())
	}

	rec = do(t, h, "PATCH", "/filters/"+id, `{"name":"N","columns":[]}`, tok)
	if rec.Code != 200 || ff.patch.Name == nil || *ff.patch.Name != "N" || ff.patch.Columns == nil || ff.patch.QueryText != nil {
		t.Fatalf("patch: %d %+v", rec.Code, ff.patch)
	}
	if rec := do(t, h, "PATCH", "/filters/"+id, `{"nope":1}`, tok); rec.Code != 422 {
		t.Fatalf("patch strict: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/filters/"+id, "", tok); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", rec.Code, rec.Body.String())
	}

	// Lists carry the page meta.
	ff.views = []port.FilterView{filterDetail(domain.AccessOwner).FilterView}
	ff.total = 9
	rec = do(t, h, "GET", "/filters?scope=shared&q=ab&page=2&page_size=5", "", tok)
	m := decode(t, rec)["meta"].(map[string]any)
	if rec.Code != 200 || ff.lastQ.Scope != port.ScopeShared || ff.lastQ.Text != "ab" || ff.lastQ.Page != 2 || m["total"].(float64) != 9 || m["page"].(float64) != 2 || m["page_size"].(float64) != 5 {
		t.Fatalf("list: %d %s %+v", rec.Code, rec.Body.String(), ff.lastQ)
	}
	rec = do(t, h, "GET", "/filters?page_size=1000&page=0", "", tok)
	if m = decode(t, rec)["meta"].(map[string]any); m["page_size"].(float64) != 50 || m["page"].(float64) != 1 {
		t.Fatalf("list defaults: %v", m)
	}
	for _, q := range []string{"page=x", "page_size=x"} {
		if rec := do(t, h, "GET", "/filters?"+q, "", tok); rec.Code != 422 {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}

	// Shares.
	ff.shares = []port.ShareView{{Share: domain.Share{Type: domain.ShareTeam, TargetID: otherID, Permission: domain.PermissionView}, TargetName: "Backend"}}
	rec = do(t, h, "PUT", "/filters/"+id+"/shares", `{"shares":[{"type":"team","target_id":"`+otherID.String()+`","permission":"view"}]}`, tok)
	row := decode(t, rec)["data"].([]any)[0].(map[string]any)
	if rec.Code != 200 || len(ff.set) != 1 || ff.set[0].Type != domain.ShareTeam || row["target_name"] != "Backend" || row["type"] != "team" {
		t.Fatalf("shares: %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{`{"shares":[{"type":"user","target_id":"zzz"}]}`, `{"shares":[],"x":1}`} {
		if rec := do(t, h, "PUT", "/filters/"+id+"/shares", bad, tok); rec.Code != 422 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}

	// Running a filter.
	ff.run = port.SearchOutput{Page: port.IssuePage{Items: []port.IssueRecord{sampleRecord()}, Total: 1, HasTotal: true}, PageNumber: 1, PageSize: 50}
	rec = do(t, h, "GET", "/filters/"+id+"/issues?fields=summary", "", tok)
	if rec.Code != 200 || len(decode(t, rec)["data"].([]any)[0].(map[string]any)) != 3 {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "GET", "/filters/"+id+"/issues?page=x", "", tok); rec.Code != 422 {
		t.Fatalf("run bad page: %d", rec.Code)
	}

	// Subscriptions.
	ff.sub = domain.Subscription{ID: uuid.New(), FilterID: filtID, CreatedAt: time.Now()}
	rec = do(t, h, "POST", "/filters/"+id+"/subscription", `{"frequency":"weekly","hour":8,"weekday":2,"timezone":"UTC"}`, tok)
	sub := decode(t, rec)["data"].(map[string]any)["schedule"].(map[string]any)
	if rec.Code != 200 || ff.sub.Schedule.Frequency != domain.FrequencyWeekly || ff.sub.Schedule.Weekday != 2 || sub["hour"].(float64) != 8 {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "POST", "/filters/"+id+"/subscription", `{"bad":1}`, tok); rec.Code != 422 {
		t.Fatalf("subscribe strict: %d", rec.Code)
	}
	ff.detail = filterDetail(domain.AccessOwner)
	if rec := do(t, h, "GET", "/filters/"+id+"/subscription", "", tok); rec.Code != 200 {
		t.Fatalf("get subscription: %d", rec.Code)
	}
	ff.detail.Subscription = nil
	if rec := do(t, h, "GET", "/filters/"+id+"/subscription", "", tok); rec.Code != 404 || errCode(t, rec) != domain.CodeSubscriptionNotFound {
		t.Fatalf("missing subscription: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/filters/"+id+"/subscription", "", tok); rec.Code != 204 {
		t.Fatalf("unsubscribe: %d", rec.Code)
	}
}

func TestFilterErrorsMapToHTTP(t *testing.T) {
	h, _, ff, _ := newHandler()
	tok := token(t, "user", userID)
	id := filtID.String()
	cases := map[error]int{
		domain.ErrFilterNotFound: 404, domain.ErrFilterForbidden: 403, domain.ErrFilterNameTaken: 409,
		domain.ErrSubscriptionNotFound:                                                       404,
		apperror.WithCode(apperror.KindValidation, domain.CodeFilterInvalid, "x"):            422,
		domain.NewQueryError([]domain.Problem{{Code: domain.CodeSyntaxError, Message: "x"}}): 422,
	}
	for err, want := range cases {
		ff.err = err
		for _, rt := range []struct{ method, path, body string }{
			{"GET", "/filters/" + id, ""}, {"PATCH", "/filters/" + id, `{}`}, {"DELETE", "/filters/" + id, ""},
			{"PUT", "/filters/" + id + "/shares", `{"shares":[]}`}, {"GET", "/filters/" + id + "/issues", ""},
			{"POST", "/filters/" + id + "/subscription", `{}`}, {"DELETE", "/filters/" + id + "/subscription", ""},
			{"POST", "/filters", `{}`}, {"GET", "/filters", ""}, {"GET", "/filters/" + id + "/subscription", ""},
		} {
			if rec := do(t, h, rt.method, rt.path, rt.body, tok); rec.Code != want {
				t.Errorf("%v on %s %s: %d, want %d", err, rt.method, rt.path, rec.Code, want)
			}
		}
	}
	// A malformed id is a filter that does not exist.
	ff.err = nil
	for _, rt := range []struct{ method, path string }{
		{"GET", "/filters/not-a-uuid"}, {"PATCH", "/filters/not-a-uuid"}, {"DELETE", "/filters/not-a-uuid"}, {"PUT", "/filters/not-a-uuid/shares"},
		{"GET", "/filters/not-a-uuid/issues"}, {"POST", "/filters/not-a-uuid/subscription"}, {"GET", "/filters/not-a-uuid/subscription"}, {"DELETE", "/filters/not-a-uuid/subscription"},
	} {
		if rec := do(t, h, rt.method, rt.path, `{}`, tok); rec.Code != 404 || errCode(t, rec) != domain.CodeFilterNotFound {
			t.Errorf("%s %s: %d", rt.method, rt.path, rec.Code)
		}
	}
	_ = bytes.MinRead
}
