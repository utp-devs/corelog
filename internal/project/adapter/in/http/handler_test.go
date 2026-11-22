package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

const testSecret = "test-secret"

// fakeProjects implementa el servicio de proyectos con respuestas fijas y errores configurables.
type fakeProjects struct {
	port.Service
	project *domain.Project
	err     error
}

func (f *fakeProjects) List(context.Context, uuid.UUID, string) ([]*domain.Project, error) {
	return []*domain.Project{f.project}, f.err
}

func (f *fakeProjects) Get(context.Context, uuid.UUID, string, uuid.UUID) (*domain.Project, error) {
	return f.project, f.err
}

func (f *fakeProjects) Create(_ context.Context, key, name, description string, projectType domain.ProjectType, leadID uuid.UUID, visibility domain.Visibility) (*domain.Project, error) {
	return f.project, f.err
}

func (f *fakeProjects) AddMember(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, string) (*port.Member, error) {
	return &port.Member{ProjectID: uuid.New(), UserID: uuid.New(), Role: "viewer", CreatedAt: time.Now()}, f.err
}

func sampleProject(t *testing.T) *domain.Project {
	t.Helper()
	p, err := domain.NewProject("CORE", "Core", "Plataforma", domain.ProjectTypeKanban, uuid.New(), domain.VisibilityTeam)
	if err != nil {
		t.Fatal(err)
	}
	p.ID = uuid.New()
	return p
}

func projectRouter(service port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, testSecret).RegisterRoutes(r)
	return r
}

func bearer(t *testing.T, role string) string {
	t.Helper()
	value, err := authtoken.Generate(testSecret, time.Hour, uuid.New(), role)
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + value
}

func send(h http.Handler, method, path, body, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

func TestProjectHTTPReadsAndErrors(t *testing.T) {
	project := sampleProject(t)
	h := projectRouter(&fakeProjects{project: project})
	auth := bearer(t, "requester")

	if got := send(h, http.MethodGet, "/projects", "", auth); got.Code != http.StatusOK {
		t.Fatalf("list = %d", got.Code)
	}
	if got := send(h, http.MethodGet, "/projects/"+project.ID.String(), "", auth); got.Code != http.StatusOK {
		t.Fatalf("get = %d", got.Code)
	}
	if got := send(h, http.MethodGet, "/projects", "", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", got.Code)
	}

	denied := projectRouter(&fakeProjects{project: project, err: apperror.New(apperror.KindForbidden, "forbidden project operation")})
	if got := send(denied, http.MethodGet, "/projects/"+project.ID.String(), "", auth); got.Code != http.StatusForbidden {
		t.Fatalf("non member get = %d, want 403", got.Code)
	}
}

func TestProjectHTTPCreateNeedsAdminAndValidBody(t *testing.T) {
	project := sampleProject(t)
	h := projectRouter(&fakeProjects{project: project})
	body := `{"key":"CORE","name":"Core","description":"x","type":"kanban","lead_id":"` + uuid.New().String() + `","visibility":"team"}`

	if got := send(h, http.MethodPost, "/projects", body, bearer(t, "agent")); got.Code != http.StatusForbidden {
		t.Fatalf("non admin create = %d, want 403", got.Code)
	}
	if got := send(h, http.MethodPost, "/projects", body, bearer(t, "admin")); got.Code != http.StatusCreated {
		t.Fatalf("admin create = %d, want 201; body %s", got.Code, got.Body.String())
	}
	bad := `{"key":"","name":"x","type":"waterfall","lead_id":"nope","visibility":"public"}`
	if got := send(h, http.MethodPost, "/projects", bad, bearer(t, "admin")); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid body = %d, want 422", got.Code)
	}
}

func TestProjectHTTPAddMemberValidatesRole(t *testing.T) {
	project := sampleProject(t)
	h := projectRouter(&fakeProjects{project: project})
	path := "/projects/" + project.ID.String() + "/members"
	valid := `{"user_id":"` + uuid.New().String() + `","role":"viewer"}`

	if got := send(h, http.MethodPost, path, valid, bearer(t, "requester")); got.Code != http.StatusCreated {
		t.Fatalf("add member = %d, want 201; body %s", got.Code, got.Body.String())
	}
	invalid := `{"user_id":"` + uuid.New().String() + `","role":"owner"}`
	if got := send(h, http.MethodPost, path, invalid, bearer(t, "requester")); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("role outside lead/contributor/viewer = %d, want 422", got.Code)
	}
}
