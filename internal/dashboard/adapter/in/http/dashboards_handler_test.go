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

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const secret = "test-secret-test-secret-test-secret"

type stub struct {
	detail   *port.DashboardDetail
	err      error
	created  port.CreateInput
	updated  port.UpdateInput
	layout   *string
	gadgets  []port.GadgetInput
	shares   []domain.Share
	listQ    port.ListQuery
	dupName  string
	viewer   port.Viewer
	deleted  bool
	dataErr  error
	lastGid  uuid.UUID
	listBody []port.DashboardView
}

func (s *stub) List(_ context.Context, v port.Viewer, q port.ListQuery) ([]port.DashboardView, int, error) {
	s.listQ, s.viewer = q, v
	return s.listBody, 42, s.err
}
func (s *stub) Create(_ context.Context, v port.Viewer, in port.CreateInput) (*port.DashboardDetail, error) {
	s.created, s.viewer = in, v
	return s.detail, s.err
}
func (s *stub) Get(context.Context, port.Viewer, uuid.UUID) (*port.DashboardDetail, error) {
	return s.detail, s.err
}
func (s *stub) Update(_ context.Context, _ port.Viewer, _ uuid.UUID, in port.UpdateInput) (*port.DashboardDetail, error) {
	s.updated = in
	return s.detail, s.err
}
func (s *stub) Delete(context.Context, port.Viewer, uuid.UUID) error {
	s.deleted = true
	return s.err
}
func (s *stub) ReplaceGadgets(_ context.Context, _ port.Viewer, _ uuid.UUID, layout *string, g []port.GadgetInput) (*port.DashboardDetail, error) {
	s.layout, s.gadgets = layout, g
	return s.detail, s.err
}
func (s *stub) SetShares(_ context.Context, _ port.Viewer, _ uuid.UUID, sh []domain.Share) (*port.DashboardDetail, error) {
	s.shares = sh
	return s.detail, s.err
}
func (s *stub) Duplicate(_ context.Context, _ port.Viewer, _ uuid.UUID, name string) (*port.DashboardDetail, error) {
	s.dupName = name
	return s.detail, s.err
}
func (s *stub) Default(context.Context, port.Viewer) (*port.DashboardDetail, error) {
	return s.detail, s.err
}
func (s *stub) GadgetData(_ context.Context, _ port.Viewer, _, gid uuid.UUID) (port.GadgetData, error) {
	s.lastGid = gid
	return port.GadgetData{GadgetID: gid, Type: "text", GeneratedAt: time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC), Data: port.TextData{Markdown: "hola"}}, s.dataErr
}

func sampleDetail(owner bool) *port.DashboardDetail {
	access := domain.AccessView
	if owner {
		access = domain.AccessOwner
	}
	gid := uuid.New()
	return &port.DashboardDetail{
		DashboardView: port.DashboardView{
			Dashboard: domain.Dashboard{ID: uuid.New(), OwnerID: uuid.New(), OrganizationID: uuid.New(), Name: "Panel", Layout: domain.Layout21, IsDefault: true,
				Gadgets: []domain.Gadget{{ID: gid, Type: domain.GadgetText, Title: "Nota", Config: json.RawMessage(`{"markdown":"x"}`), Position: 0, ColumnIndex: 1}, {ID: uuid.New(), Type: domain.GadgetFavorites, Title: "F"}}},
			OwnerName: "Ana", Access: access, IsFavorite: true, Shared: true,
		},
		Shares: []port.ShareView{{Share: domain.Share{Type: domain.ShareTeam, TargetID: uuid.New(), Permission: domain.PermissionEdit}, TargetName: "Equipo"}},
	}
}

func router(t *testing.T, s *stub) (chi.Router, string) {
	t.Helper()
	h := NewHandler(nil, secret).WithDashboards(s)
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { h.RegisterRoutes(r) })
	token, err := authtoken.Generate(secret, time.Hour, uuid.New(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	return r, token
}

func do(r http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func data(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("%q: %v", rec.Body.String(), err)
	}
	d, _ := env["data"].(map[string]any)
	return d
}

func TestDashboardRoutesRequireAuthentication(t *testing.T) {
	r, _ := router(t, &stub{})
	id := uuid.NewString()
	for _, c := range [][2]string{
		{"GET", "/api/v1/dashboards"}, {"POST", "/api/v1/dashboards"}, {"GET", "/api/v1/dashboards/" + id}, {"PATCH", "/api/v1/dashboards/" + id},
		{"DELETE", "/api/v1/dashboards/" + id}, {"PUT", "/api/v1/dashboards/" + id + "/gadgets"}, {"PUT", "/api/v1/dashboards/" + id + "/shares"},
		{"POST", "/api/v1/dashboards/" + id + "/duplicate"}, {"GET", "/api/v1/dashboards/" + id + "/gadgets/" + id + "/data"}, {"GET", "/api/v1/me/dashboard"},
		{"GET", "/api/v1/dashboard/me"},
	} {
		if rec := do(r, c[0], c[1], "", "{}"); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", c[0], c[1], rec.Code)
		}
	}
}

