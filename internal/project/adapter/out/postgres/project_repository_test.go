package postgres

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

func insertOrganization(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'Other')`, id, "o-"+id.String()[:8]); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTeam(t *testing.T, db *sqlx.DB, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO teams (id, name, organization_id) VALUES ($1, $2, $3)`, id, "team-"+id.String(), orgID); err != nil {
		t.Fatal(err)
	}
	return id
}

// newDomainProject builds a valid project of the given organization without saving it.
func newDomainProject(t *testing.T, orgID, leadID uuid.UUID, key string, template domain.Template, visibility domain.Visibility) *domain.Project {
	t.Helper()
	p, err := domain.NewProjectFromInput(domain.NewProjectInput{
		OrganizationID: orgID, Key: key, Name: "Project " + key, Template: template,
		Visibility: visibility, LeadID: leadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProjectKeyUniquePerOrganizationAndSoftDeleteFreesKey(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewProjectRepository(db)
	leadID := insertUser(t, db)
	orgA := uuid.MustParse(defaultOrganizationID)
	orgB := insertOrganization(t, db)

	first := newDomainProject(t, orgA, leadID, "UNQ", domain.TemplateKanban, domain.VisibilityOpen)
	if err := repo.CreateWithDefaults(ctx, first, port.Member{UserID: leadID, Role: "lead"}); err != nil {
		t.Fatal(err)
	}

	again := newDomainProject(t, orgA, leadID, "UNQ", domain.TemplateKanban, domain.VisibilityOpen)
	if err := repo.Save(ctx, again); apperror.CodeOf(err) != "project.key_taken" {
		t.Fatalf("same key in same org: code = %q", apperror.CodeOf(err))
	}

	other := newDomainProject(t, orgB, leadID, "UNQ", domain.TemplateKanban, domain.VisibilityOpen)
	if err := repo.Save(ctx, other); err != nil {
		t.Fatalf("same key in another org must be allowed: %v", err)
	}

	if err := repo.Delete(ctx, orgA, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(ctx, first.ID); apperror.CodeOf(err) != "project.not_found" {
		t.Fatalf("deleted project still readable: %v", err)
	}
	if _, err := repo.FindByKey(ctx, orgA, "UNQ"); apperror.CodeOf(err) != "project.not_found" {
		t.Fatalf("deleted project found by key: %v", err)
	}
	available, err := repo.KeyAvailable(ctx, orgA, "UNQ")
	if err != nil || !available {
		t.Fatalf("KeyAvailable after soft delete = %v, %v", available, err)
	}
	reused := newDomainProject(t, orgA, leadID, "UNQ", domain.TemplateKanban, domain.VisibilityOpen)
	if err := repo.Save(ctx, reused); err != nil {
		t.Fatalf("reuse of the freed key: %v", err)
	}
	if err := repo.Delete(ctx, orgA, first.ID); apperror.CodeOf(err) != "project.not_found" {
		t.Fatalf("second soft delete = %q", apperror.CodeOf(err))
	}
}

func TestAllocateIssueNumberIsAtomicUnderConcurrency(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewProjectRepository(db)
	leadID := insertUser(t, db)
	p := newDomainProject(t, uuid.MustParse(defaultOrganizationID), leadID, "CNT", domain.TemplateKanban, domain.VisibilityOpen)
	if err := repo.CreateWithDefaults(ctx, p, port.Member{UserID: leadID, Role: "lead"}); err != nil {
		t.Fatal(err)
	}

	const workers = 20
	numbers := make([]int, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			numbers[i], errs[i] = repo.AllocateIssueNumber(ctx, p.ID)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Ints(numbers)
	for i, n := range numbers {
		if n != i+1 {
			t.Fatalf("numbers = %v, want 1..%d without gaps or repeats", numbers, workers)
		}
	}
	stored, err := repo.FindByID(ctx, p.ID)
	if err != nil || stored.NextIssueNumber != workers+1 {
		t.Fatalf("next issue number = %d, %v", stored.NextIssueNumber, err)
	}

	if err := repo.Delete(ctx, uuid.MustParse(defaultOrganizationID), p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AllocateIssueNumber(ctx, p.ID); apperror.CodeOf(err) != "project.not_found" {
		t.Fatalf("allocate on deleted project = %q", apperror.CodeOf(err))
	}
}

func TestListPageVisibilityFiltersAndPagination(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewProjectRepository(db)
	orgID := uuid.MustParse(defaultOrganizationID)
	owner := insertUser(t, db)
	viewer := insertUser(t, db)
	outsider := insertUser(t, db)

	create := func(key, name string, template domain.Template, visibility domain.Visibility) *domain.Project {
		t.Helper()
		p := newDomainProject(t, orgID, owner, key, template, visibility)
		p.Name = name
		if err := repo.CreateWithDefaults(ctx, p, port.Member{UserID: owner, Role: "lead"}); err != nil {
			t.Fatal(err)
		}
		return p
	}
	openAlpha := create("OAL", "Alpha open", domain.TemplateKanban, domain.VisibilityOpen)
	openBeta := create("OBE", "Beta open", domain.TemplateScrum, domain.VisibilityOpen)
	privateSecret := create("PSE", "Secret", domain.TemplateKanban, domain.VisibilityPrivate)
	archivedGamma := create("OGA", "Gamma archived", domain.TemplateTaskTracking, domain.VisibilityOpen)
	if err := archivedGamma.Archive(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, archivedGamma); err != nil {
		t.Fatal(err)
	}

	// The viewer is an explicit member of the private project only.
	if _, err := db.Exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'viewer')`, privateSecret.ID, viewer); err != nil {
		t.Fatal(err)
	}
	scope := port.VisibilityScope{UserID: viewer, Open: true}

	names := func(page port.ProjectPage) []string {
		out := make([]string, 0, len(page.Items))
		for _, p := range page.Items {
			out = append(out, p.Name)
		}
		sort.Strings(out)
		return out
	}

	page, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Archived: port.ArchiveFilterExclude, PageSize: 25, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alpha open", "Beta open", "Secret"}; len(names(page)) != 3 || names(page)[0] != want[0] || page.Total != 3 {
		t.Fatalf("viewer list = %v total %d", names(page), page.Total)
	}

	outsiderPage, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: port.VisibilityScope{UserID: outsider}, PageSize: 25, Page: 1})
	if err != nil || outsiderPage.Total != 0 {
		t.Fatalf("outsider sees %d projects, %v", outsiderPage.Total, err)
	}

	owned, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: port.VisibilityScope{UserID: owner, All: true}, Archived: port.ArchiveFilterInclude, PageSize: 25, Page: 1})
	if err != nil || owned.Total != 4 {
		t.Fatalf("owner with archived = %d, %v", owned.Total, err)
	}

	byQuery, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Q: "ALPHA", PageSize: 25, Page: 1})
	if err != nil || byQuery.Total != 1 || byQuery.Items[0].ID != openAlpha.ID {
		t.Fatalf("name query = %d, %v", byQuery.Total, err)
	}
	byKey, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Q: "obe", PageSize: 25, Page: 1})
	if err != nil || byKey.Total != 1 || byKey.Items[0].ID != openBeta.ID {
		t.Fatalf("key query = %d, %v", byKey.Total, err)
	}

	byTemplate, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Template: domain.TemplateScrum, PageSize: 25, Page: 1})
	if err != nil || byTemplate.Total != 1 || byTemplate.Items[0].ID != openBeta.ID {
		t.Fatalf("template filter = %d, %v", byTemplate.Total, err)
	}

	if _, err := db.Exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES ($1, 'project', $2)`, viewer, openBeta.ID); err != nil {
		t.Fatal(err)
	}
	starred, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Starred: true, PageSize: 25, Page: 1})
	if err != nil || starred.Total != 1 || starred.Items[0].ID != openBeta.ID {
		t.Fatalf("starred = %d, %v", starred.Total, err)
	}

	pageOne, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Sort: port.SortByKey, Desc: true, PageSize: 2, Page: 1})
	if err != nil || len(pageOne.Items) != 2 || pageOne.Total != 3 || pageOne.Items[0].Key != "PSE" {
		t.Fatalf("page one = %d items total %d", len(pageOne.Items), pageOne.Total)
	}
	pageTwo, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: scope, Sort: port.SortByKey, Desc: true, PageSize: 2, Page: 2})
	if err != nil || len(pageTwo.Items) != 1 || pageTwo.Items[0].Key != "OAL" {
		t.Fatalf("page two = %d items", len(pageTwo.Items))
	}

	team := insertTeam(t, db, orgID)
	if _, err := db.Exec(`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'member')`, team, outsider); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project_team_roles (project_id, team_id, role) VALUES ($1, $2, 'viewer')`, privateSecret.ID, team); err != nil {
		t.Fatal(err)
	}
	viaTeam, err := repo.ListPage(ctx, port.ProjectFilter{OrganizationID: orgID, Scope: port.VisibilityScope{UserID: outsider}, PageSize: 25, Page: 1})
	if err != nil || viaTeam.Total != 1 || viaTeam.Items[0].ID != privateSecret.ID {
		t.Fatalf("team grant visibility = %d, %v", viaTeam.Total, err)
	}
}

func TestCreateWithMembersRollsBackOnForeignTeam(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewProjectRepository(db)
	leadID := insertUser(t, db)
	orgID := uuid.MustParse(defaultOrganizationID)
	foreignTeam := insertTeam(t, db, insertOrganization(t, db))

	p := newDomainProject(t, orgID, leadID, "ROL", domain.TemplateKanban, domain.VisibilityOpen)
	err := repo.CreateWithMembers(ctx, p, port.Member{UserID: leadID, Role: "lead"}, nil, []uuid.UUID{foreignTeam})
	if apperror.CodeOf(err) != "project.team_not_in_org" {
		t.Fatalf("foreign team code = %q", apperror.CodeOf(err))
	}
	var count int
	if err := db.Get(&count, `SELECT count(*) FROM projects WHERE key = 'ROL'`); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("project persisted despite failed grant: %d rows", count)
	}
}

func TestCreateWithMembersPersistsLeadMembersAndTeams(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewProjectRepository(db)
	leadID := insertUser(t, db)
	memberID := insertUser(t, db)
	orgID := uuid.MustParse(defaultOrganizationID)
	team := insertTeam(t, db, orgID)

	p := newDomainProject(t, orgID, leadID, "FULL", domain.TemplateBugTracking, domain.VisibilityPrivate)
	if err := repo.CreateWithMembers(ctx, p, port.Member{UserID: leadID, Role: "lead"},
		[]port.Member{{UserID: memberID, Role: "contributor"}}, []uuid.UUID{team}); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Template != domain.TemplateBugTracking || stored.Type != domain.ProjectTypeKanban || stored.Color != domain.DefaultColor {
		t.Fatalf("stored = template %q type %q color %q", stored.Template, stored.Type, stored.Color)
	}
	var members, grants int
	if err := db.Get(&members, `SELECT count(*) FROM project_members WHERE project_id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&grants, `SELECT count(*) FROM project_team_roles WHERE project_id = $1 AND role = 'contributor'`, p.ID); err != nil {
		t.Fatal(err)
	}
	if members != 2 || grants != 1 {
		t.Fatalf("members = %d grants = %d", members, grants)
	}
}
