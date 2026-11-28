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
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/domain"
)

func TestToCSV(t *testing.T) {
	table := domain.Table{
		Columns: []domain.Column{{Key: "a", Label: "Resumen", Type: "text"}, {Key: "n", Label: "=Total", Type: "number"}, {Key: "p", Label: "Valor", Type: "percent"}},
		Rows: [][]any{
			{"=HYPERLINK(\"http://x\")", 1.5, nil},
			{"+cmd", -3.0, true},
			{"@SUM(A1)", 2, false},
			{"-1+2", 0.0, "plain, with comma"},
			{"\tTab", 1, "ok"},
			{"normal ñ", 10.25, struct{}{}},
		},
	}
	out, err := ToCSV(table)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.HasPrefix(s, "\xEF\xBB\xBF") {
		t.Fatal("missing UTF-8 byte order mark")
	}
	lines := strings.Split(strings.TrimPrefix(s, "\xEF\xBB\xBF"), "\n")
	want := []string{
		`Resumen,'=Total,Valor`,
		`"'=HYPERLINK(""http://x"")",1.5,`,
		`'+cmd,-3,Sí`,
		`'@SUM(A1),2,No`,
		`'-1+2,0,"plain, with comma"`,
		"'\tTab,1,ok",
		`normal ñ,10.25,{}`,
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
	if SafeCell("") != "" || SafeCell("a=b") != "a=b" {
		t.Fatal("safe cells")
	}
}

type fakeService struct {
	runErr error
}

func (f fakeService) List(context.Context, uuid.UUID, string, uuid.UUID) ([]domain.Descriptor, error) {
	return domain.Catalog(), nil
}

func (f fakeService) Run(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, kind string, raw domain.RawParams) (*port.Result, error) {
	if f.runErr != nil {
		return nil, f.runErr
	}
	rep := domain.BuildVelocity(nil)
	rep.Table.Rows = append(rep.Table.Rows, []any{"=x", 1.0, 2.0, nil})
	return &port.Result{Report: rep, GeneratedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), Params: map[string]any{"kind": kind, "limit": raw.Limit}}, nil
}

func routerFor(svc port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(svc, "secret").RegisterRoutes(r)
	return r
}

func get(h http.Handler, path string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlerShapes(t *testing.T) {
	h := routerFor(fakeService{})
	token, _ := authtoken.Generate("secret", time.Hour, uuid.New(), "user")
	project := uuid.NewString()

	if rec := get(h, "/projects/"+project+"/reports", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
	if rec := get(h, "/projects/zzz/reports/velocity", token); rec.Code != http.StatusNotFound {
		t.Fatalf("bad project: %d", rec.Code)
	}
	rec := get(h, "/projects/"+project+"/reports", token)
	var list struct {
		Data []descriptorDTO `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Data) != 11 || rec.Header().Get("Cache-Control") != cacheControl {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = get(h, "/projects/"+project+"/reports/velocity?limit=3", token)
	var got struct {
		Data reportDTO `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	d := got.Data
	if d.Kind != "velocity" || d.GeneratedAt != "2024-01-02T03:04:05Z" || d.Params["limit"] != "3" || len(d.Table.Columns) != 4 || len(d.Table.Rows) != 1 || d.Series == nil {
		t.Fatalf("dto = %+v", d)
	}
	rec = get(h, "/projects/"+project+"/reports/velocity.csv", token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "'=x,1,2,") || !strings.Contains(rec.Header().Get("Content-Disposition"), "informe-velocity-2024-01-02.csv") {
		t.Fatalf("csv: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	rec = get(routerFor(fakeService{runErr: domain.ErrRangeInvalid}), "/projects/"+project+"/reports/control", token)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "report.range_invalid") {
		t.Fatalf("error mapping: %d %s", rec.Code, rec.Body.String())
	}
}
