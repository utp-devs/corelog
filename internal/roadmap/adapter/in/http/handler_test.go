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

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/roadmap/application/port"
	"github.com/jhonsferg/corelog/internal/roadmap/application/usecase"
	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

const secret = "test-secret-test-secret-test-secret"

type repo struct {
	epics  []domain.EpicRow
	deps   []domain.Dependency
	ranked []uuid.UUID
}

func (r *repo) Epics(context.Context, uuid.UUID) ([]domain.EpicRow, error) { return r.epics, nil }
func (r *repo) Versions(context.Context, uuid.UUID) ([]domain.PeriodRow, error) {
	v := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	return []domain.PeriodRow{{ID: uuid.New(), Kind: "version", Name: "v1", Status: "unreleased", End: &v, Total: 2, Done: 1}}, nil
}
func (r *repo) Sprints(context.Context, uuid.UUID) ([]domain.PeriodRow, error) { return nil, nil }
func (r *repo) Dependencies(context.Context, uuid.UUID) ([]domain.Dependency, error) {
	return r.deps, nil
}
func (r *repo) SaveRank(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
	r.ranked = ids
	return nil
}

type auth struct{ edit bool }

func (a auth) Authorize(_ context.Context, action string, _ uuid.UUID, role string, project uuid.UUID) error {
	if role == "ghost" {
		return apperror.New(apperror.KindNotFound, "no")
	}
	if action == port.ActionEditIssue && !a.edit {
		return apperror.New(apperror.KindForbidden, "no")
	}
	return nil
}

