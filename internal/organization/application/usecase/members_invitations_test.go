package usecase

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// addUser registers a user account in the fake repository and returns its ID.
func addUser(f *fixture, name, email string) uuid.UUID {
	id := uuid.New()
	f.repo.users[id] = fakeUser{name: name, email: email}
	return id
}

// addMember inserts a membership directly, bypassing the use cases.
func addMember(f *fixture, userID uuid.UUID, role domain.Role) {
	f.repo.members[memberKey{f.org, userID}] = &domain.Member{OrganizationID: f.org, UserID: userID, Role: role, JoinedAt: fixedNow}
}

// createOne invites one email as actor and returns the stored invitation with its plain token.
func createOne(t *testing.T, f *fixture, actor uuid.UUID, email string, role domain.Role) port.InvitationResult {
	t.Helper()
	res, err := f.svc.CreateInvitations(context.Background(), actor, f.org, []port.InviteInput{{Email: email, Role: role}})
	if err != nil {
		t.Fatalf("create invitation for %s: %v", email, err)
	}
	return res[0]
}

// outcome classifies an error for the permission matrix.
func outcome(err error) string {
	if err == nil {
		return "ok"
	}
	switch apperror.KindOf(err) {
	case apperror.KindForbidden:
		return "forbidden"
	case apperror.KindNotFound:
		return "notfound"
	}
	return "other:" + string(apperror.KindOf(err)) + "/" + apperror.CodeOf(err)
}

// TestPermissionMatrix checks every member operation for owner, admin, member, guest and an outsider.
// Each cell runs on a fresh fixture, because the operations mutate state.
func TestPermissionMatrix(t *testing.T) {
	ctx := context.Background()
	const (
		allowed = "ok"
		denied  = "forbidden"
		hidden  = "notfound"
	)
	ops := []struct {
		name string
		run  func(t *testing.T, f *fixture, actor uuid.UUID) error
		want map[string]string
	}{
		{
			name: "ListMembers",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				_, err := f.svc.ListMembers(ctx, actor, f.org)
				return err
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": allowed, "guest": denied, "outsider": hidden},
		},
		{
			name: "ChangeMemberRole",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				return f.svc.ChangeMemberRole(ctx, actor, f.org, f.membr, domain.RoleGuest)
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": denied, "guest": denied, "outsider": hidden},
		},
		{
			name: "RemoveMember",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				return f.svc.RemoveMember(ctx, actor, f.org, f.guest)
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": denied, "guest": denied, "outsider": hidden},
		},
		{
			name: "CreateInvitations",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				_, err := f.svc.CreateInvitations(ctx, actor, f.org, []port.InviteInput{{Email: "matrix@acme.test", Role: domain.RoleMember}})
				return err
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": denied, "guest": denied, "outsider": hidden},
		},
		{
			name: "ListInvitations",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				_, err := f.svc.ListInvitations(ctx, actor, f.org, true)
				return err
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": denied, "guest": denied, "outsider": hidden},
		},
		{
			name: "ResendInvitation",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				inv := createOne(t, f, f.owner, "resend@acme.test", domain.RoleMember)
				_, err := f.svc.ResendInvitation(ctx, actor, f.org, inv.Invitation.ID)
				return err
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": denied, "guest": denied, "outsider": hidden},
		},
		{
			name: "RevokeInvitation",
			run: func(t *testing.T, f *fixture, actor uuid.UUID) error {
				inv := createOne(t, f, f.owner, "revoke@acme.test", domain.RoleMember)
				return f.svc.RevokeInvitation(ctx, actor, f.org, inv.Invitation.ID)
			},
			want: map[string]string{"owner": allowed, "admin": allowed, "member": denied, "guest": denied, "outsider": hidden},
		},
	}
	actors := map[string]func(f *fixture) uuid.UUID{
		"owner":    func(f *fixture) uuid.UUID { return f.owner },
		"admin":    func(f *fixture) uuid.UUID { return f.admin },
		"member":   func(f *fixture) uuid.UUID { return f.membr },
		"guest":    func(f *fixture) uuid.UUID { return f.guest },
		"outsider": func(f *fixture) uuid.UUID { return f.out },
	}
	for _, op := range ops {
		for _, name := range []string{"owner", "admin", "member", "guest", "outsider"} {
			t.Run(op.name+"/"+name, func(t *testing.T) {
				f := newFixture(t)
				err := op.run(t, f, actors[name](f))
				if got := outcome(err); got != op.want[name] {
					t.Fatalf("outcome = %s, want %s (%v)", got, op.want[name], err)
				}
			})
		}
	}
}

