package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
)

// fixedAccess answers every lookup with one access context, whoever the caller is.
type fixedAccess struct {
	access authdomain.Access
}

func (f *fixedAccess) FindAccess(context.Context, uuid.UUID, uuid.UUID) (authdomain.Access, error) {
	return f.access, nil
}

func (f *fixedAccess) FindRole(context.Context, uuid.UUID, uuid.UUID) (*authdomain.ProjectRole, error) {
	return f.access.ProjectRole, nil
}

// memSchemeStore is an in-memory scheme repository for the HTTP tests.
type memSchemeStore struct {
	rows map[uuid.UUID]authdomain.Scheme
}

func (m *memSchemeStore) Load(_ context.Context, projectID uuid.UUID) (*authdomain.Scheme, error) {
	s, ok := m.rows[projectID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (m *memSchemeStore) Replace(_ context.Context, projectID uuid.UUID, scheme authdomain.Scheme) error {
	m.rows[projectID] = scheme
	return nil
}

func (m *memSchemeStore) Reset(_ context.Context, projectID uuid.UUID) error {
	delete(m.rows, projectID)
	return nil
}

var _ authport.SchemeRepository = (*memSchemeStore)(nil)

func rolePtr(r authdomain.ProjectRole) *authdomain.ProjectRole { return &r }

// permissionsRouter serves the permission routes for a caller whose access is fixed.
func permissionsRouter(access authdomain.Access, store *memSchemeStore) http.Handler {
	authorizer := authusecase.NewAuthorizer(&fixedAccess{access: access}).WithSchemes(store)
	service := authusecase.NewPermissionService(authorizer, store)
	r := chi.NewRouter()
	NewHandler(nil, nil, testSecret).WithPermissions(service).RegisterRoutes(r)
	return r
}

func newStore() *memSchemeStore {
	return &memSchemeStore{rows: map[uuid.UUID]authdomain.Scheme{}}
}

// errorCode extracts error.code from an error response body.
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error body %s: %v", body, err)
	}
	return payload.Error.Code
}

