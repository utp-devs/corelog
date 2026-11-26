package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

func requireErr(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s/%s, got nil", kind, code)
	}
	if got := apperror.KindOf(err); got != kind {
		t.Fatalf("kind = %s, want %s (%v)", got, kind, err)
	}
	if got := apperror.CodeOf(err); got != code {
		t.Fatalf("code = %s, want %s", got, code)
	}
}

func TestCreateOrganizationDerivesSlugAndOwner(t *testing.T) {
	f := newFixture(t)
	creator := uuid.New()
	org, err := f.svc.CreateOrganization(context.Background(), creator, port.CreateInput{Name: "Équipo Nueva"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if org.ID == uuid.Nil || org.Slug != "equipo-nueva" {
		t.Fatalf("unexpected org: %+v", org)
	}
	role, err := f.svc.RequireRole(context.Background(), creator, org.ID, domain.RoleOwner)
	if err != nil || role != domain.RoleOwner {
		t.Fatalf("creator must be owner: role=%s err=%v", role, err)
	}
	if f.tx.Calls != 2 { // setup create + this create
		t.Fatalf("tx calls = %d, want 2", f.tx.Calls)
	}
	got := f.pub.Events()
	if len(got) != 2 || got[0].Type != EventOrgCreated || got[1].Type != EventMemberAdded {
		t.Fatalf("events = %v", eventTypes(f.pub))
	}
	if got[0].OrgID == nil || *got[0].OrgID != org.ID || got[0].ActorID == nil || *got[0].ActorID != creator {
		t.Fatalf("event scope wrong: %+v", got[0])
	}
}

func TestCreateOrganizationErrors(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cases := []struct {
		name string
		in   port.CreateInput
		kind apperror.Kind
		code string
	}{
		{"empty name", port.CreateInput{Name: "  "}, apperror.KindValidation, "org.name_required"},
		{"invalid slug", port.CreateInput{Name: "X", Slug: "Bad Slug"}, apperror.KindValidation, "org.slug_invalid"},
		{"reserved slug", port.CreateInput{Name: "X", Slug: "settings"}, apperror.KindValidation, "org.slug_reserved"},
		{"taken slug", port.CreateInput{Name: "Other", Slug: "acme"}, apperror.KindConflict, "org.slug_taken"},
	}
	before := len(f.pub.Events())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.CreateOrganization(ctx, f.owner, tc.in)
			requireErr(t, err, tc.kind, tc.code)
		})
	}
	if len(f.pub.Events()) != before {
		t.Fatal("failed creations must not publish events")
	}
}

func TestCreateOrganizationPublishFailureIsReturned(t *testing.T) {
	f := newFixture(t)
	boom := errors.New("outbox down")
	f.svc.pub = failingPublisher{err: boom}
	_, err := f.svc.CreateOrganization(context.Background(), uuid.New(), port.CreateInput{Name: "Fresh", Slug: "fresh-org"})
	if !errors.Is(err, boom) {
		t.Fatalf("publish error not propagated: %v", err)
	}
}

func TestGetOrganizationVisibility(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.GetOrganization(ctx, f.guest, f.org); err != nil {
		t.Fatalf("guest must read the organization: %v", err)
	}
	_, err := f.svc.GetOrganization(ctx, f.out, f.org)
	requireErr(t, err, apperror.KindNotFound, "org.not_found")
	_, err = f.svc.GetOrganization(ctx, f.owner, uuid.New())
	requireErr(t, err, apperror.KindNotFound, "org.not_found")
}

func TestUpdateSettingsPermissions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	settings := domain.DefaultSettings()
	settings.WhoCanCreateTeams = domain.CreatorAdmins

	cases := []struct {
		name  string
		actor uuid.UUID
		kind  apperror.Kind
		code  string
	}{
		{"owner allowed", f.owner, "", ""},
		{"admin allowed", f.admin, "", ""},
		{"member forbidden", f.membr, apperror.KindForbidden, "org.forbidden"},
		{"guest forbidden", f.guest, apperror.KindForbidden, "org.forbidden"},
		{"outsider not found", f.out, apperror.KindNotFound, "org.not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.UpdateOrganization(ctx, tc.actor, f.org, port.UpdateInput{Settings: fullPatch(settings)})
			if tc.kind == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			requireErr(t, err, tc.kind, tc.code)
		})
	}
}