func TestDashboardResponsesAndShareVisibility(t *testing.T) {
	s := &stub{detail: sampleDetail(true)}
	r, token := router(t, s)
	rec := do(r, "GET", "/api/v1/dashboards/"+uuid.NewString(), token, "")
	d := data(t, rec)
	if rec.Code != 200 || d["layout"] != "2-1" || d["access"] != "owner" || d["can_edit"] != true || d["can_manage"] != true || d["is_default"] != true || d["is_favorite"] != true || d["gadget_count"] != float64(2) {
		t.Fatalf("owner: %d %v", rec.Code, d)
	}
	gadgets := d["gadgets"].([]any)
	g0 := gadgets[0].(map[string]any)
	if g0["type"] != "text" || g0["column_index"] != float64(1) || g0["config"].(map[string]any)["markdown"] != "x" {
		t.Fatalf("gadget: %v", g0)
	}
	if gadgets[1].(map[string]any)["config"] == nil {
		t.Fatal("a gadget without config must serialize an empty object")
	}
	if sh := d["shares"].([]any); sh[0].(map[string]any)["target_name"] != "Equipo" || sh[0].(map[string]any)["permission"] != "edit" {
		t.Fatalf("shares: %v", d["shares"])
	}
	if d["owner"].(map[string]any)["name"] != "Ana" {
		t.Fatalf("owner: %v", d["owner"])
	}
	// A viewer gets neither the manage flag nor the shares.
	s.detail = sampleDetail(false)
	d = data(t, do(r, "GET", "/api/v1/dashboards/"+uuid.NewString(), token, ""))
	if d["can_edit"] != false || d["can_manage"] != false || d["shares"] != nil || d["access"] != "view" {
		t.Fatalf("viewer: %v", d)
	}
}

