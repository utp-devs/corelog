package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// TestGetMemberProfileRules checks who may read which profile: members read any member, guests read only
// themselves, outsiders and non-member targets are hidden.
func TestGetMemberProfileRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.repo.users[f.membr] = fakeUser{
		name: "Mario Member", email: membrEmail, jobTitle: "Ingeniero", department: "Operaciones",
		timezone: "Europe/Madrid", hasAvatar: true,
	}

	cases := []struct {
		name   string
		actor  string
		target string
		kind   apperror.Kind
		code   string
	}{
		{"owner reads member", "owner", "membr", "", ""},
		{"member reads owner", "membr", "owner", "", ""},
		{"guest reads self", "guest", "guest", "", ""},
		{"guest reads member", "guest", "membr", apperror.KindForbidden, "org.forbidden"},
		{"guest reads owner", "guest", "owner", apperror.KindForbidden, "org.forbidden"},
		{"member reads non-member target", "membr", "out", apperror.KindNotFound, "org.member_not_found"},
		{"outsider reads member", "out", "membr", apperror.KindNotFound, "org.not_found"},
	}
	actorOf := map[string]uuid.UUID{
		"owner": f.owner, "admin": f.admin, "membr": f.membr, "guest": f.guest, "out": f.out,
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			profile, err := f.svc.GetMemberProfile(ctx, actorOf[c.actor], f.org, actorOf[c.target])
			if c.kind == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if profile.UserID != actorOf[c.target] {
					t.Fatalf("profile user = %v", profile.UserID)
				}
				return
			}
			requireErr(t, err, c.kind, c.code)
		})
	}

	profile, err := f.svc.GetMemberProfile(ctx, f.owner, f.org, f.membr)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.JobTitle != "Ingeniero" || profile.Department != "Operaciones" || !profile.HasAvatar ||
		profile.Timezone != "Europe/Madrid" || profile.Role != "member" || profile.DisplayName != "Mario Member" {
		t.Fatalf("profile = %+v", profile)
	}
}
