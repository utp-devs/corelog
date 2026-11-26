package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var testNow = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

func assertAppErr(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	if got := apperror.KindOf(err); got != kind {
		t.Fatalf("kind = %s, want %s (err: %v)", got, kind, err)
	}
	if got := apperror.CodeOf(err); got != code {
		t.Fatalf("code = %s, want %s", got, code)
	}
}

func TestValidateSlug(t *testing.T) {
	cases := []struct {
		name string
		slug string
		code string // "" means valid
	}{
		{"simple", "acme", ""},
		{"digits and dashes", "acme-2026", ""},
		{"min length 3", "abc", ""},
		{"max length 40", strings.Repeat("a", 40), ""},
		{"too short", "ab", "org.slug_invalid"},
		{"too long", strings.Repeat("a", 41), "org.slug_invalid"},
		{"uppercase", "Acme", "org.slug_invalid"},
		{"leading dash", "-acme", "org.slug_invalid"},
		{"trailing dash", "acme-", "org.slug_invalid"},
		{"underscore", "acme_corp", "org.slug_invalid"},
		{"accent", "logística", "org.slug_invalid"},
		{"empty", "", "org.slug_invalid"},
		{"reserved admin", "admin", "org.slug_reserved"},
		{"reserved api", "api", "org.slug_reserved"},
		{"reserved corelog-admin", "corelog-admin", "org.slug_reserved"},
		{"reserved invitations", "invitations", "org.slug_reserved"},
		{"not reserved prefix", "adminx", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSlug(tc.slug)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("ValidateSlug(%q) = %v, want nil", tc.slug, err)
				}
				return
			}
			kind := apperror.KindValidation
			assertAppErr(t, err, kind, tc.code)
		})
	}
}

