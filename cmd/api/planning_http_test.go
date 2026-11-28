package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// planningUser adds an organization member without any project role and returns their token and id.
func (e *transitionEnv) planningUser(name string) (string, uuid.UUID) {
	e.t.Helper()
	id := uuid.New()
	e.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
	e.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ('00000000-0000-4000-8000-000000000002', $1, 'member')`, id)
	token, err := authtoken.Generate(transitionTestSecret, time.Hour, id, "user")
	if err != nil {
		e.t.Fatal(err)
	}
	return token, id
}

func (e *transitionEnv) userID(token string) uuid.UUID {
	e.t.Helper()
	claims, err := authtoken.Parse(transitionTestSecret, token)
	if err != nil {
		e.t.Fatal(err)
	}
	return claims.UserID
}

func list(t *testing.T, body map[string]any, key string) []any {
	t.Helper()
	items, ok := tfData(t, body)[key].([]any)
	if !ok {
		t.Fatalf("no %q list in %v", key, body)
	}
	return items
}

// TestDashboardsThroughTheRealRouter checks the contract, the permission matrix and, above all, that a dashboard
// shared with somebody never shows them issues of a project they cannot browse.
func TestDashboardsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	plain, _ := env.planningUser("plain") // organization member without a role in the private project
	issue := env.createIssue(env.lead)
	_ = issue

	if status, _ := c.do(http.MethodGet, "/dashboards", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", status)
	}
	if status, body := c.do(http.MethodPost, "/dashboards", env.outside, map[string]any{"name": "x"}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "dashboard.no_organization" {
		t.Fatalf("outsider create: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodPost, "/dashboards", env.lead, map[string]any{"name": " "}); status != http.StatusUnprocessableEntity {
		t.Fatalf("blank name: %d", status)
	}
	status, body := c.do(http.MethodPost, "/dashboards", env.lead, map[string]any{"name": "Equipo", "layout": "2-1"})
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	d := tfData(t, body)
	id := d["id"].(string)
	if d["layout"] != "2-1" || d["access"] != "owner" || d["can_manage"] != true || len(d["gadgets"].([]any)) != 0 && d["gadgets"] != nil {
		t.Fatalf("created: %v", d)
	}
	if status, body := c.do(http.MethodPost, "/dashboards", env.lead, map[string]any{"name": "equipo"}); status != http.StatusConflict || tfErrorCode(t, body) != "dashboard.name_taken" {
		t.Fatalf("duplicate name: %d %v", status, body)
	}

	// Gadgets: validation and atomic replacement.
	bad := []map[string]any{{"type": "weather"}, {"type": "assigned_to_me", "config": map[string]any{"limit": 99}}, {"type": "text", "column_index": 5}}
	for _, g := range bad {
		if status, _ := c.do(http.MethodPut, "/dashboards/"+id+"/gadgets", env.lead, map[string]any{"gadgets": []any{g}}); status != http.StatusUnprocessableEntity {
			t.Fatalf("invalid gadget %v: %d", g, status)
		}
	}
	status, body = c.do(http.MethodPut, "/dashboards/"+id+"/gadgets", env.lead, map[string]any{"gadgets": []any{
		map[string]any{"type": "issue_statistics", "config": map[string]any{"group_by": "status"}, "column_index": 0},
		map[string]any{"type": "filter_results", "config": map[string]any{"query": "statusCategory != done"}, "column_index": 0},
		map[string]any{"type": "text", "config": map[string]any{"markdown": "hola"}, "column_index": 1},
	}})
	if status != http.StatusOK {
		t.Fatalf("gadgets: %d %v", status, body)
	}
	gadgets := list(t, body, "gadgets")
	if len(gadgets) != 3 {
		t.Fatalf("gadgets = %d", len(gadgets))
	}
	stats, results := gadgets[0].(map[string]any)["id"].(string), gadgets[1].(map[string]any)["id"].(string)

	// Sharing: only the owner, only with members of the organization.
	viewerID := env.userID(env.viewer)
	share := func(token string, shares ...map[string]any) (int, map[string]any) {
		return c.do(http.MethodPut, "/dashboards/"+id+"/shares", token, map[string]any{"shares": shares})
	}
	if status, _ := share(env.viewer, map[string]any{"type": "user", "target_id": viewerID.String()}); status != http.StatusNotFound {
		t.Fatalf("share by a stranger: %d", status)
	}
	if status, body := share(env.lead, map[string]any{"type": "user", "target_id": env.userID(env.outside).String()}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "dashboard.invalid_share" {
		t.Fatalf("share with a user of no organization: %d %v", status, body)
	}
	if status, body := share(env.lead, map[string]any{"type": "user", "target_id": viewerID.String(), "permission": "view"}); status != http.StatusOK {
		t.Fatalf("share with the viewer: %d %v", status, body)
	} else if shares := list(t, body, "shares"); len(shares) != 1 || shares[0].(map[string]any)["target_name"] != "viewer" {
		t.Fatalf("shares: %v", shares)
	}

	// Permission matrix on the shared dashboard.
	if status, body := c.do(http.MethodGet, "/dashboards/"+id, env.viewer, nil); status != http.StatusOK || tfData(t, body)["access"] != "view" || tfData(t, body)["shares"] != nil {
		t.Fatalf("viewer get: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodPatch, "/dashboards/"+id, env.viewer, map[string]any{"name": "x"}); status != http.StatusForbidden {
		t.Fatalf("viewer patch: %d", status)
	}
	if status, _ := c.do(http.MethodDelete, "/dashboards/"+id, env.viewer, nil); status != http.StatusForbidden {
		t.Fatalf("viewer delete: %d", status)
	}
	for _, token := range []string{env.contrib, plain, env.outside} {
		if status, _ := c.do(http.MethodGet, "/dashboards/"+id, token, nil); status != http.StatusNotFound {
			t.Fatalf("unshared reader: %d", status)
		}
		if status, _ := c.do(http.MethodGet, "/dashboards/"+id+"/gadgets/"+stats+"/data", token, nil); status != http.StatusNotFound {
			t.Fatalf("unshared data: %d", status)
		}
	}
	if status, _ := c.do(http.MethodGet, "/dashboards/"+id+"/gadgets/"+uuid.NewString()+"/data", env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("unknown gadget: %d", status)
	}

	// Data: the lead sees the issue of the private project, the viewer too (project member), a plain member of the
	// organization with the dashboard shared with the whole organization sees none of it.
	total := func(token, gadget string) float64 {
		t.Helper()
		status, body := c.do(http.MethodGet, "/dashboards/"+id+"/gadgets/"+gadget+"/data", token, nil)
		if status != http.StatusOK {
			t.Fatalf("data: %d %v", status, body)
		}
		return tfData(t, body)["data"].(map[string]any)["total"].(float64)
	}
	if total(env.lead, stats) != 1 || total(env.lead, results) != 1 || total(env.viewer, stats) != 1 {
		t.Fatalf("members of the project must see the issue")
	}
	if status, _ := share(env.lead, map[string]any{"type": "user", "target_id": viewerID.String()}, map[string]any{"type": "organization", "target_id": uuid.NewString()}); status != http.StatusOK {
		t.Fatalf("organization share: %d", status)
	}
	if got := total(plain, stats); got != 0 {
		t.Fatalf("shared dashboard leaked %v issues of a private project", got)
	}
	if got := total(plain, results); got != 0 {
		t.Fatalf("shared filter results leaked %v issues", got)
	}
	status, body = c.do(http.MethodGet, "/dashboards/"+id+"/gadgets/"+results+"/data", plain, nil)
	if items := tfData(t, body)["data"].(map[string]any)["items"].([]any); status != http.StatusOK || len(items) != 0 {
		t.Fatalf("plain results: %d %v", status, items)
	}

	// List, favourites-free copy and deletion.
	if status, body := c.do(http.MethodGet, "/dashboards?scope=shared", plain, nil); status != http.StatusOK || len(body["data"].([]any)) != 1 || body["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("plain list: %d %v", status, body)
	}
	if status, body := c.do(http.MethodGet, "/dashboards?scope=mine", plain, nil); status != http.StatusOK || len(body["data"].([]any)) != 0 {
		t.Fatalf("plain mine: %d %v", status, body)
	}
	status, body = c.do(http.MethodPost, "/dashboards/"+id+"/duplicate", plain, nil)
	if status != http.StatusCreated || tfData(t, body)["name"] != "Copia de Equipo" || tfData(t, body)["access"] != "owner" || len(list(t, body, "gadgets")) != 3 {
		t.Fatalf("duplicate: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodDelete, "/dashboards/"+id, env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ := c.do(http.MethodGet, "/dashboards/"+id, env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("get after delete: %d", status)
	}
}

// TestDefaultDashboardGadgetsReturnData creates the personal dashboard and fetches the data of every gadget type.
func TestDefaultDashboardGadgetsReturnData(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	env.createIssue(env.contrib)
	status, body := c.do(http.MethodGet, "/me/dashboard", env.contrib, nil)
	if status != http.StatusOK || tfData(t, body)["is_default"] != true || len(list(t, body, "gadgets")) != 6 {
		t.Fatalf("default: %d %v", status, body)
	}
	id := tfData(t, body)["id"].(string)
	if status, body := c.do(http.MethodGet, "/me/dashboard", env.contrib, nil); status != http.StatusOK || tfData(t, body)["id"] != id {
		t.Fatalf("second call: %d", status)
	}
	// Every other gadget type, added with the defaults.
	types := []string{"created_by_me", "filter_results", "issue_statistics", "pie", "two_dimensional", "sprint_burndown", "text", "calendar_upcoming"}
	gadgets := []any{}
	for _, g := range list(t, body, "gadgets") {
		gadgets = append(gadgets, map[string]any{"id": g.(map[string]any)["id"], "type": g.(map[string]any)["type"], "config": g.(map[string]any)["config"], "column_index": g.(map[string]any)["column_index"]})
	}
	for _, ty := range types {
		g := map[string]any{"type": ty, "column_index": 1}
		if ty == "filter_results" {
			g["config"] = map[string]any{"query": "assignee = currentUser()"}
		}
		gadgets = append(gadgets, g)
	}
	if status, body = c.do(http.MethodPut, "/dashboards/"+id+"/gadgets", env.contrib, map[string]any{"gadgets": gadgets}); status != http.StatusOK {
		t.Fatalf("replace: %d %v", status, body)
	}
	for _, g := range list(t, body, "gadgets") {
		gm := g.(map[string]any)
		status, data := c.do(http.MethodGet, "/dashboards/"+id+"/gadgets/"+gm["id"].(string)+"/data", env.contrib, nil)
		if status != http.StatusOK || tfData(t, data)["type"] != gm["type"] || tfData(t, data)["data"] == nil {
			t.Errorf("%s: %d %v", gm["type"], status, data)
		}
	}
	// A saved filter that does not exist is a clean 404, an invalid query a 422.
	status, body = c.do(http.MethodPut, "/dashboards/"+id+"/gadgets", env.contrib, map[string]any{"gadgets": []any{
		map[string]any{"type": "filter_results", "config": map[string]any{"filter_id": uuid.NewString()}},
		map[string]any{"type": "filter_results", "config": map[string]any{"query": "nonsense ==="}},
	}})
	if status != http.StatusOK {
		t.Fatalf("filters: %d %v", status, body)
	}
	for _, g := range list(t, body, "gadgets") {
		status, data := c.do(http.MethodGet, "/dashboards/"+id+"/gadgets/"+g.(map[string]any)["id"].(string)+"/data", env.contrib, nil)
		if status != http.StatusNotFound && status != http.StatusUnprocessableEntity {
			t.Errorf("broken filter gadget: %d %v", status, data)
		}
	}
}

// TestCalendarEventsThroughTheRealRouter covers creation, visibility, recurrence, one-occurrence edits, RSVP and the
// private ICS feed.
func TestCalendarEventsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	contribID := env.userID(env.contrib)

	if status, _ := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", status)
	}
	event := map[string]any{
		"title": "Revisión semanal; con, comas", "description": "Línea 1\nLínea 2", "starts_at": "2026-12-07T09:00:00Z", "ends_at": "2026-12-07T10:00:00Z",
		"timezone": "UTC", "recurrence_rule": "FREQ=WEEKLY;BYDAY=MO;COUNT=4", "attendees": []string{contribID.String()},
	}
	status, body := c.do(http.MethodPost, "/calendar/events", env.lead, event)
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	created := tfData(t, body)
	id := created["id"].(string)
	if created["can_edit"] != true || created["recurrence_rule"] != "FREQ=WEEKLY;BYDAY=MO;COUNT=4" || created["visibility"] != "private" || len(created["attendees"].([]any)) != 1 {
		t.Fatalf("created: %v", created)
	}
	for name, mutate := range map[string]func(map[string]any){
		"bad rule":        func(m map[string]any) { m["recurrence_rule"] = "FREQ=HOURLY" },
		"bad timezone":    func(m map[string]any) { m["timezone"] = "Mars/Base" },
		"outsider guest":  func(m map[string]any) { m["attendees"] = []string{env.userID(env.outside).String()} },
		"organization":    func(m map[string]any) { m["visibility"] = "organization" },
		"project no edit": func(m map[string]any) { m["visibility"] = "project"; m["project_id"] = env.project.String() },
	} {
		copyEvent := map[string]any{}
		for k, v := range event {
			copyEvent[k] = v
		}
		mutate(copyEvent)
		token := env.lead
		if name == "project no edit" {
			token = env.viewer
		}
		status, _ := c.do(http.MethodPost, "/calendar/events", token, copyEvent)
		want := http.StatusUnprocessableEntity
		if name == "organization" || name == "project no edit" {
			want = http.StatusForbidden
		}
		if status != want {
			t.Errorf("%s: %d, want %d", name, status, want)
		}
	}

	// Visibility: the guest sees it, the viewer of the project (not invited, private event) gets 404.
	if status, body := c.do(http.MethodGet, "/calendar/events/"+id, env.contrib, nil); status != http.StatusOK || tfData(t, body)["my_response"] != "needs_action" || tfData(t, body)["can_edit"] != false {
		t.Fatalf("guest get: %d %v", status, body)
	}
	if status, body := c.do(http.MethodGet, "/calendar/events/"+id, env.viewer, nil); status != http.StatusNotFound || tfErrorCode(t, body) != "calendar.event_not_found" {
		t.Fatalf("stranger get: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodPatch, "/calendar/events/"+id, env.contrib, map[string]any{"title": "x"}); status != http.StatusForbidden {
		t.Fatalf("guest edit: %d", status)
	}
	if status, _ := c.do(http.MethodPut, "/calendar/events/"+id+"/attendees/me", env.viewer, map[string]any{"response": "accepted"}); status != http.StatusNotFound {
		t.Fatalf("stranger rsvp: %d", status)
	}
	if status, _ := c.do(http.MethodPut, "/calendar/events/"+id+"/attendees/me", env.lead, map[string]any{"response": "accepted"}); status != http.StatusForbidden {
		t.Fatalf("owner that is not a guest rsvp: %d", status)
	}
	if status, _ := c.do(http.MethodPut, "/calendar/events/"+id+"/attendees/me", env.contrib, map[string]any{"response": "maybe"}); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad rsvp: %d", status)
	}
	if status, _ := c.do(http.MethodPut, "/calendar/events/"+id+"/attendees/me", env.contrib, map[string]any{"response": "accepted"}); status != http.StatusOK {
		t.Fatalf("rsvp: %d", status)
	}

	// The merged calendar expands the series (4 Mondays) for the owner and the guest, and shows nothing to others.
	window := "/calendar?from=2026-12-01&to=2027-01-31&types=events"
	status, body = c.do(http.MethodGet, window, env.contrib, nil)
	items := list(t, body, "items")
	if status != http.StatusOK || len(items) != 4 {
		t.Fatalf("guest calendar: %d %v", status, body)
	}
	first := items[0].(map[string]any)
	if first["recurring"] != true || first["occurrence_start"] != "2026-12-07T09:00:00Z" || first["my_response"] != "accepted" || first["date"] != "2026-12-07" {
		t.Fatalf("first item: %v", first)
	}
	if status, body := c.do(http.MethodGet, window, env.viewer, nil); status != http.StatusOK || len(list(t, body, "items")) != 0 {
		t.Fatalf("leak to a stranger: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodGet, "/calendar?from=2026-01-01&to=2027-12-31", env.lead, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("range over a year: %d", status)
	}
	if status, _ := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31&types=birthdays", env.lead, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown type: %d", status)
	}
	if status, _ := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31&projects="+uuid.NewString(), env.lead, nil); status == http.StatusOK {
		t.Fatalf("a project that does not exist was accepted")
	}

	// One occurrence: move the second Monday, cancel the third; the series keeps the rest.
	second, third := "2026-12-14T09:00:00Z", "2026-12-21T09:00:00Z"
	if status, body := c.do(http.MethodPatch, "/calendar/events/"+id+"?occurrence=2026-12-15T09:00:00Z", env.lead, map[string]any{"title": "x"}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "calendar.invalid_occurrence" {
		t.Fatalf("foreign occurrence: %d %v", status, body)
	}
	if status, body := c.do(http.MethodPatch, "/calendar/events/"+id+"?occurrence="+second, env.lead, map[string]any{"title": "Movida", "starts_at": "2026-12-14T16:00:00Z", "ends_at": "2026-12-14T17:00:00Z"}); status != http.StatusOK {
		t.Fatalf("move occurrence: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodDelete, "/calendar/events/"+id+"?occurrence="+third, env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("cancel occurrence: %d", status)
	}
	_, body = c.do(http.MethodGet, window, env.lead, nil)
	items = list(t, body, "items")
	if len(items) != 3 || items[1].(map[string]any)["title"] != "Movida" || items[1].(map[string]any)["starts_at"] != "2026-12-14T16:00:00Z" || items[1].(map[string]any)["occurrence_start"] != second {
		t.Fatalf("after exceptions: %v", items)
	}

	// The private ICS feed.
	status, body = c.do(http.MethodPost, "/me/calendar/feed", env.lead, nil)
	if status != http.StatusCreated {
		t.Fatalf("create feed: %d %v", status, body)
	}
	feed := tfData(t, body)
	token := feed["token"].(string)
	if !strings.HasSuffix(feed["url"].(string), "/api/v1/calendar/export.ics?token="+token) {
		t.Fatalf("feed url: %v", feed["url"])
	}
	if status, body := c.do(http.MethodGet, "/me/calendar/feed", env.lead, nil); status != http.StatusOK || tfData(t, body)["enabled"] != true || tfData(t, body)["token"] != nil {
		t.Fatalf("feed info must not repeat the token: %d %v", status, body)
	}
	ics := func(token string, headers ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/calendar/export.ics?token="+token, nil)
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		rec := httptest.NewRecorder()
		c.router.ServeHTTP(rec, req)
		return rec
	}
	rec := ics(token)
	text := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") || !strings.HasPrefix(text, "BEGIN:VCALENDAR\r\n") || !strings.HasSuffix(text, "END:VCALENDAR\r\n") {
		t.Fatalf("ics: %d %v %q", rec.Code, rec.Header(), text)
	}
	for _, want := range []string{"RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=4\r\n", "EXDATE:20261221T090000Z\r\n", "RECURRENCE-ID:20261214T090000Z\r\n", `SUMMARY:Revisión semanal\; con\, comas`, `DESCRIPTION:Línea 1\nLínea 2`} {
		if !strings.Contains(text, want) {
			t.Errorf("ics lacks %q\n%s", want, text)
		}
	}
	for _, line := range strings.Split(text, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line over 75 octets: %q", line)
		}
	}
	if again := ics(token, "If-None-Match", rec.Header().Get("ETag")); again.Code != http.StatusNotModified {
		t.Fatalf("conditional: %d", again.Code)
	}
	if rec := ics(strings.Repeat("a", 43)); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token: %d", rec.Code)
	}
	// The guest has no feed of their own, and the lead's feed does not include somebody else's private events.
	token2 := tfData(t, func() map[string]any { _, b := c.do(http.MethodPost, "/me/calendar/feed", env.viewer, nil); return b }())["token"].(string)
	if rec := ics(token2); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "SUMMARY:") {
		t.Fatalf("a stranger's feed shows the private series: %d\n%s", rec.Code, rec.Body)
	}
	// Rotation invalidates the old token; revoking disables the feed.
	_, body = c.do(http.MethodPost, "/me/calendar/feed", env.lead, nil)
	rotated := tfData(t, body)["token"].(string)
	if ics(token).Code != http.StatusNotFound || ics(rotated).Code != http.StatusOK {
		t.Fatal("rotation did not replace the token")
	}
	if status, _ := c.do(http.MethodDelete, "/me/calendar/feed", env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("revoke: %d", status)
	}
	if ics(rotated).Code != http.StatusNotFound {
		t.Fatal("a revoked token still works")
	}
	if status, _ := c.do(http.MethodDelete, "/me/calendar/feed", env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("second revoke: %d", status)
	}

	// Deleting the event removes it from the calendar.
	if status, _ := c.do(http.MethodDelete, "/calendar/events/"+id, env.lead, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ := c.do(http.MethodGet, "/calendar/events/"+id, env.lead, nil); status != http.StatusNotFound {
		t.Fatalf("get after delete: %d", status)
	}
}

// TestCalendarMergesDerivedDatesAndProjectEvents checks the derived dates, the project event permissions and that the
// legacy per-project endpoint keeps working next to the new one.
func TestCalendarMergesDerivedDatesAndProjectEvents(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	issue := env.createIssue(env.contrib)
	env.exec(`UPDATE issues SET due_date = '2026-12-10', start_date = '2026-12-02' WHERE id = $1`, issue)
	env.exec(`INSERT INTO sprints (project_id, name, start_date, end_date) VALUES ($1, 'Sprint X', '2026-12-03', '2026-12-17')`, env.project)
	env.exec(`INSERT INTO versions (project_id, name, release_date) VALUES ($1, 'v2', '2026-12-20')`, env.project)

	status, body := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31&projects="+env.project.String(), env.viewer, nil)
	if status != http.StatusOK {
		t.Fatalf("derived: %d %v", status, body)
	}
	kinds := map[string]string{}
	for _, it := range list(t, body, "items") {
		m := it.(map[string]any)
		kinds[m["kind"].(string)] = m["date"].(string)
		if m["all_day"] != true || m["project_key"] != "TRN" || m["source_id"] == "" || m["link"] == "" {
			t.Fatalf("derived item: %v", m)
		}
	}
	want := map[string]string{"issue_due": "2026-12-10", "issue_start": "2026-12-02", "sprint_start": "2026-12-03", "sprint_end": "2026-12-17", "version_release": "2026-12-20"}
	for k, v := range want {
		if kinds[k] != v {
			t.Errorf("%s = %q, want %q (%v)", k, kinds[k], v, kinds)
		}
	}
	// Without the project filter only the member projects count; the outsider has none and cannot ask for this one.
	if status, body := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31", env.outside, nil); status != http.StatusOK || len(list(t, body, "items")) != 0 {
		t.Fatalf("outsider: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31&projects="+env.project.String(), env.outside, nil); status != http.StatusForbidden && status != http.StatusNotFound {
		t.Fatalf("outsider asking for the project: %d", status)
	}
	if status, body := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31&types=sprints", env.viewer, nil); status != http.StatusOK || len(list(t, body, "items")) != 2 {
		t.Fatalf("types filter: %d %v", status, body)
	}
	// Legacy endpoint.
	if status, body := c.do(http.MethodGet, "/calendar/events?project_id="+env.project.String()+"&from=2026-12-01&to=2026-12-31", env.viewer, nil); status != http.StatusOK || len(body["data"].([]any)) < 3 {
		t.Fatalf("legacy endpoint: %d %v", status, body)
	}

	// Project events: contributors edit, viewers read.
	status, body = c.do(http.MethodPost, "/calendar/events", env.contrib, map[string]any{"title": "Demo", "starts_at": "2026-12-09T10:00:00Z", "ends_at": "2026-12-09T11:00:00Z", "visibility": "project", "project_id": env.project.String()})
	if status != http.StatusCreated {
		t.Fatalf("project event: %d %v", status, body)
	}
	id := tfData(t, body)["id"].(string)
	if status, body := c.do(http.MethodGet, "/calendar/events/"+id, env.viewer, nil); status != http.StatusOK || tfData(t, body)["can_edit"] != false {
		t.Fatalf("viewer read: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodPatch, "/calendar/events/"+id, env.viewer, map[string]any{"title": "x"}); status != http.StatusForbidden {
		t.Fatalf("viewer edit: %d", status)
	}
	if status, _ := c.do(http.MethodPatch, "/calendar/events/"+id, env.lead, map[string]any{"title": "Demo con cliente"}); status != http.StatusOK {
		t.Fatalf("lead edit of a project event: %d", status)
	}
	if status, _ := c.do(http.MethodGet, "/calendar/events/"+id, env.outside, nil); status != http.StatusNotFound {
		t.Fatalf("outsider read: %d", status)
	}
	if status, body := c.do(http.MethodGet, "/calendar?from=2026-12-01&to=2026-12-31&types=event", env.viewer, nil); status != http.StatusOK || len(list(t, body, "items")) != 1 {
		t.Fatalf("project event in the viewer's calendar: %d %v", status, body)
	}
}

// TestRoadmapThroughTheRealRouter checks the derived dates, progress, dependencies, the manual order and permissions.
func TestRoadmapThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	c := env.client
	leadID := env.userID(env.lead)
	insert := func(kind, summary, status string, parent any, start, due any) string {
		id := uuid.NewString()
		env.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, key_number, issue_type_id, priority_id, parent_id, start_date, due_date)
			SELECT $1, $2, $3, '', $4, 'medium', $5, (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2),
			       (SELECT it.id FROM issue_types it WHERE it.organization_id = p.organization_id AND it.legacy_key = $6),
			       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = p.organization_id AND pr.is_default), $7, $8, $9
			FROM projects p WHERE p.id = $2`, id, env.project, summary, status, leadID, kind, parent, start, due)
		return id
	}
	e1 := insert("epic", "Plataforma", "open", nil, nil, nil)
	e2 := insert("epic", "Pagos", "open", nil, "2026-02-01", "2026-03-01")
	insert("epic", "Sin hijas ni fechas", "open", nil, nil, nil)
	insert("story", "h1", "closed", e1, "2026-01-10", "2026-01-20")
	insert("story", "h2", "open", e1, nil, "2026-02-15")
	env.exec(`INSERT INTO issue_links (id, source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, $3, 'blocks', $4)`, uuid.New(), e1, e2, leadID)
	base := "/projects/" + env.project.String() + "/roadmap"

	if status, _ := c.do(http.MethodGet, base, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", status)
	}
	if status, _ := c.do(http.MethodGet, base, env.outside, nil); status != http.StatusForbidden && status != http.StatusNotFound {
		t.Fatalf("outsider: %d", status)
	}
	if status, _ := c.do(http.MethodGet, base+"?group_by=nope", env.viewer, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad group: %d", status)
	}
	status, body := c.do(http.MethodGet, base+"?from=2026-01-01&to=2026-12-31", env.viewer, nil)
	if status != http.StatusOK {
		t.Fatalf("roadmap: %d %v", status, body)
	}
	bars := map[string]map[string]any{}
	for _, b := range list(t, body, "bars") {
		m := b.(map[string]any)
		bars[m["title"].(string)] = m
	}
	plat := bars["Plataforma"]
	progress := plat["progress"].(map[string]any)
	if plat["start"] != "2026-01-10" || plat["end"] != "2026-02-15" || plat["start_source"] != "derived" || plat["end_source"] != "derived" || progress["total"] != float64(2) || progress["done"] != float64(1) || progress["percent"] != float64(50) {
		t.Fatalf("derived bar: %v", plat)
	}
	if pagos := bars["Pagos"]; pagos["start_source"] != "own" || pagos["dated"] != true {
		t.Fatalf("own bar: %v", pagos)
	}
	if undated := bars["Sin hijas ni fechas"]; undated["dated"] != false || undated["start"] != nil {
		t.Fatalf("undated bar: %v", undated)
	}
	if deps := list(t, body, "dependencies"); len(deps) != 1 || deps[0].(map[string]any)["from"] != e1 || deps[0].(map[string]any)["to"] != e2 {
		t.Fatalf("dependencies: %v", deps)
	}
	if status, body := c.do(http.MethodGet, base+"?from=2026-06-01&to=2026-06-30", env.viewer, nil); status != http.StatusOK || len(list(t, body, "bars")) != 1 {
		t.Fatalf("window without dated epics keeps only the undated one: %d %v", status, body)
	}
	if status, body := c.do(http.MethodGet, base+"/dependencies", env.viewer, nil); status != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("dependencies endpoint: %d %v", status, body)
	}

	// Manual order: viewers cannot change it, contributors can; foreign ids are rejected.
	if status, _ := c.do(http.MethodPut, base+"/rank", env.viewer, map[string]any{"epic_ids": []string{e2, e1}}); status != http.StatusForbidden {
		t.Fatalf("viewer rank: %d", status)
	}
	if status, body := c.do(http.MethodPut, base+"/rank", env.contrib, map[string]any{"epic_ids": []string{uuid.NewString()}}); status != http.StatusUnprocessableEntity || tfErrorCode(t, body) != "roadmap.invalid_rank" {
		t.Fatalf("unknown epic: %d %v", status, body)
	}
	if status, _ := c.do(http.MethodPut, base+"/rank", env.contrib, map[string]any{"epic_ids": []string{e2, e1}}); status != http.StatusNoContent {
		t.Fatalf("rank: %d", status)
	}
	_, body = c.do(http.MethodGet, base, env.viewer, nil)
	if first := list(t, body, "bars")[0].(map[string]any); first["id"] != e2 {
		t.Fatalf("manual order not applied: %v", first)
	}
	byVersion, byBody := c.do(http.MethodGet, base+"?group_by=version", env.viewer, nil)
	if byVersion != http.StatusOK || tfData(t, byBody)["group_by"] != "version" {
		t.Fatalf("group by version: %d", byVersion)
	}
}
