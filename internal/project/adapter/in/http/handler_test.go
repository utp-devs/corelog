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

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

const testSecret = "test-secret"

// fakeProjects implements the project service and the project manager with fixed answers. It
// records the arguments of the calls the tests inspect.
type fakeProjects struct {
	port.Service
	port.ProjectManager
	project *domain.Project
	err     error

	listed []*domain.Project // projects returned by ListProjects
	total  int               // total returned by ListProjects; defaults to len(listed)

	orgs        []uuid.UUID // organizations received by ListProjects
	filter      port.ProjectFilter
	createInput port.CreateInput
	createOrg   uuid.UUID
	gotID       uuid.UUID // project id received by Get, Archive, Restore or DeleteProject
	lifecycle   []string  // "archive" or "restore", in call order
	confirm     string    // confirmation received by DeleteProject
	keyChecked  string    // key received by KeyAvailable
	available   bool
	keyErr      error     // error returned by FindByKey
	keyOrg      uuid.UUID // organization received by FindByKey
	keyValue    string    // key received by FindByKey
}

func (f *fakeProjects) FindByKey(_ context.Context, _ uuid.UUID, _ string, orgID uuid.UUID, key string) (*domain.Project, error) {
	f.keyOrg = orgID
	f.keyValue = key
	if f.keyErr != nil {
		return nil, f.keyErr
	}
	return f.project, f.err
}

func (f *fakeProjects) Get(_ context.Context, _ uuid.UUID, _ string, id uuid.UUID) (*domain.Project, error) {
	f.gotID = id
	return f.project, f.err
}

func (f *fakeProjects) AddMember(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, string) (*port.Member, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &port.Member{ProjectID: uuid.New(), UserID: uuid.New(), Role: "viewer", CreatedAt: time.Now()}, nil
}

func (f *fakeProjects) CreateProject(_ context.Context, _, orgID uuid.UUID, input port.CreateInput) (*domain.Project, error) {
	f.createOrg = orgID
	f.createInput = input
	return f.project, f.err
}

func (f *fakeProjects) ListProjects(_ context.Context, _ uuid.UUID, _ string, orgID uuid.UUID, filter port.ProjectFilter) (port.ProjectPage, error) {
	f.orgs = append(f.orgs, orgID)
	f.filter = filter
	if f.err != nil {
		return port.ProjectPage{}, f.err
	}
	total := f.total
	if total == 0 {
		total = len(f.listed)
	}
	return port.ProjectPage{Items: f.listed, Total: total}, nil
}

func (f *fakeProjects) Archive(_ context.Context, _ uuid.UUID, _ string, id uuid.UUID) (*domain.Project, error) {
	f.gotID = id
	f.lifecycle = append(f.lifecycle, "archive")
	return f.project, f.err
}

func (f *fakeProjects) Restore(_ context.Context, _ uuid.UUID, _ string, id uuid.UUID) (*domain.Project, error) {
	f.gotID = id
	f.lifecycle = append(f.lifecycle, "restore")
	return f.project, f.err
}

func (f *fakeProjects) DeleteProject(_ context.Context, _ uuid.UUID, _ string, id uuid.UUID, confirmation string) error {
	f.gotID = id
	f.confirm = confirmation
	return f.err
}

func (f *fakeProjects) KeyAvailable(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, key string) (bool, error) {
	f.keyChecked = key
	return f.available, f.err
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

func projectRouter(service *fakeProjects) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, service, testSecret).RegisterRoutes(r)
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
	service := &fakeProjects{project: project, listed: []*domain.Project{project}}
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

