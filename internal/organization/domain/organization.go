// Package domain holds the pure rules of the organization hexagon: entities, value objects,
// role rules and typed errors. It has no transport or persistence concerns.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Organization is the top-level tenant. ID stays uuid.Nil until the repository assigns it.
type Organization struct {
	ID          uuid.UUID
	Slug        string
	Name        string
	Description string
	Settings    Settings
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// Member is a user's membership in an organization. The pair (OrganizationID, UserID) is the key.
type Member struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           Role
	JoinedAt       time.Time
}

// NewOrganization builds an organization with default settings. An empty slug is derived from the name.
// An explicit slug is validated as given, without normalization.
func NewOrganization(name, slug, description string, createdBy uuid.UUID, now time.Time) (*Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if strings.TrimSpace(slug) == "" {
		slug = NormalizeSlug(name)
	}
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}
	at := now.UTC()
	return &Organization{
		Slug:        slug,
		Name:        name,
		Description: strings.TrimSpace(description),
		Settings:    DefaultSettings(),
		CreatedBy:   createdBy,
		CreatedAt:   at,
		UpdatedAt:   at,
	}, nil
}

// NewMember builds a membership after checking the role.
func NewMember(orgID, userID uuid.UUID, role Role, now time.Time) (*Member, error) {
	if !role.IsValid() {
		return nil, ErrRoleInvalid
	}
	return &Member{OrganizationID: orgID, UserID: userID, Role: role, JoinedAt: now.UTC()}, nil
}

// ApplySettings validates and replaces the settings.
func (o *Organization) ApplySettings(s Settings, now time.Time) error {
	if err := s.Validate(); err != nil {
		return err
	}
	o.Settings = s
	o.UpdatedAt = now.UTC()
	return nil
}

// ConfirmDeletion checks the slug typed by the user before a destructive action.
func (o *Organization) ConfirmDeletion(confirmSlug string) error {
	if confirmSlug != o.Slug {
		return ErrConfirmationMismatch
	}
	return nil
}

// MarkDeleted sets the soft-delete timestamp.
func (o *Organization) MarkDeleted(now time.Time) {
	at := now.UTC()
	o.DeletedAt = &at
	o.UpdatedAt = at
}

// IsDeleted reports whether the organization is soft-deleted.
func (o *Organization) IsDeleted() bool {
	return o.DeletedAt != nil
}
