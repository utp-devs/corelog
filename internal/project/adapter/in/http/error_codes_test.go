package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

type projectErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Details []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"details"`
	} `json:"error"`
}

func TestProjectHTTPErrorCodes(t *testing.T) {
	project := sampleProject(t)
	validBody := `{"key":"CORE","name":"Core","description":"x","type":"kanban","lead_id":"` + uuid.New().String() + `","visibility":"team"}`

	cases := []struct {
		name     string
		service  *fakeProjects
		method   string
		path     string
		body     string
		status   int
		wantCode string
	}{
		{
			name:     "duplicate key returns project.key_taken",
			service:  &fakeProjects{project: project, err: domain.ErrProjectKeyTaken},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusConflict,
			wantCode: "project.key_taken",
		},
		{
			name:     "missing project returns project.not_found",
			service:  &fakeProjects{project: project, err: domain.ErrProjectNotFound},
			method:   http.MethodGet,
			path:     "/projects/" + project.ID.String(),
			status:   http.StatusNotFound,
			wantCode: "project.not_found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := projectRouter(tc.service)
			rec := send(h, tc.method, tc.path, tc.body, bearer(t, "admin"))
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d; body %s", rec.Code, tc.status, rec.Body.String())
			}
			var env projectErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if env.Error.Code != tc.wantCode {
				t.Fatalf("code %q, want %q", env.Error.Code, tc.wantCode)
			}
		})
	}

	t.Run("invalid DTO returns validation.failed with details", func(t *testing.T) {
		h := projectRouter(&fakeProjects{project: project})
		bad := `{"key":"","name":"x","type":"waterfall","lead_id":"nope","visibility":"public"}`
		rec := send(h, http.MethodPost, "/projects", bad, bearer(t, "admin"))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422", rec.Code)
		}
		var env projectErrorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Error.Code != "validation.failed" {
			t.Fatalf("code %q, want validation.failed", env.Error.Code)
		}
		fields := map[string]bool{}
		for _, d := range env.Error.Details {
			fields[d.Field] = d.Message != ""
		}
		for _, want := range []string{"key", "name", "type", "lead_id", "visibility"} {
			if !fields[want] {
				t.Errorf("missing detail for %q; details %+v", want, env.Error.Details)
			}
		}
	})

}
