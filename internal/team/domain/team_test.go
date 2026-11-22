package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewTeam(t *testing.T) {
	t.Run("creates valid team", func(t *testing.T) {
		team, err := NewTeam("Platform Engineering")
		if err != nil {
			t.Fatalf("unexpected error creating team: %v", err)
		}
		if team.ID == uuid.Nil {
			t.Errorf("expected non-nil team ID")
		}
		if team.Name != "Platform Engineering" {
			t.Errorf("got name %s, want 'Platform Engineering'", team.Name)
		}
		if team.CreatedAt.IsZero() || team.UpdatedAt.IsZero() {
			t.Errorf("expected non-zero timestamps")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		_, err := NewTeam("")
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})

	t.Run("rejects whitespace-only name", func(t *testing.T) {
		_, err := NewTeam("   ")
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})
}

func TestTeam_Rename(t *testing.T) {
	team, err := NewTeam("Equipo de prueba")
	if err != nil {
		t.Fatalf("unexpected error creating team: %v", err)
	}

	t.Run("renames team successfully and updates timestamp", func(t *testing.T) {
		initialUpdated := team.UpdatedAt
		time.Sleep(10 * time.Millisecond)

		err := team.Rename("Nuevo Nombre")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if team.Name != "Nuevo Nombre" {
			t.Errorf("got name %s, want 'Nuevo Nombre'", team.Name)
		}
		if !team.UpdatedAt.After(initialUpdated) {
			t.Errorf("expected UpdatedAt to be updated")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		err := team.Rename("")
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})

	t.Run("rejects whitespace-only name", func(t *testing.T) {
		err := team.Rename("   ")
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})
}

