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
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// assignViewerResolver da el rol viewer en el proyecto de pruebas: puede ver la incidencia pero no editarla.
type assignViewerResolver struct{}

func (assignViewerResolver) FindRole(context.Context, uuid.UUID, uuid.UUID) (*string, error) {
	role := "viewer"
	return &role, nil
}

func (assignViewerResolver) ListProjectIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000001")}, nil
}

func (r assignViewerResolver) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return authorizeWithResolver(ctx, r, action, actorID, globalRole, projectID)
}

func assignRequestFor(t *testing.T, router http.Handler, issueID uuid.UUID, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/issues/"+issueID.String()+"/assign", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type assignErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Details []struct {
			Field string `json:"field"`
		} `json:"details"`
	} `json:"error"`
}

func TestIssueHTTP_AssignAndUnassign(t *testing.T) {
	router, jwtSecret, repo, dir := setupIssueRouter()

	assigner := uuid.New()
	assignee := uuid.New()
	team := uuid.New()
	dir.teams[assigner] = &team
	dir.teams[assignee] = &team
	tokenAssigner, _ := authtoken.Generate(jwtSecret, time.Hour, assigner, "requester")

	projectID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	newIssue := func(t *testing.T, assigned *uuid.UUID) *domain.Issue {
		t.Helper()
		issue, err := domain.NewIssue("Assign HTTP", "Desc", domain.PriorityMedium, assigner)
		if err != nil {
			t.Fatal(err)
		}
		issue.ProjectID = projectID
		issue.AssigneeID = assigned
		if err := repo.Save(context.Background(), issue); err != nil {
			t.Fatal(err)
		}
		return issue
	}

	t.Run("PATCH assign with null clears the assignee (200)", func(t *testing.T) {
		issue := newIssue(t, &assignee)
		rec := assignRequestFor(t, router, issue.ID, tokenAssigner, `{"assignee_id":null}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var body struct {
			Data struct {
				AssigneeID *string `json:"assignee_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Data.AssigneeID != nil {
			t.Errorf("assignee_id = %q, want absent after unassign", *body.Data.AssigneeID)
		}
		stored, _ := repo.FindByID(context.Background(), issue.ID)
		if stored.AssigneeID != nil {
			t.Errorf("stored assignee = %v, want nil", stored.AssigneeID)
		}
	})

	t.Run("PATCH assign with null on unassigned issue is idempotent (200)", func(t *testing.T) {
		issue := newIssue(t, nil)
		rec := assignRequestFor(t, router, issue.ID, tokenAssigner, `{"assignee_id":null}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("PATCH assign with a uuid still assigns (200)", func(t *testing.T) {
		issue := newIssue(t, nil)
		rec := assignRequestFor(t, router, issue.ID, tokenAssigner, `{"assignee_id":"`+assignee.String()+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("PATCH assign rejects an omitted assignee_id (422 validation.failed)", func(t *testing.T) {
		issue := newIssue(t, &assignee)
		rec := assignRequestFor(t, router, issue.ID, tokenAssigner, `{}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		var body assignErrorBody
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body.Error.Code != "validation.failed" {
			t.Errorf("code = %q, want validation.failed", body.Error.Code)
		}
		if len(body.Error.Details) != 1 || body.Error.Details[0].Field != "assignee_id" {
			t.Errorf("details = %+v, want one entry for assignee_id", body.Error.Details)
		}
		stored, _ := repo.FindByID(context.Background(), issue.ID)
		if stored.AssigneeID == nil {
			t.Errorf("omitted field must not clear the assignee")
		}
	})

	for name, body := range map[string]string{
		"invalid uuid":       `{"assignee_id":"not-a-uuid"}`,
		"non-string value":   `{"assignee_id":42}`,
		"empty string":       `{"assignee_id":""}`,
		"uuid version not 4": `{"assignee_id":"00000000-0000-1000-8000-000000000001"}`,
	} {
		t.Run("PATCH assign rejects "+name+" (422 validation.failed)", func(t *testing.T) {
			issue := newIssue(t, &assignee)
			rec := assignRequestFor(t, router, issue.ID, tokenAssigner, body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
			}
			var parsed assignErrorBody
			_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
			if parsed.Error.Code != "validation.failed" {
				t.Errorf("code = %q, want validation.failed", parsed.Error.Code)
			}
		})
	}

	t.Run("PATCH assign with null requires authentication (401)", func(t *testing.T) {
		issue := newIssue(t, &assignee)
		rec := assignRequestFor(t, router, issue.ID, "", `{"assignee_id":null}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("PATCH assign with null on unknown issue returns 404", func(t *testing.T) {
		rec := assignRequestFor(t, router, uuid.New(), tokenAssigner, `{"assignee_id":null}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})

	t.Run("PATCH assign with null without edit permission returns 403", func(t *testing.T) {
		viewerRouter := chiWithService(usecase.NewIssueService(repo, dir, assignViewerResolver{}, &memorySavedFilterRepository{}), jwtSecret)
		issue := newIssue(t, &assignee)
		rec := assignRequestFor(t, viewerRouter, issue.ID, tokenAssigner, `{"assignee_id":null}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		stored, _ := repo.FindByID(context.Background(), issue.ID)
		if stored.AssigneeID == nil {
			t.Errorf("denied unassign must not clear the assignee")
		}
	})
}

// chiWithService monta el router de incidencias sobre un servicio concreto.
func chiWithService(service issueport.Service, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()
	issuehttp.NewHandler(service, jwtSecret).RegisterRoutes(r)
	return r
}
