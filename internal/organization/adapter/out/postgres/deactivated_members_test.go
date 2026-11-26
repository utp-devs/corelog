package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// TestMembersExcludeDeactivatedAccountsPostgres checks that a closed account disappears from the member
// list and from the person profile, while its organization row (if it still exists) is not returned.
func TestMembersExcludeDeactivatedAccountsPostgres(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Acme", "acme-deactivated")
	if err := repo.AddMember(ctx, &domain.Member{OrganizationID: org.ID, UserID: owner, Role: domain.RoleOwner, JoinedAt: testNow}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	closed := newUser(t, db)
	if err := repo.AddMember(ctx, &domain.Member{OrganizationID: org.ID, UserID: closed, Role: domain.RoleMember, JoinedAt: testNow}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET deactivated_at = now() WHERE id = $1`, closed); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	views, err := repo.ListMemberViews(ctx, org.ID)
	if err != nil {
		t.Fatalf("ListMemberViews: %v", err)
	}
	if len(views) != 1 || views[0].UserID != owner {
		t.Fatalf("views = %+v, want only the owner", views)
	}
	if _, err := repo.GetMemberProfile(ctx, org.ID, closed); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("profile of a deactivated member: want ErrMemberNotFound, got %v", err)
	}
}