func TestInviteRolePolicyAndInputs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	many := make([]port.InviteInput, 51)
	for i := range many {
		many[i] = port.InviteInput{Email: uuid.NewString() + "@acme.test", Role: domain.RoleGuest}
	}
	cases := []struct {
		name  string
		actor uuid.UUID
		in    []port.InviteInput
		kind  apperror.Kind
		code  string
	}{
		{"admin invites admin", f.admin, []port.InviteInput{{Email: "a1@acme.test", Role: domain.RoleAdmin}}, apperror.KindForbidden, "org.forbidden"},
		{"owner invites owner", f.owner, []port.InviteInput{{Email: "a2@acme.test", Role: domain.RoleOwner}}, apperror.KindValidation, "org.role_invalid"},
		{"unknown role", f.owner, []port.InviteInput{{Email: "a3@acme.test", Role: "boss"}}, apperror.KindValidation, "org.role_invalid"},
		{"empty batch", f.owner, nil, apperror.KindValidation, "org.invitation_batch_invalid"},
		{"oversized batch", f.owner, many, apperror.KindValidation, "org.invitation_batch_invalid"},
		{"malformed email", f.owner, []port.InviteInput{{Email: "no-es-correo", Role: domain.RoleMember}}, apperror.KindValidation, "org.invitation_email_invalid"},
		{"display name form", f.owner, []port.InviteInput{{Email: "Ana <ana@acme.test>", Role: domain.RoleMember}}, apperror.KindValidation, "org.invitation_email_invalid"},
		{"invite existing member", f.owner, []port.InviteInput{{Email: membrEmail, Role: domain.RoleGuest}}, apperror.KindConflict, "org.already_member"},
		{"duplicate inside batch", f.owner, []port.InviteInput{{Email: "dup@acme.test", Role: domain.RoleMember}, {Email: "DUP@acme.test", Role: domain.RoleGuest}}, apperror.KindConflict, "org.invitation_exists"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.CreateInvitations(ctx, tc.actor, f.org, tc.in)
			requireErr(t, err, tc.kind, tc.code)
		})
	}
	if len(f.pub.Events()) != 0 {
		t.Fatalf("rejected batches must publish nothing, got %v", eventTypes(f.pub))
	}
	// Admins may invite member and guest.
	if _, err := f.svc.CreateInvitations(ctx, f.admin, f.org, []port.InviteInput{{Email: "ok@acme.test", Role: domain.RoleGuest}}); err != nil {
		t.Fatalf("admin invites guest: %v", err)
	}
}

func TestDuplicatePendingInvitation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	createOne(t, f, f.owner, "new@acme.test", domain.RoleMember)
	f.pub = newPub(f)
	_, err := f.svc.CreateInvitations(ctx, f.admin, f.org, []port.InviteInput{{Email: "NEW@acme.test", Role: domain.RoleGuest}})
	requireErr(t, err, apperror.KindConflict, "org.invitation_exists")
	if len(f.pub.Events()) != 0 {
		t.Fatalf("duplicate invitation must publish nothing, got %v", eventTypes(f.pub))
	}
}

func TestInvitationLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	newUser := addUser(f, "Nuevo Usuario", "new@acme.test")

	inv := createOne(t, f, f.owner, "new@acme.test", domain.RoleMember)
	if inv.Invitation.ID == uuid.Nil || inv.PlainToken == "" {
		t.Fatalf("create must assign id and return a token: %+v", inv)
	}
	if inv.Invitation.TokenHash != domain.HashToken(inv.PlainToken) || inv.Invitation.TokenHash == inv.PlainToken {
		t.Fatal("only the hash of the token may be stored")
	}
	if got := eventTypes(f.pub); !reflect.DeepEqual(got, []string{EventInvitationCreated}) {
		t.Fatalf("events after create = %v", got)
	}

	preview, err := f.svc.GetInvitationPreview(ctx, inv.PlainToken)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.OrganizationName != "Acme" || preview.Email != "new@acme.test" || preview.Role != domain.RoleMember || preview.Status != domain.InvitationPending {
		t.Fatalf("preview = %+v", preview)
	}

	member, err := f.svc.AcceptInvitation(ctx, newUser, "  NEW@acme.test ", inv.PlainToken)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if member.UserID != newUser || member.OrganizationID != f.org || member.Role != domain.RoleMember {
		t.Fatalf("membership = %+v", member)
	}
	want := []string{EventInvitationCreated, EventInvitationAccepted, EventMemberAdded}
	if got := eventTypes(f.pub); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}

	views, err := f.svc.ListMembers(ctx, f.membr, f.org)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	found := false
	for _, v := range views {
		if v.UserID == newUser {
			found = v.Email == "new@acme.test" && v.Name == "Nuevo Usuario" && v.Role == domain.RoleMember
		}
	}
	if !found {
		t.Fatalf("accepted member missing from list: %+v", views)
	}

	preview, _ = f.svc.GetInvitationPreview(ctx, inv.PlainToken)
	if preview.Status != domain.InvitationAccepted {
		t.Fatalf("status after accept = %s", preview.Status)
	}
	_, err = f.svc.AcceptInvitation(ctx, newUser, "new@acme.test", inv.PlainToken)
	requireErr(t, err, apperror.KindConflict, "org.invitation_used")
}

