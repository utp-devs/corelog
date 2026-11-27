package usecase

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// memProjects guarda proyectos y membresias en memoria.
type memProjects struct {
	projects   map[uuid.UUID]*domain.Project
	members    map[uuid.UUID]map[uuid.UUID]port.Member // project -> user -> member
	teamGrants map[uuid.UUID][]uuid.UUID
}

func newMemProjects() *memProjects {
	return &memProjects{
		projects:   map[uuid.UUID]*domain.Project{},
		members:    map[uuid.UUID]map[uuid.UUID]port.Member{},
		teamGrants: map[uuid.UUID][]uuid.UUID{},
	}
}

func (m *memProjects) Save(_ context.Context, p *domain.Project) error {
	p.ID = uuid.New()
	m.projects[p.ID] = p
	return nil
}

func (m *memProjects) FindByID(_ context.Context, id uuid.UUID) (*domain.Project, error) {
	p, ok := m.projects[id]
	if !ok || p.IsDeleted() {
		return nil, domain.ErrProjectNotFound
	}
	return p, nil
}

func (m *memProjects) FindByKey(_ context.Context, orgID uuid.UUID, key string) (*domain.Project, error) {
	for _, p := range m.projects {
		if p.OrganizationID == orgID && p.Key == key && !p.IsDeleted() {
			return p, nil
		}
	}
	return nil, domain.ErrProjectNotFound
}

