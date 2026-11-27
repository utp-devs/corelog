package http_test

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

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	relhttp "github.com/jhonsferg/corelog/internal/release/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/release/application/port"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

const testSecret = "test-secret-with-enough-length-0123456789"

// stubService records the last release input and returns the configured error.
type stubService struct {
	err          error
	lastRelease  port.ReleaseInput
	releaseCalls int
	version      *domain.Version
}

func (s *stubService) List(context.Context, uuid.UUID, string, uuid.UUID, port.ListFilter) ([]*domain.Version, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []*domain.Version{s.version}, nil
}

func (s *stubService) Create(context.Context, uuid.UUID, string, uuid.UUID, port.CreateInput) (*domain.Version, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.version, nil
}

func (s *stubService) Get(context.Context, uuid.UUID, string, uuid.UUID) (*domain.Version, error) {
	return s.version, s.err
}

func (s *stubService) Update(context.Context, uuid.UUID, string, uuid.UUID, domain.VersionPatch) (*domain.Version, error) {
	return s.version, s.err
}

func (s *stubService) Delete(context.Context, uuid.UUID, string, uuid.UUID, *uuid.UUID) error {
	return s.err
}

func (s *stubService) Release(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, in port.ReleaseInput) (*port.ReleaseResult, error) {
	s.releaseCalls++
	s.lastRelease = in
	if s.err != nil {
		return nil, s.err
	}
	return &port.ReleaseResult{Version: s.version, MovedIssues: 2, UnresolvedIssues: 1}, nil
}

func (s *stubService) Archive(context.Context, uuid.UUID, string, uuid.UUID) (*domain.Version, error) {
	return s.version, s.err
}

func (s *stubService) Notes(context.Context, uuid.UUID, string, uuid.UUID) (*domain.ReleaseNotes, error) {
	if s.err != nil {
		return nil, s.err
	}
	notes := domain.BuildReleaseNotes(s.version, nil)
	return &notes, nil
}

type stubProjects struct{ id uuid.UUID }

func (p stubProjects) ResolveProjectKey(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, key string) (uuid.UUID, error) {
	if key != "REL" {
		return uuid.Nil, apperror.WithCode(apperror.KindNotFound, "project.not_found", "no existe")
	}
	return p.id, nil
}

type fixture struct {
	svc     *stubService
	router  http.Handler
	token   string
	project uuid.UUID
	version uuid.UUID
}

func newFixture(t *testing.T, svc *stubService) *fixture {
	t.Helper()
	project := uuid.New()
	version := uuid.New()
	if svc.version == nil {
		v, err := domain.NewVersion(project, "1.0", "", nil, nil, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		v.ID = version
		svc.version = v
	}
	token, err := authtoken.Generate(testSecret, time.Hour, uuid.New(), "user")
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{svc: svc, router: mount(svc, stubProjects{id: project}), token: token, project: project, version: version}
}

func mount(svc port.Service, projects port.ProjectKeys) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		relhttp.NewHandler(svc, projects, testSecret).RegisterRoutes(r)
	})
	return r
}

func (f *fixture) do(t *testing.T, method, path, body string, authorized bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorized {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// dataOf returns the content of the "data" envelope of a success response.
func dataOf(t *testing.T, rec *httptest.ResponseRecorder) []byte {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || envelope.Data == nil {
		t.Fatalf("no data envelope: %s", rec.Body.String())
	}
	return envelope.Data
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error.Code
}

func TestRoutesRequireAuthentication(t *testing.T) {
	f := newFixture(t, &stubService{})
	rec := f.do(t, http.MethodGet, "/api/v1/versions/"+f.version.String(), "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestErrorMappingToStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", domain.ErrVersionNotFound, http.StatusNotFound, "version.not_found"},
		{"forbidden", apperror.New(apperror.KindForbidden, "no"), http.StatusForbidden, ""},
		{"name taken", domain.ErrVersionNameTaken, http.StatusConflict, "version.name_taken"},
		{"invalid transition", domain.ErrVersionTransition, http.StatusConflict, "version.invalid_transition"},
		{"invalid dates", domain.ErrVersionDates, http.StatusUnprocessableEntity, "version.invalid_dates"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, &stubService{err: tc.err})
			rec := f.do(t, http.MethodGet, "/api/v1/versions/"+f.version.String(), "", true)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
			if tc.code != "" && errorCode(t, rec) != tc.code {
				t.Fatalf("code = %q, want %q", errorCode(t, rec), tc.code)
			}
		})
	}
}

