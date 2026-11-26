package http_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestTeamHTTP_ErrorCodes checks the stable team.* codes returned to clients.
func TestTeamHTTP_ErrorCodes(t *testing.T) {
	f := newFixture(t)
	orgID := f.orgID.String()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
		status int
		code   string
	}{
		{"unknown organization on list", http.MethodGet, "/teams?organization_id=" + uuid.New().String(), "", f.ownerTok, http.StatusNotFound, "team.organization_not_found"},
		{"invalid color", http.MethodPost, "/teams", `{"organization_id":"` + orgID + `","name":"Color","avatar_color":"zzz"}`, f.ownerTok, http.StatusUnprocessableEntity, ""},
		{"creation not allowed for guest", http.MethodPost, "/teams", `{"organization_id":"` + orgID + `","name":"Guest"}`, f.guestTok, http.StatusForbidden, "team.creation_not_allowed"},
		{"team not found", http.MethodGet, "/teams/" + uuid.New().String(), "", f.ownerTok, http.StatusNotFound, "team.not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.do(tc.method, tc.path, tc.body, tc.token)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.code != "" {
				if got := errorCode(t, rec); got != tc.code {
					t.Fatalf("code %q, want %q", got, tc.code)
				}
			}
		})
	}
}