func TestUpdateSettingsValidationAndEvent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	bad := domain.DefaultSettings()
	bad.HoursPerDay = 30
	_, err := f.svc.UpdateOrganization(ctx, f.owner, f.org, port.UpdateInput{Settings: fullPatch(bad)})
	requireErr(t, err, apperror.KindValidation, "org.settings_invalid")
	if len(f.pub.Events()) != 0 {
		t.Fatal("invalid settings must not publish")
	}

	good := domain.DefaultSettings()
	good.Timezone = "Europe/Madrid"
	org, err := f.svc.UpdateOrganization(ctx, f.admin, f.org, port.UpdateInput{Settings: fullPatch(good)})
	if err != nil {
		t.Fatalf("admin update: %v", err)
	}
	if org.Settings.Timezone != "Europe/Madrid" {
		t.Fatalf("settings not returned: %+v", org.Settings)
	}
	if got := eventTypes(f.pub); len(got) != 1 || got[0] != EventOrgUpdated {
		t.Fatalf("events = %v", got)
	}
}

func TestListMineExcludesDeletedAndOutsiders(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	list, err := f.svc.ListMine(ctx, f.admin)
	if err != nil || len(list) != 1 || list[0].Role != domain.RoleAdmin {
		t.Fatalf("ListMine admin = %+v, %v", list, err)
	}
	if list, _ := f.svc.ListMine(ctx, f.out); len(list) != 0 {
		t.Fatalf("outsider sees %d organizations", len(list))
	}
	if err := f.svc.DeleteOrganization(ctx, f.owner, f.org, "acme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := f.svc.ListMine(ctx, f.owner); len(list) != 0 {
		t.Fatalf("deleted organization still listed: %d", len(list))
	}
}

func TestDeleteOrganization(t *testing.T) {
	ctx := context.Background()

	t.Run("admin forbidden", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.DeleteOrganization(ctx, f.admin, f.org, "acme"), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("member forbidden", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.DeleteOrganization(ctx, f.membr, f.org, "acme"), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("outsider not found", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.DeleteOrganization(ctx, f.out, f.org, "acme"), apperror.KindNotFound, "org.not_found")
	})
	t.Run("slug mismatch", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.DeleteOrganization(ctx, f.owner, f.org, "ACME"), apperror.KindValidation, "org.confirmation_mismatch")
		if _, err := f.svc.GetOrganization(ctx, f.owner, f.org); err != nil {
			t.Fatalf("org must survive a mismatched confirmation: %v", err)
		}
	})
	t.Run("owner soft deletes", func(t *testing.T) {
		f := newFixture(t)
		if err := f.svc.DeleteOrganization(ctx, f.owner, f.org, "acme"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		_, err := f.svc.GetOrganization(ctx, f.owner, f.org)
		requireErr(t, err, apperror.KindNotFound, "org.not_found")
		stored := f.repo.orgs[f.org]
		if stored.DeletedAt == nil {
			t.Fatal("organization must be soft deleted, not removed")
		}
		// The slug stays reserved after soft delete.
		_, err = f.svc.CreateOrganization(ctx, f.owner, port.CreateInput{Name: "Again", Slug: "acme"})
		requireErr(t, err, apperror.KindConflict, "org.slug_taken")
		if got := eventTypes(f.pub); len(got) != 1 || got[0] != EventOrgDeleted {
			t.Fatalf("events = %v", got)
		}
	})
}

