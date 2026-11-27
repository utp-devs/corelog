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

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

const testSecret = "test-secret"

// fakeService records the last patch and returns canned values.
type fakeService struct {
	port.Service
	err       error
	note      domain.Note
	lastPatch domain.Patch
	lastQuery port.ListQuery
	lastPin   *bool
	page      port.ListPage
}

func (f *fakeService) Create(_ context.Context, _ uuid.UUID, fields domain.Fields) (domain.Note, error) {
	if f.err != nil {
		return domain.Note{}, f.err
	}
	return domain.Note{ID: f.note.ID, Title: fields.Title, Color: "yellow", Tags: fields.Tags}, nil
}

func (f *fakeService) Get(context.Context, uuid.UUID, uuid.UUID) (domain.Note, error) {
	return f.note, f.err
}

func (f *fakeService) Update(_ context.Context, _, _ uuid.UUID, patch domain.Patch) (domain.Note, error) {
	f.lastPatch = patch
	return f.note, f.err
}

func (f *fakeService) Delete(context.Context, uuid.UUID, uuid.UUID) error { return f.err }

func (f *fakeService) SetPinned(_ context.Context, _, _ uuid.UUID, pinned bool) (domain.Note, error) {
	f.lastPin = &pinned
	n := f.note
	n.Pinned = pinned
	return n, f.err
}

func (f *fakeService) List(_ context.Context, _ uuid.UUID, q port.ListQuery) (port.ListPage, error) {
	f.lastQuery = q
	return f.page, f.err
}

func (f *fakeService) ListTags(context.Context, uuid.UUID) ([]port.TagCount, error) {
	return []port.TagCount{{Tag: "work", Count: 2}}, f.err
}

func noteRouter(svc port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(svc, testSecret).RegisterRoutes(r)
	return r
}