func (m *memProjects) FindByOrganization(_ context.Context, orgID uuid.UUID, scope port.VisibilityScope) ([]*domain.Project, error) {
	out := []*domain.Project{}
	for _, p := range m.projects {
		if p.OrganizationID != orgID || p.IsDeleted() {
			continue
		}
		_, explicit := m.members[p.ID][scope.UserID]
		if scope.All || explicit || (scope.Open && p.Visibility == domain.VisibilityOpen) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *memProjects) ListPage(_ context.Context, filter port.ProjectFilter) (port.ProjectPage, error) {
	all, _ := m.FindByOrganization(context.Background(), filter.OrganizationID, filter.Scope)
	var items []*domain.Project
	for _, p := range all {
		if filter.Archived == port.ArchiveFilterExclude && p.IsArchived() {
			continue
		}
		if filter.Archived == port.ArchiveFilterOnly && !p.IsArchived() {
			continue
		}
		if filter.Template != "" && p.Template != filter.Template {
			continue
		}
		items = append(items, p)
	}
	return port.ProjectPage{Items: items, Total: len(items)}, nil
}

func (m *memProjects) Update(_ context.Context, p *domain.Project) error {
	m.projects[p.ID] = p
	return nil
}

// Delete soft deletes: the project stays stored but every read stops seeing it.
func (m *memProjects) Delete(_ context.Context, orgID, id uuid.UUID) error {
	if p, ok := m.projects[id]; ok && p.OrganizationID == orgID && !p.IsDeleted() {
		now := time.Now().UTC()
		p.DeletedAt = &now
	}
	return nil
}

func (m *memProjects) KeyAvailable(_ context.Context, orgID uuid.UUID, key string) (bool, error) {
	for _, p := range m.projects {
		if p.OrganizationID == orgID && p.Key == key && !p.IsDeleted() {
			return false, nil
		}
	}
	return true, nil
}

func (m *memProjects) AllocateIssueNumber(_ context.Context, projectID uuid.UUID) (int, error) {
	p, ok := m.projects[projectID]
	if !ok || p.IsDeleted() {
		return 0, domain.ErrProjectNotFound
	}
	number := p.NextIssueNumber
	p.NextIssueNumber++
	return number, nil
}

func (m *memProjects) FindRole(_ context.Context, projectID, userID uuid.UUID) (*string, error) {
	member, ok := m.members[projectID][userID]
	if !ok {
		return nil, nil
	}
	return &member.Role, nil
}

func (m *memProjects) ListProjectIDs(_ context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for projectID, users := range m.members {
		if _, ok := users[userID]; ok {
			out = append(out, projectID)
		}
	}
	return out, nil
}

func (m *memProjects) List(_ context.Context, projectID uuid.UUID) ([]port.Member, error) {
	var out []port.Member
	for _, member := range m.members[projectID] {
		out = append(out, member)
	}
	return out, nil
}

func (m *memProjects) Add(_ context.Context, member port.Member) error {
	if m.members[member.ProjectID] == nil {
		m.members[member.ProjectID] = map[uuid.UUID]port.Member{}
	}
	m.members[member.ProjectID][member.UserID] = member
	return nil
}

func (m *memProjects) UpdateRole(_ context.Context, projectID, userID uuid.UUID, role string) error {
	member := m.members[projectID][userID]
	member.Role = role
	m.members[projectID][userID] = member
	return nil
}

func (m *memProjects) Remove(_ context.Context, projectID, userID uuid.UUID) error {
	delete(m.members[projectID], userID)
	return nil
}

// fakeOrgs implementa port.OrgDirectory con roles y politicas fijas por organizacion.
type fakeOrgs struct {
	roles  map[uuid.UUID]map[uuid.UUID]string
	policy map[uuid.UUID][2]string // who_can_create_projects, default_project_visibility
}

func newFakeOrgs() *fakeOrgs {
	return &fakeOrgs{roles: map[uuid.UUID]map[uuid.UUID]string{}, policy: map[uuid.UUID][2]string{}}
}

func (f *fakeOrgs) join(orgID, userID uuid.UUID, role string) {
	if f.roles[orgID] == nil {
		f.roles[orgID] = map[uuid.UUID]string{}
	}
	f.roles[orgID][userID] = role
}

func (f *fakeOrgs) MemberRole(_ context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	role, ok := f.roles[orgID][userID]
	return role, ok, nil
}

func (f *fakeOrgs) ProjectCreationPolicy(_ context.Context, orgID uuid.UUID) (string, string, error) {
	p, ok := f.policy[orgID]
	if !ok {
		return "members", "open", nil
	}
	return p[0], p[1], nil
}

// roleAuth decide con los mismos nombres de accion que la matriz, segun el rol de proyecto del actor.
type roleAuth struct {
	repo *memProjects
}

func (a roleAuth) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	if globalRole == "admin" {
		return nil
	}
	role, _ := a.repo.FindRole(ctx, projectID, actorID)
	if role == nil {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	if *role == "lead" || action == port.ActionViewProject {
		return nil
	}
	return apperror.New(apperror.KindForbidden, "forbidden project operation")
}

// failingMembers simula la carrera en la que el usuario se borra entre la comprobacion y el insert.
type failingMembers struct{ *memProjects }

func (failingMembers) Add(context.Context, port.Member) error { return domain.ErrUserNotFound }

type fixture struct {
	ctx        context.Context
	repo       *memProjects
	orgs       *fakeOrgs
	svc        *ProjectService
	orgA, orgB uuid.UUID
	owner      uuid.UUID // owner de la organizacion A
	admin      uuid.UUID // admin de la organizacion A
	member     uuid.UUID // member de A
	guest      uuid.UUID // guest de A
	outsider   uuid.UUID // miembro solo de B
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := newMemProjects()
	orgs := newFakeOrgs()
	f := &fixture{
		ctx: context.Background(), repo: repo, orgs: orgs,
		orgA: uuid.New(), orgB: uuid.New(), owner: uuid.New(), admin: uuid.New(),
		member: uuid.New(), guest: uuid.New(), outsider: uuid.New(),
	}
	orgs.join(f.orgA, f.owner, domain.OrgRoleOwner)
	orgs.join(f.orgA, f.admin, domain.OrgRoleAdmin)
	orgs.join(f.orgA, f.member, domain.OrgRoleMember)
	orgs.join(f.orgA, f.guest, domain.OrgRoleGuest)
	orgs.join(f.orgB, f.outsider, domain.OrgRoleOwner)
	f.svc = NewProjectService(repo, repo, orgs).WithAuthorizer(roleAuth{repo: repo})
	return f
}

func (f *fixture) create(t *testing.T, actor, org uuid.UUID, key string, visibility domain.Visibility) *domain.Project {
	t.Helper()
	p, err := f.svc.Create(f.ctx, actor, org, key, key, "", domain.ProjectTypeKanban, visibility)
	if err != nil {
		t.Fatalf("Create %s: %v", key, err)
	}
	return p
}

func (f *fixture) addExplicit(t *testing.T, projectID, userID uuid.UUID, role string) {
	t.Helper()
	if err := f.repo.Add(f.ctx, port.Member{ProjectID: projectID, UserID: userID, Role: role}); err != nil {
		t.Fatal(err)
	}
}

func wantError(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want kind %v code %q", kind, code)
	}
	if apperror.KindOf(err) != kind {
		t.Fatalf("kind = %v (%v), want %v", apperror.KindOf(err), err, kind)
	}
	if code != "" && apperror.CodeOf(err) != code {
		t.Fatalf("code = %q, want %q", apperror.CodeOf(err), code)
	}
}

