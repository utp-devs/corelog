package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// newInvitation builds a pending invitation with a fresh token hash.
func newInvitation(t *testing.T, orgID, by uuid.UUID, email string, role domain.Role) *domain.Invitation {
	t.Helper()
	_, hash, err := domain.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	inv, err := domain.NewInvitation(orgID, email, role, by, hash, testNow)
	if err != nil {
		t.Fatalf("new invitation: %v", err)
	}
	return inv
}

func TestInvitationCRUD(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Acme", "acme-inv")

	inv := newInvitation(t, org.ID, owner, "Invitee@Example.test", domain.RoleMember)
	if err := repo.CreateInvitation(ctx, inv); err != nil {
		t.Fatalf("create: %v", err)
	}
	if inv.ID == uuid.Nil {
		t.Fatal("create must assign the ID")
	}

	got, err := repo.GetInvitationByID(ctx, org.ID, inv.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Email != "invitee@example.test" || got.Role != domain.RoleMember || got.InvitedBy != owner || got.TokenHash != inv.TokenHash {
		t.Fatalf("round trip = %+v", got)
	}
	if !got.ExpiresAt.Equal(testNow.Add(domain.InvitationTTL)) || got.AcceptedAt != nil || got.RevokedAt != nil {
		t.Fatalf("timestamps = %v accepted=%v revoked=%v", got.ExpiresAt, got.AcceptedAt, got.RevokedAt)
	}

	// Isolation: an invitation id is invisible from another organization.
	other := newOrg(t, repo, owner, "Otra", "otra-inv")
	_, err = repo.GetInvitationByID(ctx, other.ID, inv.ID)
	requireIs(t, err, domain.ErrInvitationNotFound)

	byHash, err := repo.GetInvitationByTokenHash(ctx, inv.TokenHash)
	if err != nil || byHash.ID != inv.ID {
		t.Fatalf("lookup by hash: %v", err)
	}
	_, err = repo.GetInvitationByTokenHash(ctx, "missing-hash")
	requireIs(t, err, domain.ErrInvitationNotFound)

	acceptedAt := testNow.Add(time.Minute)
	got.AcceptedAt = &acceptedAt
	if err := repo.UpdateInvitation(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, _ := repo.GetInvitationByID(ctx, org.ID, inv.ID)
	if reloaded.AcceptedAt == nil || !reloaded.AcceptedAt.Equal(acceptedAt) {
		t.Fatalf("accepted_at not persisted: %v", reloaded.AcceptedAt)
	}

	pending, err := repo.ListInvitations(ctx, org.ID, true)
	if err != nil || len(pending) != 0 {
		t.Fatalf("accepted invitation must leave the pending list: %d %v", len(pending), err)
	}
	all, _ := repo.ListInvitations(ctx, org.ID, false)
	if len(all) != 1 {
		t.Fatalf("unfiltered list = %d", len(all))
	}

	ghost := *got
	ghost.ID = uuid.New()
	requireIs(t, repo.UpdateInvitation(ctx, &ghost), domain.ErrInvitationNotFound)
}

func TestPendingInvitationConstraint(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Acme", "acme-pend")

	first := newInvitation(t, org.ID, owner, "dup@example.test", domain.RoleMember)
	if err := repo.CreateInvitation(ctx, first); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := newInvitation(t, org.ID, owner, "DUP@example.test", domain.RoleGuest)
	requireIs(t, repo.CreateInvitation(ctx, second), domain.ErrInvitationExists)

	// Once the pending one is revoked, a new invitation for the same email is allowed.
	revokedAt := testNow.Add(time.Minute)
	first.RevokedAt = &revokedAt
	if err := repo.UpdateInvitation(ctx, first); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := repo.CreateInvitation(ctx, second); err != nil {
		t.Fatalf("after revoke: %v", err)
	}

	// The same email in another organization is independent.
	other := newOrg(t, repo, owner, "Otra", "otra-pend")
	if err := repo.CreateInvitation(ctx, newInvitation(t, other.ID, owner, "dup@example.test", domain.RoleMember)); err != nil {
		t.Fatalf("other org: %v", err)
	}
}

func TestMemberViewsAndFindByEmail(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Acme", "acme-views")
	if err := repo.AddMember(ctx, &domain.Member{OrganizationID: org.ID, UserID: owner, Role: domain.RoleOwner, JoinedAt: testNow}); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	member := newUser(t, db)
	if err := repo.AddMember(ctx, &domain.Member{OrganizationID: org.ID, UserID: member, Role: domain.RoleGuest, JoinedAt: testNow.Add(time.Hour)}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET display_name = 'Mario Pérez' WHERE id = $1`, member); err != nil {
		t.Fatalf("set display name: %v", err)
	}

	views, err := repo.ListMemberViews(ctx, org.ID)
	if err != nil {
		t.Fatalf("list views: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("views = %d, want 2", len(views))
	}
	var found port.MemberView
	for _, v := range views {
		if v.UserID == member {
			found = v
		}
	}
	if found.Name != "Mario Pérez" || found.Email != member.String()+"@example.test" || found.Role != domain.RoleGuest {
		t.Fatalf("member view = %+v", found)
	}

	ok, err := repo.FindMemberByEmail(ctx, org.ID, member.String()+"@EXAMPLE.test")
	if err != nil || !ok {
		t.Fatalf("find by email (case-insensitive): %v %v", ok, err)
	}
	ok, _ = repo.FindMemberByEmail(ctx, org.ID, "nobody@example.test")
	if ok {
		t.Fatal("unknown email must not match")
	}

	// Views of a deleted organization are hidden.
	if err := repo.SoftDelete(ctx, org.ID, testNow.Add(time.Hour)); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	views, _ = repo.ListMemberViews(ctx, org.ID)
	if len(views) != 0 {
		t.Fatalf("deleted org views = %d", len(views))
	}
}

// TestInviteAcceptThroughPostgres runs the real use case over Postgres: invite, preview, accept, and
// the member appears in the list with its events in the outbox.
func TestInviteAcceptThroughPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	svc := newPostgresService(db)
	owner := newUser(t, db)
	invitee := newUser(t, db)
	inviteeEmail := invitee.String() + "@example.test"

	org, err := svc.CreateOrganization(ctx, owner, port.CreateInput{Name: "Equipo Invitación", Slug: "equipo-invitacion"})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	res, err := svc.CreateInvitations(ctx, owner, org.ID, []port.InviteInput{{Email: inviteeEmail, Role: domain.RoleMember}})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if n := countEvents(t, db, org.ID, "invitation.created"); n != 1 {
		t.Fatalf("invitation.created events = %d", n)
	}

	preview, err := svc.GetInvitationPreview(ctx, res[0].PlainToken)
	if err != nil || preview.OrganizationName != "Equipo Invitación" || preview.Status != domain.InvitationPending {
		t.Fatalf("preview = %+v, %v", preview, err)
	}

	if _, err := svc.AcceptInvitation(ctx, invitee, inviteeEmail, res[0].PlainToken); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if n := countEvents(t, db, org.ID, "invitation.accepted"); n != 1 {
		t.Fatalf("invitation.accepted events = %d", n)
	}
	if n := countEvents(t, db, org.ID, "member.added"); n != 2 { // owner at creation, invitee now
		t.Fatalf("member.added events = %d", n)
	}

	members, err := svc.ListMembers(ctx, invitee, org.ID)
	if err != nil {
		t.Fatalf("list members as new member: %v", err)
	}
	var listed bool
	for _, m := range members {
		if m.UserID == invitee && m.Role == domain.RoleMember && m.Email == inviteeEmail {
			listed = true
		}
	}
	if len(members) != 2 || !listed {
		t.Fatalf("members = %+v", members)
	}

	_, err = svc.AcceptInvitation(ctx, invitee, inviteeEmail, res[0].PlainToken)
	requireAppErr(t, err, "conflict", "org.invitation_used")
}

// TestMemberProfileColumns checks the member read models: profile columns come from users, the avatar key
// is reduced to has_avatar, a non-member has no profile, and a deleted organization hides everything.
func TestMemberProfileColumns(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Acme", "acme-profile")
	if err := repo.AddMember(ctx, &domain.Member{OrganizationID: org.ID, UserID: owner, Role: domain.RoleOwner, JoinedAt: testNow}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	member := newUser(t, db)
	if err := repo.AddMember(ctx, &domain.Member{OrganizationID: org.ID, UserID: member, Role: domain.RoleMember, JoinedAt: testNow.Add(time.Hour)}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET display_name = 'Mario', job_title = 'Ingeniero',
		department = 'Operaciones', avatar_key = 'avatars/x.png', timezone = 'Europe/Madrid' WHERE id = $1`, member); err != nil {
		t.Fatalf("set profile: %v", err)
	}

	views, err := repo.ListMemberViews(ctx, org.ID)
	if err != nil {
		t.Fatalf("list views: %v", err)
	}
	var found port.MemberView
	for _, v := range views {
		if v.UserID == member {
			found = v
		}
	}
	if found.Name != "Mario" || found.DisplayName != "Mario" || found.JobTitle != "Ingeniero" ||
		found.Department != "Operaciones" || !found.HasAvatar {
		t.Fatalf("member view = %+v", found)
	}
	if views[0].JobTitle != "" || views[0].HasAvatar {
		t.Fatalf("owner without profile data = %+v", views[0])
	}

	profile, err := repo.GetMemberProfile(ctx, org.ID, member)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.Timezone != "Europe/Madrid" || profile.Role != domain.RoleMember || profile.Email != member.String()+"@example.test" {
		t.Fatalf("profile = %+v", profile)
	}

	if _, err := repo.GetMemberProfile(ctx, org.ID, newUser(t, db)); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("non-member: %v", err)
	}
	if err := repo.SoftDelete(ctx, org.ID, testNow.Add(time.Hour)); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.GetMemberProfile(ctx, org.ID, member); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("deleted org profile: %v", err)
	}
}
