package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

// fakeRepo keeps teams and memberships in memory and applies the same
// organization scoping as the SQL adapter.
type fakeRepo struct {
	teams   map[uuid.UUID]*domain.Team
	members map[uuid.UUID]map[uuid.UUID]*domain.Member // teamID -> userID -> member
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{teams: map[uuid.UUID]*domain.Team{}, members: map[uuid.UUID]map[uuid.UUID]*domain.Member{}}
}

func copyTeam(t *domain.Team) *domain.Team { c := *t; return &c }

func (r *fakeRepo) Save(_ context.Context, t *domain.Team) error {
	for _, other := range r.teams {
		if other.Name == t.Name {
			return domain.ErrTeamNameTaken
		}
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	r.teams[t.ID] = copyTeam(t)
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Team, error) {
	t, ok := r.teams[id]
	if !ok {
		return nil, domain.ErrTeamNotFound
	}
	return copyTeam(t), nil
}

func (r *fakeRepo) FindByName(_ context.Context, name string) (*domain.Team, error) {
	for _, t := range r.teams {
		if t.Name == name {
			return copyTeam(t), nil
		}
	}
	return nil, domain.ErrTeamNotFound
}

func (r *fakeRepo) FindByOrganization(_ context.Context, orgID uuid.UUID) ([]*domain.Team, error) {
	var out []*domain.Team
	for _, t := range r.teams {
		if t.OrganizationID == orgID {
			out = append(out, copyTeam(t))
		}
	}
	return out, nil
}

func (r *fakeRepo) FindByOrganizationAndMember(_ context.Context, orgID, userID uuid.UUID) ([]*domain.Team, error) {
	var out []*domain.Team
	for _, t := range r.teams {
		if _, ok := r.members[t.ID][userID]; ok && t.OrganizationID == orgID {
			out = append(out, copyTeam(t))
		}
	}
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, t *domain.Team) error {
	cur, ok := r.teams[t.ID]
	if !ok || cur.OrganizationID != t.OrganizationID {
		return domain.ErrTeamNotFound
	}
	r.teams[t.ID] = copyTeam(t)
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, orgID, id uuid.UUID) error {
	cur, ok := r.teams[id]
	if !ok || cur.OrganizationID != orgID {
		return domain.ErrTeamNotFound
	}
	delete(r.teams, id)
	delete(r.members, id)
	return nil
}

func (r *fakeRepo) teamInOrg(orgID, teamID uuid.UUID) bool {
	t, ok := r.teams[teamID]
	return ok && t.OrganizationID == orgID
}

func (r *fakeRepo) FindMember(_ context.Context, orgID, teamID, userID uuid.UUID) (*domain.Member, error) {
	if !r.teamInOrg(orgID, teamID) {
		return nil, domain.ErrMemberNotFound
	}
	m, ok := r.members[teamID][userID]
	if !ok {
		return nil, domain.ErrMemberNotFound
	}
	c := *m
	return &c, nil
}

func (r *fakeRepo) ListMembers(_ context.Context, orgID, teamID uuid.UUID) ([]*domain.Member, error) {
	if !r.teamInOrg(orgID, teamID) {
		return nil, domain.ErrTeamNotFound
	}
	var out []*domain.Member
	for _, m := range r.members[teamID] {
		c := *m
		out = append(out, &c)
	}
	return out, nil
}

func (r *fakeRepo) AddMember(_ context.Context, orgID uuid.UUID, m *domain.Member) error {
	if !r.teamInOrg(orgID, m.TeamID) {
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

func (r *fakeRepo) UpdateMemberRole(_ context.Context, orgID, teamID, userID uuid.UUID, role string) error {
	m, ok := r.members[teamID][userID]
	if !ok || !r.teamInOrg(orgID, teamID) {
		return domain.ErrMemberNotFound
	}
	m.Role = role
	return nil
}

func (r *fakeRepo) RemoveMember(_ context.Context, orgID, teamID, userID uuid.UUID) error {
	if _, ok := r.members[teamID][userID]; !ok || !r.teamInOrg(orgID, teamID) {
		return domain.ErrMemberNotFound
	}
	delete(r.members[teamID], userID)
	return nil
}

// fakeOrgs is an in-memory OrgDirectory.
type fakeOrgs struct {
	roles    map[uuid.UUID]map[uuid.UUID]string // orgID -> userID -> role
	policies map[uuid.UUID]string
}

func newFakeOrgs() *fakeOrgs {
	return &fakeOrgs{roles: map[uuid.UUID]map[uuid.UUID]string{}, policies: map[uuid.UUID]string{}}
}

func (o *fakeOrgs) join(orgID, userID uuid.UUID, role string) {
	if o.roles[orgID] == nil {
		o.roles[orgID] = map[uuid.UUID]string{}
	}
	o.roles[orgID][userID] = role
}

func (o *fakeOrgs) MemberRole(_ context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	role, ok := o.roles[orgID][userID]
	return role, ok, nil
}

func (o *fakeOrgs) TeamCreationPolicy(_ context.Context, orgID uuid.UUID) (string, error) {
	if p, ok := o.policies[orgID]; ok {
		return p, nil
	}
	return domain.CreationPolicyMembers, nil
}

type fixture struct {
	svc      *TeamService
	repo     *fakeRepo
	orgs     *fakeOrgs
	orgA     uuid.UUID
	orgB     uuid.UUID
	owner    uuid.UUID // owner of org A
	admin    uuid.UUID // admin of org A
	member   uuid.UUID // member of org A
	guest    uuid.UUID // guest of org A
	outsider uuid.UUID // member of org B only
}

func newFixture() *fixture {
	f := &fixture{
		repo:     newFakeRepo(),
		orgs:     newFakeOrgs(),
		orgA:     uuid.New(),
		orgB:     uuid.New(),
		owner:    uuid.New(),
		admin:    uuid.New(),
		member:   uuid.New(),
		guest:    uuid.New(),
		outsider: uuid.New(),
	}
	f.orgs.join(f.orgA, f.owner, domain.OrgRoleOwner)
	f.orgs.join(f.orgA, f.admin, domain.OrgRoleAdmin)
	f.orgs.join(f.orgA, f.member, domain.OrgRoleMember)
	f.orgs.join(f.orgA, f.guest, domain.OrgRoleGuest)
	f.orgs.join(f.orgB, f.outsider, domain.OrgRoleOwner)
	f.svc = NewTeamService(f.repo, f.orgs, &database.FakeTxManager{})
	return f
}

// seedTeam creates a team in org with creator as maintainer, bypassing policy.
func (f *fixture) seedTeam(t *testing.T, orgID uuid.UUID, name string, maintainer uuid.UUID) *domain.Team {
	t.Helper()
	team, err := domain.NewTeam(domain.NewTeamInput{OrganizationID: orgID, CreatedBy: maintainer, Name: name})
	if err != nil {
		t.Fatalf("new team: %v", err)
	}
	if err := f.repo.Save(context.Background(), team); err != nil {
		t.Fatalf("save: %v", err)
	}
	m, _ := domain.NewMember(team.ID, maintainer, domain.RoleMaintainer)
	if err := f.repo.AddMember(context.Background(), orgID, m); err != nil {
		t.Fatalf("add member: %v", err)
	}
	return team
}

func (f *fixture) addTeamMember(t *testing.T, team *domain.Team, user uuid.UUID, role string) {
	t.Helper()
	m, _ := domain.NewMember(team.ID, user, role)
	if err := f.repo.AddMember(context.Background(), team.OrganizationID, m); err != nil {
		t.Fatalf("add team member: %v", err)
	}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	if got := apperror.CodeOf(err); got != code {
		t.Fatalf("code %q (err %v), want %q", got, err, code)
	}
}

func TestCreatePolicyMatrix(t *testing.T) {
	tests := []struct {
		name   string
		role   string // "" means not a member
		policy string
		allow  bool
	}{
		{name: "owner under members", role: domain.OrgRoleOwner, policy: domain.CreationPolicyMembers, allow: true},
		{name: "owner under admins", role: domain.OrgRoleOwner, policy: domain.CreationPolicyAdmins, allow: true},
		{name: "admin under members", role: domain.OrgRoleAdmin, policy: domain.CreationPolicyMembers, allow: true},
		{name: "admin under admins", role: domain.OrgRoleAdmin, policy: domain.CreationPolicyAdmins, allow: true},
		{name: "member under members", role: domain.OrgRoleMember, policy: domain.CreationPolicyMembers, allow: true},
		{name: "member under admins", role: domain.OrgRoleMember, policy: domain.CreationPolicyAdmins, allow: false},
		{name: "guest under members", role: domain.OrgRoleGuest, policy: domain.CreationPolicyMembers, allow: false},
		{name: "guest under admins", role: domain.OrgRoleGuest, policy: domain.CreationPolicyAdmins, allow: false},
		{name: "non member under members", role: "", policy: domain.CreationPolicyMembers, allow: false},
		{name: "non member under admins", role: "", policy: domain.CreationPolicyAdmins, allow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.orgs.policies[f.orgA] = tt.policy
			actor := uuid.New()
			if tt.role != "" {
				actor = uuid.New()
				f.orgs.join(f.orgA, actor, tt.role)
			}

			team, err := f.svc.Create(context.Background(), actor, port.CreateInput{OrganizationID: f.orgA, Name: "Equipo " + tt.name})
			if tt.allow {
				if err != nil {
					t.Fatalf("expected allowed, got %v", err)
				}
				if team.CreatedBy != actor {
					t.Errorf("creator not recorded")
				}
				m, err := f.repo.FindMember(context.Background(), f.orgA, team.ID, actor)
				if err != nil || m.Role != domain.RoleMaintainer {
					t.Fatalf("creator must be maintainer, got %v / %v", m, err)
				}
				return
			}
			if tt.role == "" {
				wantCode(t, err, "team.organization_not_found")
				return
			}
			wantCode(t, err, "team.creation_not_allowed")
		})
	}
}

func TestCreateDuplicateNameConflicts(t *testing.T) {
	f := newFixture()
	f.seedTeam(t, f.orgA, "Plataforma", f.owner)
	_, err := f.svc.Create(context.Background(), f.owner, port.CreateInput{OrganizationID: f.orgA, Name: "Plataforma"})
	wantCode(t, err, "team.name_taken")
}

func TestCreateRequiresOrganization(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Create(context.Background(), f.owner, port.CreateInput{Name: "X"})
	wantCode(t, err, "team.organization_required")
}

func TestCrossOrgIsolation(t *testing.T) {
	f := newFixture()
	teamB := f.seedTeam(t, f.orgB, "Equipo B", f.outsider)
	ctx := context.Background()

	// The owner of org A must not see, change or delete the team of org B.
	if _, err := f.svc.Get(ctx, f.owner, teamB.ID); err == nil || apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("Get across orgs: want NotFound, got %v", err)
	}
	if _, err := f.svc.Update(ctx, f.owner, teamB.ID, domain.TeamPatch{Name: strPtr("Hackeado")}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("Update across orgs: want NotFound, got %v", err)
	}
	if err := f.svc.Delete(ctx, f.owner, teamB.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("Delete across orgs: want NotFound, got %v", err)
	}
	if _, err := f.svc.ListMembers(ctx, f.owner, teamB.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("ListMembers across orgs: want NotFound, got %v", err)
	}
	if _, err := f.svc.AddMember(ctx, f.owner, teamB.ID, f.member, ""); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("AddMember across orgs: want NotFound, got %v", err)
	}
	wantCode(t, errOf(f.svc.List(ctx, f.owner, f.orgB, uuid.Nil)), "team.organization_not_found")

	// Lists are scoped to the requested organization.
	teams, err := f.svc.List(ctx, f.outsider, f.orgB, uuid.Nil)
	if err != nil || len(teams) != 1 {
		t.Fatalf("org B member list: %v / %d", err, len(teams))
	}
	f.seedTeam(t, f.orgA, "Equipo A", f.owner)
	teams, _ = f.svc.List(ctx, f.owner, f.orgA, uuid.Nil)
	for _, tm := range teams {
		if tm.OrganizationID != f.orgA {
			t.Fatalf("list leaked team of another org")
		}
	}
}

