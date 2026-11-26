package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func addOrgMember(t *testing.T, db *sqlx.DB, org, user uuid.UUID, role string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, org, user, role); err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, db *sqlx.DB, table string, user uuid.UUID) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM `+table+` WHERE user_id = $1`, user); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMembershipDetacherSoleOwnerCount(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	detacher := NewMembershipDetacher(db)
	repo := NewUserRepository(db)

	sole := newUserFixture(t, repo, "Sole Owner")
	shared := newUserFixture(t, repo, "Shared Owner")
	coOwner := newUserFixture(t, repo, "Co Owner")
	soleOrg := insertOrganization(t, db, "sole-org")
	sharedOrg := insertOrganization(t, db, "shared-org")
	addOrgMember(t, db, soleOrg, sole.ID, "owner")
	addOrgMember(t, db, sharedOrg, shared.ID, "owner")
	addOrgMember(t, db, sharedOrg, coOwner.ID, "owner")

	n, err := detacher.SoleOwnedOrganizations(ctx, sole.ID)
	if err != nil || n != 1 {
		t.Fatalf("sole owner: n=%d err=%v, want 1", n, err)
	}
	n, err = detacher.SoleOwnedOrganizations(ctx, shared.ID)
	if err != nil || n != 0 {
		t.Fatalf("owner with a co-owner: n=%d err=%v, want 0", n, err)
	}

	if _, err := db.Exec(`UPDATE organizations SET deleted_at = now() WHERE id = $1`, soleOrg); err != nil {
		t.Fatal(err)
	}
	n, err = detacher.SoleOwnedOrganizations(ctx, sole.ID)
	if err != nil || n != 0 {
		t.Fatalf("deleted organization must not block: n=%d err=%v", n, err)
	}
}

func TestMembershipDetacherRemovesAllMemberships(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	user := newUserFixture(t, repo, "Leaving Member")
	other := newUserFixture(t, repo, "Staying Owner")
	org := insertOrganization(t, db, "cascade-org")
	addOrgMember(t, db, org, other.ID, "owner")
	addOrgMember(t, db, org, user.ID, "member")
	team := insertTeam(t, db, org, "Cascade team")
	if _, err := db.Exec(`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)`, team, user.ID); err != nil {
		t.Fatal(err)
	}
	projectID := uuid.New()
	if _, err := db.Exec(
		`INSERT INTO projects (id, key, name, lead_id, organization_id) VALUES ($1, 'CASC', 'Cascade', $2, $3)`,
		projectID, other.ID, org,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'viewer')`, projectID, user.ID); err != nil {
		t.Fatal(err)
	}

	if err := NewMembershipDetacher(db).DetachMemberships(ctx, user.ID); err != nil {
		t.Fatalf("DetachMemberships: %v", err)
	}
	for _, table := range []string{"organization_members", "team_members", "project_members"} {
		if got := countRows(t, db, table, user.ID); got != 0 {
			t.Errorf("%s still has %d rows for the user", table, got)
		}
	}
	if got := countRows(t, db, "organization_members", other.ID); got != 1 {
		t.Errorf("other members must be kept, organization_members rows = %d", got)
	}
}

// TestDeactivateAccountCascadeAtomicPostgres runs the real use case over the real repositories: the
// account is closed, memberships are removed and sessions are revoked in one transaction.
func TestDeactivateAccountCascadeAtomicPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	sessions := NewSessionRepository(db)
	svc := usecase.NewUserService(repo, sessions).WithAccountClosure(database.NewTxManager(db), NewMembershipDetacher(db))

	user, err := domain.NewUser("Closing Member", "closing@corelog.test", mustHash(t, "clave-segura-1"), domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, user); err != nil {
		t.Fatal(err)
	}
	org := insertOrganization(t, db, "closing-org")
	addOrgMember(t, db, org, user.ID, "member")
	if _, err := svc.OpenSession(ctx, user.ID, "test", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeactivateAccount(ctx, user.ID, "clave-segura-1"); err != nil {
		t.Fatalf("DeactivateAccount: %v", err)
	}
	if got := countRows(t, db, "organization_members", user.ID); got != 0 {
		t.Fatalf("organization membership not removed: %d", got)
	}
	var active int
	if err := db.Get(&active, `SELECT COUNT(*) FROM user_sessions WHERE user_id = $1 AND revoked_at IS NULL`, user.ID); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("sessions still active: %d", active)
	}
	stored, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IsActive() {
		t.Fatal("account must be deactivated")
	}
}

// TestDeactivateAccountBlockedForSoleOwnerPostgres checks that the typed conflict leaves the account and
// its memberships untouched.
func TestDeactivateAccountBlockedForSoleOwnerPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	svc := usecase.NewUserService(repo, NewSessionRepository(db)).
		WithAccountClosure(database.NewTxManager(db), NewMembershipDetacher(db))

	user, err := domain.NewUser("Sole Closing", "sole-closing@corelog.test", mustHash(t, "clave-segura-1"), domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, user); err != nil {
		t.Fatal(err)
	}
	org := insertOrganization(t, db, "sole-closing-org")
	addOrgMember(t, db, org, user.ID, "owner")

	err = svc.DeactivateAccount(ctx, user.ID, "clave-segura-1")
	if !errors.Is(err, domain.ErrSoleOwner) {
		t.Fatalf("want ErrSoleOwner, got %v", err)
	}
	if got := countRows(t, db, "organization_members", user.ID); got != 1 {
		t.Fatalf("owner membership must be kept, rows = %d", got)
	}
	stored, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.IsActive() {
		t.Fatal("account must stay active")
	}
}

// TestUserSearchExcludesDeactivatedPostgres checks that the user search never returns closed accounts.
func TestUserSearchExcludesDeactivatedPostgres(t *testing.T) {
	db := testdb.New(t)
	repo := NewUserRepository(db)
	live := newUserFixture(t, repo, "Live Searchable")
	closed := newUserFixture(t, repo, "Closed Searchable")
	if _, err := db.Exec(`UPDATE users SET deactivated_at = now() WHERE id = $1`, closed.ID); err != nil {
		t.Fatal(err)
	}

	found, err := repo.Search(context.Background(), port.SearchFilter{Query: "Searchable"})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range found {
		if u.ID == closed.ID {
			t.Fatal("search returned a deactivated account")
		}
	}
	seen := false
	for _, u := range found {
		if u.ID == live.ID {
			seen = true
		}
	}
	if !seen {
		t.Fatal("search lost the live account")
	}
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}
