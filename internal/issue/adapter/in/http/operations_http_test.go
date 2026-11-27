package http_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// fakeOps answers the operations endpoints with canned results so the HTTP layer can be checked alone.
type fakeOps struct {
	bulk    issueport.BulkResult
	bulkErr error
	moveErr error
	cloneOK *domain.Issue
	issue   *domain.Issue
	batch   issueport.ExportBatch
	inputs  []issueport.BulkInput
}

func (f *fakeOps) BulkChange(_ context.Context, _ uuid.UUID, _ string, in issueport.BulkInput) (issueport.BulkResult, error) {
	f.inputs = append(f.inputs, in)
	return f.bulk, f.bulkErr
}

func (f *fakeOps) CloneIssue(context.Context, uuid.UUID, uuid.UUID, string, issueport.CloneInput) (*domain.Issue, error) {
	return f.cloneOK, nil
}

func (f *fakeOps) MoveIssue(context.Context, uuid.UUID, uuid.UUID, string, issueport.MoveInput) (*issueport.MoveResult, error) {
	if f.moveErr != nil {
		return nil, f.moveErr
	}
	return &issueport.MoveResult{Issue: f.issue, Warnings: []string{"aviso"}}, nil
}

func (f *fakeOps) ConvertIssue(context.Context, uuid.UUID, uuid.UUID, string, issueport.ConvertInput) (*issueport.PatchResult, error) {
	return &issueport.PatchResult{Issue: f.issue}, nil
}

func (f *fakeOps) ExportIssues(_ context.Context, _ uuid.UUID, _ string, _ issueport.ListFilter,
	start func(bool) error, emit func(issueport.ExportBatch) error) error {
	if err := start(true); err != nil {
		return err
	}
	return emit(f.batch)
}

func (f *fakeOps) ExportFileName() string { return "incidencias-2026-11-27.csv" }

var _ issueport.IssueOperations = (*fakeOps)(nil)

const httpTestSecret = "test-secret-key-that-is-long-enough-32-bytes"

func opsRouter(t *testing.T, ops *fakeOps) (*chi.Mux, *memoryIssueRepo) {
	t.Helper()
	repo := newMemoryIssueRepo()
	dir := newMemoryDirectory()
	service := usecase.NewIssueService(repo, dir, projectRoleResolver{}, &memorySavedFilterRepository{})
	handler := issuehttp.NewHandler(service, httpTestSecret).WithOperations(ops)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r, repo
}

func bearer(t *testing.T, user uuid.UUID) string {
	t.Helper()
	token, err := authtoken.Generate(httpTestSecret, time.Hour, user, "user")
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + token
}

