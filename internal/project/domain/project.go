package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

type ProjectType string

const (
	ProjectTypeKanban ProjectType = "kanban"
	ProjectTypeScrum  ProjectType = "scrum"
)

type Visibility string

const (
	// VisibilityPrivate: only org owners/admins and explicit project members see the project.
	VisibilityPrivate Visibility = "private"
	// VisibilityOpen: every org member sees the project (guests need an explicit project membership).
	VisibilityOpen Visibility = "open"
	// VisibilityTeam is the legacy name of VisibilityOpen. It is accepted on input and stored as open.
	VisibilityTeam Visibility = "team"
)

// Organization roles that matter to project rules.
const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
	OrgRoleGuest  = "guest"
)

// Values of the organization setting who_can_create_projects.
const (
	CreationPolicyMembers = "members"
	CreationPolicyAdmins  = "admins"
)

var (
	ErrProjectNotFound = apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado")
	ErrInvalidProject  = apperror.WithCode(apperror.KindValidation, "project.invalid", "proyecto no valido")
	ErrProjectKeyTaken = apperror.WithCode(apperror.KindConflict, "project.key_taken", "ya existe un proyecto con esta clave en la organizacion")
	// ErrOrganizationNotFound hides organizations the actor does not belong to.
	ErrOrganizationNotFound = apperror.WithCode(apperror.KindNotFound, "organization.not_found", "organizacion no encontrada")
	// ErrProjectCreationNotAllowed: the actor's org role cannot create projects under the org policy.
	ErrProjectCreationNotAllowed = apperror.WithCode(apperror.KindForbidden, "project.creation_not_allowed", "no tienes permiso para crear proyectos en esta organizacion")
	// ErrUserNotInOrg: the target user is not a member of the project's organization.
	ErrUserNotInOrg = apperror.WithCode(apperror.KindValidation, "project.user_not_in_org", "el usuario no pertenece a la organizacion del proyecto")
	// ErrUserNotFound: the referenced user does not exist (database foreign key violation).
	ErrUserNotFound = apperror.WithCode(apperror.KindNotFound, "project.user_not_found", "usuario no encontrado")
)

type Project struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Key            string
	Name           string
	Description    string
	Type           ProjectType
	LeadID         uuid.UUID
	Visibility     Visibility
	ArchivedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UpdateInput struct {
	Name        string
	Description string
	Type        ProjectType
	LeadID      uuid.UUID
	Visibility  Visibility
}

// ParseVisibility validates a visibility value and maps the legacy "team" to "open".
func ParseVisibility(value string) (Visibility, error) {
	switch Visibility(strings.TrimSpace(value)) {
	case VisibilityPrivate:
		return VisibilityPrivate, nil
	case VisibilityOpen, VisibilityTeam:
		return VisibilityOpen, nil
	default:
		return "", ErrInvalidProject
	}
}

// DefaultVisibility converts the organization setting default_project_visibility into a visibility.
// Unknown settings fall back to private, the restrictive choice.
func DefaultVisibility(setting string) Visibility {
	visibility, err := ParseVisibility(setting)
	if err != nil {
		return VisibilityPrivate
	}
	return visibility
}

// CanCreateProject applies the org policy: owners and admins always; members only when the
// policy is "members"; guests never.
func CanCreateProject(orgRole, policy string) bool {
	switch orgRole {
	case OrgRoleOwner, OrgRoleAdmin:
		return true
	case OrgRoleMember:
		return policy == CreationPolicyMembers
	default:
		return false
	}
}

func NewProject(organizationID uuid.UUID, key, name, description string, projectType ProjectType, visibility Visibility, leadID uuid.UUID) (*Project, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	name = strings.TrimSpace(name)

	if organizationID == uuid.Nil || key == "" || len(key) > 20 || name == "" || len(name) > 120 {
		return nil, ErrInvalidProject
	}
	if projectType != ProjectTypeKanban && projectType != ProjectTypeScrum {
		return nil, ErrInvalidProject
	}
	visibility, err := ParseVisibility(string(visibility))
	if err != nil {
		return nil, err
	}
	if leadID == uuid.Nil {
		return nil, ErrInvalidProject
	}

	now := time.Now().UTC()
	return &Project{
		OrganizationID: organizationID,
		Key:            key,
		Name:           name,
		Description:    strings.TrimSpace(description),
		Type:           projectType,
		LeadID:         leadID,
		Visibility:     visibility,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (p *Project) Update(input UpdateInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 120 {
		return ErrInvalidProject
	}
	if input.Type != ProjectTypeKanban && input.Type != ProjectTypeScrum {
		return ErrInvalidProject
	}
	visibility, err := ParseVisibility(string(input.Visibility))
	if err != nil {
		return err
	}
	if input.LeadID == uuid.Nil {
		return ErrInvalidProject
	}

	p.Name = input.Name
	p.Description = strings.TrimSpace(input.Description)
	p.Type = input.Type
	p.LeadID = input.LeadID
	p.Visibility = visibility
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *Project) Archive() {
	now := time.Now().UTC()
	p.ArchivedAt = &now
	p.UpdatedAt = now
}
