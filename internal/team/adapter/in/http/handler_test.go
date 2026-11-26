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

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/database"
	teamhttp "github.com/jhonsferg/corelog/internal/team/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/team/application/usecase"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

const testSecret = "test-secret-key-that-is-long-enough-32-bytes"

// memRepo is a small in-memory port.Repository with the same org scoping as SQL.
type memRepo struct {
	teams   map[uuid.UUID]*domain.Team
	members map[uuid.UUID]map[uuid.UUID]*domain.Member
}

func newMemRepo() *memRepo {
	return &memRepo{teams: map[uuid.UUID]*domain.Team{}, members: map[uuid.UUID]map[uuid.UUID]*domain.Member{}}
}

func (r *memRepo) Save(_ context.Context, t *domain.Team) error {
	for _, o := range r.teams {
		if o.Name == t.Name {
			return domain.ErrTeamNameTaken
		}
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	c := *t
	r.teams[t.ID] = &c
	return nil
}

func (r *memRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Team, error) {
	t, ok := r.teams[id]
	if !ok {
		return nil, domain.ErrTeamNotFound
	}
	c := *t
	return &c, nil
}

func (r *memRepo) FindByName(_ context.Context, name string) (*domain.Team, error) {
	for _, t := range r.teams {
		if t.Name == name {
			c := *t
			return &c, nil
		}
	}
	return nil, domain.ErrTeamNotFound
}

func (r *memRepo) FindByOrganization(_ context.Context, orgID uuid.UUID) ([]*domain.Team, error) {
	var out []*domain.Team
	for _, t := range r.teams {
		if t.OrganizationID == orgID {
			c := *t
			out = append(out, &c)
		}
	}
	return out, nil
}

func (r *memRepo) FindByOrganizationAndMember(_ context.Context, orgID, userID uuid.UUID) ([]*domain.Team, error) {
	var out []*domain.Team
	for _, t := range r.teams {
		if _, ok := r.members[t.ID][userID]; ok && t.OrganizationID == orgID {
			c := *t
			out = append(out, &c)
		}
	}
	return out, nil
}

func (r *memRepo) Update(_ context.Context, t *domain.Team) error {
	cur, ok := r.teams[t.ID]
	if !ok || cur.OrganizationID != t.OrganizationID {
		return domain.ErrTeamNotFound
	}
	c := *t
	r.teams[t.ID] = &c
	return nil
}

func (r *memRepo) Delete(_ context.Context, orgID, id uuid.UUID) error {
	cur, ok := r.teams[id]
	if !ok || cur.OrganizationID != orgID {
		return domain.ErrTeamNotFound
	}
	delete(r.teams, id)
	return nil
}

func (r *memRepo) FindMember(_ context.Context, orgID, teamID, userID uuid.UUID) (*domain.Member, error) {
	if t, ok := r.teams[teamID]; !ok || t.OrganizationID != orgID {
		return nil, domain.ErrMemberNotFound
	}
	m, ok := r.members[teamID][userID]
	if !ok {
		return nil, domain.ErrMemberNotFound
	}
	c := *m
	return &c, nil
}

func (r *memRepo) ListMembers(_ context.Context, orgID, teamID uuid.UUID) ([]*domain.Member, error) {
	if t, ok := r.teams[teamID]; !ok || t.OrganizationID != orgID {
		return nil, domain.ErrTeamNotFound
	}
	var out []*domain.Member
	for _, m := range r.members[teamID] {
		c := *m
		out = append(out, &c)
	}
	return out, nil
}

func (r *memRepo) AddMember(_ context.Context, orgID uuid.UUID, m *domain.Member) error {
	if t, ok := r.teams[m.TeamID]; !ok || t.OrganizationID != orgID {
		return domain.ErrTeamNotFound
	}
	if r.members[m.TeamID] == nil {
		r.members[m.TeamID] = map[uuid.UUID]*domain.Member{}
	}
	if _, ok := r.members[m.TeamID][m.UserID]; ok {
		return domain.ErrMemberExists
	}
	c := *m
	r.members[m.TeamID][m.UserID] = &c
	return nil
}

func (r *memRepo) UpdateMemberRole(_ context.Context, orgID, teamID, userID uuid.UUID, role string) error {
	m, ok := r.members[teamID][userID]
	if !ok {
		return domain.ErrMemberNotFound
	}
	if t, ok := r.teams[teamID]; !ok || t.OrganizationID != orgID {
		return domain.ErrMemberNotFound
	}
	m.Role = role
	return nil
}

func (r *memRepo) RemoveMember(_ context.Context, orgID, teamID, userID uuid.UUID) error {
	if _, ok := r.members[teamID][userID]; !ok {
		return domain.ErrMemberNotFound
	}
	if t, ok := r.teams[teamID]; !ok || t.OrganizationID != orgID {
		return domain.ErrMemberNotFound
	}
	delete(r.members[teamID], userID)
	return nil
}

// stubOrgs answers OrgDirectory from a fixed table.
type stubOrgs struct {
	roles  map[uuid.UUID]map[uuid.UUID]string
	policy string
}

func (o stubOrgs) MemberRole(_ context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	role, ok := o.roles[orgID][userID]
	return role, ok, nil
}

func (o stubOrgs) TeamCreationPolicy(context.Context, uuid.UUID) (string, error) {
	return o.policy, nil
}

type fixture struct {
	router    *chi.Mux
	repo      *memRepo
	orgID     uuid.UUID
	ownerTok  string
	memberTok string
	guestTok  string
	ownerID   uuid.UUID
	memberID  uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{repo: newMemRepo(), orgID: uuid.New(), ownerID: uuid.New(), memberID: uuid.New()}
	guestID := uuid.New()
	orgs := stubOrgs{
		policy: domain.CreationPolicyMembers,
		roles: map[uuid.UUID]map[uuid.UUID]string{
			f.orgID: {f.ownerID: domain.OrgRoleOwner, f.memberID: domain.OrgRoleMember, guestID: domain.OrgRoleGuest},
		},
	}
	service := usecase.NewTeamService(f.repo, orgs, &database.FakeTxManager{})
	handler := teamhttp.NewHandler(service, testSecret)
	f.router = chi.NewRouter()
	handler.RegisterRoutes(f.router)

	f.ownerTok, _ = authtoken.Generate(testSecret, time.Hour, f.ownerID, "user")
	f.memberTok, _ = authtoken.Generate(testSecret, time.Hour, f.memberID, "user")
	f.guestTok, _ = authtoken.Generate(testSecret, time.Hour, guestID, "user")
	return f
}

func (f *fixture) do(method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v (%s)", err, rec.Body.String())
	}
	return env.Error.Code
}

