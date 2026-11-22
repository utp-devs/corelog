package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewProject(t *testing.T) {
	project, err := NewProject(" core ", "Core Platform", "Test project", ProjectTypeKanban, uuid.New(), VisibilityTeam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Key != "CORE" {
		t.Fatalf("got key %q, want CORE", project.Key)
	}
	if project.Name != "Core Platform" {
		t.Fatalf("got name %q", project.Name)
	}
	if project.CreatedAt.IsZero() || project.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps")
	}
}

func TestNewProjectRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name        string
		projectKey  string
		projectName string
		projectType ProjectType
		leadID      uuid.UUID
		visibility  Visibility
	}{
		{"empty key", "", "Core", ProjectTypeKanban, uuid.New(), VisibilityTeam},
		{"empty name", "CORE", "", ProjectTypeKanban, uuid.New(), VisibilityTeam},
		{"invalid type", "CORE", "Core", ProjectType("invalid"), uuid.New(), VisibilityTeam},
		{"missing lead", "CORE", "Core", ProjectTypeKanban, uuid.Nil, VisibilityTeam},
		{"invalid visibility", "CORE", "Core", ProjectTypeKanban, uuid.New(), Visibility("invalid")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewProject(tt.projectKey, tt.projectName, "", tt.projectType, tt.leadID, tt.visibility); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
