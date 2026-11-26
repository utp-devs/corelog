package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Team roles inside a team.
const (
	RoleMaintainer = "maintainer"
	RoleMember     = "member"
)

// Organization roles that matter to teams.
const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
	OrgRoleGuest  = "guest"
)

// Organization policy values for who_can_create_teams.
const (
	CreationPolicyMembers = "members"
	CreationPolicyAdmins  = "admins"
)

const (
	maxNameLength        = 120
	maxDescriptionLength = 500
)

var avatarColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Team struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Description    string
	AvatarColor    string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewTeamInput carries the attributes required to create a team.
type NewTeamInput struct {
	OrganizationID uuid.UUID
	CreatedBy      uuid.UUID
	Name           string
	Description    string
	AvatarColor    string
}

// TeamPatch changes the editable attributes of a team. Nil fields are left unchanged.
type TeamPatch struct {
	Name        *string
	Description *string
	AvatarColor *string
}

// Member is a user's membership in a team.
type Member struct {
	TeamID    uuid.UUID
	UserID    uuid.UUID
	Role      string
	CreatedAt time.Time
}

func NewTeam(in NewTeamInput) (*Team, error) {
	if in.OrganizationID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	if err := validateDescription(in.Description); err != nil {
		return nil, err
	}
	if err := validateAvatarColor(in.AvatarColor); err != nil {
		return nil, err
	}

	// ID is left as uuid.Nil: the database generates it.
	now := time.Now().UTC()
	return &Team{
		OrganizationID: in.OrganizationID,
		Name:           name,
		Description:    in.Description,
		AvatarColor:    in.AvatarColor,
		CreatedBy:      in.CreatedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Apply validates and applies a patch. Nothing changes when validation fails.
func (t *Team) Apply(p TeamPatch) error {
	name := t.Name
	if p.Name != nil {
		n, err := normalizeName(*p.Name)
		if err != nil {
			return err
		}
		name = n
	}
	description := t.Description
	if p.Description != nil {
		if err := validateDescription(*p.Description); err != nil {
			return err
		}
		description = *p.Description
	}
	color := t.AvatarColor
	if p.AvatarColor != nil {
		if err := validateAvatarColor(*p.AvatarColor); err != nil {
			return err
		}
		color = *p.AvatarColor
	}

	t.Name = name
	t.Description = description
	t.AvatarColor = color
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// ValidRole reports whether role is a team role.
func ValidRole(role string) bool {
	return role == RoleMaintainer || role == RoleMember
}

// NewMember builds a membership. An empty role defaults to member.
func NewMember(teamID, userID uuid.UUID, role string) (*Member, error) {
	if role == "" {
		role = RoleMember
	}
	if !ValidRole(role) {
		return nil, ErrInvalidRole
	}
	return &Member{TeamID: teamID, UserID: userID, Role: role}, nil
}

// IsOrgAdmin reports whether an organization role may manage every team of the org.
func IsOrgAdmin(role string) bool {
	return role == OrgRoleOwner || role == OrgRoleAdmin
}

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxNameLength {
		return "", ErrInvalidName
	}
	return name, nil
}

func validateDescription(d string) error {
	if len([]rune(d)) > maxDescriptionLength {
		return ErrInvalidDescription
	}
	return nil
}

func validateAvatarColor(c string) error {
	if c != "" && !avatarColorPattern.MatchString(c) {
		return ErrInvalidAvatarColor
	}
	return nil
}