func TestCreateRejectsUnknownFieldsAndBadDates(t *testing.T) {
	f := newFixture(t, &stubService{})
	path := "/api/v1/projects/" + f.project.String() + "/versions"
	if rec := f.do(t, http.MethodPost, path, `{"name":"1.0","owner":"x"}`, true); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: %d", rec.Code)
	}
	if rec := f.do(t, http.MethodPost, path, `{"name":"1.0","start_date":"01/02/2026"}`, true); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad date: %d", rec.Code)
	}
	if rec := f.do(t, http.MethodPost, path, `{"name":"1.0"}`, true); rec.Code != http.StatusCreated {
		t.Fatalf("valid create: %d %s", rec.Code, rec.Body.String())
	}
}

func TestProjectKeyResolvesAndUnknownKeyIs404(t *testing.T) {
	f := newFixture(t, &stubService{})
	if rec := f.do(t, http.MethodGet, "/api/v1/projects/REL/versions", "", true); rec.Code != http.StatusOK {
		t.Fatalf("known key: %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodGet, "/api/v1/projects/NOPE/versions", "", true); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown key: %d", rec.Code)
	}
}

func TestListStatusFilterIsValidated(t *testing.T) {
	f := newFixture(t, &stubService{})
	path := "/api/v1/projects/" + f.project.String() + "/versions"
	if rec := f.do(t, http.MethodGet, path+"?status=bogus", "", true); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bogus status: %d", rec.Code)
	}
	if rec := f.do(t, http.MethodGet, path+"?status=released", "", true); rec.Code != http.StatusOK {
		t.Fatalf("released filter: %d", rec.Code)
	}
}

func TestReleaseMoveTargetParsing(t *testing.T) {
	target := uuid.New()
	tests := []struct {
		name     string
		body     string
		status   int
		mode     string
		targetID uuid.UUID
	}{
		{"absent keeps issues", `{}`, http.StatusOK, "none", uuid.Nil},
		{"null keeps issues", `{"move_unresolved_to":null}`, http.StatusOK, "none", uuid.Nil},
		{"new version", `{"move_unresolved_to":"new"}`, http.StatusOK, "new", uuid.Nil},
		{"existing version", `{"move_unresolved_to":"` + target.String() + `"}`, http.StatusOK, "version", target},
		{"garbage target", `{"move_unresolved_to":"later"}`, http.StatusUnprocessableEntity, "", uuid.Nil},
		{"number target", `{"move_unresolved_to":7}`, http.StatusUnprocessableEntity, "", uuid.Nil},
		{"empty body is valid", ``, http.StatusOK, "none", uuid.Nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubService{}
			f := newFixture(t, svc)
			rec := f.do(t, http.MethodPost, "/api/v1/versions/"+f.version.String()+"/release", tc.body, true)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status != http.StatusOK {
				if svc.releaseCalls != 0 {
					t.Fatal("invalid body must not reach the service")
				}
				return
			}
			if string(svc.lastRelease.Move.Mode) != tc.mode || svc.lastRelease.Move.VersionID != tc.targetID {
				t.Fatalf("move = %+v", svc.lastRelease.Move)
			}
		})
	}
}

func TestReleaseResponseCarriesCounts(t *testing.T) {
	f := newFixture(t, &stubService{})
	rec := f.do(t, http.MethodPost, "/api/v1/versions/"+f.version.String()+"/release", `{"release_date":"2026-11-27"}`, true)
	var body struct {
		MovedIssues      int `json:"moved_issues"`
		UnresolvedIssues int `json:"unresolved_issues"`
	}
	if err := json.Unmarshal(dataOf(t, rec), &body); err != nil {
		t.Fatal(err)
	}
	if body.MovedIssues != 2 || body.UnresolvedIssues != 1 {
		t.Fatalf("counts = %+v", body)
	}
}

func TestDeleteRejectsMalformedMoveTo(t *testing.T) {
	f := newFixture(t, &stubService{})
	rec := f.do(t, http.MethodDelete, "/api/v1/versions/"+f.version.String()+"?move_to=nope", "", true)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "version.move_target_invalid" {
		t.Fatalf("status %d code %s", rec.Code, errorCode(t, rec))
	}
}

func TestNotesReturnsJSONGroupsAndMarkdown(t *testing.T) {
	f := newFixture(t, &stubService{})
	rec := f.do(t, http.MethodGet, "/api/v1/versions/"+f.version.String()+"/notes", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(dataOf(t, rec), &body); err != nil {
		t.Fatal(err)
	}
	if groups, ok := body["groups"].([]any); !ok || groups == nil {
		t.Fatalf("groups must be an array, got %#v", body["groups"])
	}
	if markdown, _ := body["markdown"].(string); !strings.HasPrefix(markdown, "# Notas de la versión 1.0") {
		t.Fatalf("markdown = %q", markdown)
	}
}
