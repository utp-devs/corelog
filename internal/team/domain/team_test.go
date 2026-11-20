package domain

import "testing"

func TestNewTeamRejectsWhitespaceOnlyName(t *testing.T) {
	_, err := NewTeam("   ")
	if err != ErrInvalidName {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestRenameRejectsWhitespaceOnlyName(t *testing.T) {
	team, err := NewTeam("Equipo de prueba")
	if err != nil {
		t.Fatalf("unexpected error creating team: %v", err)
	}

	err = team.Rename("   ")
	if err != ErrInvalidName {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}