func TestNormalizeSlug(t *testing.T) {
	cases := map[string]string{
		"Acme Corp":            "acme-corp",
		"Ñandú Logística":      "nandu-logistica",
		"  Café   del   Mar  ": "cafe-del-mar",
		"Año Nuevo!!":          "ano-nuevo",
		"¡Hola, mundo!":        "hola-mundo",
		"Über_Straße":          "uber-strasse",
		"---":                  "",
		"":                     "",
		"ÁÉÍÓÚ àèìòù":          "aeiou-aeiou",
		"Team 2026 / Backend":  "team-2026-backend",
	}
	for in, want := range cases {
		if got := NormalizeSlug(in); got != want {
			t.Errorf("NormalizeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeSlugTruncatesAndTrims(t *testing.T) {
	long := strings.Repeat("abcd ", 20) // 100 chars with spaces
	got := NormalizeSlug(long)
	if len(got) > MaxSlugLength {
		t.Fatalf("len = %d, want <= %d", len(got), MaxSlugLength)
	}
	if strings.HasSuffix(got, "-") || strings.HasPrefix(got, "-") {
		t.Fatalf("slug %q has leading or trailing dash", got)
	}
	if err := ValidateSlug(got); err != nil {
		t.Fatalf("normalized long name is not a valid slug: %v", err)
	}
}

func TestNewOrganization(t *testing.T) {
	creator := uuid.New()

	org, err := NewOrganization("  Acme Corp  ", "", " Desc ", creator, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org.ID != uuid.Nil {
		t.Errorf("ID must stay uuid.Nil, got %s", org.ID)
	}
	if org.Slug != "acme-corp" || org.Name != "Acme Corp" || org.Description != "Desc" {
		t.Errorf("unexpected fields: %+v", org)
	}
	if org.CreatedBy != creator || !org.CreatedAt.Equal(testNow) || org.DeletedAt != nil {
		t.Errorf("unexpected audit fields: %+v", org)
	}
	if err := org.Settings.Validate(); err != nil {
		t.Errorf("default settings must be valid: %v", err)
	}

	explicit, err := NewOrganization("Acme", "acme-team", "", creator, testNow)
	if err != nil || explicit.Slug != "acme-team" {
		t.Fatalf("explicit slug not kept: %v %+v", err, explicit)
	}

	_, err = NewOrganization("   ", "", "", creator, testNow)
	assertAppErr(t, err, apperror.KindValidation, "org.name_required")

	_, err = NewOrganization("Acme", "Acme", "", creator, testNow)
	assertAppErr(t, err, apperror.KindValidation, "org.slug_invalid")

	_, err = NewOrganization("Admin", "", "", creator, testNow)
	assertAppErr(t, err, apperror.KindValidation, "org.slug_reserved")
}

func TestConfirmDeletion(t *testing.T) {
	org, _ := NewOrganization("Acme", "acme", "", uuid.New(), testNow)
	if err := org.ConfirmDeletion("acme"); err != nil {
		t.Fatalf("matching slug rejected: %v", err)
	}
	assertAppErr(t, org.ConfirmDeletion("ACME"), apperror.KindValidation, "org.confirmation_mismatch")
	assertAppErr(t, org.ConfirmDeletion(""), apperror.KindValidation, "org.confirmation_mismatch")
}

func TestMarkDeleted(t *testing.T) {
	org, _ := NewOrganization("Acme", "acme", "", uuid.New(), testNow)
	if org.IsDeleted() {
		t.Fatal("new organization must not be deleted")
	}
	later := testNow.Add(time.Hour)
	org.MarkDeleted(later)
	if !org.IsDeleted() || !org.DeletedAt.Equal(later) || !org.UpdatedAt.Equal(later) {
		t.Fatalf("MarkDeleted did not set timestamps: %+v", org)
	}
}

func TestApplySettings(t *testing.T) {
	org, _ := NewOrganization("Acme", "acme", "", uuid.New(), testNow)
	s := DefaultSettings()
	s.WhoCanCreateTeams = CreatorAdmins
	if err := org.ApplySettings(s, testNow.Add(time.Minute)); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	if org.Settings.WhoCanCreateTeams != CreatorAdmins {
		t.Fatal("settings not applied")
	}
	bad := DefaultSettings()
	bad.Timezone = "Mars/Olympus"
	err := org.ApplySettings(bad, testNow)
	assertAppErr(t, err, apperror.KindValidation, "org.settings_invalid")
	if org.Settings.Timezone != "UTC" {
		t.Fatal("invalid settings must not be applied")
	}
}

func TestDefaultSettings(t *testing.T) {
	d := DefaultSettings()
	if d.WhoCanCreateProjects != CreatorMembers || d.WhoCanCreateTeams != CreatorMembers {
		t.Errorf("creators default: %+v", d)
	}
	if d.DefaultProjectVisibility != VisibilityOpen || d.Require2FA {
		t.Errorf("visibility/2fa default: %+v", d)
	}
	if d.HoursPerDay != 8 || d.DaysPerWeek != 5 || d.Timezone != "UTC" {
		t.Errorf("D-18 defaults: %+v", d)
	}
	if d.MaxUploadBytes != 10<<20 {
		t.Errorf("max upload default = %d", d.MaxUploadBytes)
	}
}

func TestSettingsValidate(t *testing.T) {
	valid := DefaultSettings()
	if err := valid.Validate(); err != nil {
		t.Fatalf("defaults invalid: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Settings)
		field  string
	}{
		{"bad creator projects", func(s *Settings) { s.WhoCanCreateProjects = "everyone" }, "who_can_create_projects"},
		{"bad creator teams", func(s *Settings) { s.WhoCanCreateTeams = "" }, "who_can_create_teams"},
		{"bad visibility", func(s *Settings) { s.DefaultProjectVisibility = "team" }, "default_project_visibility"},
		{"upload zero", func(s *Settings) { s.MaxUploadBytes = 0 }, "max_upload_bytes"},
		{"upload above cap", func(s *Settings) { s.MaxUploadBytes = 101 << 20 }, "max_upload_bytes"},
		{"hours zero", func(s *Settings) { s.HoursPerDay = 0 }, "hours_per_day"},
		{"hours 25", func(s *Settings) { s.HoursPerDay = 25 }, "hours_per_day"},
		{"hours NaN", func(s *Settings) { s.HoursPerDay = nan() }, "hours_per_day"},
		{"days 8", func(s *Settings) { s.DaysPerWeek = 8 }, "days_per_week"},
		{"days Inf", func(s *Settings) { s.DaysPerWeek = inf() }, "days_per_week"},
		{"timezone empty", func(s *Settings) { s.Timezone = "" }, "timezone"},
		{"timezone Local", func(s *Settings) { s.Timezone = "Local" }, "timezone"},
		{"timezone unknown", func(s *Settings) { s.Timezone = "Mars/Olympus" }, "timezone"},
		{"timezone IANA ok", func(s *Settings) { s.Timezone = "Europe/Madrid" }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := DefaultSettings()
			tc.mutate(&s)
			err := s.Validate()
			if tc.field == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			assertAppErr(t, err, apperror.KindValidation, "org.settings_invalid")
			details := apperror.DetailsOf(err)
			if len(details) != 1 || details[0].Field != tc.field {
				t.Fatalf("details = %+v, want single field %s", details, tc.field)
			}
		})
	}
}

func TestSettingsValidateCollectsAllFields(t *testing.T) {
	s := Settings{}
	err := s.Validate()
	assertAppErr(t, err, apperror.KindValidation, "org.settings_invalid")
	if n := len(apperror.DetailsOf(err)); n != 7 {
		t.Fatalf("details = %d, want 7", n)
	}
}

func TestNewMember(t *testing.T) {
	m, err := NewMember(uuid.New(), uuid.New(), RoleGuest, testNow)
	if err != nil || m.Role != RoleGuest {
		t.Fatalf("NewMember: %v %+v", err, m)
	}
	_, err = NewMember(uuid.New(), uuid.New(), Role("agent"), testNow)
	assertAppErr(t, err, apperror.KindValidation, "org.role_invalid")
}

func TestRoleRank(t *testing.T) {
	order := []Role{RoleOwner, RoleAdmin, RoleMember, RoleGuest}
	for i := 0; i < len(order)-1; i++ {
		if order[i].Rank() <= order[i+1].Rank() {
			t.Fatalf("rank(%s) must exceed rank(%s)", order[i], order[i+1])
		}
	}
	if Role("x").Rank() != 0 || Role("x").IsValid() {
		t.Fatal("unknown role must rank 0 and be invalid")
	}
}

func TestCanChangeRoleMatrix(t *testing.T) {
	roles := []Role{RoleOwner, RoleAdmin, RoleMember, RoleGuest}

	// Explicit allow list for admin; owner is allowed everything; member and guest nothing.
	adminAllowed := map[[2]Role]bool{
		{RoleMember, RoleMember}: true,
		{RoleMember, RoleGuest}:  true,
		{RoleGuest, RoleMember}:  true,
		{RoleGuest, RoleGuest}:   true,
	}

	for _, actor := range roles {
		for _, target := range roles {
			for _, next := range roles {
				err := CanChangeRole(actor, target, next)
				var allowed bool
				switch actor {
				case RoleOwner:
					allowed = true
				case RoleAdmin:
					allowed = adminAllowed[[2]Role{target, next}]
				}
				if allowed && err != nil {
					t.Errorf("CanChangeRole(%s,%s,%s) = %v, want nil", actor, target, next, err)
				}
				if !allowed {
					if !errors.Is(err, ErrForbidden) {
						t.Errorf("CanChangeRole(%s,%s,%s) = %v, want forbidden", actor, target, next, err)
					}
					assertAppErr(t, err, apperror.KindForbidden, "org.forbidden")
				}
			}
		}
	}

	assertAppErr(t, CanChangeRole(Role("x"), RoleMember, RoleMember), apperror.KindValidation, "org.role_invalid")
	assertAppErr(t, CanChangeRole(RoleOwner, RoleMember, Role("")), apperror.KindValidation, "org.role_invalid")
}

func TestValidateLastOwner(t *testing.T) {
	if err := ValidateLastOwner(1); err != nil {
		t.Fatalf("one owner remaining must be allowed: %v", err)
	}
	assertAppErr(t, ValidateLastOwner(0), apperror.KindConflict, "org.last_owner")
	assertAppErr(t, ValidateLastOwner(-1), apperror.KindConflict, "org.last_owner")
}

func nan() float64 {
	zero := 0.0
	return zero / zero
}

func inf() float64 {
	one := 1.0
	zero := 0.0
	return one / zero
}