func TestProjectHTTPListMetaAndFiltersPassThrough(t *testing.T) {
	project := sampleProject(t)
	service := &fakeProjects{listed: []*domain.Project{project}, total: 41}
	h := projectRouter(service)
	leadID := uuid.New()
	path := "/projects?organization_id=" + project.OrganizationID.String() +
		"&q=core&template=scrum&lead_id=" + leadID.String() +
		"&starred=true&archived=only&sort=key&desc=true&page=2&page_size=10"

	got := send(h, http.MethodGet, path, "", bearer(t, "user"))
	if got.Code != http.StatusOK {
		t.Fatalf("list = %d; body %s", got.Code, got.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Total    int `json:"total"`
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Meta.Total != 41 || body.Meta.Page != 2 || body.Meta.PageSize != 10 {
		t.Fatalf("body = %s", got.Body.String())
	}
	f := service.filter
	if f.Q != "core" || f.Template != domain.TemplateScrum || f.LeadID == nil || *f.LeadID != leadID ||
		!f.Starred || f.Archived != port.ArchiveFilterOnly || f.Sort != port.SortByKey || !f.Desc ||
		f.Page != 2 || f.PageSize != 10 {
		t.Fatalf("filter not passed through: %+v", f)
	}
}

func TestProjectHTTPListClampsPageSizeAndRejectsBadQuery(t *testing.T) {
	project := sampleProject(t)
	service := &fakeProjects{}
	h := projectRouter(service)
	orgQuery := "organization_id=" + project.OrganizationID.String()

	got := send(h, http.MethodGet, "/projects?"+orgQuery+"&page_size=500", "", bearer(t, "user"))
	if got.Code != http.StatusOK {
		t.Fatalf("list = %d", got.Code)
	}
	if !strings.Contains(got.Body.String(), `"page_size":100`) || !strings.Contains(got.Body.String(), `"page":1`) {
		t.Fatalf("meta not clamped: %s", got.Body.String())
	}
	for _, query := range []string{"page=x", "starred=maybe", "lead_id=nope", "type=waterfall"} {
		if got := send(h, http.MethodGet, "/projects?"+orgQuery+"&"+query, "", bearer(t, "user")); got.Code != http.StatusUnprocessableEntity {
			t.Fatalf("query %q = %d, want 422", query, got.Code)
		}
	}
}

func TestProjectHTTPCreateWithTemplate(t *testing.T) {
	project := sampleProject(t)
	service := &fakeProjects{project: project}
	h := projectRouter(service)
	member := uuid.New()
	team := uuid.New()
	body := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core",` +
		`"template":"bug_tracking","visibility":"open","color":"red","icon":"bug","default_assignee_policy":"lead",` +
		`"member_ids":["` + member.String() + `"],"team_ids":["` + team.String() + `"]}`

	got := send(h, http.MethodPost, "/projects", body, bearer(t, "user"))
	if got.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201; body %s", got.Code, got.Body.String())
	}
	in := service.createInput
	if in.Template != domain.TemplateBugTracking || in.Key != "CORE" || in.Color != "red" || in.Icon != "bug" ||
		in.DefaultAssigneePolicy != domain.AssigneePolicyLead || in.Visibility != domain.VisibilityOpen {
		t.Fatalf("create input = %+v", in)
	}
	if len(in.MemberIDs) != 1 || in.MemberIDs[0] != member || len(in.TeamIDs) != 1 || in.TeamIDs[0] != team {
		t.Fatalf("members or teams not passed: %+v", in)
	}
	if service.createOrg != project.OrganizationID {
		t.Fatalf("organization not passed")
	}
	for _, field := range []string{`"template":`, `"color":`, `"icon":`, `"default_assignee_policy":`, `"archived_at":null`} {
		if !strings.Contains(got.Body.String(), field) {
			t.Fatalf("response lacks %s: %s", field, got.Body.String())
		}
	}

	legacy := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core","type":"scrum"}`
	if got := send(h, http.MethodPost, "/projects", legacy, bearer(t, "user")); got.Code != http.StatusCreated {
		t.Fatalf("legacy create = %d", got.Code)
	}
	if service.createInput.Template != domain.TemplateScrum {
		t.Fatalf("legacy type did not derive scrum template: %s", service.createInput.Template)
	}

	none := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core"}`
	if got := send(h, http.MethodPost, "/projects", none, bearer(t, "user")); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create without template or type = %d, want 422", got.Code)
	}
	badMember := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core","template":"kanban","member_ids":["nope"]}`
	if got := send(h, http.MethodPost, "/projects", badMember, bearer(t, "user")); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid member id = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodPost, "/projects", body, ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("create without token = %d, want 401", got.Code)
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

func TestProjectHTTPArchiveRestoreDelete(t *testing.T) {
	project := sampleProject(t)
	service := &fakeProjects{project: project}
	h := projectRouter(service)
	auth := bearer(t, "user")
	base := "/projects/" + project.ID.String()

	if got := send(h, http.MethodPost, base+"/archive", "", auth); got.Code != http.StatusOK {
		t.Fatalf("archive = %d", got.Code)
	}
	if got := send(h, http.MethodPost, base+"/restore", "", auth); got.Code != http.StatusOK {
		t.Fatalf("restore = %d", got.Code)
	}
	if len(service.lifecycle) != 2 || service.lifecycle[0] != "archive" || service.lifecycle[1] != "restore" {
		t.Fatalf("lifecycle calls = %v", service.lifecycle)
	}

	if got := send(h, http.MethodDelete, base, `{"confirmation":"Core"}`, auth); got.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204; body %s", got.Code, got.Body.String())
	}
	if service.confirm != "Core" || service.gotID != project.ID {
		t.Fatalf("delete did not pass confirmation or id: %q %s", service.confirm, service.gotID)
	}
	if got := send(h, http.MethodDelete, base, "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("delete without body = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodPost, base+"/archive", "", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("archive without token = %d, want 401", got.Code)
	}
}

