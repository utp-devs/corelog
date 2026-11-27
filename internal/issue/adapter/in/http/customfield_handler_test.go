package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// fakeCustomFieldService returns canned results so the handler's status and body mapping can be checked.
type fakeCustomFieldService struct {
	err    error
	field  *domain.CustomField
	layout port.LayoutView
}

func (f *fakeCustomFieldService) ListCustomFields(context.Context, uuid.UUID, uuid.UUID) ([]*domain.CustomField, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []*domain.CustomField{f.field}, nil
}

func (f *fakeCustomFieldService) CreateCustomField(_ context.Context, _, _ uuid.UUID, in port.CustomFieldInput) (*domain.CustomField, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.CustomField{ID: uuid.New(), Name: in.Name, Type: in.Type, Options: []domain.FieldOption{{ID: "o1", Label: "Alta"}}}, nil
}

func (f *fakeCustomFieldService) UpdateCustomField(_ context.Context, _, _, id uuid.UUID, _ port.CustomFieldPatch) (*domain.CustomField, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.CustomField{ID: id, Name: "Renombrado", Type: domain.FieldText}, nil
}

func (f *fakeCustomFieldService) DeleteCustomField(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.err
}

func (f *fakeCustomFieldService) ReplaceCustomFieldContexts(_ context.Context, _, _, id uuid.UUID, _ []port.ContextInput) (*domain.CustomField, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.CustomField{ID: id, Name: "Contexto", Type: domain.FieldDate}, nil
}

func (f *fakeCustomFieldService) ProjectLayout(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) (port.LayoutView, error) {
	return f.layout, f.err
}

func (f *fakeCustomFieldService) ReplaceProjectLayout(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, []domain.LayoutItem) (port.LayoutView, error) {
	return f.layout, f.err
}

func (f *fakeCustomFieldService) ScopeFor(context.Context, uuid.UUID, uuid.UUID) ([]domain.ScopedField, error) {
	return nil, f.err
}

func newCustomFieldRouter(svc port.CustomFieldService) chi.Router {
	r := chi.NewRouter()
	issuehttp.NewCustomFieldHandler(svc, catalogTestSecret).RegisterCustomFieldRoutes(r)
	return r
}

func TestCustomFieldRoutesRequireAuthentication(t *testing.T) {
	router := newCustomFieldRouter(&fakeCustomFieldService{})
	org := uuid.NewString()
	project := uuid.NewString()
	paths := []struct{ method, path string }{
		{http.MethodGet, "/organizations/" + org + "/custom-fields"},
		{http.MethodPost, "/organizations/" + org + "/custom-fields"},
		{http.MethodPatch, "/organizations/" + org + "/custom-fields/" + uuid.NewString()},
		{http.MethodDelete, "/organizations/" + org + "/custom-fields/" + uuid.NewString()},
		{http.MethodPut, "/organizations/" + org + "/custom-fields/" + uuid.NewString() + "/contexts"},
		{http.MethodGet, "/projects/" + project + "/fields?issue_type_id=" + uuid.NewString()},
		{http.MethodPut, "/projects/" + project + "/fields?issue_type_id=" + uuid.NewString()},
	}
	for _, p := range paths {
		rec := catalogRequest(t, router, p.method, p.path, `{}`, false)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token = %d, want 401", p.method, p.path, rec.Code)
		}
	}
}

func TestCustomFieldCreateStatusesAndCodes(t *testing.T) {
	org := uuid.NewString()
	path := "/organizations/" + org + "/custom-fields"

	rec := catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPost, path,
		`{"name":"Severidad","type":"select","options":[{"label":"Alta","color":"red"}]}`, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPost, path,
		`{"name":"X","type":"color"}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown type = %d, want 422", rec.Code)
	}

	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPost, path,
		`{"name":"X","type":"text","unknown":1}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown body field = %d, want 422", rec.Code)
	}

	cases := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"name taken", domain.ErrCustomFieldNameTaken, http.StatusConflict, "custom_field.name_taken"},
		{"forbidden", apperror.New(apperror.KindForbidden, "forbidden"), http.StatusForbidden, ""},
		{"outsider", apperror.WithCode(apperror.KindNotFound, "org.not_found", "organization not found"), http.StatusNotFound, "org.not_found"},
		{"invalid options", domain.ErrCustomFieldOptions, http.StatusUnprocessableEntity, "custom_field.invalid_options"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: tc.err}), http.MethodPost, path,
				`{"name":"X","type":"text"}`, true)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.code != "" && errorCode(t, rec) != tc.code {
				t.Errorf("code = %s, want %s", errorCode(t, rec), tc.code)
			}
		})
	}
}

