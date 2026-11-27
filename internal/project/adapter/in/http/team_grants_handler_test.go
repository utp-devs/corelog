package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// teamFakeProjects extends the project fake with the team grant and member operations.
type teamFakeProjects struct {
	*fakeProjects
	grants  []port.TeamGrant
	entries []port.MemberEntry
	err     error
	removed uuid.UUID
	set     port.TeamGrant
}

func (f *teamFakeProjects) ListMembers(context.Context, uuid.UUID, string, uuid.UUID) ([]port.MemberEntry, error) {
	return f.entries, f.err
}

func (f *teamFakeProjects) ListTeamGrants(context.Context, uuid.UUID, string, uuid.UUID) ([]port.TeamGrant, error) {
	return f.grants, f.err
}

func (f *teamFakeProjects) SetTeamGrant(_ context.Context, _ uuid.UUID, _ string, projectID, teamID uuid.UUID, role string) (*port.TeamGrant, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.set = port.TeamGrant{ProjectID: projectID, TeamID: teamID, TeamName: "Core", Role: role, CreatedAt: time.Now().UTC()}
	return &f.set, nil
}

func (f *teamFakeProjects) RemoveTeamGrant(_ context.Context, _ uuid.UUID, _ string, _, teamID uuid.UUID) error {
	f.removed = teamID
	return f.err
}

func teamRouter(service *teamFakeProjects) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, service.fakeProjects, testSecret).RegisterRoutes(r)
	return r
}

func TestTeamGrantRoutesStatusCodes(t *testing.T) {
	project := sampleProject(t)
	team := uuid.New()
	base := "/projects/" + project.ID.String() + "/team-roles"
	auth := bearer(t, "user")

	service := &teamFakeProjects{fakeProjects: &fakeProjects{project: project}}
	h := teamRouter(service)

	service.grants = []port.TeamGrant{{ProjectID: project.ID, TeamID: team, TeamName: "Core", Role: "viewer", CreatedAt: time.Now()}}
	got := send(h, http.MethodGet, base, "", auth)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"team_name":"Core"`) {
		t.Fatalf("list team roles = %d %s", got.Code, got.Body.String())
	}

	body := `{"team_id":"` + team.String() + `","role":"contributor"}`
	if got := send(h, http.MethodPut, base, body, auth); got.Code != http.StatusOK {
		t.Fatalf("set team role = %d %s", got.Code, got.Body.String())
	}
	if service.set.TeamID != team || service.set.Role != "contributor" {
		t.Fatalf("set received %+v", service.set)
	}
	if got := send(h, http.MethodPut, base, `{"team_id":"nope","role":"viewer"}`, auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid team id = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodPut, base, `{"team_id":"`+team.String()+`","role":"owner"}`, auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid role = %d, want 422", got.Code)
	}
	if got := send(h, http.MethodPut, base, body, ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous set = %d, want 401", got.Code)
	}

	service.err = domain.ErrTeamNotInOrg
	if got := send(h, http.MethodPut, base, body, auth); got.Code != http.StatusUnprocessableEntity || !strings.Contains(got.Body.String(), "project.team_not_in_org") {
		t.Fatalf("team of another organization = %d %s", got.Code, got.Body.String())
	}
	service.err = nil

	if got := send(h, http.MethodDelete, base+"/"+team.String(), "", auth); got.Code != http.StatusNoContent {
		t.Fatalf("remove team role = %d %s", got.Code, got.Body.String())
	}
	if service.removed != team {
		t.Fatalf("removed team %s, want %s", service.removed, team)
	}
	service.err = domain.ErrTeamGrantNotFound
	if got := send(h, http.MethodDelete, base+"/"+team.String(), "", auth); got.Code != http.StatusNotFound {
		t.Fatalf("missing grant = %d, want 404", got.Code)
	}
	if got := send(h, http.MethodDelete, base+"/not-a-uuid", "", auth); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid team path = %d, want 422", got.Code)
	}
}

func TestMembersResponseFlagsTeamUsers(t *testing.T) {
	project := sampleProject(t)
	team := uuid.New()
	viaTeam := uuid.New()
	service := &teamFakeProjects{
		fakeProjects: &fakeProjects{project: project},
		entries: []port.MemberEntry{
			{Member: port.Member{ProjectID: project.ID, UserID: uuid.New(), Role: "lead"}, EffectiveRole: "lead"},
			{Member: port.Member{ProjectID: project.ID, UserID: viaTeam, Role: "contributor"}, EffectiveRole: "contributor", ViaTeam: true, TeamID: &team, TeamName: "Core"},
		},
	}
	got := send(teamRouter(service), http.MethodGet, "/projects/"+project.ID.String()+"/members", "", bearer(t, "user"))
	if got.Code != http.StatusOK {
		t.Fatalf("members = %d %s", got.Code, got.Body.String())
	}
	var env struct {
		Data []projectMemberResponse `json:"data"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("members = %d", len(env.Data))
	}
	if env.Data[0].ViaTeam || env.Data[0].EffectiveRole != "lead" {
		t.Fatalf("direct member = %+v", env.Data[0])
	}
	teamUser := env.Data[1]
	if !teamUser.ViaTeam || teamUser.TeamID == nil || *teamUser.TeamID != team.String() || teamUser.TeamName != "Core" || teamUser.EffectiveRole != "contributor" {
		t.Fatalf("team member = %+v", teamUser)
	}
	if teamUser.CreatedAt != nil {
		t.Fatalf("team-only member must not carry created_at: %+v", teamUser)
	}
}
