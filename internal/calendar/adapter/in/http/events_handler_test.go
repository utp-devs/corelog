package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const secret = "test-secret-test-secret-test-secret"

type stubService struct {
	create   func(port.Viewer, port.EventInput) (*port.EventView, error)
	update   func(uuid.UUID, *time.Time, port.EventPatch) (*port.EventView, error)
	items    func(port.ItemsQuery) (port.ItemsResult, error)
	delErr   error
	lastOcc  *time.Time
	lastItem port.ItemsQuery
	feed     string
	feedErr  error
	revoked  bool
	respond  domain.Response
}

func (s *stubService) Create(_ context.Context, v port.Viewer, in port.EventInput) (*port.EventView, error) {
	return s.create(v, in)
}
func (s *stubService) Get(_ context.Context, _ port.Viewer, id uuid.UUID) (*port.EventView, error) {
	return nil, domain.ErrEventNotFound
}
func (s *stubService) Update(_ context.Context, _ port.Viewer, id uuid.UUID, occ *time.Time, p port.EventPatch) (*port.EventView, error) {
	return s.update(id, occ, p)
}
func (s *stubService) Delete(_ context.Context, _ port.Viewer, _ uuid.UUID, occ *time.Time) error {
	s.lastOcc = occ
	return s.delErr
}
func (s *stubService) Respond(_ context.Context, _ port.Viewer, _ uuid.UUID, r domain.Response) error {
	s.respond = r
	return nil
}
func (s *stubService) Items(_ context.Context, _ port.Viewer, q port.ItemsQuery) (port.ItemsResult, error) {
	s.lastItem = q
	return s.items(q)
}
func (s *stubService) FeedInfo(context.Context, uuid.UUID) (*time.Time, error) { return nil, nil }
func (s *stubService) CreateFeed(context.Context, uuid.UUID) (port.FeedToken, error) {
	return port.FeedToken{Token: "tok-123", CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, nil
}
func (s *stubService) RevokeFeed(context.Context, uuid.UUID) error {
	s.revoked = true
	return nil
}
func (s *stubService) RenderFeed(_ context.Context, token string) (string, error) {
	if token != s.feed {
		return "", domain.ErrFeedNotFound
	}
	return "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", s.feedErr
}

func newRouter(t *testing.T, s *stubService) (chi.Router, string) {
	t.Helper()
	h := NewHandler(nil, secret).WithEvents(s, "https://corelog.example/")
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { h.RegisterRoutes(r) })
	token, err := authtoken.Generate(secret, time.Hour, uuid.New(), "user")
	if err != nil {
		t.Fatal(err)
	}
	return r, token
}

func do(r http.Handler, method, path, token, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func envelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestEventRoutesRequireAuthentication(t *testing.T) {
	r, _ := newRouter(t, &stubService{})
	id := uuid.New().String()
	for _, c := range [][2]string{
		{"GET", "/api/v1/calendar"}, {"POST", "/api/v1/calendar/events"}, {"GET", "/api/v1/calendar/events/" + id}, {"PATCH", "/api/v1/calendar/events/" + id},
		{"DELETE", "/api/v1/calendar/events/" + id}, {"PUT", "/api/v1/calendar/events/" + id + "/attendees/me"}, {"GET", "/api/v1/calendar/events?project_id=" + id},
		{"GET", "/api/v1/me/calendar/feed"}, {"POST", "/api/v1/me/calendar/feed"}, {"DELETE", "/api/v1/me/calendar/feed"},
	} {
		if rec := do(r, c[0], c[1], "", "{}"); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", c[0], c[1], rec.Code)
		}
	}
}

