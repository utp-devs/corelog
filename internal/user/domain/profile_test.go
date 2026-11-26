package domain

import (
	"errors"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestNewUser_DisplayNameAndTimezoneDefaults(t *testing.T) {
	user, err := NewUser("Ana Prueba", "ana@example.com", "$2a$10$hash", RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "Ana Prueba" {
		t.Errorf("display_name must default to name, got %q", user.DisplayName)
	}
	if user.Timezone != DefaultTimezone {
		t.Errorf("timezone must default to UTC, got %q", user.Timezone)
	}
	if user.LastLoginAt != nil || user.EmailVerifiedAt != nil || user.AvatarKey != nil {
		t.Error("last_login_at, email_verified_at and avatar_key must start empty")
	}
}

func TestValidateTimezone(t *testing.T) {
	tests := []struct {
		name string
		tz   string
		ok   bool
	}{
		{"IANA zone", "America/Bogota", true},
		{"UTC", "UTC", true},
		{"empty is rejected", "", false},
		{"Local is rejected", "Local", false},
		{"unknown zone", "Mars/Olympus", false},
		{"offset is not a zone name", "+05:00", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimezone(tt.tz)
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrInvalidTimezone) {
				t.Fatalf("want ErrInvalidTimezone, got %v", err)
			}
		})
	}
}

func TestUser_SetTimezoneRejectsInvalidAndKeepsValue(t *testing.T) {
	user, _ := NewUser("Ana", "ana@example.com", "$2a$10$hash", RoleUser)
	if err := user.SetTimezone("Nowhere/City"); apperror.CodeOf(err) != "user.timezone_invalid" {
		t.Fatalf("want user.timezone_invalid, got %v", err)
	}
	if user.Timezone != DefaultTimezone {
		t.Errorf("rejected timezone must not be stored, got %q", user.Timezone)
	}
	if err := user.SetTimezone("Europe/Madrid"); err != nil || user.Timezone != "Europe/Madrid" {
		t.Errorf("valid timezone not stored: %v, %q", err, user.Timezone)
	}
}

func TestUser_SetDisplayNameRejectsEmpty(t *testing.T) {
	user, _ := NewUser("Ana", "ana@example.com", "$2a$10$hash", RoleUser)
	if err := user.SetDisplayName(""); !errors.Is(err, ErrInvalidDisplayName) {
		t.Fatalf("want ErrInvalidDisplayName, got %v", err)
	}
	if err := user.SetDisplayName("Ana M."); err != nil || user.DisplayName != "Ana M." {
		t.Errorf("display name not stored: %v, %q", err, user.DisplayName)
	}
}