func TestCreatePolicyMatrix(t *testing.T) {
	for _, policy := range []string{domain.CreationPolicyMembers, domain.CreationPolicyAdmins} {
		for _, role := range []string{domain.OrgRoleOwner, domain.OrgRoleAdmin, domain.OrgRoleMember, domain.OrgRoleGuest, "outsider"} {
			t.Run(fmt.Sprintf("%s/%s", role, policy), func(t *testing.T) {
				f := newFixture(t)
				f.orgs.policy[f.orgA] = [2]string{policy, "open"}
				actor := map[string]uuid.UUID{
					domain.OrgRoleOwner: f.owner, domain.OrgRoleAdmin: f.admin,
					domain.OrgRoleMember: f.member, domain.OrgRoleGuest: f.guest, "outsider": f.outsider,
				}[role]

				p, err := f.svc.Create(f.ctx, actor, f.orgA, "NEW", "New", "", domain.ProjectTypeScrum, "")
				switch {
				case role == "outsider":
					wantError(t, err, apperror.KindNotFound, "organization.not_found")
				case role == domain.OrgRoleGuest, role == domain.OrgRoleMember && policy == domain.CreationPolicyAdmins:
					wantError(t, err, apperror.KindForbidden, "project.creation_not_allowed")
				default:
					if err != nil {
						t.Fatalf("Create: %v", err)
					}
					if p.OrganizationID != f.orgA || p.LeadID != actor {
						t.Fatalf("project org/lead = %s/%s", p.OrganizationID, p.LeadID)
					}
					if got := f.repo.members[p.ID][actor].Role; got != "lead" {
						t.Fatalf("creator role = %q, want lead", got)
					}
				}
			})
		}
	}
}

func TestCreateVisibilityDefaultsAndLegacyAlias(t *testing.T) {
	f := newFixture(t)
	f.orgs.policy[f.orgA] = [2]string{domain.CreationPolicyMembers, string(domain.VisibilityPrivate)}

	def, err := f.svc.Create(f.ctx, f.owner, f.orgA, "DEF", "Def", "", domain.ProjectTypeKanban, "")
	if err != nil || def.Visibility != domain.VisibilityPrivate {
		t.Fatalf("default visibility = %v, %v; want private", def, err)
	}
	legacy, err := f.svc.Create(f.ctx, f.owner, f.orgA, "LEG", "Leg", "", domain.ProjectTypeKanban, domain.VisibilityTeam)
	if err != nil || legacy.Visibility != domain.VisibilityOpen {
		t.Fatalf("team input stored as %v, %v; want open", legacy, err)
	}
}