func TestAcceptRejectsEmailMismatchWithoutSideEffects(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	inv := createOne(t, f, f.owner, "new@acme.test", domain.RoleGuest)
	before := len(f.pub.Events())
	other := addUser(f, "Otra Persona", "other@acme.test")

	_, err := f.svc.AcceptInvitation(ctx, other, "other@acme.test", inv.PlainToken)
	requireErr(t, err, apperror.KindForbidden, "org.email_mismatch")
	if len(f.pub.Events()) != before {
		t.Fatal("mismatch must publish nothing")
	}
	preview, _ := f.svc.GetInvitationPreview(ctx, inv.PlainToken)
	if preview.Status != domain.InvitationPending {
		t.Fatalf("invitation must stay pending, got %s", preview.Status)
	}
}

func TestAcceptWhenAlreadyMember(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := addUser(f, "Ya Miembro", "again@acme.test")
	inv := createOne(t, f, f.owner, "again@acme.test", domain.RoleMember)
	addMember(f, user, domain.RoleGuest) // joined by another path after the invitation was sent

	_, err := f.svc.AcceptInvitation(ctx, user, "again@acme.test", inv.PlainToken)
	requireErr(t, err, apperror.KindConflict, "org.already_member")
	preview, _ := f.svc.GetInvitationPreview(ctx, inv.PlainToken)
	if preview.Status != domain.InvitationPending {
		t.Fatalf("failed accept must leave the invitation pending, got %s", preview.Status)
	}
}

func TestInvitationExpiry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := addUser(f, "Tarde", "late@acme.test")
	inv := createOne(t, f, f.owner, "late@acme.test", domain.RoleMember)

	f.clk.Advance(domain.InvitationTTL - time.Second)
	if p, _ := f.svc.GetInvitationPreview(ctx, inv.PlainToken); p.Status != domain.InvitationPending {
		t.Fatalf("one second before expiry status = %s", p.Status)
	}
	f.clk.Advance(time.Second)
	p, err := f.svc.GetInvitationPreview(ctx, inv.PlainToken)
	if err != nil || p.Status != domain.InvitationExpired {
		t.Fatalf("at expiry: status=%v err=%v", p, err)
	}
	_, err = f.svc.AcceptInvitation(ctx, user, "late@acme.test", inv.PlainToken)
	requireErr(t, err, apperror.KindConflict, "org.invitation_expired")

	pending, err := f.svc.ListInvitations(ctx, f.owner, f.org, true)
	if err != nil || len(pending) != 0 {
		t.Fatalf("expired invitation must not be listed as pending: %d %v", len(pending), err)
	}
	all, _ := f.svc.ListInvitations(ctx, f.owner, f.org, false)
	if len(all) != 1 {
		t.Fatalf("unfiltered list = %d, want 1", len(all))
	}
	// Expired invitations cannot be resent or revoked; a new invitation must replace them.
	_, err = f.svc.ResendInvitation(ctx, f.owner, f.org, inv.Invitation.ID)
	requireErr(t, err, apperror.KindConflict, "org.invitation_expired")
	err = f.svc.RevokeInvitation(ctx, f.owner, f.org, inv.Invitation.ID)
	requireErr(t, err, apperror.KindConflict, "org.invitation_expired")
}

func TestExpiredLeftoverIsRetiredOnReinvite(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	old := createOne(t, f, f.owner, "late@acme.test", domain.RoleMember)
	f.clk.Advance(domain.InvitationTTL)
	f.pub = newPub(f)

	again := createOne(t, f, f.owner, "late@acme.test", domain.RoleGuest)
	want := []string{EventInvitationRevoked, EventInvitationCreated}
	if got := eventTypes(f.pub); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	p, _ := f.svc.GetInvitationPreview(ctx, old.PlainToken)
	if p.Status != domain.InvitationRevoked {
		t.Fatalf("retired invitation status = %s", p.Status)
	}
	if p2, _ := f.svc.GetInvitationPreview(ctx, again.PlainToken); p2.Status != domain.InvitationPending {
		t.Fatalf("new invitation status = %s", p2.Status)
	}
}

