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

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const testSecret = "test-secret"

// fakeBoard implementa el servicio del tablero con respuestas configurables.
type fakeBoard struct {
	port.Service
	view    *port.BoardView
	moveErr error
	wipErr  error
}

func (f *fakeBoard) Get(context.Context, uuid.UUID, uuid.UUID, string) (*port.BoardView, error) {
	return f.view, nil
}

func (f *fakeBoard) MoveCard(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID) error {
	return f.moveErr
}

func (f *fakeBoard) UpdateColumnWIP(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID, *int) error {
	return f.wipErr
}

func boardRouter(service port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, testSecret).RegisterRoutes(r)
	return r
}

func authorized(t *testing.T, request *http.Request) *http.Request {
	t.Helper()
	value, err := authtoken.Generate(testSecret, time.Hour, uuid.New(), "agent")
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+value)
	request.Header.Set("Content-Type", "application/json")
	return request
}

func sampleView() *port.BoardView {
	wip := 3
	return &port.BoardView{
		ID: uuid.New(), ProjectID: uuid.New(), Name: "Default", Type: "kanban", CanManageBoard: true,
		Columns: []port.ColumnView{{ID: uuid.New(), Name: "En curso", Status: "in_progress", WIPLimit: &wip}},
	}
}

func TestBoardHTTPGetReportsManageCapability(t *testing.T) {
	h := boardRouter(&fakeBoard{view: sampleView()})
	projectID := uuid.New()
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, authorized(t, httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/board", nil)))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data struct {
			CanManageBoard bool `json:"can_manage_board"`
			Columns        []struct {
				WIPLimit *int `json:"wip_limit"`
			} `json:"columns"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.CanManageBoard || body.Data.Columns[0].WIPLimit == nil || *body.Data.Columns[0].WIPLimit != 3 {
		t.Fatalf("unexpected board body: %s", recorder.Body.String())
	}
}

func TestBoardHTTPMoveAndWIPErrors(t *testing.T) {
	projectID, issueID, columnID := uuid.New(), uuid.New(), uuid.New()
	move := "/projects/" + projectID.String() + "/board/cards/" + issueID.String() + "/move"
	patch := "/projects/" + projectID.String() + "/board/columns/" + columnID.String()

	h := boardRouter(&fakeBoard{view: sampleView()})
	recorder := httptest.NewRecorder()
	body := `{"column_id":"` + columnID.String() + `"}`
	h.ServeHTTP(recorder, authorized(t, httptest.NewRequest(http.MethodPost, move, strings.NewReader(body))))
	if recorder.Code != http.StatusOK {
		t.Fatalf("move = %d", recorder.Code)
	}

	full := boardRouter(&fakeBoard{view: sampleView(), moveErr: domain.ErrWIPLimitExceeded})
	recorder = httptest.NewRecorder()
	full.ServeHTTP(recorder, authorized(t, httptest.NewRequest(http.MethodPost, move, strings.NewReader(body))))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("move into a full column = %d, want 409", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, authorized(t, httptest.NewRequest(http.MethodPost, move, strings.NewReader(`{}`))))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("move without column = %d, want 422", recorder.Code)
	}

	wip := boardRouter(&fakeBoard{view: sampleView(), wipErr: domain.ErrInvalidWIPLimit})
	recorder = httptest.NewRecorder()
	wip.ServeHTTP(recorder, authorized(t, httptest.NewRequest(http.MethodPatch, patch, strings.NewReader(`{"wip_limit":0}`))))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wip 0 = %d, want 422", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, authorized(t, httptest.NewRequest(http.MethodPatch, patch, strings.NewReader(`{"wip_limit":null}`))))
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear wip = %d", recorder.Code)
	}
}
