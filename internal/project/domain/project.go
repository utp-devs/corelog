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

// AssigneePolicy decides who gets an issue when no assignee is given.
type AssigneePolicy string

const (
	AssigneePolicyUnassigned AssigneePolicy = "unassigned"
	AssigneePolicyLead       AssigneePolicy = "lead"
)

// DefaultColor is the project color when none is given.
const DefaultColor = "blue"

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
	// ErrTeamNotInOrg: a team passed on creation does not belong to the project's organization.
	ErrTeamNotInOrg = apperror.WithCode(apperror.KindValidation, "project.team_not_in_org", "el equipo no pertenece a la organizacion del proyecto")
	// ErrProjectArchived: the project is archived and therefore read-only.
	ErrProjectArchived = apperror.WithCode(apperror.KindConflict, "project.archived", "el proyecto esta archivado y no admite cambios")
	// ErrProjectAlreadyArchived: archive was requested for a project that is already archived.
	ErrProjectAlreadyArchived = apperror.WithCode(apperror.KindConflict, "project.already_archived", "el proyecto ya esta archivado")
	// ErrProjectNotArchived: restore was requested for a project that is not archived.
	ErrProjectNotArchived = apperror.WithCode(apperror.KindConflict, "project.not_archived", "el proyecto no esta archivado")
	// ErrConfirmationMismatch: the typed confirmation does not equal the project name.
	ErrConfirmationMismatch = apperror.WithCode(apperror.KindValidation, "project.confirmation_mismatch", "el nombre escrito no coincide con el nombre del proyecto")
	// ErrInvalidAssigneePolicy: unknown default assignee policy.
	ErrInvalidAssigneePolicy = apperror.WithCode(apperror.KindValidation, "project.invalid_assignee_policy", "politica de asignacion por defecto no valida")
)

type Project struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	Key                   string
	Name                  string
	Description           string
	Type                  ProjectType
	Template              Template
	LeadID                uuid.UUID
	Visibility            Visibility
	Color                 string
	Icon                  string
	DefaultAssigneePolicy AssigneePolicy
	NextIssueNumber       int
	ArchivedAt            *time.Time
	DeletedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// UpdateInput carries the editable fields. Color, Icon and DefaultAssigneePolicy keep the current
// value when empty.
type UpdateInput struct {
	Name                  string
	Description           string
	Type                  ProjectType
	LeadID                uuid.UUID
	Visibility            Visibility
	Color                 string
	Icon                  string
	DefaultAssigneePolicy AssigneePolicy
}