func TestTeamHTTP_Handlers(t *testing.T) {
	f := newFixture(t)
	createBody := `{"organization_id":"` + f.orgID.String() + `","name":"Engineering"}`

	t.Run("requires authentication (401)", func(t *testing.T) {
		if rec := f.do(http.MethodPost, "/teams", createBody, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("status %d, want 401", rec.Code)
		}
	})

	t.Run("guest cannot create (403)", func(t *testing.T) {
		rec := f.do(http.MethodPost, "/teams", createBody, f.guestTok)
		if rec.Code != http.StatusForbidden || errorCode(t, rec) != "team.creation_not_allowed" {
			t.Errorf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("org member creates team (201) with new fields", func(t *testing.T) {
		rec := f.do(http.MethodPost, "/teams", `{"organization_id":"`+f.orgID.String()+`","name":"Producto","description":"Roadmap","avatar_color":"#3366FF"}`, f.memberTok)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		var env struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		body := env.Data
		if body["organization_id"] != f.orgID.String() || body["description"] != "Roadmap" || body["avatar_color"] != "#3366FF" || body["created_by"] != f.memberID.String() {
			t.Errorf("unexpected body %v", body)
		}
	})

	t.Run("unknown fields are rejected (422)", func(t *testing.T) {
		rec := f.do(http.MethodPost, "/teams", `{"organization_id":"`+f.orgID.String()+`","name":"X Team","extra":1}`, f.ownerTok)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d", rec.Code)
		}
	})

	t.Run("invalid body (422)", func(t *testing.T) {
		rec := f.do(http.MethodPost, "/teams", `{"organization_id":"`+f.orgID.String()+`","name":""}`, f.ownerTok)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d", rec.Code)
		}
	})

	t.Run("duplicate name returns team.name_taken (409)", func(t *testing.T) {
		if rec := f.do(http.MethodPost, "/teams", createBody, f.ownerTok); rec.Code != http.StatusCreated {
			t.Fatalf("first create status %d: %s", rec.Code, rec.Body.String())
		}
		rec := f.do(http.MethodPost, "/teams", createBody, f.ownerTok)
		if rec.Code != http.StatusConflict || errorCode(t, rec) != "team.name_taken" {
			t.Errorf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("list requires organization_id (422)", func(t *testing.T) {
		rec := f.do(http.MethodGet, "/teams", "", f.ownerTok)
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "team.organization_required" {
			t.Errorf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("list of own organization (200)", func(t *testing.T) {
		rec := f.do(http.MethodGet, "/teams?organization_id="+f.orgID.String(), "", f.guestTok)
		if rec.Code != http.StatusOK {
			t.Errorf("status %d", rec.Code)
		}
	})

	t.Run("list of foreign organization (404)", func(t *testing.T) {
		rec := f.do(http.MethodGet, "/teams?organization_id="+uuid.New().String(), "", f.ownerTok)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d", rec.Code)
		}
	})

	t.Run("get unknown team (404)", func(t *testing.T) {
		rec := f.do(http.MethodGet, "/teams/"+uuid.New().String(), "", f.ownerTok)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d", rec.Code)
		}
	})

	t.Run("patch and delete by team maintainer or org owner", func(t *testing.T) {
		team, _ := domain.NewTeam(domain.NewTeamInput{OrganizationID: f.orgID, Name: "Diseno"})
		_ = f.repo.Save(context.Background(), team)
		_ = f.repo.AddMember(context.Background(), f.orgID, &domain.Member{TeamID: team.ID, UserID: f.memberID, Role: domain.RoleMaintainer})

		rec := f.do(http.MethodPatch, "/teams/"+team.ID.String(), `{"description":"nuevo"}`, f.memberTok)
		if rec.Code != http.StatusOK {
			t.Fatalf("patch status %d: %s", rec.Code, rec.Body.String())
		}
		if rec := f.do(http.MethodPatch, "/teams/"+team.ID.String(), `{"avatar_color":"rojo"}`, f.ownerTok); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("bad color status %d", rec.Code)
		}
		if rec := f.do(http.MethodDelete, "/teams/"+team.ID.String(), "", f.guestTok); rec.Code != http.StatusForbidden {
			t.Errorf("guest delete status %d", rec.Code)
		}
		if rec := f.do(http.MethodDelete, "/teams/"+team.ID.String(), "", f.ownerTok); rec.Code != http.StatusNoContent {
			t.Errorf("owner delete status %d", rec.Code)
		}
	})

	t.Run("member endpoints: add, role change, remove", func(t *testing.T) {
		team, _ := domain.NewTeam(domain.NewTeamInput{OrganizationID: f.orgID, Name: "Soporte"})
		_ = f.repo.Save(context.Background(), team)
		_ = f.repo.AddMember(context.Background(), f.orgID, &domain.Member{TeamID: team.ID, UserID: f.ownerID, Role: domain.RoleMaintainer})
		base := "/teams/" + team.ID.String() + "/members"

		rec := f.do(http.MethodPost, base, `{"user_id":"`+f.memberID.String()+`","role":"member"}`, f.ownerTok)
		if rec.Code != http.StatusCreated {
			t.Fatalf("add status %d: %s", rec.Code, rec.Body.String())
		}
		if rec := f.do(http.MethodPost, base, `{"user_id":"`+uuid.New().String()+`"}`, f.ownerTok); errorCode(t, rec) != "team.user_not_in_org" {
			t.Errorf("outsider add body %s", rec.Body.String())
		}
		if rec := f.do(http.MethodPatch, base+"/"+f.memberID.String(), `{"role":"maintainer"}`, f.ownerTok); rec.Code != http.StatusOK {
			t.Errorf("role change status %d", rec.Code)
		}
		if rec := f.do(http.MethodGet, base, "", f.memberTok); rec.Code != http.StatusOK {
			t.Errorf("list members status %d", rec.Code)
		}
		if rec := f.do(http.MethodDelete, base+"/"+f.memberID.String(), "", f.ownerTok); rec.Code != http.StatusNoContent {
			t.Errorf("remove status %d", rec.Code)
		}
	})
}

func TestTeamListMemberFilterHTTP(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mine, _ := domain.NewTeam(domain.NewTeamInput{OrganizationID: f.orgID, Name: "Con miembro"})
	_ = f.repo.Save(ctx, mine)
	_ = f.repo.AddMember(ctx, f.orgID, &domain.Member{TeamID: mine.ID, UserID: f.memberID, Role: domain.RoleMember})
	other, _ := domain.NewTeam(domain.NewTeamInput{OrganizationID: f.orgID, Name: "Sin miembro"})
	_ = f.repo.Save(ctx, other)

	listIDs := func(t *testing.T, rec *httptest.ResponseRecorder) []string {
		t.Helper()
		var body struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
		ids := make([]string, 0, len(body.Data))
		for _, d := range body.Data {
			ids = append(ids, d.ID)
		}
		return ids
	}

	rec := f.do(http.MethodGet, "/teams?organization_id="+f.orgID.String()+"&member_id="+f.memberID.String(), "", f.ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("filter status %d: %s", rec.Code, rec.Body.String())
	}
	if ids := listIDs(t, rec); len(ids) != 1 || ids[0] != mine.ID.String() {
		t.Fatalf("filtered ids = %v, want [%s]", ids, mine.ID)
	}

	// A guest of the organization may use the filter; an outsider gets the organization 404.
	rec = f.do(http.MethodGet, "/teams?organization_id="+f.orgID.String()+"&member_id="+f.memberID.String(), "", f.guestTok)
	if rec.Code != http.StatusOK || len(listIDs(t, rec)) != 1 {
		t.Fatalf("guest filter status %d: %s", rec.Code, rec.Body.String())
	}
	rec = f.do(http.MethodGet, "/teams?organization_id="+uuid.New().String()+"&member_id="+f.memberID.String(), "", f.ownerTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign org filter status %d", rec.Code)
	}

	rec = f.do(http.MethodGet, "/teams?organization_id="+f.orgID.String()+"&member_id=nope", "", f.ownerTok)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "team.member_id_invalid" {
		t.Fatalf("invalid member_id: status %d body %s", rec.Code, rec.Body.String())
	}
}