func TestVisibilityRulesPerRole(t *testing.T) {
	f := newFixture(t)
	open := f.create(t, f.owner, f.orgA, "OPEN", domain.VisibilityOpen)
	private := f.create(t, f.owner, f.orgA, "PRIV", domain.VisibilityPrivate)
	explicitMember := uuid.New()
	f.orgs.join(f.orgA, explicitMember, domain.OrgRoleMember)
	f.addExplicit(t, private.ID, explicitMember, "viewer")
	f.addExplicit(t, open.ID, f.guest, "viewer")

	tests := []struct {
		name     string
		actor    uuid.UUID
		wantOpen bool
		wantPriv bool
		wantList []string
	}{
		{"owner sees all", f.owner, true, true, []string{"OPEN", "PRIV"}},
		{"admin sees all", f.admin, true, true, []string{"OPEN", "PRIV"}},
		{"member sees open only", f.member, true, false, []string{"OPEN"}},
		{"member explicit on private", explicitMember, true, true, []string{"OPEN", "PRIV"}},
		{"guest explicit on open only", f.guest, true, false, []string{"OPEN"}},
		{"other org member sees nothing", f.outsider, false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errOpen := f.svc.Get(f.ctx, tt.actor, "requester", open.ID)
			_, errPriv := f.svc.Get(f.ctx, tt.actor, "requester", private.ID)
			checkGetVisible(t, errOpen, tt.wantOpen)
			checkGetVisible(t, errPriv, tt.wantPriv)

			if tt.actor == f.outsider {
				_, err := f.svc.List(f.ctx, tt.actor, "requester", f.orgA)
				wantError(t, err, apperror.KindNotFound, "organization.not_found")
				return
			}
			listed, err := f.svc.List(f.ctx, tt.actor, "requester", f.orgA)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			var keys []string
			for _, p := range listed {
				keys = append(keys, p.Key)
			}
			sort.Strings(keys)
			if fmt.Sprint(keys) != fmt.Sprint(sorted(tt.wantList)) {
				t.Fatalf("List keys = %v, want %v", keys, tt.wantList)
			}
		})
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func checkGetVisible(t *testing.T, err error, want bool) {
	t.Helper()
	if want && err != nil {
		t.Fatalf("Get: %v, want visible", err)
	}
	if !want {
		wantError(t, err, apperror.KindNotFound, "project.not_found")
	}
}

func TestCrossOrganizationIsolation(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "CORE", domain.VisibilityOpen)
	f.addExplicit(t, p.ID, f.outsider, "lead") // aunque tenga membresia de proyecto, no es miembro de A

	_, err := f.svc.Get(f.ctx, f.outsider, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")
	_, err = f.svc.Update(f.ctx, f.outsider, "requester", p.ID, domain.UpdateInput{Name: "x", Type: domain.ProjectTypeKanban, Visibility: domain.VisibilityOpen, LeadID: f.outsider})
	wantError(t, err, apperror.KindNotFound, "project.not_found")
	wantError(t, f.svc.Delete(f.ctx, f.outsider, "requester", p.ID), apperror.KindNotFound, "project.not_found")
	_, err = f.svc.ListMembers(f.ctx, f.outsider, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")
	_, err = f.svc.ListLabels(f.ctx, f.outsider, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")
	_, err = f.svc.CreateLabel(f.ctx, f.outsider, "requester", p.ID, "bug", "red")
	wantError(t, err, apperror.KindNotFound, "project.not_found")
	_, err = f.svc.AddMember(f.ctx, f.outsider, "requester", p.ID, f.outsider, "viewer")
	wantError(t, err, apperror.KindNotFound, "project.not_found")

	_, err = f.svc.List(f.ctx, f.owner, "requester", f.orgB)
	wantError(t, err, apperror.KindNotFound, "organization.not_found")

	// La clave del proyecto de A sigue viva en A: el owner de B no la ve.
	if _, err := f.svc.Get(f.ctx, f.owner, "requester", p.ID); err != nil {
		t.Fatalf("owner of A must read A: %v", err)
	}
}

func TestManagePermissions(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "CORE", domain.VisibilityOpen)
	contributor := uuid.New()
	f.orgs.join(f.orgA, contributor, domain.OrgRoleMember)
	f.addExplicit(t, p.ID, contributor, "contributor")
	update := domain.UpdateInput{Name: "Core", Type: domain.ProjectTypeKanban, Visibility: domain.VisibilityOpen, LeadID: f.owner}

	_, err := f.svc.Update(f.ctx, f.member, "requester", p.ID, update)
	wantError(t, err, apperror.KindForbidden, "")
	_, err = f.svc.Update(f.ctx, contributor, "requester", p.ID, update)
	wantError(t, err, apperror.KindForbidden, "")
	wantError(t, f.svc.Delete(f.ctx, f.member, "requester", p.ID), apperror.KindForbidden, "")
	if _, err := f.svc.Update(f.ctx, f.admin, "requester", p.ID, update); err != nil {
		t.Fatalf("org admin implicit lead must update: %v", err)
	}
	if err := f.svc.Delete(f.ctx, f.admin, "requester", p.ID); err != nil {
		t.Fatalf("org admin must delete: %v", err)
	}
}

func TestAddMemberRequiresOrganizationMembership(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "CORE", domain.VisibilityOpen)

	_, err := f.svc.AddMember(f.ctx, f.owner, "requester", p.ID, f.outsider, "viewer")
	wantError(t, err, apperror.KindValidation, "project.user_not_in_org")

	_, err = f.svc.AddMember(f.ctx, f.owner, "requester", p.ID, uuid.New(), "viewer")
	wantError(t, err, apperror.KindValidation, "project.user_not_in_org")

	if _, err := f.svc.AddMember(f.ctx, f.owner, "requester", p.ID, f.guest, "viewer"); err != nil {
		t.Fatalf("guest of the org may be added explicitly: %v", err)
	}
}

func TestAddMemberRaceWithDeletedUserReturnsTypedNotFound(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "CORE", domain.VisibilityOpen)
	svc := NewProjectService(f.repo, failingMembers{f.repo}, f.orgs).WithAuthorizer(roleAuth{repo: f.repo})

	_, err := svc.AddMember(f.ctx, f.owner, "requester", p.ID, f.member, "viewer")
	wantError(t, err, apperror.KindNotFound, "project.user_not_found")
}

