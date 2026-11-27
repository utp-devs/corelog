package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

func addOrganizationMember(t *testing.T, db *sqlx.DB, orgID, userID uuid.UUID, role string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, orgID, userID, role); err != nil {
		t.Fatal(err)
	}
}

func addTeamMember(t *testing.T, db *sqlx.DB, teamID, userID uuid.UUID) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)`, teamID, userID); err != nil {
		t.Fatal(err)
	}
}

// TestTeamGrantRepositoryPostgres covers the grant CRUD and the team users read: only live members of
// the project organization are returned, and a removed grant reports not found.
func TestTeamGrantRepositoryPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	grants := NewTeamGrantRepository(db)
	orgID := uuid.MustParse(defaultOrganizationID)
	leadID, orgMember, outsider := insertUser(t, db), insertUser(t, db), insertUser(t, db)
	addOrganizationMember(t, db, orgID, orgMember, "member")

	p := newDomainProject(t, orgID, leadID, "TGR", domain.TemplateKanban, domain.VisibilityPrivate)
	if err := projects.CreateWithDefaults(ctx, p, port.Member{UserID: leadID, Role: "lead"}); err != nil {
		t.Fatal(err)
	}
	team := insertTeam(t, db, orgID)
	addTeamMember(t, db, team, orgMember)
	addTeamMember(t, db, team, outsider) // not an organization member: must not be listed

	name, found, err := grants.TeamInOrganization(ctx, orgID, team)
	if err != nil || !found || name == "" {
		t.Fatalf("TeamInOrganization = %q, %v, %v", name, found, err)
	}
	_, found, err = grants.TeamInOrganization(ctx, uuid.New(), team)
	if err != nil || found {
		t.Fatalf("TeamInOrganization(other org) found=%v err=%v", found, err)
	}

	if err := grants.UpsertTeamGrant(ctx, port.TeamGrant{ProjectID: p.ID, TeamID: team, Role: "viewer"}); err != nil {
		t.Fatal(err)
	}
	if err := grants.UpsertTeamGrant(ctx, port.TeamGrant{ProjectID: p.ID, TeamID: team, Role: "contributor"}); err != nil {
		t.Fatal(err)
	}
	list, err := grants.ListTeamGrants(ctx, p.ID)
	if err != nil || len(list) != 1 || list[0].Role != "contributor" || list[0].TeamName != name {
		t.Fatalf("ListTeamGrants = %+v, %v", list, err)
	}

	users, err := grants.ListTeamUsers(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].UserID != orgMember || users[0].Role != "contributor" || users[0].TeamID != team {
		t.Fatalf("ListTeamUsers = %+v, want only the organization member", users)
	}

	if err := grants.RemoveTeamGrant(ctx, p.ID, team); err != nil {
		t.Fatal(err)
	}
	err = grants.RemoveTeamGrant(ctx, p.ID, team)
	if err != domain.ErrTeamGrantNotFound {
		t.Fatalf("second remove = %v, want ErrTeamGrantNotFound", err)
	}
}

// TestFindKeyCandidatesAndTypeFilterPostgres covers the key candidates reachable by a user across
// organizations and the legacy type filter over templates.
func TestFindKeyCandidatesAndTypeFilterPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewProjectRepository(db)
	orgA := uuid.MustParse(defaultOrganizationID)
	orgB := insertOrganization(t, db)
	user, stranger := insertUser(t, db), insertUser(t, db)
	addOrganizationMember(t, db, orgA, user, "member")

	inA := newDomainProject(t, orgA, user, "CAND", domain.TemplateScrum, domain.VisibilityOpen)
	if err := repo.CreateWithDefaults(ctx, inA, port.Member{UserID: user, Role: "lead"}); err != nil {
		t.Fatal(err)
	}
	inB := newDomainProject(t, orgB, stranger, "CAND", domain.TemplateKanban, domain.VisibilityPrivate)
	if err := repo.CreateWithDefaults(ctx, inB, port.Member{UserID: stranger, Role: "lead"}); err != nil {
		t.Fatal(err)
	}

	candidates, err := repo.FindKeyCandidates(ctx, "cand", user, false)
	if err != nil || len(candidates) != 1 || candidates[0].ID != inA.ID {
		t.Fatalf("candidates for member = %d, %v; want only organization A", len(candidates), err)
	}
	all, err := repo.FindKeyCandidates(ctx, "CAND", user, true)
	if err != nil || len(all) != 2 {
		t.Fatalf("candidates for platform admin = %d, %v; want 2", len(all), err)
	}

	page, err := repo.ListPage(ctx, port.ProjectFilter{
		OrganizationID: orgA, Scope: port.VisibilityScope{UserID: user, All: true},
		Type: domain.ProjectTypeScrum, Archived: port.ArchiveFilterExclude, Sort: port.SortByName, PageSize: 10, Page: 1,
	})
	if err != nil || page.Total != 1 || page.Items[0].ID != inA.ID {
		t.Fatalf("type=scrum page = %+v, %v", page, err)
	}
	page, err = repo.ListPage(ctx, port.ProjectFilter{
		OrganizationID: orgB, Scope: port.VisibilityScope{UserID: stranger, All: true},
		Type: domain.ProjectTypeKanban, Archived: port.ArchiveFilterExclude, Sort: port.SortByName, PageSize: 10, Page: 1,
	})
	if err != nil || page.Total != 1 || page.Items[0].ID != inB.ID {
		t.Fatalf("type=kanban page = %+v, %v", page, err)
	}
}