func TestManageAuthorization(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	team := f.seedTeam(t, f.orgA, "Producto", f.member) // member is maintainer here
	teamMemberOnly := uuid.New()
	f.orgs.join(f.orgA, teamMemberOnly, domain.OrgRoleMember)
	f.addTeamMember(t, team, teamMemberOnly, domain.RoleMember)

	// Org owner/admin: allowed without team membership.
	if _, err := f.svc.Update(ctx, f.admin, team.ID, domain.TeamPatch{Description: strPtr("ok")}); err != nil {
		t.Errorf("org admin update: %v", err)
	}
	// Team maintainer: allowed.
	if _, err := f.svc.Update(ctx, f.member, team.ID, domain.TeamPatch{Description: strPtr("ok")}); err != nil {
		t.Errorf("maintainer update: %v", err)
	}
	// Team member without maintainer role: Forbidden.
	_, err := f.svc.Update(ctx, teamMemberOnly, team.ID, domain.TeamPatch{Description: strPtr("no")})
	wantCode(t, err, "team.forbidden")
	// Org guest, even when visible: Forbidden.
	wantCode(t, f.svc.Delete(ctx, f.guest, team.ID), "team.forbidden")
	// Org member not in the team: Forbidden for management, allowed to read.
	other := uuid.New()
	f.orgs.join(f.orgA, other, domain.OrgRoleMember)
	wantCode(t, f.svc.Delete(ctx, other, team.ID), "team.forbidden")
	if _, err := f.svc.Get(ctx, other, team.ID); err != nil {
		t.Errorf("org member read: %v", err)
	}
	// Guests can read teams of their organization.
	if _, err := f.svc.Get(ctx, f.guest, team.ID); err != nil {
		t.Errorf("guest read: %v", err)
	}
}