// schemeBody builds a PUT body from the default grants with one permission changed.
func schemeBody(t *testing.T, changed string, holders ...string) string {
	t.Helper()
	var grants []permissionGrantDTO
	for _, g := range authdomain.DefaultScheme().Grants() {
		dto := permissionGrantDTO{Permission: string(g.Permission), Holders: []string{}}
		for _, h := range g.Holders {
			dto.Holders = append(dto.Holders, string(h))
		}
		if dto.Permission == changed {
			dto.Holders = holders
		}
		grants = append(grants, dto)
	}
	raw, err := json.Marshal(replacePermissionsRequest{Grants: grants})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func decodeScheme(t *testing.T, body []byte) permissionSchemeDTO {
	t.Helper()
	var env struct {
		Data permissionSchemeDTO `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode scheme %s: %v", body, err)
	}
	return env.Data
}

func holdersOf(scheme permissionSchemeDTO, permission string) []string {
	for _, g := range scheme.Grants {
		if g.Permission == permission {
			return g.Holders
		}
	}
	return nil
}

func TestPermissionsRequireAuthentication(t *testing.T) {
	project := uuid.New().String()
	router := permissionsRouter(authdomain.Access{OrgRole: authdomain.OrgRoleOwner}, newStore())
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/projects/" + project + "/permissions"},
		{http.MethodPut, "/projects/" + project + "/permissions"},
		{http.MethodPost, "/projects/" + project + "/permissions/reset"},
		{http.MethodGet, "/projects/" + project + "/permissions/me"},
	} {
		if rec := send(router, tc.method, tc.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestGetPermissionsReturnsDefaultScheme(t *testing.T) {
	project := uuid.New().String()
	lead := authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleLead), Visibility: authdomain.VisibilityOpen}
	router := permissionsRouter(lead, newStore())

	rec := send(router, http.MethodGet, "/projects/"+project+"/permissions", "", bearer(t, "user"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	scheme := decodeScheme(t, rec.Body.Bytes())
	if !scheme.IsDefault || len(scheme.Grants) != len(authdomain.Permissions) {
		t.Fatalf("default flag %v, %d grants", scheme.IsDefault, len(scheme.Grants))
	}
	if got := holdersOf(scheme, "create_issue"); strings.Join(got, ",") != "lead,contributor" {
		t.Fatalf("create_issue holders = %v", got)
	}
}

func TestPutPermissionsAllowedOnlyWithEditProject(t *testing.T) {
	project := uuid.New().String()
	store := newStore()
	body := schemeBody(t, "create_issue", "lead", "viewer")

	for _, tc := range []struct {
		name   string
		access authdomain.Access
		want   int
	}{
		{"viewer", authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleViewer), Visibility: authdomain.VisibilityOpen}, http.StatusForbidden},
		{"contributor", authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleContributor), Visibility: authdomain.VisibilityOpen}, http.StatusForbidden},
		{"lead", authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleLead), Visibility: authdomain.VisibilityOpen}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(permissionsRouter(tc.access, store), http.MethodPut, "/projects/"+project+"/permissions", body, bearer(t, "user"))
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestPutPermissionsInvalidSchemeIs422(t *testing.T) {
	project := uuid.New().String()
	lead := authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleLead), Visibility: authdomain.VisibilityOpen}
	router := permissionsRouter(lead, newStore())

	cases := map[string]string{
		"lead loses manage_members": schemeBody(t, "manage_members", "contributor"),
		"unknown holder":            schemeBody(t, "create_issue", "janitor"),
		"unknown field":             `{"grants":[],"extra":true}`,
		"not json":                  `{"grants":`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := send(router, http.MethodPut, "/projects/"+project+"/permissions", body, bearer(t, "user"))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
			}
		})
	}
	rec := send(router, http.MethodPut, "/projects/"+project+"/permissions", schemeBody(t, "manage_members", "contributor"), bearer(t, "user"))
	if code := errorCode(t, rec.Body.Bytes()); code != authdomain.CodeInvalidScheme {
		t.Fatalf("code = %q, want %s", code, authdomain.CodeInvalidScheme)
	}
}

func TestOutsiderOnPrivateProjectGetsNotFound(t *testing.T) {
	project := uuid.New().String()
	outsider := authdomain.Access{OrgRole: authdomain.OrgRoleNone, Visibility: authdomain.VisibilityPrivate}
	router := permissionsRouter(outsider, newStore())
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/projects/" + project + "/permissions", ""},
		{http.MethodPut, "/projects/" + project + "/permissions", schemeBody(t, "create_issue", "viewer")},
		{http.MethodPost, "/projects/" + project + "/permissions/reset", ""},
		{http.MethodGet, "/projects/" + project + "/permissions/me", ""},
	} {
		rec := send(router, tc.method, tc.path, tc.body, bearer(t, "user"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s: status %d, want 404", tc.method, tc.path, rec.Code)
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "project.not_found" {
			t.Fatalf("%s %s: code %q", tc.method, tc.path, code)
		}
	}
}

func TestResetRestoresDefaultScheme(t *testing.T) {
	project := uuid.New().String()
	store := newStore()
	lead := authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleLead), Visibility: authdomain.VisibilityOpen}
	viewer := authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleViewer), Visibility: authdomain.VisibilityOpen}

	rec := send(permissionsRouter(lead, store), http.MethodPut, "/projects/"+project+"/permissions", schemeBody(t, "create_issue", "lead", "viewer"), bearer(t, "user"))
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if scheme := decodeScheme(t, rec.Body.Bytes()); scheme.IsDefault {
		t.Fatal("a saved scheme must not be flagged as default")
	}

	rec = send(permissionsRouter(viewer, store), http.MethodPost, "/projects/"+project+"/permissions/reset", "", bearer(t, "user"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer reset: %d, want 403", rec.Code)
	}

	rec = send(permissionsRouter(lead, store), http.MethodPost, "/projects/"+project+"/permissions/reset", "", bearer(t, "user"))
	if rec.Code != http.StatusOK {
		t.Fatalf("lead reset: %d %s", rec.Code, rec.Body)
	}
	scheme := decodeScheme(t, rec.Body.Bytes())
	if !scheme.IsDefault {
		t.Fatal("reset must report the default scheme")
	}
	if got := holdersOf(scheme, "create_issue"); strings.Join(got, ",") != "lead,contributor" {
		t.Fatalf("after reset create_issue holders = %v", got)
	}
}

func TestMyPermissionsPerRole(t *testing.T) {
	project := uuid.New().String()
	cases := []struct {
		name   string
		access authdomain.Access
		role   string
		has    string
		lacks  string
	}{
		// The lead may delete the project under the default scheme.
		{"lead", authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleLead), Visibility: authdomain.VisibilityOpen}, "lead", "delete_project", ""},
		{"contributor", authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleContributor), Visibility: authdomain.VisibilityOpen}, "contributor", "create_issue", "edit_project"},
		{"viewer", authdomain.Access{OrgRole: authdomain.OrgRoleMember, ProjectRole: rolePtr(authdomain.ProjectRoleViewer), Visibility: authdomain.VisibilityOpen}, "viewer", "browse_project", "create_issue"},
		{"org owner", authdomain.Access{OrgRole: authdomain.OrgRoleOwner, Visibility: authdomain.VisibilityPrivate}, "lead", "delete_project", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(permissionsRouter(tc.access, newStore()), http.MethodGet, "/projects/"+project+"/permissions/me", "", bearer(t, "user"))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var env struct {
				Data effectivePermissionsDTO `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if env.Data.Role != tc.role {
				t.Fatalf("role = %q, want %q", env.Data.Role, tc.role)
			}
			granted := map[string]bool{}
			for _, p := range env.Data.Permissions {
				granted[p] = true
			}
			if !granted[tc.has] {
				t.Errorf("missing %s in %v", tc.has, env.Data.Permissions)
			}
			if tc.lacks != "" && granted[tc.lacks] {
				t.Errorf("unexpected %s in %v", tc.lacks, env.Data.Permissions)
			}
		})
	}
}
