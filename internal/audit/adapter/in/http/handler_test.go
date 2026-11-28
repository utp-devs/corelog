package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	httpadapter "github.com/jhonsferg/corelog/internal/audit/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/application/usecase"
	"github.com/jhonsferg/corelog/internal/audit/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const secret = "test-secret-key-that-is-long-enough-32-bytes"

type fakeRepo struct {
	entries []domain.Entry
	last    port.ListFilter
}

func (f *fakeRepo) Save(context.Context, domain.Entry) error { return nil }
func (f *fakeRepo) List(_ context.Context, flt port.ListFilter) (port.ListPage, error) {
	f.last = flt
	var out []domain.Entry
	for _, e := range f.entries {
		if flt.OrganizationID != nil && (e.OrganizationID == nil || *e.OrganizationID != *flt.OrganizationID) {
			continue
		}
		out = append(out, e)
	}
	page := port.ListPage{Entries: out}
	if flt.Limit > 0 && len(out) > flt.Limit {
		page.Entries = out[:flt.Limit]
		page.NextCursor = page.Entries[len(page.Entries)-1].ID
	}
	return page, nil
}
func (f *fakeRepo) PurgeOlderThan(context.Context, time.Time) (int64, error) { return 0, nil }
func (f *fakeRepo) ActorName(context.Context, uuid.UUID) (string, error)     { return "", nil }

type fakeOrgs struct{ admins map[[2]uuid.UUID]bool }

func (f fakeOrgs) RequireAdmin(_ context.Context, u, o uuid.UUID) error {
	if f.admins[[2]uuid.UUID{u, o}] {
		return nil
	}
	return apperror.WithCode(apperror.KindForbidden, "org.forbidden", "sin permiso")
}

type env struct {
	router chi.Router
	repo   *fakeRepo
	orgA   uuid.UUID
	orgB   uuid.UUID
	admin  string // platform admin
	owner  string // owner of org A
	member string
}

func token(t *testing.T, id uuid.UUID, role string) string {
	t.Helper()
	tok, err := authtoken.Generate(secret, time.Hour, id, role)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func setup(t *testing.T) *env {
	t.Helper()
	orgA, orgB := uuid.New(), uuid.New()
	ownerID, memberID, adminID := uuid.New(), uuid.New(), uuid.New()
	mk := func(id int64, org uuid.UUID, summary string) domain.Entry {
		o := org
		return domain.Entry{ID: id, OrganizationID: &o, Action: "org.updated", EntityType: "organization",
			Summary: summary, CreatedAt: time.Date(2026, 1, 1, 0, 0, int(id), 0, time.UTC), ActorName: "Ana"}
	}
	repo := &fakeRepo{entries: []domain.Entry{mk(3, orgA, "=cmd|' /C calc'!A0"), mk(2, orgB, "otra org"), mk(1, orgA, "primero")}}
	svc := usecase.NewService(repo, fakeOrgs{admins: map[[2]uuid.UUID]bool{{ownerID, orgA}: true}})
	r := chi.NewRouter()
	httpadapter.NewHandler(svc, secret).RegisterRoutes(r)
	return &env{router: r, repo: repo, orgA: orgA, orgB: orgB,
		admin: token(t, adminID, "admin"), owner: token(t, ownerID, "user"), member: token(t, memberID, "user")}
}

func (e *env) get(path, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestPlatformAuditPermissions(t *testing.T) {
	t.Parallel()
	e := setup(t)
	for name, tc := range map[string]struct {
		bearer string
		want   int
	}{
		"anonymous":      {"", http.StatusUnauthorized},
		"plain user":     {e.member, http.StatusForbidden},
		"org owner":      {e.owner, http.StatusForbidden},
		"platform admin": {e.admin, http.StatusOK},
	} {
		if rec := e.get("/admin/audit", tc.bearer); rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d", name, rec.Code, tc.want)
		}
		if rec := e.get("/admin/audit/export", tc.bearer); rec.Code != tc.want {
			t.Errorf("%s export: status %d, want %d", name, rec.Code, tc.want)
		}
	}
}

func TestOrganizationAuditScopeAndRoles(t *testing.T) {
	t.Parallel()
	e := setup(t)
	if rec := e.get("/organizations/"+e.orgA.String()+"/audit", e.member); rec.Code != http.StatusForbidden {
		t.Fatalf("member: %d", rec.Code)
	}
	if rec := e.get("/organizations/"+e.orgB.String()+"/audit", e.owner); rec.Code != http.StatusForbidden {
		t.Fatalf("owner of another org: %d", rec.Code)
	}
	if rec := e.get("/organizations/not-a-uuid/audit", e.owner); rec.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d", rec.Code)
	}
	rec := e.get("/organizations/"+e.orgA.String()+"/audit", e.owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "otra org") {
		t.Fatal("entries of another organization leaked")
	}
	// The organization scope wins over any filter the caller sends.
	e.get("/organizations/"+e.orgA.String()+"/audit?actor="+uuid.NewString(), e.owner)
	if e.repo.last.OrganizationID == nil || *e.repo.last.OrganizationID != e.orgA {
		t.Fatal("organization filter was not forced")
	}
}

func TestAuditFilterValidationAndPaging(t *testing.T) {
	t.Parallel()
	e := setup(t)
	for _, q := range []string{"actor=nope", "from=ayer", "to=2026-13-40", "limit=0", "limit=999", "cursor=!!!", "from=2026-02-01&to=2026-01-01"} {
		rec := e.get("/admin/audit?"+q, e.admin)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "audit.filter_invalid") {
			t.Errorf("%q: %d %s", q, rec.Code, rec.Body)
		}
	}
	rec := e.get("/admin/audit?limit=2&entity_type=organization&action=org.updated&from=2026-01-01&to=2026-01-31", e.admin)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"next_cursor":"`) {
		t.Fatalf("paged list: %d %s", rec.Code, rec.Body)
	}
	if e.repo.last.EntityType != "organization" || e.repo.last.Action != "org.updated" || e.repo.last.Limit != 2 ||
		e.repo.last.Until == nil || !e.repo.last.Until.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("filters not forwarded: %+v", e.repo.last)
	}
}

func TestAuditExportIsCSVAndNeutralisesFormulas(t *testing.T) {
	t.Parallel()
	e := setup(t)
	rec := e.get("/admin/audit/export", e.admin)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("headers: %d %v", rec.Code, rec.Header())
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "id,fecha,actor,accion,entidad,id_entidad,resumen,ip,id_peticion") {
		t.Fatalf("header row: %q", body)
	}
	if strings.Contains(body, ",=cmd") || !strings.Contains(body, "'=cmd") {
		t.Fatalf("formula injection not neutralised:\n%s", body)
	}
}

func TestAuditHasNoWriteEndpoints(t *testing.T) {
	t.Parallel()
	e := setup(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(m, "/admin/audit", nil)
		req.Header.Set("Authorization", "Bearer "+e.admin)
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Errorf("%s /admin/audit answered %d", m, rec.Code)
		}
	}
}
