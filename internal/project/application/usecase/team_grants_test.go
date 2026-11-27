package usecase

import (
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// memProjects also implements the team grant repository over the team state of the fake.

func (m *memProjects) ListTeamGrants(_ context.Context, projectID uuid.UUID) ([]port.TeamGrant, error) {
	out := []port.TeamGrant{}
	for team, role := range m.teamRoles[projectID] {
		out = append(out, port.TeamGrant{ProjectID: projectID, TeamID: team, TeamName: m.teamName[team], Role: role})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeamName < out[j].TeamName })
	return out, nil
}

func (m *memProjects) ListTeamUsers(_ context.Context, projectID uuid.UUID) ([]port.TeamUserGrant, error) {
	project := m.projects[projectID]
	if project == nil {
		return nil, nil
	}
	var out []port.TeamUserGrant
	for team, role := range m.teamRoles[projectID] {
		for user := range m.teamUsers[team] {
			if !m.orgMemberOf(project.OrganizationID, user) {
				continue
			}
			out = append(out, port.TeamUserGrant{UserID: user, TeamID: team, TeamName: m.teamName[team], Role: role})
		}
	}
	return out, nil
}

func (m *memProjects) TeamInOrganization(_ context.Context, orgID, teamID uuid.UUID) (string, bool, error) {
	if m.teamOrg[teamID] != orgID {
		return "", false, nil
	}
	return m.teamName[teamID], true, nil
}

func (m *memProjects) UpsertTeamGrant(_ context.Context, grant port.TeamGrant) error {
	if m.teamRoles[grant.ProjectID] == nil {
		m.teamRoles[grant.ProjectID] = map[uuid.UUID]string{}
	}
	m.teamRoles[grant.ProjectID][grant.TeamID] = grant.Role
	return nil
}

func (m *memProjects) RemoveTeamGrant(_ context.Context, projectID, teamID uuid.UUID) error {
	if _, ok := m.teamRoles[projectID][teamID]; !ok {
		return domain.ErrTeamGrantNotFound
	}
	delete(m.teamRoles[projectID], teamID)
	return nil
}

// newTeam creates a team of the organization with the given members.
func (f *fixture) newTeam(orgID uuid.UUID, name string, users ...uuid.UUID) uuid.UUID {
	team := uuid.New()
	f.repo.teamOrg[team] = orgID
	f.repo.teamName[team] = name
	f.repo.teamUsers[team] = map[uuid.UUID]bool{}
	for _, user := range users {
		f.repo.teamUsers[team][user] = true
	}
	return team
}

// TestTeamGrantGivesAccessToPrivateProject: a private project is invisible until a team is granted;
// the team viewer reads it but cannot manage members.
func TestTeamGrantGivesAccessToPrivateProject(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "TGA", domain.VisibilityPrivate)
	team := f.newTeam(f.orgA, "Payments", f.member)

	_, err := f.svc.Get(f.ctx, f.member, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")

	grant, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, team, "viewer")
	if err != nil {
		t.Fatalf("SetTeamGrant: %v", err)
	}
	if grant.TeamName != "Payments" || grant.Role != "viewer" {
		t.Fatalf("grant = %+v", grant)
	}
	if _, err := f.svc.Get(f.ctx, f.member, "requester", p.ID); err != nil {
		t.Fatalf("team viewer must read the project: %v", err)
	}
	_, err = f.svc.AddMember(f.ctx, f.member, "requester", p.ID, f.guest, "viewer")
	wantError(t, err, apperror.KindForbidden, "")
}

// TestTeamLeadGrantManagesMembers: the best role wins, so a team lead grant lets a plain member
// manage members, and removing the grant takes the rights back.
func TestTeamLeadGrantManagesMembers(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "TLD", domain.VisibilityPrivate)
	team := f.newTeam(f.orgA, "Leads", f.member)

	if _, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, team, "lead"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddMember(f.ctx, f.member, "requester", p.ID, f.guest, "viewer"); err != nil {
		t.Fatalf("team lead must add members: %v", err)
	}
	if err := f.svc.RemoveTeamGrant(f.ctx, f.owner, "requester", p.ID, team); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.AddMember(f.ctx, f.member, "requester", p.ID, f.admin, "viewer")
	wantError(t, err, apperror.KindNotFound, "project.not_found")
}

func TestTeamGrantRules(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "TGR", domain.VisibilityOpen)
	foreign := f.newTeam(f.orgB, "Other org", f.outsider)
	local := f.newTeam(f.orgA, "Local", f.member)

	_, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, foreign, "contributor")
	wantCode(t, err, "project.team_not_in_org")

	_, err = f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, local, "owner")
	wantError(t, err, apperror.KindValidation, "")

	// A contributor cannot manage team grants.
	f.addExplicit(t, p.ID, f.guest, "contributor")
	_, err = f.svc.SetTeamGrant(f.ctx, f.guest, "requester", p.ID, local, "viewer")
	wantError(t, err, apperror.KindForbidden, "")
	_, err = f.svc.ListTeamGrants(f.ctx, f.guest, "requester", p.ID)
	wantError(t, err, apperror.KindForbidden, "")

	// Upsert replaces the role of the same team.
	if _, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, local, "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, local, "contributor"); err != nil {
		t.Fatal(err)
	}
	grants, err := f.svc.ListTeamGrants(f.ctx, f.owner, "requester", p.ID)
	if err != nil || len(grants) != 1 || grants[0].Role != "contributor" {
		t.Fatalf("ListTeamGrants = %+v, %v; want one contributor grant", grants, err)
	}

	err = f.svc.RemoveTeamGrant(f.ctx, f.owner, "requester", p.ID, foreign)
	wantCode(t, err, "project.team_grant_not_found")
	if err := f.svc.RemoveTeamGrant(f.ctx, f.owner, "requester", p.ID, local); err != nil {
		t.Fatalf("RemoveTeamGrant: %v", err)
	}
}