func bearer(t *testing.T) string {
	t.Helper()
	token, err := authtoken.Generate(testSecret, time.Hour, uuid.New(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + token
}

func send(h http.Handler, method, path, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestNotesRequireAuthentication(t *testing.T) {
	h := noteRouter(&fakeService{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/notes"},
		{http.MethodPost, "/notes"},
		{http.MethodGet, "/notes/tags"},
		{http.MethodGet, "/notes/" + uuid.NewString()},
		{http.MethodPatch, "/notes/" + uuid.NewString()},
		{http.MethodDelete, "/notes/" + uuid.NewString()},
		{http.MethodPost, "/notes/" + uuid.NewString() + "/pin"},
	} {
		rec := send(h, tc.method, tc.path, "", "{}")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without token: %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestCreateReturnsCreatedNote(t *testing.T) {
	h := noteRouter(&fakeService{})
	rec := send(h, http.MethodPost, "/notes", bearer(t), `{"title":"Hola","tags":["a"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data noteResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Title != "Hola" || body.Data.Tags[0] != "a" {
		t.Fatalf("unexpected body %+v", body.Data)
	}
}

func TestCreateRejectsBadJSON(t *testing.T) {
	h := noteRouter(&fakeService{})
	auth := bearer(t)
	for name, body := range map[string]string{
		"unknown field": `{"title":"x","owner_id":"abc"}`,
		"not json":      `{"title":`,
		"trailing data": `{"title":"x"} {"title":"y"}`,
		"bad issue id":  `{"issue_id":"nope"}`,
	} {
		rec := send(h, http.MethodPost, "/notes", auth, body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d", name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"note.invalid"`) {
			t.Fatalf("%s: missing code: %s", name, rec.Body.String())
		}
	}
}

func TestCreateMapsLinkForbiddenTo404(t *testing.T) {
	h := noteRouter(&fakeService{err: domain.ErrLinkForbidden})
	rec := send(h, http.MethodPost, "/notes", bearer(t), `{"issue_id":"`+uuid.NewString()+`"}`)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "note.link_forbidden") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetOtherUsersNoteIsNotFound(t *testing.T) {
	h := noteRouter(&fakeService{err: domain.ErrNotFound})
	rec := send(h, http.MethodGet, "/notes/"+uuid.NewString(), bearer(t), "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "note.not_found") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMalformedIDIsNotFound(t *testing.T) {
	h := noteRouter(&fakeService{})
	rec := send(h, http.MethodGet, "/notes/not-a-uuid", bearer(t), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUpdateDistinguishesAbsentAndNullLinks(t *testing.T) {
	svc := &fakeService{note: domain.Note{ID: uuid.New()}}
	h := noteRouter(svc)
	auth := bearer(t)

	rec := send(h, http.MethodPatch, "/notes/"+svc.note.ID.String(), auth, `{"title":"new"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastPatch.IssueID != nil || svc.lastPatch.ClearIssue || svc.lastPatch.Title == nil {
		t.Fatalf("absent link must stay untouched: %+v", svc.lastPatch)
	}

	send(h, http.MethodPatch, "/notes/"+svc.note.ID.String(), auth, `{"issue_id":null,"project_id":""}`)
	if !svc.lastPatch.ClearIssue || !svc.lastPatch.ClearProject {
		t.Fatalf("null or empty link must clear: %+v", svc.lastPatch)
	}

	issue := uuid.New()
	send(h, http.MethodPatch, "/notes/"+svc.note.ID.String(), auth, `{"issue_id":"`+issue.String()+`"}`)
	if svc.lastPatch.IssueID == nil || *svc.lastPatch.IssueID != issue {
		t.Fatalf("issue link not passed: %+v", svc.lastPatch)
	}
}

func TestUpdateRejectsUnknownFields(t *testing.T) {
	h := noteRouter(&fakeService{note: domain.Note{ID: uuid.New()}})
	rec := send(h, http.MethodPatch, "/notes/"+uuid.NewString(), bearer(t), `{"pinned":true}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("pinned must be set through pin routes: status %d", rec.Code)
	}
}

func TestDeleteReturns204(t *testing.T) {
	h := noteRouter(&fakeService{})
	rec := send(h, http.MethodDelete, "/notes/"+uuid.NewString(), bearer(t), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	h = noteRouter(&fakeService{err: domain.ErrNotFound})
	rec = send(h, http.MethodDelete, "/notes/"+uuid.NewString(), bearer(t), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing note: status %d", rec.Code)
	}
}

func TestPinAndUnpinReturnTheNote(t *testing.T) {
	svc := &fakeService{note: domain.Note{ID: uuid.New()}}
	h := noteRouter(svc)
	auth := bearer(t)
	rec := send(h, http.MethodPost, "/notes/"+svc.note.ID.String()+"/pin", auth, "")
	if rec.Code != http.StatusOK || svc.lastPin == nil || !*svc.lastPin {
		t.Fatalf("pin: status %d", rec.Code)
	}
	rec = send(h, http.MethodPost, "/notes/"+svc.note.ID.String()+"/unpin", auth, "")
	if rec.Code != http.StatusOK || *svc.lastPin {
		t.Fatalf("unpin: status %d", rec.Code)
	}
}

func TestListParsesFiltersAndReturnsCursorMeta(t *testing.T) {
	next := port.Position{Pinned: true, UpdatedAt: time.Now().UTC(), ID: uuid.New()}
	cursor, err := pagination.EncodeCursor(next)
	if err != nil {
		t.Fatal(err)
	}
	svc := &fakeService{page: port.ListPage{Items: []domain.Note{{ID: uuid.New(), Tags: nil}}, NextCursor: "next"}}
	h := noteRouter(svc)
	rec := send(h, http.MethodGet, "/notes?q=x&tag=Work&pinned=true&limit=10&cursor="+cursor, bearer(t), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastQuery.Query != "x" || svc.lastQuery.Tag != "Work" || svc.lastQuery.Pinned == nil || !*svc.lastQuery.Pinned ||
		svc.lastQuery.Limit != 10 || svc.lastQuery.After == nil || svc.lastQuery.After.ID != next.ID {
		t.Fatalf("query not parsed: %+v", svc.lastQuery)
	}
	var body struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Limit      int     `json:"limit"`
			NextCursor *string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.NextCursor == nil || *body.Meta.NextCursor != "next" || len(body.Data) != 1 {
		t.Fatalf("unexpected page: %s", rec.Body.String())
	}
	if tags, ok := body.Data[0]["tags"].([]any); !ok || tags == nil {
		t.Fatalf("tags must serialize as an array: %s", rec.Body.String())
	}
}

func TestListRejectsInvalidFilters(t *testing.T) {
	h := noteRouter(&fakeService{})
	auth := bearer(t)
	for _, path := range []string{
		"/notes?pinned=maybe",
		"/notes?issue=abc",
		"/notes?project=abc",
		"/notes?limit=ten",
		"/notes?cursor=!!!",
	} {
		rec := send(h, http.MethodGet, path, auth, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
	}
}

func TestTagsEndpoint(t *testing.T) {
	h := noteRouter(&fakeService{})
	rec := send(h, http.MethodGet, "/notes/tags", bearer(t), "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"tag":"work"`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}