func TestKeyIsUniquePerOrganization(t *testing.T) {
	f := newFixture(t)
	f.orgs.join(f.orgB, f.owner, domain.OrgRoleOwner)
	f.create(t, f.owner, f.orgA, "CORE", domain.VisibilityOpen)
	f.create(t, f.owner, f.orgB, "CORE", domain.VisibilityOpen)

	_, err := f.svc.Create(f.ctx, f.owner, f.orgA, "CORE", "Dup", "", domain.ProjectTypeKanban, domain.VisibilityOpen)
	wantError(t, err, apperror.KindConflict, "project.key_taken")
}

func TestCatalogRequiresProjectAccess(t *testing.T) {
	f := newFixture(t)
	p := f.create(t, f.owner, f.orgA, "CORE", domain.VisibilityOpen)

	labels, err := f.svc.ListLabels(f.ctx, f.member, "requester", p.ID)
	if err != nil || len(labels) != 0 {
		t.Fatalf("ListLabels without catalog = %v, %v; want empty", labels, err)
	}
	if _, err := f.svc.CreateLabel(f.ctx, f.owner, "requester", p.ID, "bug", "red"); err == nil {
		t.Fatal("CreateLabel without catalog must fail")
	}
}

func TestGetMissingProjectIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Get(f.ctx, f.owner, "requester", uuid.New())
	wantError(t, err, apperror.KindNotFound, "project.not_found")
}

// CreateWithMembers stores the project and its initial relations, like the transactional adapter.
func (m *memProjects) CreateWithMembers(ctx context.Context, p *domain.Project, lead port.Member, members []port.Member, teamIDs []uuid.UUID) error {
	if err := m.Save(ctx, p); err != nil {
		return err
	}
	lead.ProjectID = p.ID
	if err := m.Add(ctx, lead); err != nil {
		return err
	}
	for _, member := range members {
		member.ProjectID = p.ID
		if err := m.Add(ctx, member); err != nil {
			return err
		}
	}
	if len(teamIDs) > 0 {
		m.teamGrants[p.ID] = teamIDs
	}
	return nil
}

// CreateWithDefaults is the creation path without initial relations.
func (m *memProjects) CreateWithDefaults(ctx context.Context, p *domain.Project, lead port.Member) error {
	return m.CreateWithMembers(ctx, p, lead, nil, nil)
}
