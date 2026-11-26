package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestNewProject(t *testing.T) {
	orgID := uuid.New()
	project, err := NewProject(orgID, " core ", "Core Platform", "Test project", ProjectTypeKanban, VisibilityOpen, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Key != "CORE" {
		t.Fatalf("got key %q, want CORE", project.Key)
	}
	if project.Name != "Core Platform" {
		t.Fatalf("got name %q", project.Name)
	}
	if project.OrganizationID != orgID {
		t.Fatalf("organization id not set")
	}
	if project.CreatedAt.IsZero() || project.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps")
	}
}

func TestNewProjectNormalizesLegacyVisibility(t *testing.T) {
	project, err := NewProject(uuid.New(), "CORE", "Core", "", ProjectTypeScrum, VisibilityTeam, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Visibility != VisibilityOpen {
		t.Fatalf("legacy team visibility stored as %q, want open", project.Visibility)
	}
}

func TestNewProjectRejectsInvalidData(t *testing.T) {
	org := uuid.New()
	tests := []struct {
		name        string
		orgID       uuid.UUID
		projectKey  string
		projectName string
		projectType ProjectType
		leadID      uuid.UUID
		visibility  Visibility
	}{
		{"missing organization", uuid.Nil, "CORE", "Core", ProjectTypeKanban, uuid.New(), VisibilityOpen},
		{"empty key", org, "", "Core", ProjectTypeKanban, uuid.New(), VisibilityOpen},
		{"empty name", org, "CORE", "", ProjectTypeKanban, uuid.New(), VisibilityOpen},
		{"invalid type", org, "CORE", "Core", ProjectType("invalid"), uuid.New(), VisibilityOpen},
		{"missing lead", org, "CORE", "Core", ProjectTypeKanban, uuid.Nil, VisibilityOpen},
		{"invalid visibility", org, "CORE", "Core", ProjectTypeKanban, uuid.New(), Visibility("invalid")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewProject(tt.orgID, tt.projectKey, tt.projectName, "", tt.projectType, tt.visibility, tt.leadID); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseVisibilityAndDefault(t *testing.T) {
	if v, err := ParseVisibility("team"); err != nil || v != VisibilityOpen {
		t.Fatalf("team = %q, %v; want open", v, err)
	}
	if _, err := ParseVisibility("public"); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("public: got %v, want validation", err)
	}
	if DefaultVisibility("open") != VisibilityOpen || DefaultVisibility("private") != VisibilityPrivate {
		t.Fatal("default visibility from settings not mapped")
	}
	if DefaultVisibility("garbage") != VisibilityPrivate {
		t.Fatal("unknown default must fall back to private")
	}
}

func TestCanCreateProjectPolicyMatrix(t *testing.T) {
	tests := []struct {
		role   string
		policy string
		want   bool
	}{
		{OrgRoleOwner, CreationPolicyMembers, true},
		{OrgRoleOwner, CreationPolicyAdmins, true},
		{OrgRoleAdmin, CreationPolicyMembers, true},
		{OrgRoleAdmin, CreationPolicyAdmins, true},
		{OrgRoleMember, CreationPolicyMembers, true},
		{OrgRoleMember, CreationPolicyAdmins, false},
		{OrgRoleGuest, CreationPolicyMembers, false},
		{OrgRoleGuest, CreationPolicyAdmins, false},
		{"", CreationPolicyMembers, false},
	}
	for _, tt := range tests {
		if got := CanCreateProject(tt.role, tt.policy); got != tt.want {
			t.Errorf("CanCreateProject(%q, %q) = %v, want %v", tt.role, tt.policy, got, tt.want)
		}
	}
}
