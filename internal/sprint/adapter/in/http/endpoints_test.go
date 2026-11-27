package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// fakeSprints implementa el servicio de sprints con errores configurables.
type fakeSprints struct {
	port.Service
	sprint *domain.Sprint
	err    error
}

func (f *fakeSprints) ResolveSprintProject(context.Context, uuid.UUID) (uuid.UUID, error) {
	return f.sprint.ProjectID, f.err
}

func (f *fakeSprints) List(context.Context, uuid.UUID, string, uuid.UUID) ([]*domain.Sprint, error) {
	return []*domain.Sprint{f.sprint}, f.err
}

func (f *fakeSprints) Create(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, _, _ string, _, _ *time.Time) (*domain.Sprint, error) {
	return f.sprint, f.err
}

func (f *fakeSprints) Start(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, port.StartInput) (*domain.Sprint, error) {
	return f.sprint, f.err
}

func (f *fakeSprints) Complete(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, port.CompleteInput) (*port.CompleteResult, error) {
	return &port.CompleteResult{Sprint: f.sprint}, f.err
}

func (f *fakeSprints) AssignIssue(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.err
}

func (f *fakeSprints) RemoveIssue(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.err
}

func (f *fakeSprints) Burndown(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) ([]port.BurndownPoint, error) {
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	return []port.BurndownPoint{{Date: start, Ideal: 8, Remaining: 8}}, f.err
}

func (f *fakeSprints) Velocity(context.Context, uuid.UUID, string, uuid.UUID, int) ([]port.VelocityPoint, error) {
	return []port.VelocityPoint{{Name: "Sprint 1", Points: 13}}, f.err
}

func (f *fakeSprints) ListIssues(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) ([]port.IssueRef, error) {
	return nil, f.err
}

func sprintRouter(service port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, "test-secret").RegisterRoutes(r)
	return r
}

func sprintAuth(t *testing.T) string {
	t.Helper()
	value, err := authtoken.Generate("test-secret", time.Hour, uuid.New(), "agent")
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + value
}

func sprintSend(h http.Handler, method, path, body, auth string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", auth)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

func TestSprintHTTPLifecycleStatusCodes(t *testing.T) {
	project, sprintID, issueID := uuid.New(), uuid.New(), uuid.New()
	base := "/sprints"
	query := "?project_id=" + project.String()
	sprint, _ := domain.NewSprint(time.Now(), project, "Sprint 1", "", nil, nil)
	sprint.ID = sprintID
	auth := sprintAuth(t)

	ok := sprintRouter(&fakeSprints{sprint: sprint})
	if got := sprintSend(ok, http.MethodGet, base+query, "", auth); got.Code != http.StatusOK {
		t.Fatalf("list = %d", got.Code)
	}
	if got := sprintSend(ok, http.MethodPost, base+query, `{"name":"Sprint 1","start_date":"2026-11-02","end_date":"2026-11-08"}`, auth); got.Code != http.StatusCreated {
		t.Fatalf("create = %d, body %s", got.Code, got.Body.String())
	}
	if got := sprintSend(ok, http.MethodPost, base+"/"+sprintID.String()+"/start"+query, "", auth); got.Code != http.StatusOK {
		t.Fatalf("start = %d", got.Code)
	}
	if got := sprintSend(ok, http.MethodPost, base+"/"+sprintID.String()+"/close"+query, `{"unfinished":"backlog"}`, auth); got.Code != http.StatusOK {
		t.Fatalf("close = %d", got.Code)
	}
	assign := base + "/" + sprintID.String() + "/issues/" + issueID.String() + query
	if got := sprintSend(ok, http.MethodPut, assign, "", auth); got.Code != http.StatusNoContent {
		t.Fatalf("assign = %d", got.Code)
	}
	if got := sprintSend(ok, http.MethodDelete, assign, "", auth); got.Code != http.StatusNoContent {
		t.Fatalf("remove = %d", got.Code)
	}
	if got := sprintSend(ok, http.MethodGet, base+"/"+sprintID.String()+"/burndown"+query, "", auth); got.Code != http.StatusOK {
		t.Fatalf("burndown = %d", got.Code)
	}
	if got := sprintSend(ok, http.MethodGet, base+"/velocity"+query+"&limit=3", "", auth); got.Code != http.StatusOK {
		t.Fatalf("velocity = %d", got.Code)
	}
	if got := sprintSend(ok, http.MethodGet, base+"/"+sprintID.String()+"/issues"+query, "", auth); got.Code != http.StatusOK {
		t.Fatalf("sprint issues = %d", got.Code)
	}

	conflict := sprintRouter(&fakeSprints{sprint: sprint, err: domain.ErrActiveSprintExists})
	if got := sprintSend(conflict, http.MethodPost, base+"/"+sprintID.String()+"/start"+query, "", auth); got.Code != http.StatusConflict {
		t.Fatalf("second active sprint = %d, want 409", got.Code)
	}
	if got := sprintSend(ok, http.MethodGet, base+"/velocity"+query+"&limit=abc", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non numeric limit = %d, want 422", got.Code)
	}
	if got := sprintSend(ok, http.MethodPost, base+query, `{"name":"x","start_date":"2026/11/02"}`, auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed date = %d, want 422", got.Code)
	}
}
