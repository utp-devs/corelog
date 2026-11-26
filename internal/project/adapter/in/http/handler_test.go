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
	orgs    []uuid.UUID // organizaciones recibidas en List
}

func (f *fakeProjects) List(_ context.Context, _ uuid.UUID, _ string, orgID uuid.UUID) ([]*domain.Project, error) {
	f.orgs = append(f.orgs, orgID)
	return []*domain.Project{f.project}, f.err
}

func (f *fakeProjects) Get(context.Context, uuid.UUID, string, uuid.UUID) (*domain.Project, error) {
	return f.project, f.err
}

func (f *fakeProjects) Create(_ context.Context, _, _ uuid.UUID, _, _, _ string, _ domain.ProjectType, _ domain.Visibility) (*domain.Project, error) {
	return f.project, f.err
}

func (f *fakeProjects) AddMember(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, string) (*port.Member, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &port.Member{ProjectID: uuid.New(), UserID: uuid.New(), Role: "viewer", CreatedAt: time.Now()}, nil
}

func sampleProject(t *testing.T) *domain.Project {
	t.Helper()
	p, err := domain.NewProject(uuid.New(), "CORE", "Core", "Plataforma", domain.ProjectTypeKanban, domain.VisibilityOpen, uuid.New())
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
	service := &fakeProjects{project: project}
	h := projectRouter(service)
	auth := bearer(t, "requester")
	orgQuery := "/projects?organization_id=" + project.OrganizationID.String()

	got := send(h, http.MethodGet, orgQuery, "", auth)
	if got.Code != http.StatusOK {
		t.Fatalf("list = %d", got.Code)
	}
	if !strings.Contains(got.Body.String(), `"organization_id":"`+project.OrganizationID.String()+`"`) {
		t.Fatalf("list response lacks organization_id: %s", got.Body.String())
	}
	if len(service.orgs) != 1 || service.orgs[0] != project.OrganizationID {
		t.Fatalf("list did not forward organization_id: %v", service.orgs)
	}
	if got := send(h, http.MethodGet, "/projects/"+project.ID.String(), "", auth); got.Code != http.StatusOK {
		t.Fatalf("get = %d", got.Code)
	}
	if got := send(h, http.MethodGet, "/projects", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("list without organization_id = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodGet, "/projects?organization_id=nope", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("list with invalid organization_id = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodGet, orgQuery, "", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", got.Code)
	}
}

func TestProjectHTTPCreateTakesOrganizationAndValidBody(t *testing.T) {
	project := sampleProject(t)
	h := projectRouter(&fakeProjects{project: project})
	body := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core","description":"x","type":"kanban","visibility":"open"}`

	if got := send(h, http.MethodPost, "/projects", body, bearer(t, "user")); got.Code != http.StatusCreated {
		t.Fatalf("org member create = %d, want 201; body %s", got.Code, got.Body.String())
	}
	// visibility is optional: the org default applies.
	noVisibility := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core","type":"scrum"}`
	if got := send(h, http.MethodPost, "/projects", noVisibility, bearer(t, "user")); got.Code != http.StatusCreated {
		t.Fatalf("create without visibility = %d, want 201", got.Code)
	}
	bad := `{"organization_id":"nope","key":"","name":"x","type":"waterfall","visibility":"public"}`
	if got := send(h, http.MethodPost, "/projects", bad, bearer(t, "user")); got.Code != http.StatusUnprocessableEntity {
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
