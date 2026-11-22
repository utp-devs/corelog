package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// memProjects guarda proyectos y membresias en memoria.
type memProjects struct {
	projects map[uuid.UUID]*domain.Project
	members  map[uuid.UUID]map[uuid.UUID]port.Member // project -> user -> member
}

func newMemProjects() *memProjects {
	return &memProjects{projects: map[uuid.UUID]*domain.Project{}, members: map[uuid.UUID]map[uuid.UUID]port.Member{}}
}

func (m *memProjects) Save(_ context.Context, p *domain.Project) error {
	p.ID = uuid.New()
	m.projects[p.ID] = p
	return nil
}

func (m *memProjects) FindByID(_ context.Context, id uuid.UUID) (*domain.Project, error) {
	p, ok := m.projects[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return p, nil
}

func (m *memProjects) FindByKey(_ context.Context, key string) (*domain.Project, error) {
	for _, p := range m.projects {
		if p.Key == key {
			return p, nil
		}
	}
	return nil, domain.ErrProjectNotFound
}

func (m *memProjects) FindAll(context.Context) ([]*domain.Project, error) {
	out := make([]*domain.Project, 0, len(m.projects))
	for _, p := range m.projects {
		out = append(out, p)
	}
	return out, nil
}

func (m *memProjects) Update(_ context.Context, p *domain.Project) error {
	m.projects[p.ID] = p
	return nil
}

func (m *memProjects) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.projects, id)
	return nil
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
	if *role == "lead" {
		return nil
	}
	if action == port.ActionViewProject {
		return nil
	}
	return apperror.New(apperror.KindForbidden, "forbidden project operation")
}

func TestProjectServiceAccessAndMembers(t *testing.T) {
	ctx := context.Background()
	repo := newMemProjects()
	svc := NewProjectService(repo, repo, nil)
	svc.WithAuthorizer(roleAuth{repo: repo})

	lead, contributor, outsider := uuid.New(), uuid.New(), uuid.New()
	created, err := svc.Create(ctx, "CORE", "Core", "Plataforma", domain.ProjectTypeKanban, lead, domain.VisibilityTeam)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Add(ctx, port.Member{ProjectID: created.ID, UserID: lead, Role: "lead"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(ctx, port.Member{ProjectID: created.ID, UserID: contributor, Role: "contributor"}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Get(ctx, outsider, "requester", created.ID); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("outsider reads the project: got %v, want forbidden", err)
	}
	if _, err := svc.Get(ctx, contributor, "requester", created.ID); err != nil {
		t.Fatalf("member reads the project: %v", err)
	}

	if _, err := svc.Update(ctx, contributor, "requester", created.ID, domain.UpdateInput{Name: "Otro", Description: "x"}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor edits the project: got %v, want forbidden", err)
	}
	if err := svc.Delete(ctx, contributor, "requester", created.ID); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor deletes the project: got %v, want forbidden", err)
	}

	members, err := svc.ListMembers(ctx, lead, "requester", created.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("ListMembers = %d, %v; want 2", len(members), err)
	}
	if _, err := svc.AddMember(ctx, lead, "requester", created.ID, outsider, "hacker"); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("invalid role: got %v, want validation", err)
	}
	if _, err := svc.AddMember(ctx, contributor, "requester", created.ID, outsider, "viewer"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor adds a member: got %v, want forbidden", err)
	}
	if _, err := svc.AddMember(ctx, lead, "requester", created.ID, outsider, "viewer"); err != nil {
		t.Fatalf("lead adds a member: %v", err)
	}
	if _, err := svc.UpdateMemberRole(ctx, lead, "requester", created.ID, outsider, "contributor"); err != nil {
		t.Fatalf("lead changes a role: %v", err)
	}
	if err := svc.RemoveMember(ctx, lead, "requester", created.ID, outsider); err != nil {
		t.Fatalf("lead removes a member: %v", err)
	}
}

func TestProjectServiceListShowsOnlyVisibleProjects(t *testing.T) {
	ctx := context.Background()
	repo := newMemProjects()
	svc := NewProjectService(repo, repo, nil)
	svc.WithAuthorizer(roleAuth{repo: repo})
	user := uuid.New()

	mine, _ := svc.Create(ctx, "MINE", "Mio", "", domain.ProjectTypeScrum, user, domain.VisibilityTeam)
	_, _ = svc.Create(ctx, "THEIRS", "Ajeno", "", domain.ProjectTypeScrum, uuid.New(), domain.VisibilityTeam)
	_ = repo.Add(ctx, port.Member{ProjectID: mine.ID, UserID: user, Role: "viewer"})

	visible, err := svc.List(ctx, user, "requester")
	if err != nil || len(visible) != 1 || visible[0].ID != mine.ID {
		t.Fatalf("List for a member = %d, %v; want only its own project", len(visible), err)
	}
	all, _ := svc.List(ctx, uuid.New(), "admin")
	if len(all) != 2 {
		t.Fatalf("admin List = %d, want 2", len(all))
	}
}

func TestProjectServiceCatalogWithoutRepositoryIsEmptyOrInvalid(t *testing.T) {
	ctx := context.Background()
	repo := newMemProjects()
	svc := NewProjectService(repo, repo, nil)

	labels, err := svc.ListLabels(ctx, uuid.New())
	if err != nil || len(labels) != 0 {
		t.Fatalf("ListLabels without catalog = %v, %v; want empty", labels, err)
	}
	if _, err := svc.CreateLabel(ctx, uuid.New(), "bug", "#ff0000"); err == nil {
		t.Fatal("CreateLabel without catalog must fail")
	}
}