func TestRevokeInvitation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := addUser(f, "Revocado", "revoked@acme.test")
	inv := createOne(t, f, f.owner, "revoked@acme.test", domain.RoleMember)
	f.pub = newPub(f)

	if err := f.svc.RevokeInvitation(ctx, f.admin, f.org, inv.Invitation.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := eventTypes(f.pub); !reflect.DeepEqual(got, []string{EventInvitationRevoked}) {
		t.Fatalf("events = %v", got)
	}
	p, _ := f.svc.GetInvitationPreview(ctx, inv.PlainToken)
	if p.Status != domain.InvitationRevoked {
		t.Fatalf("status = %s", p.Status)
	}
	_, err := f.svc.AcceptInvitation(ctx, user, "revoked@acme.test", inv.PlainToken)
	requireErr(t, err, apperror.KindConflict, "org.invitation_revoked")
	requireErr(t, f.svc.RevokeInvitation(ctx, f.owner, f.org, inv.Invitation.ID), apperror.KindConflict, "org.invitation_revoked")
	_, err = f.svc.ResendInvitation(ctx, f.owner, f.org, inv.Invitation.ID)
	requireErr(t, err, apperror.KindConflict, "org.invitation_revoked")

	// A revoked invitation of another organization id is invisible: cross-org isolation.
	requireErr(t, f.svc.RevokeInvitation(ctx, f.owner, uuid.New(), inv.Invitation.ID), apperror.KindNotFound, "org.not_found")
}

func TestResendInvalidatesOldToken(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := addUser(f, "Reenviado", "resend@acme.test")
	old := createOne(t, f, f.owner, "resend@acme.test", domain.RoleMember)

	f.clk.Advance(3 * 24 * time.Hour)
	f.pub = newPub(f)
	res, err := f.svc.ResendInvitation(ctx, f.admin, f.org, old.Invitation.ID)
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	if res.PlainToken == old.PlainToken || res.PlainToken == "" {
		t.Fatal("resend must issue a new token")
	}
	if want := f.clk.Now().Add(domain.InvitationTTL); !res.Invitation.ExpiresAt.Equal(want) {
		t.Fatalf("expiry = %v, want %v", res.Invitation.ExpiresAt, want)
	}
	if got := eventTypes(f.pub); !reflect.DeepEqual(got, []string{EventInvitationResent}) {
		t.Fatalf("events = %v", got)
	}

	_, err = f.svc.GetInvitationPreview(ctx, old.PlainToken)
	requireErr(t, err, apperror.KindNotFound, "org.invitation_not_found")
	_, err = f.svc.AcceptInvitation(ctx, user, "resend@acme.test", old.PlainToken)
	requireErr(t, err, apperror.KindNotFound, "org.invitation_not_found")

	if _, err := f.svc.AcceptInvitation(ctx, user, "resend@acme.test", res.PlainToken); err != nil {
		t.Fatalf("accept with new token: %v", err)
	}
}

// TestUnknownTokensLookLikeEachOther checks enumeration safety: every unknown token gets the same error.
func TestUnknownTokensLookLikeEachOther(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := addUser(f, "Curioso", "curious@acme.test")
	tokens := []string{"", "nope", domain.HashToken("nope"), createOne(t, f, f.owner, "someone@acme.test", domain.RoleGuest).PlainToken + "x"}

	var first error
	for i, tok := range tokens {
		_, errPreview := f.svc.GetInvitationPreview(ctx, tok)
		_, errAccept := f.svc.AcceptInvitation(ctx, user, "curious@acme.test", tok)
		requireErr(t, errPreview, apperror.KindNotFound, "org.invitation_not_found")
		requireErr(t, errAccept, apperror.KindNotFound, "org.invitation_not_found")
		if i == 0 {
			first = errPreview
		}
		if errPreview.Error() != first.Error() || errAccept.Error() != first.Error() {
			t.Fatalf("unknown token %q returned a different error shape: %v vs %v", tok, errPreview, first)
		}
	}
}