func TestCustomFieldUpdateDeleteAndContextsMapErrors(t *testing.T) {
	org := uuid.NewString()
	field := uuid.NewString()
	base := "/organizations/" + org + "/custom-fields/" + field

	rec := catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPatch, base, `{"name":"Nuevo"}`, true)
	if rec.Code != http.StatusOK {
		t.Errorf("patch = %d, want 200", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: domain.ErrCustomFieldNotFound}), http.MethodPatch, base, `{"name":"Nuevo"}`, true)
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "custom_field.not_found" {
		t.Errorf("patch missing = %d %s, want 404 custom_field.not_found", rec.Code, errorCode(t, rec))
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: domain.ErrCustomFieldNameTaken}), http.MethodPatch, base, `{"name":"Nuevo"}`, true)
	if rec.Code != http.StatusConflict {
		t.Errorf("patch taken name = %d, want 409", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPatch, base, `{"name":"Nuevo","id":"x"}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("patch unknown field = %d, want 422", rec.Code)
	}

	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodDelete, base, "", true)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: apperror.New(apperror.KindForbidden, "no")}), http.MethodDelete, base, "", true)
	if rec.Code != http.StatusForbidden {
		t.Errorf("delete by member = %d, want 403", rec.Code)
	}

	contexts := base + "/contexts"
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPut, contexts, `{"contexts":[{"required":true,"default_value":"2026-12-01"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Errorf("contexts = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPut, contexts, `{"contexts":[{"project_id":"no-es-uuid"}]}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("contexts with bad uuid = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: domain.ErrCustomFieldContextDup}), http.MethodPut, contexts, `{"contexts":[]}`, true)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "custom_field.context_duplicate" {
		t.Errorf("duplicate context = %d %s", rec.Code, errorCode(t, rec))
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: apperror.New(apperror.KindNotFound, "missing")}), http.MethodPut, contexts, `{"contexts":[]}`, true)
	if rec.Code != http.StatusNotFound {
		t.Errorf("contexts of missing field = %d, want 404", rec.Code)
	}
}

func TestProjectLayoutRoutesValidateAndMapErrors(t *testing.T) {
	project := uuid.NewString()
	issueType := uuid.NewString()
	path := "/projects/" + project + "/fields"

	rec := catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodGet, path, "", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("layout without issue_type_id = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodGet, path+"?issue_type_id=nope", "", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("layout with bad issue_type_id = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodGet, path+"?issue_type_id="+issueType, "", true)
	if rec.Code != http.StatusOK {
		t.Errorf("layout = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: apperror.New(apperror.KindNotFound, "project not found")}), http.MethodGet, path+"?issue_type_id="+issueType, "", true)
	if rec.Code != http.StatusNotFound {
		t.Errorf("layout of hidden project = %d, want 404", rec.Code)
	}

	body := `{"items":[{"kind":"system","key":"summary","required":true},{"kind":"custom","field_id":"` + uuid.NewString() + `"}]}`
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPut, path+"?issue_type_id="+issueType, body, true)
	if rec.Code != http.StatusOK {
		t.Errorf("layout put = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: apperror.WithCode(apperror.KindValidation, "project.field_layout_invalid", "la disposicion de campos no es valida")}),
		http.MethodPut, path+"?issue_type_id="+issueType, body, true)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "project.field_layout_invalid" {
		t.Errorf("layout invalid = %d %s", rec.Code, errorCode(t, rec))
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{}), http.MethodPut, path+"?issue_type_id="+issueType,
		`{"items":[{"kind":"custom","field_id":"no-uuid"}]}`, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("layout with bad field id = %d, want 422", rec.Code)
	}
	rec = catalogRequest(t, newCustomFieldRouter(&fakeCustomFieldService{err: apperror.New(apperror.KindForbidden, "no")}),
		http.MethodPut, path+"?issue_type_id="+issueType, body, true)
	if rec.Code != http.StatusForbidden {
		t.Errorf("layout put by viewer = %d, want 403", rec.Code)
	}
}