// TestListMembersIncludesTeamUsers: direct members come first with their effective role; users
// reached only through a team are flagged via_team with the team that gives the role.
func TestListMembersIncludesTeamUsers(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "LMT", domain.VisibilityOpen)
	f.addExplicit(t, p.ID, f.member, "viewer")
	team := f.newTeam(f.orgA, "Core", f.member, f.guest)
	if _, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, team, "contributor"); err != nil {
		t.Fatal(err)
	}

	members, err := f.svc.ListMembers(f.ctx, f.owner, "requester", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	byUser := map[uuid.UUID]port.MemberEntry{}
	for _, m := range members {
		byUser[m.UserID] = m
	}
	if got := byUser[f.owner]; got.Role != "lead" || got.EffectiveRole != "lead" || got.ViaTeam {
		t.Fatalf("owner entry = %+v", got)
	}
	if got := byUser[f.member]; got.Role != "viewer" || got.EffectiveRole != "contributor" || got.ViaTeam {
		t.Fatalf("direct member with team grant = %+v, want role viewer and effective contributor", got)
	}
	guest, ok := byUser[f.guest]
	if !ok {
		t.Fatalf("team-only user missing: %+v", members)
	}
	if !guest.ViaTeam || guest.TeamName != "Core" || guest.TeamID == nil || *guest.TeamID != team || guest.EffectiveRole != "contributor" {
		t.Fatalf("team-only entry = %+v", guest)
	}
	if len(members) != 3 {
		t.Fatalf("members = %d, want 3", len(members))
	}
}

// TestTeamUsersOutsideOrganizationAreNotListed: a team member who left the organization is not shown
// and gets no access through the team.
func TestTeamUsersOutsideOrganizationAreNotListed(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "TUO", domain.VisibilityPrivate)
	team := f.newTeam(f.orgA, "Ghosts", f.outsider)
	if _, err := f.svc.SetTeamGrant(f.ctx, f.owner, "requester", p.ID, team, "lead"); err != nil {
		t.Fatal(err)
	}
	members, err := f.svc.ListMembers(f.ctx, f.owner, "requester", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.UserID == f.outsider {
			t.Fatalf("outsider listed as member: %+v", m)
		}
	}
	_, err = f.svc.Get(f.ctx, f.outsider, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")
}

// TestLastLeadRules: the last direct lead cannot be removed or demoted, not even by oneself.
func TestLastLeadRules(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "LLR", domain.VisibilityOpen)
	f.addExplicit(t, p.ID, f.member, "lead")

	// With two leads, one may be demoted or removed.
	if _, err := f.svc.UpdateMemberRole(f.ctx, f.owner, "requester", p.ID, f.member, "viewer"); err != nil {
		t.Fatalf("demote with another lead left: %v", err)
	}
	f.addExplicit(t, p.ID, f.member, "lead")
	if err := f.svc.RemoveMember(f.ctx, f.owner, "requester", p.ID, f.member); err != nil {
		t.Fatalf("remove with another lead left: %v", err)
	}

	// The owner is now the only direct lead: demoting or removing self fails.
	_, err := f.svc.UpdateMemberRole(f.ctx, f.owner, "requester", p.ID, f.owner, "viewer")
	wantCode(t, err, "project.last_lead")
	err = f.svc.RemoveMember(f.ctx, f.owner, "requester", p.ID, f.owner)
	wantCode(t, err, "project.last_lead")

	// Changing a non-lead member never touches the rule.
	f.addExplicit(t, p.ID, f.guest, "viewer")
	if _, err := f.svc.UpdateMemberRole(f.ctx, f.owner, "requester", p.ID, f.guest, "contributor"); err != nil {
		t.Fatalf("contributor change: %v", err)
	}
}

// TestDeleteByLeadAndOrgAdmin covers the delete rule: the lead deletes with the exact name, an org
// admin deletes too, and a wrong name is a validation error.
func TestDeleteByLeadAndOrgAdmin(t *testing.T) {
	f := newFixture(t)
	// The member creates the project (default policy) and so becomes its lead.
	leadProject := createFromTemplate(t, f, f.member, f.orgA, "LEADDEL", domain.TemplateKanban)
	err := f.svc.DeleteProject(f.ctx, f.member, "requester", leadProject.ID, "wrong")
	wantCode(t, err, "project.confirmation_mismatch")
	if err := f.svc.DeleteProject(f.ctx, f.member, "requester", leadProject.ID, "Project LEADDEL"); err != nil {
		t.Fatalf("lead delete with the exact name: %v", err)
	}

	adminTarget := createFromTemplate(t, f, f.owner, f.orgA, "ADMDEL", domain.TemplateKanban)
	if err := f.svc.DeleteProject(f.ctx, f.admin, "requester", adminTarget.ID, "Project ADMDEL"); err != nil {
		t.Fatalf("org admin delete: %v", err)
	}
	if _, err := f.svc.Get(f.ctx, f.owner, "requester", adminTarget.ID); err == nil {
		t.Fatal("deleted project still readable")
	}
}