func TestCreateEventMapsRequestAndErrors(t *testing.T) {
	var got port.EventInput
	project := uuid.New()
	s := &stubService{create: func(v port.Viewer, in port.EventInput) (*port.EventView, error) {
		got = in
		e := domain.Event{ID: uuid.New(), OwnerID: v.UserID, OrganizationID: uuid.New(), ProjectID: in.ProjectID, Title: in.Title, StartsAt: in.StartsAt, EndsAt: in.EndsAt,
			Timezone: "UTC", RecurrenceRule: in.RecurrenceRule, Visibility: domain.VisibilityProject, Color: "blue",
			Exceptions: []domain.Exception{{OriginalStart: in.StartsAt, Cancelled: true}},
			Attendees:  []domain.Attendee{{UserID: in.Attendees[0], Name: "Ana", Response: domain.ResponseNeedsAction}}}
		return &port.EventView{Event: e, CanEdit: true, MyResponse: domain.ResponseNeedsAction}, nil
	}}
	r, token := newRouter(t, s)
	guest := uuid.New()
	body := `{"title":"Revisión","starts_at":"2026-12-07T09:00:00-03:00","ends_at":"2026-12-07","recurrence_rule":"FREQ=WEEKLY","visibility":"project","project_id":"` + project.String() + `","attendees":["` + guest.String() + `"]}`
	rec := do(r, "POST", "/api/v1/calendar/events", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !got.StartsAt.Equal(time.Date(2026, 12, 7, 12, 0, 0, 0, time.UTC)) || got.ProjectID == nil || *got.ProjectID != project || got.Attendees[0] != guest || *got.RecurrenceRule != "FREQ=WEEKLY" {
		t.Fatalf("input: %+v", got)
	}
	data := envelope(t, rec)["data"].(map[string]any)
	if data["title"] != "Revisión" || data["can_edit"] != true || data["my_response"] != "needs_action" || data["recurrence_rule"] != "FREQ=WEEKLY" {
		t.Fatalf("data: %v", data)
	}
	if ex := data["recurrence_exceptions"].([]any); len(ex) != 1 || ex[0].(map[string]any)["cancelled"] != true {
		t.Fatalf("exceptions: %v", data["recurrence_exceptions"])
	}
	if att := data["attendees"].([]any); att[0].(map[string]any)["name"] != "Ana" {
		t.Fatalf("attendees: %v", att)
	}

	for name, bad := range map[string]string{
		"unknown field":   `{"title":"x","starts_at":"2026-12-07","ends_at":"2026-12-07","bogus":1}`,
		"trailing data":   `{"title":"x","starts_at":"2026-12-07","ends_at":"2026-12-07"} {}`,
		"bad start":       `{"title":"x","starts_at":"mañana","ends_at":"2026-12-07"}`,
		"bad project id":  `{"title":"x","starts_at":"2026-12-07","ends_at":"2026-12-07","project_id":"nope"}`,
		"bad attendee id": `{"title":"x","starts_at":"2026-12-07","ends_at":"2026-12-07","attendees":["nope"]}`,
		"bad org id":      `{"title":"x","starts_at":"2026-12-07","ends_at":"2026-12-07","organization_id":"nope"}`,
		"not json":        `[`,
	} {
		if rec := do(r, "POST", "/api/v1/calendar/events", token, bad); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", name, rec.Code)
		}
	}
}