func TestMemberRoleRules(t *testing.T) {
	ctx := context.Background()
	t.Run("self role change", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.owner, f.org, f.owner, domain.RoleAdmin), apperror.KindValidation, "org.self_role_change")
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.admin, f.org, f.admin, domain.RoleMember), apperror.KindValidation, "org.self_role_change")
	})
	t.Run("admin cannot touch admins or owners", func(t *testing.T) {
		f := newFixture(t)
		admin2 := addUser(f, "Segundo Admin", "admin2@acme.test")
		addMember(f, admin2, domain.RoleAdmin)
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.admin, f.org, admin2, domain.RoleMember), apperror.KindForbidden, "org.forbidden")
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.admin, f.org, f.owner, domain.RoleMember), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("admin cannot promote to admin", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.admin, f.org, f.membr, domain.RoleAdmin), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("owner promotes and event carries old and new role", func(t *testing.T) {
		f := newFixture(t)
		f.pub = newPub(f)
		if err := f.svc.ChangeMemberRole(ctx, f.owner, f.org, f.membr, domain.RoleAdmin); err != nil {
			t.Fatalf("promote: %v", err)
		}
		ev := f.pub.Events()
		if len(ev) != 1 || ev[0].Type != EventMemberRoleChanged || ev[0].Payload["old_role"] != "member" || ev[0].Payload["new_role"] != "admin" {
			t.Fatalf("events = %+v", ev)
		}
	})
	t.Run("same role is a no-op", func(t *testing.T) {
		f := newFixture(t)
		f.pub = newPub(f)
		if err := f.svc.ChangeMemberRole(ctx, f.owner, f.org, f.membr, domain.RoleMember); err != nil {
			t.Fatalf("no-op change: %v", err)
		}
		if len(f.pub.Events()) != 0 {
			t.Fatal("no-op change must not publish")
		}
	})
	t.Run("unknown target and invalid role", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.owner, f.org, uuid.New(), domain.RoleAdmin), apperror.KindNotFound, "org.member_not_found")
		requireErr(t, f.svc.ChangeMemberRole(ctx, f.owner, f.org, f.membr, "boss"), apperror.KindValidation, "org.role_invalid")
	})
}

func TestRemoveMemberRules(t *testing.T) {
	ctx := context.Background()
	t.Run("admin cannot remove an owner", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.RemoveMember(ctx, f.admin, f.org, f.owner), apperror.KindForbidden, "org.cannot_remove_owner")
	})
	t.Run("admin cannot remove an admin", func(t *testing.T) {
		f := newFixture(t)
		admin2 := addUser(f, "Segundo Admin", "admin2@acme.test")
		addMember(f, admin2, domain.RoleAdmin)
		requireErr(t, f.svc.RemoveMember(ctx, f.admin, f.org, admin2), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("admin cannot remove itself through the members endpoint", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.RemoveMember(ctx, f.admin, f.org, f.admin), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("owner removes a guest and event is published", func(t *testing.T) {
		f := newFixture(t)
		f.pub = newPub(f)
		if err := f.svc.RemoveMember(ctx, f.owner, f.org, f.guest); err != nil {
			t.Fatalf("remove: %v", err)
		}
		ev := f.pub.Events()
		if len(ev) != 1 || ev[0].Type != EventMemberRemoved || ev[0].Payload["role"] != "guest" {
			t.Fatalf("events = %+v", ev)
		}
		if _, err := f.svc.RequireRole(ctx, f.guest, f.org, domain.RoleGuest); apperror.KindOf(err) != apperror.KindNotFound {
			t.Fatalf("removed member must lose access, got %v", err)
		}
	})
	t.Run("owner removes another owner when one remains", func(t *testing.T) {
		f := newFixture(t)
		owner2 := addUser(f, "Segundo Dueño", "owner2@acme.test")
		addMember(f, owner2, domain.RoleOwner)
		if err := f.svc.RemoveMember(ctx, f.owner, f.org, owner2); err != nil {
			t.Fatalf("remove second owner: %v", err)
		}
	})
	t.Run("last owner cannot be removed", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.RemoveMember(ctx, f.owner, f.org, f.owner), apperror.KindConflict, "org.last_owner")
	})
	t.Run("unknown target", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.RemoveMember(ctx, f.owner, f.org, uuid.New()), apperror.KindNotFound, "org.member_not_found")
	})
}

// newPub replaces the fixture event log with an empty one and returns it, so assertions see only the
// events of the operation under test.
func newPub(f *fixture) *events.MemoryPublisher {
	p := events.NewMemoryPublisher()
	f.svc.pub = p
	f.pub = p
	return p
}
