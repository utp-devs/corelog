package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want bool
	}{
		{"admin is valid", RoleAdmin, true},
		{"user is valid", RoleUser, true},
		{"agent is retired", Role("agent"), false},
		{"requester is retired", Role("requester"), false},
		{"empty string is invalid", "", false},
		{"unknown role is invalid", "superadmin", false},
		{"uppercase ADMIN is invalid", "ADMIN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.want {
				t.Errorf("Role.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	t.Run("creates valid user", func(t *testing.T) {
		user, err := NewUser("Ada Lovelace", "ada@example.com", "$2a$10$hashedpassword", RoleUser)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != uuid.Nil {
			t.Errorf("constructor must leave ID for the database to generate, got %s", user.ID)
		}
		if user.Name != "Ada Lovelace" {
			t.Errorf("got name %s, want 'Ada Lovelace'", user.Name)
		}
		if user.Email != "ada@example.com" {
			t.Errorf("got email %s, want 'ada@example.com'", user.Email)
		}
		if user.PasswordHash != "$2a$10$hashedpassword" {
			t.Errorf("got password hash %s", user.PasswordHash)
		}
		if user.Role != RoleUser {
			t.Errorf("got role %v, want %v", user.Role, RoleUser)
		}
		if user.TeamID != nil {
			t.Errorf("expected nil initial TeamID, got %v", user.TeamID)
		}
		if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
			t.Errorf("expected non-zero timestamps")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		_, err := NewUser("", "ada@example.com", "$2a$10$hash", RoleAdmin)
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})

	t.Run("rejects empty email", func(t *testing.T) {
		_, err := NewUser("Ada", "", "$2a$10$hash", RoleAdmin)
		if !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("expected ErrInvalidEmail, got %v", err)
		}
	})

	t.Run("rejects empty password hash", func(t *testing.T) {
		_, err := NewUser("Ada", "ada@example.com", "", RoleAdmin)
		if !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("expected ErrInvalidPassword, got %v", err)
		}
	})

	t.Run("rejects invalid role", func(t *testing.T) {
		_, err := NewUser("Ada", "ada@example.com", "$2a$10$hash", Role("superadmin"))
		if !errors.Is(err, ErrInvalidRole) {
			t.Errorf("expected ErrInvalidRole, got %v", err)
		}
	})
}

func TestUser_UpdateProfile(t *testing.T) {
	user, err := NewUser("Initial Name", "initial@example.com", "$2a$10$hash", RoleUser)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	t.Run("updates name and email with updated timestamp", func(t *testing.T) {
		initialUpdated := user.UpdatedAt
		time.Sleep(10 * time.Millisecond)

		err := user.UpdateProfile("Updated Name", "updated@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Name != "Updated Name" {
			t.Errorf("got name %s, want 'Updated Name'", user.Name)
		}
		if user.Email != "updated@example.com" {
			t.Errorf("got email %s, want 'updated@example.com'", user.Email)
		}
		if !user.UpdatedAt.After(initialUpdated) {
			t.Errorf("expected UpdatedAt to be after initial timestamp")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		err := user.UpdateProfile("", "valid@example.com")
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})

	t.Run("rejects empty email", func(t *testing.T) {
		err := user.UpdateProfile("Valid Name", "")
		if !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("expected ErrInvalidEmail, got %v", err)
		}
	})
}

func TestUser_ChangePasswordHash(t *testing.T) {
	user, _ := NewUser("Ada", "ada@example.com", "old_hash", RoleAdmin)
	initialUpdated := user.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	user.ChangePasswordHash("new_hash")
	if user.PasswordHash != "new_hash" {
		t.Errorf("got password hash %s, want 'new_hash'", user.PasswordHash)
	}
	if !user.UpdatedAt.After(initialUpdated) {
		t.Errorf("expected UpdatedAt to be after initial timestamp")
	}
}

func TestUser_AssignTeam(t *testing.T) {
	user, _ := NewUser("Ada", "ada@example.com", "hash", RoleAdmin)
	teamID := uuid.New()
	initialUpdated := user.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	user.AssignTeam(&teamID)
	if user.TeamID == nil || *user.TeamID != teamID {
		t.Errorf("got TeamID %v, want %v", user.TeamID, teamID)
	}
	if !user.UpdatedAt.After(initialUpdated) {
		t.Errorf("expected UpdatedAt to be updated")
	}

	// Clearing team
	user.AssignTeam(nil)
	if user.TeamID != nil {
		t.Errorf("expected nil TeamID, got %v", user.TeamID)
	}
}
