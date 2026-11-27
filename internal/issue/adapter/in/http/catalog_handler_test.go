package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const catalogTestSecret = "catalog-test-secret-0123456789abcdef"

// fakeCatalogService returns canned results so the handler's status and body mapping can be checked.
type fakeCatalogService struct {
	err        error
	items      []*domain.CatalogItem
	reassignTo *uuid.UUID
	deleted    bool
}

func (f *fakeCatalogService) ListCatalog(context.Context, uuid.UUID, uuid.UUID, domain.CatalogKind) ([]*domain.CatalogItem, error) {
	return f.items, f.err
}

func (f *fakeCatalogService) CreateCatalogItem(_ context.Context, _, _ uuid.UUID, _ domain.CatalogKind, in port.CatalogInput) (*domain.CatalogItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.CatalogItem{ID: uuid.New(), Name: in.Name, Icon: in.Icon, Color: in.Color, Kind: domain.CatalogIssueType, Usage: 2, CreatedAt: time.Now()}, nil
}

func (f *fakeCatalogService) UpdateCatalogItem(_ context.Context, _, _ uuid.UUID, _ domain.CatalogKind, id uuid.UUID, _ port.CatalogPatch) (*domain.CatalogItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.CatalogItem{ID: id, Name: "Renombrada", Kind: domain.CatalogPriority}, nil
}

func (f *fakeCatalogService) DeleteCatalogItem(_ context.Context, _, _ uuid.UUID, _ domain.CatalogKind, _ uuid.UUID, reassignTo *uuid.UUID) error {
	f.reassignTo = reassignTo
	f.deleted = true
	return f.err
}

func (f *fakeCatalogService) ReorderCatalog(context.Context, uuid.UUID, uuid.UUID, domain.CatalogKind, []uuid.UUID) ([]*domain.CatalogItem, error) {
	return f.items, f.err
}

func (f *fakeCatalogService) ProjectIssueTypes(context.Context, uuid.UUID, string, uuid.UUID) ([]*domain.CatalogItem, bool, error) {
	return f.items, true, f.err
}

func (f *fakeCatalogService) ReplaceProjectIssueTypes(context.Context, uuid.UUID, string, uuid.UUID, []uuid.UUID) ([]*domain.CatalogItem, error) {
	return f.items, f.err
}

func newCatalogRouter(svc port.CatalogService) chi.Router {
	r := chi.NewRouter()
	issuehttp.NewCatalogHandler(svc, catalogTestSecret).RegisterCatalogRoutes(r)
	return r
}

func catalogRequest(t *testing.T, router http.Handler, method, path, body string, authorized bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reader)
	if authorized {
		token, err := authtoken.Generate(catalogTestSecret, time.Hour, uuid.New(), "user")
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, rec.Body.String())
	}
	return body.Error.Code
}

