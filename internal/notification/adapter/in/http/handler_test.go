package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const testSecret = "test-secret"

// fakeService implementa solo lo que usan los endpoints probados.
type fakeService struct {
	port.Service
	items   []*domain.Notification
	markErr error
}

func (f *fakeService) List(context.Context, uuid.UUID) ([]*domain.Notification, error) {
	return f.items, nil
}

func (f *fakeService) UnreadCount(context.Context, uuid.UUID) (int, error) {
	return 2, nil
}

func (f *fakeService) MarkRead(context.Context, uuid.UUID, uuid.UUID) error {
	return f.markErr
}

func (f *fakeService) MarkAllRead(context.Context, uuid.UUID) error {
	return nil
}

func router(service port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, testSecret).RegisterRoutes(r)
	return r
}

func token(t *testing.T, user uuid.UUID) string {
	t.Helper()
	value, err := authtoken.Generate(testSecret, time.Hour, user, "requester")
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func do(t *testing.T, h http.Handler, method, path string, user uuid.UUID, withToken bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	if withToken {
		request.Header.Set("Authorization", "Bearer "+token(t, user))
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

func TestNotificationHTTPListAndCount(t *testing.T) {
	user := uuid.New()
	n, err := domain.New(user, "issue_assigned", "Te asignaron algo", "/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	h := router(&fakeService{items: []*domain.Notification{n}})

	list := do(t, h, http.MethodGet, "/notifications", user, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}
	var body struct {
		Data []struct {
			Message string `json:"message"`
			Read    bool   `json:"read"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil || len(body.Data) != 1 || body.Data[0].Read {
		t.Fatalf("list body = %s, err %v", list.Body.String(), err)
	}

	count := do(t, h, http.MethodGet, "/notifications/unread-count", user, true)
	if count.Code != http.StatusOK || !containsCount(count.Body.String(), 2) {
		t.Fatalf("unread-count = %d %s", count.Code, count.Body.String())
	}
}

func TestNotificationHTTPMarkingAndAuthentication(t *testing.T) {
	user := uuid.New()
	id := uuid.New().String()

	h := router(&fakeService{})
	if got := do(t, h, http.MethodPost, "/notifications/"+id+"/read", user, true); got.Code != http.StatusNoContent {
		t.Fatalf("mark read = %d", got.Code)
	}
	if got := do(t, h, http.MethodPost, "/notifications/read-all", user, true); got.Code != http.StatusNoContent {
		t.Fatalf("read-all = %d", got.Code)
	}
	if got := do(t, h, http.MethodPost, "/notifications/not-a-uuid/read", user, true); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid id = %d, want 422", got.Code)
	}
	if got := do(t, h, http.MethodGet, "/notifications", user, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", got.Code)
	}

	notFound := router(&fakeService{markErr: domain.ErrNotificationNotFound})
	if got := do(t, notFound, http.MethodPost, "/notifications/"+id+"/read", user, true); got.Code != http.StatusNotFound {
		t.Fatalf("someone else's notification = %d, want 404", got.Code)
	}
}

func containsCount(body string, count int) bool {
	var parsed struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	return json.Unmarshal([]byte(body), &parsed) == nil && parsed.Data.Count == count
}