func TestMemberManagement(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	team := f.seedTeam(t, f.orgA, "Diseño", f.owner)

	// Target must belong to the organization.
	_, err := f.svc.AddMember(ctx, f.owner, team.ID, f.outsider, "")
	wantCode(t, err, "team.user_not_in_org")

	// Invalid role.
	_, err = f.svc.AddMember(ctx, f.owner, team.ID, f.member, "owner")
	wantCode(t, err, "team.role_invalid")

	// Add with default role, then duplicate.
	m, err := f.svc.AddMember(ctx, f.owner, team.ID, f.member, "")
	if err != nil || m.Role != domain.RoleMember {
		t.Fatalf("add default role: %v / %v", m, err)
	}
	_, err = f.svc.AddMember(ctx, f.owner, team.ID, f.member, domain.RoleMaintainer)
	wantCode(t, err, "team.member_exists")

	// Change role and remove.
	m, err = f.svc.UpdateMemberRole(ctx, f.owner, team.ID, f.member, domain.RoleMaintainer)
	if err != nil || m.Role != domain.RoleMaintainer {
		t.Fatalf("role change: %v / %v", m, err)
	}
	if err := f.svc.RemoveMember(ctx, f.owner, team.ID, f.member); err != nil {
		t.Fatalf("remove: %v", err)
	}
	wantCode(t, f.svc.RemoveMember(ctx, f.owner, team.ID, f.member), "team.member_not_found")

	// Plain members cannot manage membership.
	f.addTeamMember(t, team, f.member, domain.RoleMember)
	_, err = f.svc.AddMember(ctx, f.member, team.ID, f.guest, "")
	wantCode(t, err, "team.forbidden")
}

func TestUpdateRenameConflict(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	f.seedTeam(t, f.orgA, "Uno", f.owner)
	two := f.seedTeam(t, f.orgA, "Dos", f.owner)

	_, err := f.svc.Update(ctx, f.owner, two.ID, domain.TeamPatch{Name: strPtr("Uno")})
	wantCode(t, err, "team.name_taken")

	// Keeping the same name is not a conflict.
	if _, err := f.svc.Update(ctx, f.owner, two.ID, domain.TeamPatch{Name: strPtr("Dos")}); err != nil {
		t.Errorf("same name: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func errOf[T any](_ T, err error) error { return err }
