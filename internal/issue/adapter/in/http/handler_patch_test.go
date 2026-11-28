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

	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// roleAuthorizer decides with the default permission scheme from a fixed role per user.
type roleAuthorizer struct {
	roles map[uuid.UUID]authdomain.ProjectRole
}

func (a roleAuthorizer) role(actor uuid.UUID) *authdomain.ProjectRole {
	if r, ok := a.roles[actor]; ok {
		return &r
	}
	return nil
}

func (a roleAuthorizer) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, _ uuid.UUID) error {
	p, ok := authdomain.PermissionForAction(action)
	if !ok || !authdomain.Allowed(authdomain.DefaultScheme(), p, a.role(actor)) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

func (a roleAuthorizer) Can(_ context.Context, actor uuid.UUID, _ string, _ uuid.UUID, permission string, holders ...string) (bool, error) {
	var specials []authdomain.Holder
	for _, h := range holders {
		specials = append(specials, authdomain.Holder(h))
	}
	return authdomain.Allowed(authdomain.DefaultScheme(), authdomain.Permission(permission), a.role(actor), specials...), nil
}

func (a roleAuthorizer) Effective(_ context.Context, actor uuid.UUID, _ string, _ uuid.UUID) ([]string, error) {
	var out []string
	for _, p := range authdomain.Permissions {
		if authdomain.Allowed(authdomain.DefaultScheme(), p, a.role(actor)) {
			out = append(out, string(p))
		}
	}
	return out, nil
}

// softDeleteRepo adds the lifecycle port to the in-memory repository.
type softDeleteRepo struct {
	*memoryIssueRepo
	deleted map[uuid.UUID]bool
}

func (r *softDeleteRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Issue, error) {
	if r.deleted[id] {
		return nil, domain.ErrIssueNotFound
	}
	return r.memoryIssueRepo.FindByID(ctx, id)
}

func (r *softDeleteRepo) SoftDelete(_ context.Context, id uuid.UUID, _ time.Time) error {
	r.deleted[id] = true
	return nil
}

func (r *softDeleteRepo) Restore(_ context.Context, id uuid.UUID, _ time.Time) error {
	delete(r.deleted, id)
	return nil
}

func (r *softDeleteRepo) FindDeleted(ctx context.Context, id uuid.UUID) (*domain.Issue, error) {
	if !r.deleted[id] {
		return nil, domain.ErrIssueNotFound
	}
	return r.memoryIssueRepo.FindByID(ctx, id)
}

func (r *softDeleteRepo) LockForUpdate(context.Context, uuid.UUID) error { return nil }

func (r *softDeleteRepo) NextRank(context.Context, uuid.UUID) (string, error) {
	return "z", nil
}

func TestIssueHTTP_PatchDeleteAndView(t *testing.T) {
	const secret = "test-secret-key-that-is-long-enough-32-bytes"
	lead, contributor, viewer, outsider := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	auth := roleAuthorizer{roles: map[uuid.UUID]authdomain.ProjectRole{
		lead: authdomain.ProjectRoleLead, contributor: authdomain.ProjectRoleContributor, viewer: authdomain.ProjectRoleViewer,
	}}
	repo := &softDeleteRepo{memoryIssueRepo: newMemoryIssueRepo(), deleted: map[uuid.UUID]bool{}}
	dir := newMemoryDirectory()
	team := uuid.New()
	for _, id := range []uuid.UUID{lead, contributor, viewer} {
		dir.teams[id] = &team
	}
	service := usecase.NewIssueService(repo, dir, auth).WithPermissions(auth)
	router := chi.NewRouter()
	issuehttp.NewHandler(service, secret).RegisterRoutes(router)
	token := func(id uuid.UUID) string {
		tok, _ := authtoken.Generate(secret, time.Hour, id, "user")
		return tok
	}
	call := func(method, path, body string, as uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if as != uuid.Nil {
			req.Header.Set("Authorization", "Bearer "+token(as))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	type envelope struct {
		Data map[string]any `json:"data"`
		Err  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode := func(rec *httptest.ResponseRecorder) envelope {
		var e envelope
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		return e
	}

	created := decode(call(http.MethodPost, "/issues", `{"summary":"Tarea inicial","description":"Detalle","due_date":"2026-12-15","story_points":3}`, contributor))
	id, _ := created.Data["id"].(string)
	if id == "" {
		t.Fatalf("create failed: %+v", created)
	}
	updatedAt, _ := created.Data["updated_at"].(string)

	t.Run("401 without a token", func(t *testing.T) {
		if rec := call(http.MethodPatch, "/issues/"+id, `{"summary":"x"}`, uuid.Nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
		if rec := call(http.MethodDelete, "/issues/"+id, "", uuid.Nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("view model", func(t *testing.T) {
		rec := call(http.MethodGet, "/issues/"+id, "", lead)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		data := decode(rec).Data
		for _, key := range []string{"permissions", "available_transitions", "counts", "breadcrumb", "due_date", "start_date", "flagged", "original_estimate_seconds"} {
			if _, ok := data[key]; !ok {
				t.Errorf("view model misses %q: %v", key, data)
			}
		}
		if data["due_date"] != "2026-12-15" {
			t.Errorf("due_date = %v, want a plain date", data["due_date"])
		}
		if perms, _ := data["permissions"].([]any); len(perms) == 0 {
			t.Error("lead has no permissions")
		}
		counts, _ := data["counts"].(map[string]any)
		for _, key := range []string{"comments", "attachments", "children", "links", "watchers", "votes"} {
			if _, ok := counts[key]; !ok {
				t.Errorf("counts misses %q: %v", key, counts)
			}
		}
	})

	t.Run("partial patch with null clear and changes", func(t *testing.T) {
		rec := call(http.MethodPatch, "/issues/"+id, `{"due_date":null,"flagged":true,"original_estimate_seconds":7200}`, contributor)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		data := decode(rec).Data
		if data["due_date"] != nil || data["flagged"] != true || data["summary"] != "Tarea inicial" {
			t.Fatalf("data = %v", data)
		}
		changes, _ := data["changes"].([]any)
		if len(changes) != 3 || data["batch_id"] == nil {
			t.Fatalf("changes = %v batch = %v", changes, data["batch_id"])
		}
	})

	t.Run("422 for unknown fields, null summary and bad types", func(t *testing.T) {
		for _, body := range []string{`{"colour":"red"}`, `{"summary":null}`, `{"story_points":"mucho"}`, `{"story_points":0.3}`, `{"parent_id":"nope"}`, `[]`, `{"start_date":"2026-02-31"}`} {
			rec := call(http.MethodPatch, "/issues/"+id, body, lead)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422 (%s)", body, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("403 viewer, 404 outsider and missing issue", func(t *testing.T) {
		if rec := call(http.MethodPatch, "/issues/"+id, `{"summary":"x"}`, viewer); rec.Code != http.StatusForbidden {
			t.Fatalf("viewer status %d", rec.Code)
		}
		if rec := call(http.MethodPatch, "/issues/"+id, `{"summary":"x"}`, outsider); rec.Code != http.StatusNotFound {
			t.Fatalf("outsider status %d", rec.Code)
		}
		if rec := call(http.MethodPatch, "/issues/"+uuid.NewString(), `{"summary":"x"}`, lead); rec.Code != http.StatusNotFound {
			t.Fatalf("missing issue status %d", rec.Code)
		}
	})

	t.Run("409 stale precondition", func(t *testing.T) {
		// An hour older than the stored value is stale whatever the clock resolution or the speed of the machine.
		createdAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			t.Fatalf("parse updated_at %q: %v", updatedAt, err)
		}
		stale := createdAt.Add(-time.Hour).Format(time.RFC3339Nano)
		rec := call(http.MethodPatch, "/issues/"+id, `{"summary":"x","expected_updated_at":"`+stale+`"}`, lead)
		if rec.Code != http.StatusConflict || decode(rec).Err.Code != "issue.stale" {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("delete needs delete_issue and hides the issue", func(t *testing.T) {
		if rec := call(http.MethodDelete, "/issues/"+id, "", contributor); rec.Code != http.StatusForbidden {
			t.Fatalf("contributor delete status %d", rec.Code)
		}
		if rec := call(http.MethodDelete, "/issues/"+id, "", lead); rec.Code != http.StatusNoContent {
			t.Fatalf("lead delete status %d", rec.Code)
		}
		if rec := call(http.MethodGet, "/issues/"+id, "", lead); rec.Code != http.StatusNotFound {
			t.Fatalf("get after delete status %d", rec.Code)
		}
		if rec := call(http.MethodPost, "/issues/"+id+"/restore", "", contributor); rec.Code != http.StatusForbidden {
			t.Fatalf("contributor restore status %d", rec.Code)
		}
		if rec := call(http.MethodPost, "/issues/"+id+"/restore", "", lead); rec.Code != http.StatusOK {
			t.Fatalf("restore status %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("legacy details endpoint still replaces fields", func(t *testing.T) {
		rec := call(http.MethodPatch, "/issues/"+id+"/details", `{"summary":"Reemplazada","description":"Otra descripcion","due_date":"2026-12-24T00:00:00Z"}`, lead)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if data := decode(rec).Data; data["summary"] != "Reemplazada" || data["due_date"] != "2026-12-24" {
			t.Fatalf("data = %v", data)
		}
	})
}