func do(t *testing.T, r http.Handler, method, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestOperationsHTTP_RequireAuthentication(t *testing.T) {
	r, _ := opsRouter(t, &fakeOps{})
	for _, call := range []struct{ method, path string }{
		{http.MethodPost, "/issues/bulk"},
		{http.MethodGet, "/issues/export"},
		{http.MethodPost, "/issues/" + uuid.NewString() + "/clone"},
		{http.MethodPost, "/issues/" + uuid.NewString() + "/move"},
		{http.MethodPost, "/issues/" + uuid.NewString() + "/convert"},
	} {
		if rec := do(t, r, call.method, call.path, "", "{}"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without token = %d, want 401", call.method, call.path, rec.Code)
		}
	}
}

func TestOperationsHTTP_BulkValidationAnswers422(t *testing.T) {
	r, _ := opsRouter(t, &fakeOps{})
	user := uuid.New()
	cases := map[string]string{
		"unknown action":      `{"issue_ids":["CORE-1"],"action":"explode"}`,
		"unknown field":       `{"issue_ids":["CORE-1"],"action":"delete","extra":true}`,
		"priority without id": `{"issue_ids":["CORE-1"],"action":"set_priority","params":{}}`,
		"labels with junk":    `{"issue_ids":["CORE-1"],"action":"set_labels","params":{"add":["nope"]}}`,
		"move without target": `{"issue_ids":["CORE-1"],"action":"move","params":{}}`,
	}
	for name, body := range cases {
		if rec := do(t, r, http.MethodPost, "/issues/bulk", bearer(t, user), body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s = %d (%s), want 422", name, rec.Code, rec.Body.String())
		}
	}
}

func TestOperationsHTTP_BulkReturnsSucceededAndFailed(t *testing.T) {
	issue := &domain.Issue{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "Ok", ProjectKey: "CORE", KeyNumber: 3, ReporterID: uuid.New()}
	ops := &fakeOps{bulk: issueport.BulkResult{
		Succeeded: []*domain.Issue{issue},
		Failed:    []issueport.BulkFailure{{Ref: "CORE-9", Code: "issue.forbidden", Message: "forbidden project operation"}},
	}}
	r, _ := opsRouter(t, ops)
	rec := do(t, r, http.MethodPost, "/issues/bulk", bearer(t, uuid.New()),
		`{"issue_ids":["CORE-3","CORE-9"],"action":"set_priority","params":{"priority_id":"`+uuid.NewString()+`"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk = %d (%s), want 200 even with failures", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Succeeded []struct {
				Issue struct {
					Key string `json:"key"`
				} `json:"issue"`
			} `json:"succeeded"`
			Failed []struct {
				Issue   string `json:"issue"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"failed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Succeeded) != 1 || body.Data.Succeeded[0].Issue.Key != "CORE-3" {
		t.Fatalf("succeeded = %+v", body.Data.Succeeded)
	}
	if len(body.Data.Failed) != 1 || body.Data.Failed[0].Issue != "CORE-9" || body.Data.Failed[0].Code != "issue.forbidden" {
		t.Fatalf("failed = %+v", body.Data.Failed)
	}
	if len(ops.inputs) != 1 || ops.inputs[0].PriorityID == uuid.Nil || ops.inputs[0].Action != domain.BulkSetPriority {
		t.Fatalf("service input = %+v", ops.inputs)
	}
}

func TestOperationsHTTP_BulkRejectsTooManyFromService(t *testing.T) {
	r, _ := opsRouter(t, &fakeOps{bulkErr: domain.ErrBulkTooMany})
	rec := do(t, r, http.MethodPost, "/issues/bulk", bearer(t, uuid.New()), `{"issue_ids":["CORE-1"],"action":"delete"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("too many = %d, want 422", rec.Code)
	}
}

func TestOperationsHTTP_ExportWritesBOMHeaderAndEscapedCells(t *testing.T) {
	reporter := uuid.New()
	issue := &domain.Issue{
		ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "=HYPERLINK(\"http://x\")", ProjectKey: "CORE", KeyNumber: 7,
		Status: domain.StatusOpen, Priority: domain.PriorityHigh, IssueType: domain.IssueTypeTask, ReporterID: reporter,
		CreatedAt: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC),
	}
	ops := &fakeOps{batch: issueport.ExportBatch{
		Issues: []*domain.Issue{issue},
		Names:  map[uuid.UUID]string{reporter: "Ana"},
		Labels: map[uuid.UUID][]string{issue.ID: {"backend", "deuda"}},
	}}
	r, _ := opsRouter(t, ops)
	rec := do(t, r, http.MethodGet, "/issues/export?columns=key,summary,reporter,labels", bearer(t, uuid.New()), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d (%s)", rec.Code, rec.Body.String())
	}
	raw := rec.Body.Bytes()
	if !bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("export has no UTF-8 BOM")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "incidencias-2026-11-27.csv") {
		t.Fatalf("content disposition = %q", cd)
	}
	if rec.Header().Get("X-Export-Truncated") != "true" {
		t.Fatal("truncation header missing")
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows[0], "|") != "Clave|Resumen|Reportado por|Etiquetas" {
		t.Fatalf("header = %v", rows[0])
	}
	if rows[1][0] != "CORE-7" || rows[1][1] != "'=HYPERLINK(\"http://x\")" || rows[1][2] != "Ana" || rows[1][3] != "backend, deuda" {
		t.Fatalf("row = %v", rows[1])
	}
}

func TestOperationsHTTP_ExportRejectsUnknownColumn(t *testing.T) {
	r, _ := opsRouter(t, &fakeOps{})
	if rec := do(t, r, http.MethodGet, "/issues/export?columns=key,password", bearer(t, uuid.New()), ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown column = %d, want 422", rec.Code)
	}
}

func TestOperationsHTTP_MoveReturnsWarningsAndMapsErrors(t *testing.T) {
	issue := &domain.Issue{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "Movida", ProjectKey: "OTHR", KeyNumber: 1, ReporterID: uuid.New()}
	r, _ := opsRouter(t, &fakeOps{issue: issue})
	target := uuid.NewString()
	rec := do(t, r, http.MethodPost, "/issues/"+issue.ID.String()+"/move", bearer(t, uuid.New()), `{"project_id":"`+target+`"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"warnings":["aviso"]`) {
		t.Fatalf("move = %d %s", rec.Code, rec.Body.String())
	}

	mapping := apperror.WithCode(apperror.KindValidation, "issue.move_mapping_required", "indica el mapeo").WithDetails(
		apperror.FieldError{Field: "field_mapping.issue_type.x", Message: "elija un valor"})
	r, _ = opsRouter(t, &fakeOps{moveErr: mapping})
	rec = do(t, r, http.MethodPost, "/issues/"+issue.ID.String()+"/move", bearer(t, uuid.New()), `{"project_id":"`+target+`"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "issue.move_mapping_required") {
		t.Fatalf("mapping required = %d %s", rec.Code, rec.Body.String())
	}

	r, _ = opsRouter(t, &fakeOps{moveErr: apperror.New(apperror.KindForbidden, "forbidden project operation")})
	if rec := do(t, r, http.MethodPost, "/issues/"+issue.ID.String()+"/move", bearer(t, uuid.New()), `{"project_id":"`+target+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("move without create permission = %d, want 403", rec.Code)
	}
	r, _ = opsRouter(t, &fakeOps{moveErr: domain.ErrStale})
	if rec := do(t, r, http.MethodPost, "/issues/"+issue.ID.String()+"/move", bearer(t, uuid.New()), `{"project_id":"`+target+`"}`); rec.Code != http.StatusConflict {
		t.Fatalf("stale move = %d, want 409", rec.Code)
	}
	r, _ = opsRouter(t, &fakeOps{moveErr: domain.ErrIssueNotFound})
	if rec := do(t, r, http.MethodPost, "/issues/"+issue.ID.String()+"/move", bearer(t, uuid.New()), `{"project_id":"`+target+`"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden issue move = %d, want 404", rec.Code)
	}
	if rec := do(t, r, http.MethodPost, "/issues/"+issue.ID.String()+"/move", bearer(t, uuid.New()), `{"project_id":"x"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad project id = %d, want 422", rec.Code)
	}
}

func TestOperationsHTTP_CloneAnswers201AndConvertRejectsBadType(t *testing.T) {
	clone := &domain.Issue{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "Copia", ProjectKey: "CORE", KeyNumber: 8, ReporterID: uuid.New()}
	r, _ := opsRouter(t, &fakeOps{cloneOK: clone, issue: clone})
	id := uuid.NewString()
	if rec := do(t, r, http.MethodPost, "/issues/"+id+"/clone", bearer(t, uuid.New()), `{"include":["links"]}`); rec.Code != http.StatusCreated {
		t.Fatalf("clone = %d, want 201", rec.Code)
	}
	if rec := do(t, r, http.MethodPost, "/issues/"+id+"/convert", bearer(t, uuid.New()), `{"issue_type_id":"nope"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("convert bad type = %d, want 422", rec.Code)
	}
	if rec := do(t, r, http.MethodPost, "/issues/"+id+"/convert", bearer(t, uuid.New()), `{"issue_type_id":"`+uuid.NewString()+`","parent_id":null}`); rec.Code != http.StatusOK {
		t.Fatalf("convert = %d, want 200", rec.Code)
	}
}
