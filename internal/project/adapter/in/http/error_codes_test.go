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
	validBody := `{"organization_id":"` + project.OrganizationID.String() + `","key":"CORE","name":"Core","description":"x","type":"kanban","visibility":"open"}`
	memberPath := "/projects/" + project.ID.String() + "/members"
	memberBody := `{"user_id":"` + uuid.New().String() + `","role":"viewer"}`

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
			name:     "creation not allowed by org policy returns project.creation_not_allowed",
			service:  &fakeProjects{project: project, err: domain.ErrProjectCreationNotAllowed},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusForbidden,
			wantCode: "project.creation_not_allowed",
		},
		{
			name:     "non member of the organization returns organization.not_found",
			service:  &fakeProjects{project: project, err: domain.ErrOrganizationNotFound},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusNotFound,
			wantCode: "organization.not_found",
		},
		{
			name:     "missing project returns project.not_found",
			service:  &fakeProjects{project: project, err: domain.ErrProjectNotFound},
			method:   http.MethodGet,
			path:     "/projects/" + project.ID.String(),
			status:   http.StatusNotFound,
			wantCode: "project.not_found",
		},
		{
			name:     "member outside the organization returns project.user_not_in_org",
			service:  &fakeProjects{project: project, err: domain.ErrUserNotInOrg},
			method:   http.MethodPost,
			path:     memberPath,
			body:     memberBody,
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.user_not_in_org",
		},
		{
			name:     "non existent user returns project.user_not_found, not 500",
			service:  &fakeProjects{project: project, err: domain.ErrUserNotFound},
			method:   http.MethodPost,
			path:     memberPath,
			body:     memberBody,
			status:   http.StatusNotFound,
			wantCode: "project.user_not_found",
		},
		{
			name:     "invalid key returns project.invalid_key",
			service:  &fakeProjects{project: project, err: domain.ErrInvalidKey},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.invalid_key",
		},
		{
			name:     "unknown template returns project.invalid_template",
			service:  &fakeProjects{project: project, err: domain.ErrInvalidTemplate},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.invalid_template",
		},
		{
			name:     "unknown assignee policy returns project.invalid_assignee_policy",
			service:  &fakeProjects{project: project, err: domain.ErrInvalidAssigneePolicy},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.invalid_assignee_policy",
		},
		{
			name:     "team outside the organization returns project.team_not_in_org",
			service:  &fakeProjects{project: project, err: domain.ErrTeamNotInOrg},
			method:   http.MethodPost,
			path:     "/projects",
			body:     validBody,
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.team_not_in_org",
		},
		{
			name:     "archiving an archived project returns project.already_archived",
			service:  &fakeProjects{project: project, err: domain.ErrProjectAlreadyArchived},
			method:   http.MethodPost,
			path:     "/projects/" + project.ID.String() + "/archive",
			status:   http.StatusConflict,
			wantCode: "project.already_archived",
		},
		{
			name:     "restoring an active project returns project.not_archived",
			service:  &fakeProjects{project: project, err: domain.ErrProjectNotArchived},
			method:   http.MethodPost,
			path:     "/projects/" + project.ID.String() + "/restore",
			status:   http.StatusConflict,
			wantCode: "project.not_archived",
		},
		{
			name:     "changes on an archived project return project.archived",
			service:  &fakeProjects{project: project, err: domain.ErrProjectArchived},
			method:   http.MethodPost,
			path:     "/projects/" + project.ID.String() + "/archive",
			status:   http.StatusConflict,
			wantCode: "project.archived",
		},
		{
			name:     "delete with a wrong confirmation returns project.confirmation_mismatch",
			service:  &fakeProjects{project: project, err: domain.ErrConfirmationMismatch},
			method:   http.MethodDelete,
			path:     "/projects/" + project.ID.String(),
			body:     `{"confirmation":"other"}`,
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.confirmation_mismatch",
		},
		{
			name:     "delete of a project outside the caller organizations returns project.not_found",
			service:  &fakeProjects{project: project, err: domain.ErrProjectNotFound},
			method:   http.MethodDelete,
			path:     "/projects/" + project.ID.String(),
			body:     `{"confirmation":"Core"}`,
			status:   http.StatusNotFound,
			wantCode: "project.not_found",
		},
		{
			name:     "key availability with an invalid key returns project.invalid_key",
			service:  &fakeProjects{project: project, err: domain.ErrInvalidKey},
			method:   http.MethodGet,
			path:     "/projects/key-available?organization_id=" + project.OrganizationID.String() + "&key=1X",
			status:   http.StatusUnprocessableEntity,
			wantCode: "project.invalid_key",
		},
		{
			name:     "listing a foreign organization returns organization.not_found",
			service:  &fakeProjects{project: project, err: domain.ErrOrganizationNotFound},
			method:   http.MethodGet,
			path:     "/projects?organization_id=" + project.OrganizationID.String(),
			status:   http.StatusNotFound,
			wantCode: "organization.not_found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := projectRouter(tc.service)
			rec := send(h, tc.method, tc.path, tc.body, bearer(t, "user"))
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
		bad := `{"organization_id":"nope","key":"","name":"x","type":"waterfall","visibility":"public"}`
		rec := send(h, http.MethodPost, "/projects", bad, bearer(t, "user"))
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
		for _, want := range []string{"organization_id", "key", "name", "type", "visibility"} {
			if !fields[want] {
				t.Errorf("missing detail for %q; details %+v", want, env.Error.Details)
			}
		}
	})
}