func TestCatalogRoutesRequireAuthentication(t *testing.T) {
	router := newCatalogRouter(&fakeCatalogService{})
	org := uuid.NewString()
	rec := catalogRequest(t, router, http.MethodGet, "/organizations/"+org+"/issue-types", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCatalogListReturnsItemsWithUsage(t *testing.T) {
	level := domain.HierarchyEpic
	svc := &fakeCatalogService{items: []*domain.CatalogItem{{
		ID: uuid.New(), Name: "Épica", Icon: "epic", Color: "purple", HierarchyLevel: level, Usage: 4,
		Kind: domain.CatalogIssueType,
	}}}
	router := newCatalogRouter(svc)
	rec := catalogRequest(t, router, http.MethodGet, "/organizations/"+uuid.NewString()+"/issue-types", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0]["usage_count"] != float64(4) || body.Data[0]["hierarchy_level"] != float64(1) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestCatalogErrorsMapToStatusAndCode(t *testing.T) {
	org := uuid.NewString()
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not a member", apperror.WithCode(apperror.KindNotFound, "org.not_found", "organization not found"), http.StatusNotFound, "org.not_found"},
		{"member below admin", apperror.WithCode(apperror.KindForbidden, "org.forbidden", "forbidden"), http.StatusForbidden, "org.forbidden"},
		{"name taken", domain.ErrCatalogNameTaken, http.StatusConflict, "catalog.name_taken"},
		{"in use", domain.ErrCatalogInUse, http.StatusConflict, "catalog.in_use"},
		{"last required", domain.ErrCatalogRequiredKind, http.StatusUnprocessableEntity, "catalog.last_required_item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := newCatalogRouter(&fakeCatalogService{err: tc.err})
			rec := catalogRequest(t, router, http.MethodPost, "/organizations/"+org+"/priorities",
				`{"name":"Crítica","icon":"flag","color":"red"}`, true)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if got := errorCode(t, rec); got != tc.code {
				t.Errorf("code = %s, want %s", got, tc.code)
			}
		})
	}
}

func TestCatalogCreateRejectsUnknownFieldsAndInvalidIds(t *testing.T) {
	router := newCatalogRouter(&fakeCatalogService{})
	org := "/organizations/" + uuid.NewString()

	rec := catalogRequest(t, router, http.MethodPost, org+"/issue-types", `{"name":"X","unexpected":true}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown field status = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, router, http.MethodPost, org+"/issue-types", `{"description":"sin nombre"}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing name status = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, router, http.MethodGet, "/organizations/not-a-uuid/issue-types", "", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad org id status = %d, want 422", rec.Code)
	}
}

func TestCatalogCreateReturnsCreated(t *testing.T) {
	router := newCatalogRouter(&fakeCatalogService{})
	rec := catalogRequest(t, router, http.MethodPost, "/organizations/"+uuid.NewString()+"/issue-types",
		`{"name":"Mejora","icon":"star","color":"indigo"}`, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCatalogDeleteForwardsReassignTarget(t *testing.T) {
	svc := &fakeCatalogService{}
	router := newCatalogRouter(svc)
	org := "/organizations/" + uuid.NewString() + "/issue-types/"
	item := uuid.NewString()
	target := uuid.New()

	rec := catalogRequest(t, router, http.MethodDelete, org+item+"?reassign_to="+target.String(), "", true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if svc.reassignTo == nil || *svc.reassignTo != target {
		t.Errorf("reassign target not forwarded: %v", svc.reassignTo)
	}

	rec = catalogRequest(t, router, http.MethodDelete, org+item+"?reassign_to=nope", "", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid reassign_to status = %d, want 422", rec.Code)
	}
}

func TestCatalogReorderAndPatchValidateIds(t *testing.T) {
	router := newCatalogRouter(&fakeCatalogService{})
	org := "/organizations/" + uuid.NewString()

	rec := catalogRequest(t, router, http.MethodPut, org+"/priorities/reorder", `{"ids":["bad"]}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad reorder id status = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, router, http.MethodPatch, org+"/priorities/bad-id", `{"name":"X"}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad item id status = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, router, http.MethodPatch, org+"/priorities/"+uuid.NewString(), `{"name":"Nueva"}`, true)
	if rec.Code != http.StatusOK {
		t.Errorf("patch status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectIssueTypesEndpoints(t *testing.T) {
	svc := &fakeCatalogService{items: []*domain.CatalogItem{{ID: uuid.New(), Name: "Tarea", Kind: domain.CatalogIssueType}}}
	router := newCatalogRouter(svc)
	project := "/projects/" + uuid.NewString() + "/issue-types"

	rec := catalogRequest(t, router, http.MethodGet, project, "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous get status = %d, want 401", rec.Code)
	}
	rec = catalogRequest(t, router, http.MethodGet, project, "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = catalogRequest(t, router, http.MethodPut, project, `{"issue_type_ids":[]}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty list status = %d, want 422", rec.Code)
	}
	ids := `{"issue_type_ids":["` + uuid.NewString() + `"]}`
	svc.err = apperror.WithCode(apperror.KindValidation, "project.issue_types_required", "x")
	rec = catalogRequest(t, router, http.MethodPut, project, ids, true)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "project.issue_types_required" {
		t.Errorf("missing kinds status = %d code = %s", rec.Code, rec.Body.String())
	}
}