func setup(t *testing.T, edit bool, role string) (http.Handler, string, *repo, uuid.UUID, uuid.UUID) {
	t.Helper()
	d := func(m time.Month, day int) *time.Time {
		x := time.Date(2026, m, day, 0, 0, 0, 0, time.UTC)
		return &x
	}
	a, b := uuid.New(), uuid.New()
	r := &repo{
		epics: []domain.EpicRow{
			{ID: a, Key: "P-1", Title: "Plataforma", StatusCategory: "in_progress", Assignee: "Ana", Start: d(1, 1), Due: d(2, 1), Total: 4, Done: 1, Todo: 2, InProgress: 1, PointsTotal: 10, PointsDone: 2.5},
			{ID: b, Key: "P-2", Title: "Pagos", ChildStart: d(3, 1), ChildDue: d(3, 31), Total: 1, Done: 1},
			{ID: uuid.New(), Key: "P-3", Title: "Sin fechas"},
		},
		deps: []domain.Dependency{{From: a, To: b, Kind: "blocks"}},
	}
	h := NewHandler(usecase.NewService(r, auth{edit: edit}), secret)
	router := chi.NewRouter()
	router.Route("/api/v1", func(rt chi.Router) { h.RegisterRoutes(rt) })
	token, err := authtoken.Generate(secret, time.Hour, uuid.New(), role)
	if err != nil {
		t.Fatal(err)
	}
	return router, token, r, a, b
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRoadmapResponseShape(t *testing.T) {
	h, token, _, a, b := setup(t, true, "user")
	project := uuid.NewString()
	rec := do(h, "GET", "/api/v1/projects/"+project+"/roadmap?from=2026-01-01&to=2026-12-31", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var env struct {
		Data struct {
			From         string           `json:"from"`
			To           string           `json:"to"`
			GroupBy      string           `json:"group_by"`
			Bars         []map[string]any `json:"bars"`
			Versions     []map[string]any `json:"versions"`
			Sprints      []any            `json:"sprints"`
			Dependencies []map[string]any `json:"dependencies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	d := env.Data
	if d.From != "2026-01-01" || d.To != "2026-12-31" || d.GroupBy != "epic" || len(d.Bars) != 3 || len(d.Versions) != 1 || d.Sprints == nil || len(d.Sprints) != 0 {
		t.Fatalf("data: %+v", d)
	}
	first := d.Bars[0]
	prog := first["progress"].(map[string]any)
	if first["id"] != a.String() || first["kind"] != "epic" || first["key"] != "P-1" || first["start"] != "2026-01-01" || first["end"] != "2026-02-01" || first["start_source"] != "own" ||
		first["dated"] != true || first["assignee"] != "Ana" || prog["percent"] != float64(25) || prog["in_progress"] != float64(1) || prog["points_done"] != 2.5 {
		t.Fatalf("first bar: %v", first)
	}
	second := d.Bars[1]
	if second["start_source"] != "derived" || second["end"] != "2026-03-31" || second["complete"] != true {
		t.Fatalf("derived bar: %v", second)
	}
	third := d.Bars[2]
	if third["start"] != nil || third["end"] != nil || third["dated"] != false || third["start_source"] != "none" {
		t.Fatalf("undated bar: %v", third)
	}
	if d.Versions[0]["kind"] != "version" || d.Versions[0]["end"] != "2026-03-31" {
		t.Fatalf("version: %v", d.Versions[0])
	}
	if len(d.Dependencies) != 1 || d.Dependencies[0]["from"] != a.String() || d.Dependencies[0]["to"] != b.String() || d.Dependencies[0]["kind"] != "blocks" {
		t.Fatalf("dependencies: %v", d.Dependencies)
	}
}

func TestRoadmapQueryValidationAndCodes(t *testing.T) {
	h, token, _, _, _ := setup(t, true, "user")
	base := "/api/v1/projects/" + uuid.NewString() + "/roadmap"
	if rec := do(h, "GET", base, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
	for _, q := range []string{"?from=2026/01/01", "?to=x", "?from=2026-05-01&to=2026-01-01", "?group_by=team", "?filter=late", "?q=" + strings.Repeat("a", 201), "?from=2000-01-01&to=2026-01-01"} {
		if rec := do(h, "GET", base+q, token, ""); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
	if rec := do(h, "GET", "/api/v1/projects/not-a-uuid/roadmap", token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("bad project id: %d", rec.Code)
	}
	ghost, gtoken, _, _, _ := setup(t, true, "ghost")
	if rec := do(ghost, "GET", base, gtoken, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden project: %d", rec.Code)
	}
	if rec := do(h, "GET", base+"?group_by=version&filter=open&q=v1", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("group by version: %d %s", rec.Code, rec.Body)
	}
}

func TestDependenciesAndRankEndpoints(t *testing.T) {
	h, token, r, a, b := setup(t, true, "user")
	base := "/api/v1/projects/" + uuid.NewString() + "/roadmap"
	rec := do(h, "GET", base+"/dependencies", token, "")
	var env struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if rec.Code != 200 || len(env.Data) != 1 || env.Data[0]["from"] != a.String() {
		t.Fatalf("dependencies: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "PUT", base+"/rank", token, `{"epic_ids":["`+b.String()+`","`+a.String()+`"]}`); rec.Code != http.StatusNoContent || len(r.ranked) != 2 || r.ranked[0] != b {
		t.Fatalf("rank: %d %v", rec.Code, r.ranked)
	}
	if rec := do(h, "PUT", base+"/rank", token, `{"epic_ids":[]}`); rec.Code != http.StatusNoContent || len(r.ranked) != 0 {
		t.Fatalf("clear rank: %d", rec.Code)
	}
	for name, body := range map[string]string{"missing": `{}`, "unknown field": `{"epic_ids":[],"x":1}`, "bad id": `{"epic_ids":["x"]}`, "duplicates": `{"epic_ids":["` + a.String() + `","` + a.String() + `"]}`, "not json": `[`, "trailing": `{"epic_ids":[]} 1`} {
		if rec := do(h, "PUT", base+"/rank", token, body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	viewer, vtoken, _, _, _ := setup(t, false, "user")
	if rec := do(viewer, "PUT", base+"/rank", vtoken, `{"epic_ids":[]}`); rec.Code != http.StatusForbidden {
		t.Fatalf("rank without edit rights: %d", rec.Code)
	}
	if rec := do(viewer, "GET", base+"/dependencies", vtoken, ""); rec.Code != http.StatusOK {
		t.Fatalf("viewer reads dependencies: %d", rec.Code)
	}
	ghost, gtoken, _, _, _ := setup(t, true, "ghost")
	if rec := do(ghost, "PUT", base+"/rank", gtoken, `{"epic_ids":[]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden project rank: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/v1/projects/nope/roadmap/rank", token, `{"epic_ids":[]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("bad project: %d", rec.Code)
	}
	if rec := do(h, "PUT", base+"/rank", "", `{}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
}
