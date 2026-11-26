package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestNewTeam(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name    string
		in      NewTeamInput
		wantErr error
	}{
		{name: "valid team", in: NewTeamInput{OrganizationID: orgID, CreatedBy: userID, Name: "Platform"}},
		{name: "valid with color and description", in: NewTeamInput{OrganizationID: orgID, Name: "Platform", Description: "infra", AvatarColor: "#1A2b3C"}},
		{name: "missing organization", in: NewTeamInput{Name: "Platform"}, wantErr: ErrOrganizationRequired},
		{name: "empty name", in: NewTeamInput{OrganizationID: orgID, Name: ""}, wantErr: ErrInvalidName},
		{name: "whitespace name", in: NewTeamInput{OrganizationID: orgID, Name: "   "}, wantErr: ErrInvalidName},
		{name: "name too long", in: NewTeamInput{OrganizationID: orgID, Name: strings.Repeat("a", 121)}, wantErr: ErrInvalidName},
		{name: "description too long", in: NewTeamInput{OrganizationID: orgID, Name: "x", Description: strings.Repeat("d", 501)}, wantErr: ErrInvalidDescription},
		{name: "color without hash", in: NewTeamInput{OrganizationID: orgID, Name: "x", AvatarColor: "1A2B3C"}, wantErr: ErrInvalidAvatarColor},
		{name: "color short", in: NewTeamInput{OrganizationID: orgID, Name: "x", AvatarColor: "#123"}, wantErr: ErrInvalidAvatarColor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team, err := NewTeam(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if team.ID != uuid.Nil {
				t.Errorf("constructor must leave ID for the database, got %s", team.ID)
			}
			if team.OrganizationID != orgID {
				t.Errorf("organization not set")
			}
			if team.CreatedBy != tt.in.CreatedBy {
				t.Errorf("created_by not set")
			}
			if team.CreatedAt.IsZero() {
				t.Errorf("expected CreatedAt")
			}
		})
	}
}

func TestTeam_Apply(t *testing.T) {
	team, err := NewTeam(NewTeamInput{OrganizationID: uuid.New(), Name: "Equipo"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	newName := "Nuevo"
	badName := " "
	badColor := "red"
	desc := "nueva descripcion"

	if err := team.Apply(TeamPatch{Name: &newName, Description: &desc}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if team.Name != "Nuevo" || team.Description != desc {
		t.Fatalf("patch not applied: %+v", team)
	}

	if err := team.Apply(TeamPatch{Name: &badName, Description: &desc}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("got %v, want ErrInvalidName", err)
	}
	if err := team.Apply(TeamPatch{AvatarColor: &badColor}); !errors.Is(err, ErrInvalidAvatarColor) {
		t.Fatalf("got %v, want ErrInvalidAvatarColor", err)
	}
	if team.Name != "Nuevo" {
		t.Errorf("failed patch must not change the team")
	}
}

func TestNewMember(t *testing.T) {
	tests := []struct {
		role     string
		wantRole string
		wantErr  error
	}{
		{role: "", wantRole: RoleMember},
		{role: RoleMember, wantRole: RoleMember},
		{role: RoleMaintainer, wantRole: RoleMaintainer},
		{role: "owner", wantErr: ErrInvalidRole},
		{role: "guest", wantErr: ErrInvalidRole},
	}
	for _, tt := range tests {
		m, err := NewMember(uuid.New(), uuid.New(), tt.role)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("role %q: got %v, want %v", tt.role, err, tt.wantErr)
			}
			continue
		}
		if err != nil || m.Role != tt.wantRole {
			t.Errorf("role %q: got %v / %v, want %s", tt.role, m, err, tt.wantRole)
		}
	}
}

func TestErrorCodes(t *testing.T) {
	if got := apperror.CodeOf(ErrUserNotInOrg); got != "team.user_not_in_org" {
		t.Errorf("code %q", got)
	}
	if got := apperror.KindOf(ErrTeamNameTaken); got != apperror.KindConflict {
		t.Errorf("kind %q", got)
	}
}
