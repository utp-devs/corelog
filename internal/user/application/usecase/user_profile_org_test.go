package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func TestUserService_RegisterDefaultsDisplayNameAndRole(t *testing.T) {
	ctx := context.Background()
	svc := NewUserService(newFakeUserRepo(), newFakeSessionRepo())
	if _, err := svc.Register(ctx, port.RegisterInput{Name: "Admin", Email: "a@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"}); err != nil {
		t.Fatal(err)
	}

	user, err := svc.Register(ctx, port.RegisterInput{Name: "Ana Prueba", Email: "ana@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != domain.RoleUser {
		t.Errorf("registration must create role %q, got %q", domain.RoleUser, user.Role)
	}
	if user.DisplayName != "Ana Prueba" {
		t.Errorf("display_name must default to name, got %q", user.DisplayName)
	}
	if user.Timezone != domain.DefaultTimezone {
		t.Errorf("timezone must default to UTC, got %q", user.Timezone)
	}
}

func TestUserService_UpdateProfileValidatesTimezone(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())
	user, _ := svc.Register(ctx, port.RegisterInput{Name: "Luis", Email: "luis@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})

	_, err := svc.UpdateProfile(ctx, user.ID, port.UpdateProfileInput{Timezone: strPtr("Mars/Olympus")})
	if !errors.Is(err, domain.ErrInvalidTimezone) {
		t.Fatalf("invalid timezone: want ErrInvalidTimezone, got %v", err)
	}
	if code := apperror.CodeOf(err); code != "user.timezone_invalid" || apperror.KindOf(err) != apperror.KindValidation {
		t.Errorf("invalid timezone: code=%q kind=%v, want user.timezone_invalid/validation", code, apperror.KindOf(err))
	}

	updated, err := svc.UpdateProfile(ctx, user.ID, port.UpdateProfileInput{
		DisplayName: strPtr("Luis P."),
		JobTitle:    strPtr("Ingeniero"),
		Department:  strPtr("Plataforma"),
		Timezone:    strPtr("Europe/Madrid"),
	})
	if err != nil {
		t.Fatalf("valid partial update: %v", err)
	}
	if updated.DisplayName != "Luis P." || updated.JobTitle != "Ingeniero" || updated.Department != "Plataforma" || updated.Timezone != "Europe/Madrid" {
		t.Errorf("profile fields not applied: %+v", updated)
	}
	if updated.Name != "Luis" || updated.Email != "luis@example.com" {
		t.Errorf("omitted fields must keep their value, got name=%q email=%q", updated.Name, updated.Email)
	}

	if _, err := svc.UpdateProfile(ctx, user.ID, port.UpdateProfileInput{DisplayName: strPtr("")}); !errors.Is(err, domain.ErrInvalidDisplayName) {
		t.Errorf("empty display_name: want ErrInvalidDisplayName, got %v", err)
	}
}

func TestUserService_AuthenticateRecordsLastLogin(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())
	user, _ := svc.Register(ctx, port.RegisterInput{Name: "Eva", Email: "eva@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})

	if _, ok := repo.logins[user.ID]; ok {
		t.Fatal("no login recorded before authenticating")
	}
	authenticated, err := svc.Authenticate(ctx, port.AuthenticateInput{Email: "eva@example.com", Password: "Tr1cky-Pass9x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.logins[user.ID]; !ok {
		t.Fatal("successful login must record last_login_at")
	}
	if authenticated.LastLoginAt == nil {
		t.Fatal("authenticated user must carry LastLoginAt")
	}

	if _, err := svc.Authenticate(ctx, port.AuthenticateInput{Email: "eva@example.com", Password: "wrong"}); err == nil {
		t.Fatal("wrong password must fail")
	}
}

func TestUserService_SearchInOrganization(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())

	org, other := uuid.New(), uuid.New()
	owner, _ := svc.Register(ctx, port.RegisterInput{Name: "Owner Ana", Email: "owner@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})
	member, _ := svc.Register(ctx, port.RegisterInput{Name: "Member Beto", Email: "member@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})
	guest, _ := svc.Register(ctx, port.RegisterInput{Name: "Guest Carla", Email: "guest@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})
	outsider, _ := svc.Register(ctx, port.RegisterInput{Name: "Outsider Dani", Email: "outsider@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})
	elsewhere, _ := svc.Register(ctx, port.RegisterInput{Name: "Elsewhere Eli", Email: "eli@example.com", Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x"})

	repo.memberships[membershipKey{org, owner.ID}] = "owner"
	repo.memberships[membershipKey{org, member.ID}] = "member"
	repo.memberships[membershipKey{org, guest.ID}] = "guest"
	repo.memberships[membershipKey{other, elsewhere.ID}] = "member"

	t.Run("member sees the members of the organization", func(t *testing.T) {
		results, err := svc.SearchInOrganization(ctx, member.ID, org, "")
		if err != nil {
			t.Fatal(err)
		}
		got := map[uuid.UUID]bool{}
		for _, u := range results {
			got[u.ID] = true
		}
		if !got[owner.ID] || !got[member.ID] || !got[guest.ID] {
			t.Errorf("member must see all org members, got %v", got)
		}
		if got[outsider.ID] || got[elsewhere.ID] {
			t.Errorf("users outside the organization must not appear")
		}
	})

	t.Run("outsider gets not found", func(t *testing.T) {
		_, err := svc.SearchInOrganization(ctx, outsider.ID, org, "")
		if !errors.Is(err, domain.ErrNotOrganizationMember) {
			t.Fatalf("want ErrNotOrganizationMember, got %v", err)
		}
		if apperror.KindOf(err) != apperror.KindNotFound {
			t.Errorf("outsider must get a not-found kind, got %v", apperror.KindOf(err))
		}
	})

	t.Run("guest only sees non-guests and users sharing a project", func(t *testing.T) {
		results, err := svc.SearchInOrganization(ctx, guest.ID, org, "")
		if err != nil {
			t.Fatal(err)
		}
		got := map[uuid.UUID]bool{}
		for _, u := range results {
			got[u.ID] = true
		}
		if !got[owner.ID] || !got[member.ID] {
			t.Errorf("guest must see non-guest members, got %v", got)
		}
		if got[guest.ID] {
			t.Errorf("guest must not see other guests without a shared project")
		}

		repo.sharedProjects[[2]uuid.UUID{guest.ID, owner.ID}] = true
		repo.memberships[membershipKey{org, owner.ID}] = "guest"
		results, err = svc.SearchInOrganization(ctx, guest.ID, org, "")
		if err != nil {
			t.Fatal(err)
		}
		got = map[uuid.UUID]bool{}
		for _, u := range results {
			got[u.ID] = true
		}
		if !got[owner.ID] {
			t.Errorf("guest must see a guest who shares a project")
		}
	})
}
