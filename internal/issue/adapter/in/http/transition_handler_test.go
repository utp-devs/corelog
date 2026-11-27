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
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// fakeTransitions is a transition service with canned answers.
type fakeTransitions struct {
	options []port.TransitionOption
	err     error
	last    port.TransitionRequest
	issue   *domain.Issue
	plan    port.TransitionPlan
}

func (f *fakeTransitions) ListTransitions(context.Context, uuid.UUID, uuid.UUID, string) ([]port.TransitionOption, error) {
	return f.options, f.err
}

func (f *fakeTransitions) TransitionIssue(_ context.Context, req port.TransitionRequest) (*port.TransitionResult, error) {
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	return &port.TransitionResult{Issue: f.issue, Plan: f.plan}, nil
}

func (f *fakeTransitions) MoveIssueToColumn(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.err
}

// transitionNames is a minimal user directory: names are not under test.
type transitionNames struct{}

func (transitionNames) SharesTeam(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (transitionNames) NamesOf(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{}, nil
}

func newTransitionRouter(t *testing.T, fake *fakeTransitions) (http.Handler, string) {
	t.Helper()
	const secret = "transition-test-secret"
	service := usecase.NewIssueService(newMemoryIssueRepo(), transitionNames{})
	router := chi.NewRouter()
	issuehttp.NewHandler(service, secret).WithTransitions(fake).RegisterRoutes(router)
	token, err := authtoken.Generate(secret, time.Hour, uuid.New(), "user")
	if err != nil {
		t.Fatal(err)
	}
	return router, token
}

func doTransition(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestTransitionEndpoints_ErrorMapping(t *testing.T) {
	issueID := uuid.New().String()
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantErr  string
	}{
		{"not found", domain.ErrIssueNotFound, http.StatusNotFound, "issue.not_found"},
		{"forbidden", apperror.New(apperror.KindForbidden, "forbidden project operation"), http.StatusForbidden, ""},
		{"not allowed", apperror.WithCode(apperror.KindForbidden, "workflow.transition_not_allowed", "no permitido"), http.StatusForbidden, "workflow.transition_not_allowed"},
		{"stale status", domain.ErrStatusConflict, http.StatusConflict, "issue.status_conflict"},
		{"unavailable", domain.ErrTransitionUnavailable, http.StatusConflict, "issue.transition_unavailable"},
		{"validation", apperror.WithCode(apperror.KindValidation, "workflow.validation_failed", "no cumple").
			WithDetails(apperror.FieldError{Field: "comment", Message: "obligatorio"}), http.StatusUnprocessableEntity, "workflow.validation_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, token := newTransitionRouter(t, &fakeTransitions{err: tc.err})
			for _, call := range []struct{ method, path, body string }{
				{http.MethodGet, "/issues/" + issueID + "/transitions", ""},
				{http.MethodPost, "/issues/" + issueID + "/transitions", `{"transition_id":"start"}`},
			} {
				rec := doTransition(router, call.method, call.path, token, call.body)
				if call.method == http.MethodGet && tc.name == "validation" {
					continue
				}
				if rec.Code != tc.wantCode {
					t.Fatalf("%s %s: status %d, want %d; body %s", call.method, call.path, rec.Code, tc.wantCode, rec.Body.String())
				}
				if tc.wantErr != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tc.wantErr)) {
					t.Fatalf("%s: body %s misses code %s", call.method, rec.Body.String(), tc.wantErr)
				}
			}
		})
	}
}

