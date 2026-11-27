package postgres

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const keyTestDefaultOrg = "00000000-0000-4000-8000-000000000002"

func keyTestUser(t *testing.T, db *sqlx.DB, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'U', 'U', $2, 'x', 'user')`,
		id, id.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, orgID, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func keyTestOrg(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'Other')`, id, "o-"+id.String()[:8]); err != nil {
		t.Fatal(err)
	}
	return id
}

func keyTestProject(t *testing.T, db *sqlx.DB, orgID, leadID uuid.UUID, key string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, lead_id) VALUES ($1, $2, $3, 'P', $4)`, id, orgID, key, leadID); err != nil {
		t.Fatal(err)
	}
	return id
}

func newKeyTestIssue(projectID, reporterID uuid.UUID, summary string) *domain.Issue {
	issue, err := domain.NewIssue(summary, "descripcion", domain.PriorityLow, reporterID)
	if err != nil {
		panic(err)
	}
	issue.ProjectID = projectID
	return issue
}

func TestIssueSaveAllocatesKeysUnderConcurrency(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewIssueRepository(db)
	org := uuid.MustParse(keyTestDefaultOrg)
	reporter := keyTestUser(t, db, org)
	project := keyTestProject(t, db, org, reporter, "CNC")

	const workers = 12
	var wg sync.WaitGroup
	issues := make([]*domain.Issue, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			issues[i] = newKeyTestIssue(project, reporter, "concurrent issue")
			errs[i] = repo.Save(ctx, issues[i])
		}(i)
	}
	wg.Wait()

	numbers := make([]int, 0, workers)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		if issues[i].ProjectKey != "CNC" {
			t.Errorf("project key = %q, want CNC", issues[i].ProjectKey)
		}
		numbers = append(numbers, issues[i].KeyNumber)
	}
	sort.Ints(numbers)
	for i, n := range numbers {
		if n != i+1 {
			t.Fatalf("numbers = %v, want 1..%d without gaps or repeats", numbers, workers)
		}
	}

	var next int
	if err := db.Get(&next, `SELECT next_issue_number FROM projects WHERE id = $1`, project); err != nil || next != workers+1 {
		t.Fatalf("next_issue_number = %d, %v; want %d", next, err, workers+1)
	}
}

func TestIssueSaveFailureDoesNotBurnANumber(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewIssueRepository(db)
	org := uuid.MustParse(keyTestDefaultOrg)
	reporter := keyTestUser(t, db, org)
	project := keyTestProject(t, db, org, reporter, "BRN")

	bad := newKeyTestIssue(project, uuid.New(), "unknown reporter")
	if err := repo.Save(ctx, bad); apperror.CodeOf(err) != "issue.reporter_not_found" {
		t.Fatalf("code = %q (%v), want issue.reporter_not_found", apperror.CodeOf(err), err)
	}
	good := newKeyTestIssue(project, reporter, "first real issue")
	if err := repo.Save(ctx, good); err != nil {
		t.Fatal(err)
	}
	if good.KeyNumber != 1 {
		t.Fatalf("key number = %d, want 1: the failed insert consumed a number", good.KeyNumber)
	}
}

func TestIssueKeyResolutionIsolatedByOrganization(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewIssueRepository(db)
	orgA := uuid.MustParse(keyTestDefaultOrg)
	orgB := keyTestOrg(t, db)
	userA := keyTestUser(t, db, orgA)
	userB := keyTestUser(t, db, orgB)
	both := keyTestUser(t, db, orgA)
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, orgB, both); err != nil {
		t.Fatal(err)
	}

	projectA := keyTestProject(t, db, orgA, userA, "ISO")
	projectB := keyTestProject(t, db, orgB, userB, "ISO")
	issueA := newKeyTestIssue(projectA, userA, "in org A")
	issueB := newKeyTestIssue(projectB, userB, "in org B")
	for _, issue := range []*domain.Issue{issueA, issueB} {
		if err := repo.Save(ctx, issue); err != nil {
			t.Fatal(err)
		}
		if issue.Key() != "ISO-1" {
			t.Fatalf("key = %q, want ISO-1 in both organizations", issue.Key())
		}
	}

	find := func(org, user uuid.UUID) []*domain.Issue {
		t.Helper()
		found, err := repo.FindKeyCandidates(ctx, "ISO", 1, org, user, false)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	if got := find(uuid.Nil, userA); len(got) != 1 || got[0].ID != issueA.ID {
		t.Errorf("user of org A sees %d candidates, want only the issue of org A", len(got))
	}
	if got := find(uuid.Nil, userB); len(got) != 1 || got[0].ID != issueB.ID {
		t.Errorf("user of org B sees %d candidates, want only the issue of org B", len(got))
	}
	if got := find(uuid.Nil, both); len(got) != 2 {
		t.Errorf("user of both organizations sees %d candidates, want 2 (ambiguous)", len(got))
	}
	if got := find(orgB, both); len(got) != 1 || got[0].ID != issueB.ID {
		t.Errorf("organization_id filter returned %d candidates, want the issue of org B", len(got))
	}
	if got := find(orgB, userA); len(got) != 0 {
		t.Errorf("user of org A reached %d issues of org B", len(got))
	}
	if all, err := repo.FindKeyCandidates(ctx, "ISO", 1, uuid.Nil, uuid.New(), true); err != nil || len(all) != 2 {
		t.Errorf("platform admin candidates = %d, %v; want 2", len(all), err)
	}
	if got, err := repo.FindKeyCandidates(ctx, "ISO", 2, uuid.Nil, userA, false); err != nil || len(got) != 0 {
		t.Errorf("unknown number returned %d candidates, %v", len(got), err)
	}

	found, err := repo.FindByID(ctx, issueA.ID)
	if err != nil || found.Key() != "ISO-1" || found.Summary != "in org A" {
		t.Errorf("FindByID = %+v, %v", found, err)
	}

	// A soft-deleted project hides its issues from key lookups.
	if _, err := db.Exec(`UPDATE projects SET deleted_at = $1 WHERE id = $2`, time.Now(), projectA); err != nil {
		t.Fatal(err)
	}
	if got := find(uuid.Nil, userA); len(got) != 0 {
		t.Errorf("deleted project still resolves: %d candidates", len(got))
	}
}
