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

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/application/usecase"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

type memSchemeRepo struct {
	rows     map[string]port.StoredScheme
	projects map[uuid.UUID]uuid.UUID
}

func key(org uuid.UUID, project *uuid.UUID) string {
	if project != nil {
		return project.String()
	}
	return "org:" + org.String()
}

func (m *memSchemeRepo) Get(_ context.Context, org uuid.UUID, project *uuid.UUID) (*port.StoredScheme, error) {
	if s, ok := m.rows[key(org, project)]; ok {
		return &s, nil
	}
	return nil, nil
}
func (m *memSchemeRepo) Save(_ context.Context, org uuid.UUID, project *uuid.UUID, s domain.Scheme, _ uuid.UUID, now time.Time) (port.StoredScheme, error) {
	m.rows[key(org, project)] = port.StoredScheme{Scheme: s, UpdatedAt: now}
	return m.rows[key(org, project)], nil
}
func (m *memSchemeRepo) Delete(_ context.Context, org uuid.UUID, project *uuid.UUID) (bool, error) {
	_, ok := m.rows[key(org, project)]
	delete(m.rows, key(org, project))
	return ok, nil
}
func (m *memSchemeRepo) ProjectOrg(_ context.Context, p uuid.UUID) (uuid.UUID, error) {
	return m.projects[p], nil
}

type roleAuth struct{ projectErr, orgMember, orgAdmin error }

func (a roleAuth) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, _ uuid.UUID) error {
	if action != "edit_project" {
		return apperror.New(apperror.KindForbidden, "wrong action "+action)
	}
	return a.projectErr
}
func (a roleAuth) RequireRole(_ context.Context, _, _ uuid.UUID, min string) error {
	if min == "admin" {
		return a.orgAdmin
	}
	return a.orgMember
}

func schemeRouter(auth roleAuth, repo *memSchemeRepo) http.Handler {
	r := chi.NewRouter()
	svc := usecase.NewSchemeService(repo, auth, auth, clock.NewFixed(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)))
	NewHandler(&fakeService{}, testSecret).WithSchemes(svc).RegisterRoutes(r)
	return r
}