func TestEventGetDeleteUpdateRespond(t *testing.T) {
	var patch port.EventPatch
	var occ *time.Time
	s := &stubService{update: func(_ uuid.UUID, o *time.Time, p port.EventPatch) (*port.EventView, error) {
		patch, occ = p, o
		return &port.EventView{Event: domain.Event{ID: uuid.New(), Title: "t", Timezone: "UTC", Visibility: domain.VisibilityPrivate, Color: "blue"}}, nil
	}}
	r, token := newRouter(t, s)
	id := uuid.New().String()
	base := "/api/v1/calendar/events/" + id

	if rec := do(r, "GET", base, token, ""); rec.Code != http.StatusNotFound || envelope(t, rec)["error"].(map[string]any)["code"] != domain.CodeEventNotFound {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	if rec := do(r, "GET", "/api/v1/calendar/events/not-a-uuid", token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d", rec.Code)
	}

	// null clears the rule and the project, absent fields stay untouched.
	if rec := do(r, "PATCH", base+"?occurrence=2026-12-14T09:00:00Z", token, `{"title":"Solo esta","recurrence_rule":null,"project_id":null,"attendees":[]}`); rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body)
	}
	if !patch.RecurrenceRule.Set || patch.RecurrenceRule.Value != nil || !patch.ProjectID.Set || patch.ProjectID.Value != nil || patch.Attendees == nil || *patch.Title != "Solo esta" || patch.Visibility != nil {
		t.Fatalf("patch: %+v", patch)
	}
	if occ == nil || !occ.Equal(time.Date(2026, 12, 14, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("occurrence = %v", occ)
	}
	rule := uuid.New()
	if rec := do(r, "PATCH", base, token, `{"recurrence_rule":"FREQ=DAILY","project_id":"`+rule.String()+`","visibility":"project","starts_at":"2026-12-07T09:00:00Z","ends_at":"2026-12-07T10:00:00Z"}`); rec.Code != http.StatusOK {
		t.Fatalf("patch 2: %d %s", rec.Code, rec.Body)
	}
	if *patch.RecurrenceRule.Value != "FREQ=DAILY" || *patch.ProjectID.Value != rule || *patch.Visibility != domain.VisibilityProject || patch.StartsAt == nil || occ != nil {
		t.Fatalf("patch 2: %+v", patch)
	}
	for _, bad := range []string{`{"recurrence_rule":5}`, `{"project_id":5}`, `{"project_id":"nope"}`, `{"starts_at":"x"}`, `{"ends_at":"x"}`, `{"attendees":["x"]}`, `{"nope":1}`} {
		if rec := do(r, "PATCH", base, token, bad); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("patch %s: %d", bad, rec.Code)
		}
	}
	if rec := do(r, "PATCH", base+"?occurrence=ayer", token, `{}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad occurrence: %d", rec.Code)
	}

	if rec := do(r, "DELETE", base+"?occurrence=2026-12-14T09:00:00Z", token, ""); rec.Code != http.StatusNoContent || s.lastOcc == nil {
		t.Fatalf("delete occurrence: %d", rec.Code)
	}
	if rec := do(r, "DELETE", base+"?occurrence=ayer", token, ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("delete bad occurrence: %d", rec.Code)
	}
	s.delErr = domain.ErrNotAttendee
	if rec := do(r, "DELETE", base, token, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("delete forbidden: %d", rec.Code)
	}

	if rec := do(r, "PUT", base+"/attendees/me", token, `{"response":"accepted"}`); rec.Code != http.StatusOK || s.respond != domain.ResponseAccepted {
		t.Fatalf("respond: %d %s", rec.Code, rec.Body)
	}
	if rec := do(r, "PUT", base+"/attendees/me", token, `{"answer":"yes"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("respond unknown field: %d", rec.Code)
	}
}