func TestTransitionEndpoints_AuthAndBodyValidation(t *testing.T) {
	router, token := newTransitionRouter(t, &fakeTransitions{})
	issueID := uuid.New().String()
	if rec := doTransition(router, http.MethodGet, "/issues/"+issueID+"/transitions", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET without token: %d, want 401", rec.Code)
	}
	if rec := doTransition(router, http.MethodPost, "/issues/"+issueID+"/transitions", "", `{"transition_id":"start"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST without token: %d, want 401", rec.Code)
	}
	for name, body := range map[string]string{
		"missing transition":  `{}`,
		"unknown member":      `{"transition_id":"start","colour":"red"}`,
		"bad expected status": `{"transition_id":"start","expected_status_id":"nope"}`,
		"not json":            `start`,
	} {
		if rec := doTransition(router, http.MethodPost, "/issues/"+issueID+"/transitions", token, body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d, want 422; body %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := doTransition(router, http.MethodPost, "/issues/not-an-issue/transitions", token, `{"transition_id":"start"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad issue reference: %d, want 422", rec.Code)
	}
}

func TestTransitionEndpoints_Shapes(t *testing.T) {
	toStatus := uuid.New()
	issue, err := domain.NewIssue("Resumen", "Texto", domain.PriorityMedium, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	issue.ID, issue.ProjectID, issue.KeyNumber, issue.ProjectKey = uuid.New(), uuid.New(), 7, "CORE"
	issue.StatusRef = &domain.StatusRef{ID: toStatus, Name: "En curso", Category: "in_progress"}
	fake := &fakeTransitions{
		options: []port.TransitionOption{{ID: "start", Name: "Iniciar", ToStatusID: toStatus, ToStatusName: "En curso", ToCategory: "in_progress", RequiresComment: true, RequiredFields: []string{"story_points"}}},
		issue:   issue,
		plan:    port.TransitionPlan{TransitionID: "start", TransitionName: "Iniciar", FromStatusID: uuid.New(), ToStatusID: toStatus, ToStatusName: "En curso", ToCategory: "in_progress"},
	}
	router, token := newTransitionRouter(t, fake)

	rec := doTransition(router, http.MethodGet, "/issues/"+issue.ID.String()+"/transitions", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Data []struct {
			ID             string   `json:"id"`
			ToStatus       string   `json:"to_status"`
			RequiredFields []string `json:"required_fields"`
			RequiresCmt    bool     `json:"requires_comment"`
			NeedsScreen    bool     `json:"needs_screen"`
			To             struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Category string `json:"category"`
			} `json:"to"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 1 || list.Data[0].ID != "start" || list.Data[0].ToStatus != "in_progress" || !list.Data[0].NeedsScreen ||
		!list.Data[0].RequiresCmt || list.Data[0].To.ID != toStatus.String() || list.Data[0].To.Category != "in_progress" || len(list.Data[0].RequiredFields) != 1 {
		t.Fatalf("list body = %s", rec.Body.String())
	}

	expected := uuid.New()
	resolution := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"transition_id": "start", "fields": map[string]string{"story_points": "3"}, "comment": "Hola",
		"resolution_id": resolution.String(), "expected_status_id": expected.String(),
	})
	rec = doTransition(router, http.MethodPost, "/issues/"+issue.ID.String()+"/transitions", token, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	if fake.last.TransitionID != "start" || fake.last.Input.Comment != "Hola" || fake.last.Input.Fields["story_points"] != "3" ||
		fake.last.Input.ResolutionID != resolution || fake.last.ExpectedStatusID != expected || fake.last.IssueID != issue.ID {
		t.Fatalf("request reached the service as %+v", fake.last)
	}
	var run struct {
		Data struct {
			Key            string `json:"key"`
			WorkflowStatus struct {
				Name string `json:"name"`
			} `json:"workflow_status"`
			Transition struct {
				ID string `json:"id"`
				To struct {
					Name string `json:"name"`
				} `json:"to"`
			} `json:"transition"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.Data.Key != "CORE-7" || run.Data.WorkflowStatus.Name != "En curso" || run.Data.Transition.ID != "start" || run.Data.Transition.To.Name != "En curso" {
		t.Fatalf("run body = %s", rec.Body.String())
	}
}

func TestTransitionEndpoints_UnwiredServiceIsAnError(t *testing.T) {
	const secret = "transition-test-secret"
	router := chi.NewRouter()
	issuehttp.NewHandler(usecase.NewIssueService(newMemoryIssueRepo(), transitionNames{}), secret).RegisterRoutes(router)
	token, _ := authtoken.Generate(secret, time.Hour, uuid.New(), "user")
	rec := doTransition(router, http.MethodGet, "/issues/"+uuid.New().String()+"/transitions", token, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unwired: %d, want 500", rec.Code)
	}
}