// NewProjectInput carries everything needed to create a project from a template.
type NewProjectInput struct {
	OrganizationID        uuid.UUID
	Key                   string
	Name                  string
	Description           string
	Template              Template
	Visibility            Visibility
	LeadID                uuid.UUID
	Color                 string
	Icon                  string
	DefaultAssigneePolicy AssigneePolicy
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

// ParseAssigneePolicy validates a default assignee policy. An empty value means unassigned.
func ParseAssigneePolicy(value string) (AssigneePolicy, error) {
	switch AssigneePolicy(strings.TrimSpace(value)) {
	case "", AssigneePolicyUnassigned:
		return AssigneePolicyUnassigned, nil
	case AssigneePolicyLead:
		return AssigneePolicyLead, nil
	default:
		return "", ErrInvalidAssigneePolicy
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

// NewProject creates a project from the legacy project type. Scrum maps to the scrum template and
// kanban to the kanban template.
func NewProject(organizationID uuid.UUID, key, name, description string, projectType ProjectType, visibility Visibility, leadID uuid.UUID) (*Project, error) {
	if projectType != ProjectTypeKanban && projectType != ProjectTypeScrum {
		return nil, ErrInvalidProject
	}
	return NewProjectFromInput(NewProjectInput{
		OrganizationID: organizationID,
		Key:            key,
		Name:           name,
		Description:    description,
		Template:       TemplateFromProjectType(projectType),
		Visibility:     visibility,
		LeadID:         leadID,
	})
}

// NewProjectFromInput validates the input and builds a project. The key is normalized and checked
// against the key rule; the type always follows the template.
func NewProjectFromInput(input NewProjectInput) (*Project, error) {
	key, err := ValidateKey(input.Key)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if input.OrganizationID == uuid.Nil || name == "" || len(name) > 120 || input.LeadID == uuid.Nil {
		return nil, ErrInvalidProject
	}
	template, err := ParseTemplate(string(input.Template))
	if err != nil {
		return nil, err
	}
	visibility, err := ParseVisibility(string(input.Visibility))
	if err != nil {
		return nil, err
	}
	policy, err := ParseAssigneePolicy(string(input.DefaultAssigneePolicy))
	if err != nil {
		return nil, err
	}
	color, icon, err := normalizeAppearance(input.Color, input.Icon)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Project{
		OrganizationID:        input.OrganizationID,
		Key:                   key,
		Name:                  name,
		Description:           strings.TrimSpace(input.Description),
		Type:                  ProjectTypeOf(template),
		Template:              template,
		LeadID:                input.LeadID,
		Visibility:            visibility,
		Color:                 color,
		Icon:                  icon,
		DefaultAssigneePolicy: policy,
		NextIssueNumber:       1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, nil
}

// Update applies the editable fields. Archived projects reject every update.
//
// The type and the template move together: scrum and kanban switch between each other, which
// switches the template too. Task and bug templates keep their template and their kanban type;
// asking for scrum on them is rejected.
func (p *Project) Update(input UpdateInput) error {
	if p.IsArchived() {
		return ErrProjectArchived
	}
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
	template, err := templateForType(p.Template, input.Type)
	if err != nil {
		return err
	}
	policy := p.DefaultAssigneePolicy
	if input.DefaultAssigneePolicy != "" {
		if policy, err = ParseAssigneePolicy(string(input.DefaultAssigneePolicy)); err != nil {
			return err
		}
	}
	color := p.Color
	if strings.TrimSpace(input.Color) != "" {
		color, _, err = normalizeAppearance(input.Color, "")
		if err != nil {
			return err
		}
	}
	icon := p.Icon
	if strings.TrimSpace(input.Icon) != "" {
		_, icon, err = normalizeAppearance("", input.Icon)
		if err != nil {
			return err
		}
	}

	p.Name = input.Name
	p.Description = strings.TrimSpace(input.Description)
	p.Template = template
	p.Type = ProjectTypeOf(template)
	p.LeadID = input.LeadID
	p.Visibility = visibility
	p.DefaultAssigneePolicy = policy
	p.Color = color
	p.Icon = icon
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// templateForType returns the template that results from a requested legacy type.
func templateForType(current Template, requested ProjectType) (Template, error) {
	switch current {
	case TemplateScrum, TemplateKanban:
		return TemplateFromProjectType(requested), nil
	default:
		if requested == ProjectTypeScrum {
			return "", ErrInvalidProject
		}
		return current, nil
	}
}

// Archive makes the project read-only. Archiving an archived project is an error.
func (p *Project) Archive(at time.Time) error {
	if p.IsArchived() {
		return ErrProjectAlreadyArchived
	}
	at = at.UTC()
	p.ArchivedAt = &at
	p.UpdatedAt = at
	return nil
}

// Restore reverts Archive. Restoring a project that is not archived is an error.
func (p *Project) Restore(at time.Time) error {
	if !p.IsArchived() {
		return ErrProjectNotArchived
	}
	p.ArchivedAt = nil
	p.UpdatedAt = at.UTC()
	return nil
}

// SoftDelete marks the project as deleted. Deleted projects disappear from every read and free
// their key.
func (p *Project) SoftDelete(at time.Time) error {
	if p.IsDeleted() {
		return ErrProjectNotFound
	}
	at = at.UTC()
	p.DeletedAt = &at
	p.UpdatedAt = at
	return nil
}

// ConfirmDeletion checks the name typed by the caller. The comparison is exact after trimming
// surrounding spaces.
func (p *Project) ConfirmDeletion(typedName string) error {
	if strings.TrimSpace(typedName) != p.Name {
		return ErrConfirmationMismatch
	}
	return nil
}

// IsArchived reports whether the project is archived.
func (p *Project) IsArchived() bool {
	return p.ArchivedAt != nil
}

// IsDeleted reports whether the project is soft deleted.
func (p *Project) IsDeleted() bool {
	return p.DeletedAt != nil
}

// normalizeAppearance validates color and icon. An empty color gets the default.
func normalizeAppearance(color, icon string) (string, string, error) {
	color = strings.ToLower(strings.TrimSpace(color))
	if color == "" {
		color = DefaultColor
	}
	icon = strings.TrimSpace(icon)
	if len(color) > 32 || len(icon) > 64 {
		return "", "", ErrInvalidProject
	}
	return color, icon, nil
}