func TestItemsQueryParsing(t *testing.T) {
	proj := uuid.New()
	start := time.Date(2026, 12, 7, 0, 0, 0, 0, time.UTC)
	occ := start.Add(9 * time.Hour)
	s := &stubService{items: func(port.ItemsQuery) (port.ItemsResult, error) {
		return port.ItemsResult{Truncated: true, Items: []port.Item{
			{Kind: "issue_due", Title: "Entrega", StartsAt: start, EndsAt: start.AddDate(0, 0, 1), AllDay: true, SourceID: uuid.New(), ProjectID: &proj, ProjectKey: "ABC", Link: "/issues/1", Color: "red"},
			{Kind: "event", Title: "Reunión", StartsAt: occ, EndsAt: occ.Add(time.Hour), SourceID: uuid.New(), Recurring: true, OccurrenceStart: &occ, CanEdit: true, MyResponse: domain.ResponseTentative},
		}}, nil
	}}
	r, token := newRouter(t, s)
	rec := do(r, "GET", "/api/v1/calendar?from=2026-12-01&to=2026-12-31&projects="+proj.String()+"&types=events,issues", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !s.lastItem.From.Equal(day(2026, 12, 1)) || !s.lastItem.To.Equal(day(2027, 1, 1)) || len(s.lastItem.Projects) != 1 || strings.Join(s.lastItem.Kinds, "|") != "events|issues" {
		t.Fatalf("query: %+v", s.lastItem)
	}
	data := envelope(t, rec)["data"].(map[string]any)
	items := data["items"].([]any)
	if data["truncated"] != true || len(items) != 2 {
		t.Fatalf("data: %v", data)
	}
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if first["date"] != "2026-12-07" || first["all_day"] != true || first["project_key"] != "ABC" || first["occurrence_start"] != nil || first["my_response"] != nil {
		t.Fatalf("first: %v", first)
	}
	if second["recurring"] != true || second["occurrence_start"] != "2026-12-07T09:00:00Z" || second["my_response"] != "tentative" || second["can_edit"] != true {
		t.Fatalf("second: %v", second)
	}
	// An RFC 3339 `to` is exclusive and used as is.
	do(r, "GET", "/api/v1/calendar?from=2026-12-01T00:00:00Z&to=2026-12-08T00:00:00Z", token, "")
	if !s.lastItem.To.Equal(day(2026, 12, 8)) {
		t.Fatalf("to = %v", s.lastItem.To)
	}
	for _, q := range []string{"", "?from=x&to=2026-12-31", "?from=2026-12-01&to=x", "?from=2026-12-01&to=2026-12-31&projects=nope"} {
		if rec := do(r, "GET", "/api/v1/calendar"+q, token, ""); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%q: %d", q, rec.Code)
		}
	}
}

func TestFeedEndpoints(t *testing.T) {
	s := &stubService{feed: "good-token-good-token"}
	r, token := newRouter(t, s)
	rec := do(r, "POST", "/api/v1/me/calendar/feed", token, "")
	data := envelope(t, rec)["data"].(map[string]any)
	if rec.Code != http.StatusCreated || data["token"] != "tok-123" || data["url"] != "https://corelog.example/api/v1/calendar/export.ics?token=tok-123" || data["enabled"] != true {
		t.Fatalf("create feed: %d %v", rec.Code, data)
	}
	rec = do(r, "GET", "/api/v1/me/calendar/feed", token, "")
	if data := envelope(t, rec)["data"].(map[string]any); data["enabled"] != false || data["token"] != nil {
		t.Fatalf("feed info: %v", data)
	}
	if rec := do(r, "DELETE", "/api/v1/me/calendar/feed", token, ""); rec.Code != http.StatusNoContent || !s.revoked {
		t.Fatalf("revoke: %d", rec.Code)
	}

	// The ICS endpoint authenticates by token only.
	rec = do(r, "GET", "/api/v1/calendar/export.ics?token=good-token-good-token", "", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") || !strings.HasPrefix(rec.Body.String(), "BEGIN:VCALENDAR") {
		t.Fatalf("export: %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "private, max-age=300" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers: %v", rec.Header())
	}
	etag := rec.Header().Get("ETag")
	if rec := do(r, "GET", "/api/v1/calendar/export.ics?token=good-token-good-token", "", "", "If-None-Match", etag); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Fatalf("conditional: %d", rec.Code)
	}
	if rec := do(r, "GET", "/api/v1/calendar/export.ics?token=wrong", "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("bad token: %d", rec.Code)
	}
	if rec := do(r, "GET", "/api/v1/calendar/export.ics", "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no token: %d", rec.Code)
	}
}

func TestLegacyEventsRouteStillWorksWithoutEventService(t *testing.T) {
	h := NewHandler(nil, secret)
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { h.RegisterRoutes(r) })
	token, _ := authtoken.Generate(secret, time.Hour, uuid.New(), "user")
	if rec := do(r, "GET", "/api/v1/calendar/events?project_id=bad&from=2026-01-01&to=2026-01-02", token, ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("legacy: %d", rec.Code)
	}
	if rec := do(r, "POST", "/api/v1/calendar/events", token, "{}"); rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
		t.Fatalf("v2 route without service: %d", rec.Code)
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
