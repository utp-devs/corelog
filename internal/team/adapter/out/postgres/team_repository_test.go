package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

// defaultOrgID is the organization created by migration 000024.
const defaultOrgID = "00000000-0000-4000-8000-000000000002"

func insertUser(t *testing.T, repo *TeamRepository, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.db.GetContext(context.Background(), &id,
		`INSERT INTO users (name, display_name, email, password_hash, role)
		 VALUES ($1, $1, $2, 'hash', 'user') RETURNING id`, email, email)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func insertOrg(t *testing.T, repo *TeamRepository, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.db.GetContext(context.Background(), &id,
		`INSERT INTO organizations (slug, name) VALUES ($1, $1) RETURNING id`, slug)
	if err != nil {
		t.Fatalf("insert org: %v", err)
	}
	return id
}

func newTeam(t *testing.T, org, creator uuid.UUID, name string) *domain.Team {
	t.Helper()
	team, err := domain.NewTeam(domain.NewTeamInput{OrganizationID: org, CreatedBy: creator, Name: name, Description: "d", AvatarColor: "#00FF00"})
	if err != nil {
		t.Fatalf("new team: %v", err)
	}
	return team
}

func TestTeamRepository_SaveAndFind(t *testing.T) {
	db := testdb.New(t)
	repo := NewTeamRepository(db)
	ctx := context.Background()
	creator := insertUser(t, repo, "creator@example.test")
	org := uuid.MustParse(defaultOrgID)

	team := newTeam(t, org, creator, "Plataforma")
	if err := repo.Save(ctx, team); err != nil {
		t.Fatalf("save: %v", err)
	}
	if team.ID == uuid.Nil {
		t.Fatal("database must generate the id")
	}

	got, err := repo.FindByID(ctx, team.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.OrganizationID != org || got.CreatedBy != creator || got.Description != "d" || got.AvatarColor != "#00FF00" {
		t.Errorf("round trip lost fields: %+v", got)
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, domain.ErrTeamNotFound) {
		t.Errorf("missing team: got %v", err)
	}
}

func TestTeamRepository_NameConflictMapsToTeamNameTaken(t *testing.T) {
	db := testdb.New(t)
	repo := NewTeamRepository(db)
	ctx := context.Background()
	creator := insertUser(t, repo, "dup@example.test")
	org := uuid.MustParse(defaultOrgID)

	if err := repo.Save(ctx, newTeam(t, org, creator, "Unico")); err != nil {
		t.Fatalf("first save: %v", err)
	}
	err := repo.Save(ctx, newTeam(t, org, creator, "Unico"))
	if !errors.Is(err, domain.ErrTeamNameTaken) {
		t.Fatalf("got %v, want ErrTeamNameTaken", err)
	}
}

func TestTeamRepository_CrossOrganizationIsolation(t *testing.T) {
	db := testdb.New(t)
	repo := NewTeamRepository(db)
	ctx := context.Background()
	user := insertUser(t, repo, "iso@example.test")
	orgA := insertOrg(t, repo, "org-a")
	orgB := insertOrg(t, repo, "org-b")

	teamA := newTeam(t, orgA, user, "Equipo A")
	teamB := newTeam(t, orgB, user, "Equipo B")
	for _, tm := range []*domain.Team{teamA, teamB} {
		if err := repo.Save(ctx, tm); err != nil {
			t.Fatalf("save: %v", err)
		}
		m, _ := domain.NewMember(tm.ID, user, domain.RoleMaintainer)
		if err := repo.AddMember(ctx, tm.OrganizationID, m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	list, err := repo.FindByOrganization(ctx, orgA)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != teamA.ID {
		t.Fatalf("list of org A leaked teams: %d", len(list))
	}

	// Writes and member operations with the wrong organization must not match.
	if err := repo.Update(ctx, &domain.Team{ID: teamB.ID, OrganizationID: orgA, Name: "X", UpdatedAt: teamB.UpdatedAt}); !errors.Is(err, domain.ErrTeamNotFound) {
		t.Errorf("update cross-org: %v", err)
	}
	if err := repo.Delete(ctx, orgA, teamB.ID); !errors.Is(err, domain.ErrTeamNotFound) {
		t.Errorf("delete cross-org: %v", err)
	}
	if _, err := repo.FindMember(ctx, orgA, teamB.ID, user); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("find member cross-org: %v", err)
	}
	if members, err := repo.ListMembers(ctx, orgA, teamB.ID); err != nil || len(members) != 0 {
		t.Errorf("list members cross-org leaked %d members (err %v)", len(members), err)
	}
	if err := repo.RemoveMember(ctx, orgA, teamB.ID, user); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("remove member cross-org: %v", err)
	}
	if err := repo.UpdateMemberRole(ctx, orgA, teamB.ID, user, domain.RoleMember); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("update role cross-org: %v", err)
	}
	other := insertUser(t, repo, "other@example.test")
	m, _ := domain.NewMember(teamB.ID, other, domain.RoleMember)
	if err := repo.AddMember(ctx, orgA, m); !errors.Is(err, domain.ErrTeamNotFound) {
		t.Errorf("add member cross-org: %v", err)
	}
}

func TestTeamRepository_MemberLifecycle(t *testing.T) {
	db := testdb.New(t)
	repo := NewTeamRepository(db)
	ctx := context.Background()
	owner := insertUser(t, repo, "life-owner@example.test")
	member := insertUser(t, repo, "life-member@example.test")
	org := uuid.MustParse(defaultOrgID)

	team := newTeam(t, org, owner, "Ciclo")
	if err := repo.Save(ctx, team); err != nil {
		t.Fatalf("save: %v", err)
	}
	m, _ := domain.NewMember(team.ID, owner, domain.RoleMaintainer)
	if err := repo.AddMember(ctx, org, m); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if m.CreatedAt.IsZero() {
		t.Error("AddMember must return created_at")
	}

	mem, _ := domain.NewMember(team.ID, member, domain.RoleMember)
	if err := repo.AddMember(ctx, org, mem); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := repo.AddMember(ctx, org, mem); !errors.Is(err, domain.ErrMemberExists) {
		t.Errorf("duplicate member: %v", err)
	}

	if err := repo.UpdateMemberRole(ctx, org, team.ID, member, domain.RoleMaintainer); err != nil {
		t.Fatalf("update role: %v", err)
	}
	got, err := repo.FindMember(ctx, org, team.ID, member)
	if err != nil || got.Role != domain.RoleMaintainer {
		t.Fatalf("role after update: %v / %v", got, err)
	}

	members, err := repo.ListMembers(ctx, org, team.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("list members: %d / %v", len(members), err)
	}

	if err := repo.RemoveMember(ctx, org, team.ID, member); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := repo.RemoveMember(ctx, org, team.ID, member); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("remove twice: %v", err)
	}

	if err := repo.Delete(ctx, org, team.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