func send(t *testing.T, h http.Handler, method, path, body string, user uuid.UUID, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if user != uuid.Nil {
		token, err := authtoken.Generate(testSecret, time.Hour, user, role)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error.Code
}

func TestProjectSchemeEndpointsAndPermissionMatrix(t *testing.T) {
	org, project, user := uuid.New(), uuid.New(), uuid.New()
	path := "/projects/" + project.String() + "/notification-scheme"
	body := `{"rules":[{"event":"issue_created","recipients":[{"type":"watchers"},{"type":"role","role":"contributor"}],"channels":["in_app","email"]}]}`
	newRepo := func() *memSchemeRepo {
		return &memSchemeRepo{rows: map[string]port.StoredScheme{}, projects: map[uuid.UUID]uuid.UUID{project: org}}
	}

	t.Run("lead reads the default, saves, reads back and resets", func(t *testing.T) {
		h := schemeRouter(roleAuth{}, newRepo())
		rec := send(t, h, http.MethodGet, path, "", user, "requester")
		if rec.Code != http.StatusOK {
			t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
		}
		var got struct {
			Data struct {
				Scope     string  `json:"scope"`
				ProjectID *string `json:"project_id"`
				IsDefault bool    `json:"is_default"`
				Rules     []struct {
					Event      string `json:"event"`
					Overridden bool   `json:"overridden"`
					Channels   []string
					Recipients []struct {
						Type string `json:"type"`
						Role string `json:"role"`
					}
				} `json:"rules"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Scope != "project" || got.Data.ProjectID == nil || !got.Data.IsDefault || len(got.Data.Rules) != len(domain.AllEvents) {
			t.Fatalf("default view: %+v", got.Data)
		}
		if got.Data.Rules[1].Event != "issue_assigned" || got.Data.Rules[1].Recipients[0].Type != "assignee" {
			t.Fatalf("default rule: %+v", got.Data.Rules[1])
		}

		rec = send(t, h, http.MethodPut, path, body, user, "requester")
		if rec.Code != http.StatusOK {
			t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		first := got.Data.Rules[0]
		if got.Data.IsDefault || first.Event != "issue_created" || !first.Overridden || len(first.Channels) != 2 || first.Recipients[1].Role != "contributor" {
			t.Fatalf("saved view: %+v", got.Data)
		}
		rec = send(t, h, http.MethodDelete, path, "", user, "requester")
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != http.StatusOK || !got.Data.IsDefault || got.Data.Rules[0].Overridden {
			t.Fatalf("reset: %d %+v", rec.Code, got.Data)
		}
	})

	t.Run("authorization outcomes", func(t *testing.T) {
		cases := map[string]struct {
			auth roleAuth
			want int
		}{
			"edit_project holder": {roleAuth{}, http.StatusOK},
			"visible but denied":  {roleAuth{projectErr: apperror.New(apperror.KindForbidden, "no")}, http.StatusForbidden},
			"hidden project":      {roleAuth{projectErr: apperror.New(apperror.KindNotFound, "no")}, http.StatusNotFound},
		}
		for name, c := range cases {
			for method, payload := range map[string]string{http.MethodGet: "", http.MethodPut: body, http.MethodDelete: ""} {
				h := schemeRouter(c.auth, newRepo())
				if rec := send(t, h, method, path, payload, user, "requester"); rec.Code != c.want {
					t.Errorf("%s %s: %d, want %d", name, method, rec.Code, c.want)
				}
			}
		}
	})

	t.Run("anonymous callers and bad ids", func(t *testing.T) {
		h := schemeRouter(roleAuth{}, newRepo())
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			if rec := send(t, h, method, path, body, uuid.Nil, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s anonymous: %d", method, rec.Code)
			}
		}
		if rec := send(t, h, http.MethodGet, "/projects/not-a-uuid/notification-scheme", "", user, "requester"); rec.Code != http.StatusNotFound {
			t.Errorf("bad id: %d", rec.Code)
		}
		if rec := send(t, h, http.MethodGet, "/projects/"+uuid.NewString()+"/notification-scheme", "", user, "requester"); rec.Code != http.StatusNotFound {
			t.Errorf("unknown project: %d", rec.Code)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		h := schemeRouter(roleAuth{}, newRepo())
		bad := map[string]string{
			"not json":           `nope`,
			"missing rules":      `{}`,
			"unknown field":      `{"rules":[],"extra":1}`,
			"unknown event":      `{"rules":[{"event":"boom","recipients":[],"channels":[]}]}`,
			"unknown recipient":  `{"rules":[{"event":"issue_created","recipients":[{"type":"boss"}],"channels":["in_app"]}]}`,
			"user without id":    `{"rules":[{"event":"issue_created","recipients":[{"type":"user"}],"channels":["in_app"]}]}`,
			"bad id":             `{"rules":[{"event":"issue_created","recipients":[{"type":"user","id":"zzz"}],"channels":["in_app"]}]}`,
			"unknown channel":    `{"rules":[{"event":"issue_created","recipients":[],"channels":["sms"]}]}`,
			"duplicate event":    `{"rules":[{"event":"issue_created","recipients":[],"channels":[]},{"event":"issue_created","recipients":[],"channels":[]}]}`,
			"role with bad name": `{"rules":[{"event":"issue_created","recipients":[{"type":"role","role":"Bad Role"}],"channels":["in_app"]}]}`,
		}
		for name, payload := range bad {
			rec := send(t, h, http.MethodPut, path, payload, user, "requester")
			if rec.Code != http.StatusUnprocessableEntity || errCode(t, rec) != domain.CodeSchemeInvalid {
				t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
			}
		}
	})
}

func TestOrganizationSchemeEndpointsAndPermissionMatrix(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	path := "/organizations/" + org.String() + "/notification-scheme"
	body := `{"rules":[{"event":"sprint_started","recipients":[{"type":"project_lead"}],"channels":["email"]}]}`
	forbidden := apperror.New(apperror.KindForbidden, "no")
	hidden := apperror.New(apperror.KindNotFound, "no")
	cases := []struct {
		name   string
		auth   roleAuth
		method string
		want   int
	}{
		{"admin reads", roleAuth{}, http.MethodGet, http.StatusOK},
		{"admin writes", roleAuth{}, http.MethodPut, http.StatusOK},
		{"admin resets", roleAuth{}, http.MethodDelete, http.StatusOK},
		{"member reads", roleAuth{orgAdmin: forbidden}, http.MethodGet, http.StatusOK},
		{"member cannot write", roleAuth{orgAdmin: forbidden}, http.MethodPut, http.StatusForbidden},
		{"member cannot reset", roleAuth{orgAdmin: forbidden}, http.MethodDelete, http.StatusForbidden},
		{"outsider reads", roleAuth{orgMember: hidden, orgAdmin: hidden}, http.MethodGet, http.StatusNotFound},
		{"outsider writes", roleAuth{orgMember: hidden, orgAdmin: hidden}, http.MethodPut, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := schemeRouter(c.auth, &memSchemeRepo{rows: map[string]port.StoredScheme{}, projects: map[uuid.UUID]uuid.UUID{}})
			payload := ""
			if c.method == http.MethodPut {
				payload = body
			}
			if rec := send(t, h, c.method, path, payload, user, "requester"); rec.Code != c.want {
				t.Fatalf("%d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
		})
	}
	t.Run("saved organization rule is inherited and scoped", func(t *testing.T) {
		h := schemeRouter(roleAuth{}, &memSchemeRepo{rows: map[string]port.StoredScheme{}, projects: map[uuid.UUID]uuid.UUID{}})
		if rec := send(t, h, http.MethodPut, path, body, user, "requester"); rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		rec := send(t, h, http.MethodGet, path, "", user, "requester")
		var got struct {
			Data struct {
				Scope     string  `json:"scope"`
				ProjectID *string `json:"project_id"`
				IsDefault bool    `json:"is_default"`
			} `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got.Data.Scope != "organization" || got.Data.ProjectID != nil || got.Data.IsDefault {
			t.Fatalf("view: %+v", got.Data)
		}
		if rec := send(t, h, http.MethodGet, "/organizations/x/notification-scheme", "", user, "requester"); rec.Code != http.StatusNotFound {
			t.Fatalf("bad org id: %d", rec.Code)
		}
	})
}

type fakeOutbox struct {
	entries []port.OutboxEntry
	retried []uuid.UUID
}

func (f *fakeOutbox) List(_ context.Context, status string, limit, offset int) ([]port.OutboxEntry, int, error) {
	if status == "bogus" {
		return nil, 0, apperror.WithCode(apperror.KindValidation, usecase.CodeOutboxStatusInvalid, "estado")
	}
	return f.entries, len(f.entries), nil
}
func (f *fakeOutbox) Retry(_ context.Context, id uuid.UUID) error {
	if len(f.entries) == 0 || f.entries[0].ID != id {
		return apperror.WithCode(apperror.KindConflict, usecase.CodeOutboxNotRetryable, "no")
	}
	f.retried = append(f.retried, id)
	return nil
}

func TestAdminEmailOutboxEndpoints(t *testing.T) {
	user := uuid.New()
	entry := port.OutboxEntry{ID: uuid.New(), UserID: &user, To: "ana.lopez@example.test", Kind: "issue_created", Subject: "Asunto", Status: "dead", Attempts: 6, LastError: "smtp down", CreatedAt: time.Now()}
	outbox := &fakeOutbox{entries: []port.OutboxEntry{entry}}
	r := chi.NewRouter()
	NewHandler(&fakeService{}, testSecret).WithOutboxAdmin(outbox).RegisterRoutes(r)

	if rec := send(t, r, http.MethodGet, "/admin/email-outbox", "", uuid.Nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	if rec := send(t, r, http.MethodGet, "/admin/email-outbox", "", user, "requester"); rec.Code != http.StatusForbidden {
		t.Fatalf("non admin list: %d", rec.Code)
	}
	if rec := send(t, r, http.MethodPost, "/admin/email-outbox/"+entry.ID.String()+"/retry", "", user, "requester"); rec.Code != http.StatusForbidden {
		t.Fatalf("non admin retry: %d", rec.Code)
	}

	rec := send(t, r, http.MethodGet, "/admin/email-outbox?status=dead&page=1&page_size=10", "", user, "admin")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ana.lopez") {
		t.Fatal("the recipient address is exposed in clear")
	}
	var body struct {
		Data []struct {
			ID       string `json:"id"`
			To       string `json:"to"`
			Status   string `json:"status"`
			Attempts int    `json:"attempts"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Data) != 1 || body.Data[0].To != "a***@example.test" || body.Data[0].Status != "dead" || body.Data[0].Attempts != 6 || body.Meta.Total != 1 {
		t.Fatalf("body: %+v", body)
	}
	if rec := send(t, r, http.MethodGet, "/admin/email-outbox?status=bogus", "", user, "admin"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad status: %d", rec.Code)
	}
	if rec := send(t, r, http.MethodGet, "/admin/email-outbox?page=0", "", user, "admin"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad page: %d", rec.Code)
	}
	if rec := send(t, r, http.MethodPost, "/admin/email-outbox/"+entry.ID.String()+"/retry", "", user, "admin"); rec.Code != http.StatusNoContent || len(outbox.retried) != 1 {
		t.Fatalf("retry: %d", rec.Code)
	}
	if rec := send(t, r, http.MethodPost, "/admin/email-outbox/"+uuid.NewString()+"/retry", "", user, "admin"); rec.Code != http.StatusConflict {
		t.Fatalf("retry of a live message: %d", rec.Code)
	}
	if rec := send(t, r, http.MethodPost, "/admin/email-outbox/not-a-uuid/retry", "", user, "admin"); rec.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d", rec.Code)
	}
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{"ana@example.test": "a***@example.test", "@x.test": "***", "nodomain": "***", "": "***"} {
		if got := maskEmail(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestPreferencesAcceptEmailModeAndOwnActions(t *testing.T) {
	h := router(&fakeService{})
	user := uuid.New()
	rec := send(t, h, http.MethodPut, "/notifications/preferences", `{"in_app":true,"email":"daily","notify_own_actions":true}`, user, "requester")
	var body struct {
		Data struct {
			InApp            bool   `json:"in_app"`
			Email            string `json:"email"`
			NotifyOwnActions bool   `json:"notify_own_actions"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || body.Data.Email != "daily" || !body.Data.NotifyOwnActions || !body.Data.InApp {
		t.Fatalf("%d %+v", rec.Code, body)
	}
	rec = send(t, h, http.MethodGet, "/notifications/preferences", "", user, "requester")
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Data.Email != "immediate" {
		t.Fatalf("default email mode %q", body.Data.Email)
	}
}