func TestLeaveOrganization(t *testing.T) {
	ctx := context.Background()

	t.Run("member leaves", func(t *testing.T) {
		f := newFixture(t)
		if err := f.svc.LeaveOrganization(ctx, f.membr, f.org); err != nil {
			t.Fatalf("leave: %v", err)
		}
		_, err := f.svc.RequireRole(ctx, f.membr, f.org, domain.RoleGuest)
		requireErr(t, err, apperror.KindNotFound, "org.not_found")
		if got := eventTypes(f.pub); len(got) != 1 || got[0] != EventMemberRemoved {
			t.Fatalf("events = %v", got)
		}
	})
	t.Run("guest leaves", func(t *testing.T) {
		f := newFixture(t)
		if err := f.svc.LeaveOrganization(ctx, f.guest, f.org); err != nil {
			t.Fatalf("guest leave: %v", err)
		}
	})
	t.Run("outsider not found", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.LeaveOrganization(ctx, f.out, f.org), apperror.KindNotFound, "org.not_found")
	})
	t.Run("last owner blocked", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.LeaveOrganization(ctx, f.owner, f.org), apperror.KindConflict, "org.last_owner")
		if len(f.pub.Events()) != 0 {
			t.Fatal("blocked leave must not publish")
		}
	})
	t.Run("owner may leave when another owner exists", func(t *testing.T) {
		f := newFixture(t)
		f.repo.members[memberKey{f.org, f.admin}].Role = domain.RoleOwner
		if err := f.svc.LeaveOrganization(ctx, f.owner, f.org); err != nil {
			t.Fatalf("leave with second owner: %v", err)
		}
	})
}

func TestTransferOwnership(t *testing.T) {
	ctx := context.Background()

	t.Run("owner transfers to member", func(t *testing.T) {
		f := newFixture(t)
		if err := f.svc.TransferOwnership(ctx, f.owner, f.org, f.membr); err != nil {
			t.Fatalf("transfer: %v", err)
		}
		if role, _ := f.svc.RequireRole(ctx, f.membr, f.org, domain.RoleOwner); role != domain.RoleOwner {
			t.Fatalf("target role = %s", role)
		}
		if role, _ := f.svc.RequireRole(ctx, f.owner, f.org, domain.RoleAdmin); role != domain.RoleAdmin {
			t.Fatalf("actor role = %s", role)
		}
		got := f.pub.Events()
		if len(got) != 2 || got[0].Type != EventMemberRoleChanged || got[1].Type != EventMemberRoleChanged {
			t.Fatalf("events = %v", eventTypes(f.pub))
		}
		if owners, _ := f.repo.CountOwners(ctx, f.org); owners != 1 {
			t.Fatalf("owners = %d, want 1", owners)
		}
	})
	t.Run("admin forbidden", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.TransferOwnership(ctx, f.admin, f.org, f.membr), apperror.KindForbidden, "org.forbidden")
	})
	t.Run("outsider not found", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.TransferOwnership(ctx, f.out, f.org, f.membr), apperror.KindNotFound, "org.not_found")
	})
	t.Run("target not a member", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.TransferOwnership(ctx, f.owner, f.org, f.out), apperror.KindNotFound, "org.member_not_found")
	})
	t.Run("transfer to self", func(t *testing.T) {
		f := newFixture(t)
		requireErr(t, f.svc.TransferOwnership(ctx, f.owner, f.org, f.owner), apperror.KindValidation, "org.transfer_self")
	})
	t.Run("publish failure is returned", func(t *testing.T) {
		f := newFixture(t)
		f.svc.pub = failingPublisher{err: errors.New("boom")}
		if err := f.svc.TransferOwnership(ctx, f.owner, f.org, f.membr); err == nil {
			t.Fatal("expected publish error")
		}
	})
}

func TestRequireRoleRanks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cases := []struct {
		name string
		user uuid.UUID
		min  domain.Role
		kind apperror.Kind
		code string
	}{
		{"owner satisfies owner", f.owner, domain.RoleOwner, "", ""},
		{"admin satisfies member", f.admin, domain.RoleMember, "", ""},
		{"member fails admin", f.membr, domain.RoleAdmin, apperror.KindForbidden, "org.forbidden"},
		{"guest fails member", f.guest, domain.RoleMember, apperror.KindForbidden, "org.forbidden"},
		{"guest satisfies guest", f.guest, domain.RoleGuest, "", ""},
		{"outsider hidden", f.out, domain.RoleGuest, apperror.KindNotFound, "org.not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.RequireRole(ctx, tc.user, f.org, tc.min)
			if tc.kind == "" {
				if err != nil {
					t.Fatalf("unexpected: %v", err)
				}
				return
			}
			requireErr(t, err, tc.kind, tc.code)
		})
	}
}

// Ensure the fakes satisfy the publisher contract used by the service.
var _ events.Publisher = failingPublisher{}