func TestDashboardCreateUpdateDelete(t *testing.T) {
	s := &stub{detail: sampleDetail(true)}
	r, token := router(t, s)
	org := uuid.New()
	rec := do(r, "POST", "/api/v1/dashboards", token, `{"name":"Panel","description":"d","layout":"3","organization_id":"`+org.String()+`"}`)
	if rec.Code != http.StatusCreated || s.created.Name != "Panel" || s.created.Layout != "3" || s.created.OrganizationID != org || !s.viewer.PlatformAdmin {
		t.Fatalf("create: %d %+v", rec.Code, s.created)
	}
	for name, body := range map[string]string{"unknown field": `{"name":"x","bogus":1}`, "bad org": `{"name":"x","organization_id":"nope"}`, "not json": `{`, "trailing": `{"name":"x"} 1`, "empty": ``} {
		if rec := do(r, "POST", "/api/v1/dashboards", token, body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	id := uuid.NewString()
	if rec := do(r, "PATCH", "/api/v1/dashboards/"+id, token, `{"name":"Nuevo","is_default":true}`); rec.Code != 200 || *s.updated.Name != "Nuevo" || !*s.updated.IsDefault || s.updated.Layout != nil {
		t.Fatalf("patch: %d %+v", rec.Code, s.updated)
	}
	if rec := do(r, "PATCH", "/api/v1/dashboards/"+id, token, `{"colour":"red"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("patch unknown field: %d", rec.Code)
	}
	if rec := do(r, "DELETE", "/api/v1/dashboards/"+id, token, ""); rec.Code != http.StatusNoContent || !s.deleted {
		t.Fatalf("delete: %d", rec.Code)
	}
	for _, m := range []string{"GET", "PATCH", "DELETE"} {
		if rec := do(r, m, "/api/v1/dashboards/not-a-uuid", token, `{}`); rec.Code != http.StatusNotFound {
			t.Errorf("%s with a bad id: %d", m, rec.Code)
		}
	}
	s.err = domain.ErrForbidden
	if rec := do(r, "DELETE", "/api/v1/dashboards/"+id, token, ""); rec.Code != http.StatusForbidden || data(t, rec) != nil {
		t.Fatalf("forbidden: %d", rec.Code)
	}
	s.err = domain.ErrNameTaken
	if rec := do(r, "POST", "/api/v1/dashboards", token, `{"name":"x"}`); rec.Code != http.StatusConflict {
		t.Fatalf("conflict: %d", rec.Code)
	}
	s.err = domain.ErrNotFound
	if rec := do(r, "GET", "/api/v1/dashboards/"+id, token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("not found: %d", rec.Code)
	}
}

func TestDashboardList(t *testing.T) {
	s := &stub{listBody: []port.DashboardView{sampleDetail(true).DashboardView}}
	r, token := router(t, s)
	rec := do(r, "GET", "/api/v1/dashboards?scope=shared&q=eq&page=2&page_size=500", token, "")
	if rec.Code != 200 || s.listQ.Scope != "shared" || s.listQ.Query != "eq" || s.listQ.Page != 2 {
		t.Fatalf("list: %d %+v", rec.Code, s.listQ)
	}
	var env struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if len(env.Data) != 1 || env.Data[0]["gadgets"] != nil || env.Data[0]["gadget_count"] != float64(2) || env.Meta["total"] != float64(42) || env.Meta["page_size"] != float64(100) {
		t.Fatalf("body: %s", rec.Body)
	}
}

func TestReplaceGadgetsAndShares(t *testing.T) {
	s := &stub{detail: sampleDetail(true)}
	r, token := router(t, s)
	id, gid := uuid.NewString(), uuid.New()
	base := "/api/v1/dashboards/" + id
	body := `{"layout":"2-1","gadgets":[{"id":"` + gid.String() + `","type":"text","title":"T","config":{"markdown":"a"},"column_index":1,"position":3},{"type":"favorites","column_index":0}]}`
	if rec := do(r, "PUT", base+"/gadgets", token, body); rec.Code != 200 {
		t.Fatalf("gadgets: %d %s", rec.Code, rec.Body)
	}
	if *s.layout != "2-1" || len(s.gadgets) != 2 || *s.gadgets[0].ID != gid || s.gadgets[0].ColumnIndex != 1 || *s.gadgets[0].Position != 3 || string(s.gadgets[0].Config) != `{"markdown":"a"}` || s.gadgets[1].ID != nil {
		t.Fatalf("gadgets mapped: %+v", s.gadgets)
	}
	if rec := do(r, "PUT", base+"/gadgets", token, `{"gadgets":[]}`); rec.Code != 200 || s.layout != nil || len(s.gadgets) != 0 {
		t.Fatalf("empty gadgets: %d", rec.Code)
	}
	for name, bad := range map[string]string{"missing": `{}`, "bad id": `{"gadgets":[{"id":"x","type":"text"}]}`, "unknown": `{"gadgets":[{"type":"text","size":3}]}`} {
		if rec := do(r, "PUT", base+"/gadgets", token, bad); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}

	target := uuid.New()
	if rec := do(r, "PUT", base+"/shares", token, `{"shares":[{"type":"user","target_id":"`+target.String()+`","permission":"edit"}]}`); rec.Code != 200 ||
		len(s.shares) != 1 || s.shares[0].TargetID != target || s.shares[0].Permission != domain.PermissionEdit || s.shares[0].Type != domain.ShareUser {
		t.Fatalf("shares: %d %+v", rec.Code, s.shares)
	}
	for name, bad := range map[string]string{"missing": `{}`, "bad target": `{"shares":[{"type":"user","target_id":"x"}]}`, "unknown": `{"shares":[{"kind":"user"}]}`} {
		if rec := do(r, "PUT", base+"/shares", token, bad); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
}

func TestDuplicateDefaultAndGadgetData(t *testing.T) {
	s := &stub{detail: sampleDetail(true)}
	r, token := router(t, s)
	id := uuid.NewString()
	if rec := do(r, "POST", "/api/v1/dashboards/"+id+"/duplicate", token, ``); rec.Code != http.StatusCreated || s.dupName != "" {
		t.Fatalf("duplicate without body: %d", rec.Code)
	}
	if rec := do(r, "POST", "/api/v1/dashboards/"+id+"/duplicate", token, `{"name":"Copia"}`); rec.Code != http.StatusCreated || s.dupName != "Copia" {
		t.Fatalf("duplicate: %d", rec.Code)
	}
	if rec := do(r, "POST", "/api/v1/dashboards/"+id+"/duplicate", token, `{"nombre":"x"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate unknown field: %d", rec.Code)
	}
	if rec := do(r, "GET", "/api/v1/me/dashboard", token, ""); rec.Code != 200 || data(t, rec)["name"] != "Panel" {
		t.Fatalf("default: %d", rec.Code)
	}

	gid := uuid.New()
	rec := do(r, "GET", "/api/v1/dashboards/"+id+"/gadgets/"+gid.String()+"/data", token, "")
	d := data(t, rec)
	if rec.Code != 200 || s.lastGid != gid || d["type"] != "text" || d["gadget_id"] != gid.String() || d["data"].(map[string]any)["markdown"] != "hola" {
		t.Fatalf("data: %d %v", rec.Code, d)
	}
	if rec.Header().Get("Cache-Control") != "private, max-age=15" {
		t.Fatalf("cache header = %q", rec.Header().Get("Cache-Control"))
	}
	if rec := do(r, "GET", "/api/v1/dashboards/"+id+"/gadgets/nope/data", token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("bad gadget id: %d", rec.Code)
	}
	s.dataErr = domain.ErrGadgetNotFound
	if rec := do(r, "GET", "/api/v1/dashboards/"+id+"/gadgets/"+gid.String()+"/data", token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing gadget: %d", rec.Code)
	}
}

func TestLegacySummaryRouteWorksWithoutDashboards(t *testing.T) {
	h := NewHandler(nil, secret)
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { h.RegisterRoutes(r) })
	if rec := do(r, "GET", "/api/v1/dashboard/me", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("legacy: %d", rec.Code)
	}
	if rec := do(r, "GET", "/api/v1/dashboards", "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("v2 routes without service: %d", rec.Code)
	}
}