func TestProjectHTTPDeleteCrossOrganizationIsNotFound(t *testing.T) {
	project := sampleProject(t)
	h := projectRouter(&fakeProjects{err: domain.ErrProjectNotFound})
	got := send(h, http.MethodDelete, "/projects/"+project.ID.String(), `{"confirmation":"Core"}`, bearer(t, "user"))
	if got.Code != http.StatusNotFound {
		t.Fatalf("delete of a project outside the caller organizations = %d, want 404", got.Code)
	}
}

func TestProjectHTTPKeyAvailable(t *testing.T) {
	project := sampleProject(t)
	service := &fakeProjects{available: true}
	h := projectRouter(service)
	auth := bearer(t, "user")
	orgQuery := "organization_id=" + project.OrganizationID.String()

	got := send(h, http.MethodGet, "/projects/key-available?"+orgQuery+"&key=core", "", auth)
	if got.Code != http.StatusOK {
		t.Fatalf("key-available = %d; body %s", got.Code, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), `{"data":{"available":true,"key":"CORE"}}`) {
		t.Fatalf("body = %s", got.Body.String())
	}
	if service.keyChecked != "CORE" {
		t.Fatalf("key not normalized before the check: %q", service.keyChecked)
	}

	service.available = false
	got = send(h, http.MethodGet, "/projects/key-available?"+orgQuery+"&key=CORE", "", auth)
	if !strings.Contains(got.Body.String(), `{"data":{"available":false,"key":"CORE"}}`) {
		t.Fatalf("taken key body = %s", got.Body.String())
	}
	if got := send(h, http.MethodGet, "/projects/key-available?"+orgQuery+"&key=1X", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid key = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodGet, "/projects/key-available?key=CORE", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing organization_id = %d, want 422", got.Code)
	}
}

func TestProjectHTTPKeyOrUUIDResolution(t *testing.T) {
	project := sampleProject(t)
	service := &fakeProjects{project: project, listed: []*domain.Project{project}}
	h := projectRouter(service)
	auth := bearer(t, "user")
	orgQuery := "organization_id=" + project.OrganizationID.String()

	if got := send(h, http.MethodGet, "/projects/core?"+orgQuery, "", auth); got.Code != http.StatusOK {
		t.Fatalf("get by key = %d; body %s", got.Code, got.Body.String())
	}
	if service.gotID != project.ID {
		t.Fatalf("key resolved to %s, want %s", service.gotID, project.ID)
	}

	if got := send(h, http.MethodPost, "/projects/CORE/archive?"+orgQuery, "", auth); got.Code != http.StatusOK {
		t.Fatalf("archive by key = %d", got.Code)
	}

	service.keyErr = domain.ErrProjectNotFound
	if got := send(h, http.MethodGet, "/projects/NOPE?"+orgQuery, "", auth); got.Code != http.StatusNotFound {
		t.Fatalf("unknown key = %d, want 404", got.Code)
	}

	// organization_id is optional: the use case resolves the key among the caller's projects.
	service.keyErr = nil
	service.keyOrg = uuid.Nil
	if got := send(h, http.MethodGet, "/projects/CORE", "", auth); got.Code != http.StatusOK {
		t.Fatalf("key without organization_id = %d, want 200; body %s", got.Code, got.Body.String())
	}
	if service.keyOrg != uuid.Nil || service.keyValue != "CORE" {
		t.Fatalf("FindByKey received org %s key %q", service.keyOrg, service.keyValue)
	}

	// Several organizations: the caller must narrow the lookup.
	service.keyErr = domain.ErrProjectKeyAmbiguous
	got := send(h, http.MethodGet, "/projects/CORE", "", auth)
	if got.Code != http.StatusConflict || !strings.Contains(got.Body.String(), "project.key_ambiguous") {
		t.Fatalf("ambiguous key = %d %s, want 409 project.key_ambiguous", got.Code, got.Body.String())
	}

	if got := send(h, http.MethodGet, "/projects/CORE?organization_id=nope", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed organization_id = %d, want 422", got.Code)
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
