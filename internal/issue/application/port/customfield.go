package port

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// CustomFieldStore is the out-port of custom field definitions, contexts, project layouts and issue values.
type CustomFieldStore interface {
	// ListFields returns the definitions of an organization with their contexts, in position order.
	ListFields(ctx context.Context, orgID uuid.UUID) ([]*domain.CustomField, error)
	// FindField returns one definition of the organization with its contexts. Other organizations get not found.
	FindField(ctx context.Context, orgID, fieldID uuid.UUID) (*domain.CustomField, error)
	// InsertField stores a new definition and sets its id. A taken name returns domain.ErrCustomFieldNameTaken.
	InsertField(ctx context.Context, field *domain.CustomField) error
	// UpdateField stores name, description, options, settings and position of a definition.
	UpdateField(ctx context.Context, field *domain.CustomField) error
	// DeleteField removes a definition and its contexts.
	DeleteField(ctx context.Context, field *domain.CustomField) error
	// ReplaceContexts replaces every context of a field.
	ReplaceContexts(ctx context.Context, field *domain.CustomField, contexts []domain.FieldContext) error
	// RemoveValues deletes the stored values of a field from every issue of the organization.
	RemoveValues(ctx context.Context, orgID, fieldID uuid.UUID) error
	// CountOptionUsage counts the issues of the organization whose value uses the option.
	CountOptionUsage(ctx context.Context, orgID, fieldID uuid.UUID, optionID string) (int, error)
	// ProjectOrganization returns the organization of a project. Missing projects return not found.
	ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	// IssueTypeInOrganization reports whether the issue type belongs to the organization.
	IssueTypeInOrganization(ctx context.Context, orgID, issueTypeID uuid.UUID) (bool, error)
	// FindLayout returns the stored layout of a project and issue type. The flag is false when none is stored.
	FindLayout(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]domain.LayoutItem, bool, error)
	// SaveLayout stores the layout of a project and issue type, replacing any previous one.
	SaveLayout(ctx context.Context, projectID, issueTypeID uuid.UUID, items []domain.LayoutItem) error
}

// CustomFieldScopeReader resolves the custom fields that apply to issues of a project and issue type. The
// issue use cases depend on it to validate values and the HTTP layer to render them.
type CustomFieldScopeReader interface {
	ScopeFor(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]domain.ScopedField, error)
}

// ProjectBrowseChecker tells whether a user can browse a project. Values of user fields must name such a user.
type ProjectBrowseChecker interface {
	CanBrowse(ctx context.Context, projectID, userID uuid.UUID) (bool, error)
}

// CustomFieldInput is the body of a definition creation.
type CustomFieldInput struct {
	Name        string
	Description string
	Type        domain.FieldType
	Options     []domain.OptionInput
	NumberMin   *float64
	NumberMax   *float64
}

// CustomFieldPatch is a partial change of a definition. Nil fields keep their value. Settings replace both
// bounds when Settings is set.
type CustomFieldPatch struct {
	Name        *string
	Description *string
	// Options replaces the options list when set. Removed options that values use are archived, not deleted.
	Options  *[]domain.OptionInput
	Settings *CustomFieldSettings
}

// CustomFieldSettings holds the number bounds of a definition.
type CustomFieldSettings struct {
	Min *float64
	Max *float64
}

// ContextInput is one context of a definition. Default is the raw JSON default value (null for none).
type ContextInput struct {
	ProjectID   *uuid.UUID
	IssueTypeID *uuid.UUID
	Required    bool
	Default     json.RawMessage
	Position    int
}

// LayoutEntry is one row of a project layout as shown to clients. Field is set for custom entries.
type LayoutEntry struct {
	Item  domain.LayoutItem
	Field *domain.CustomField
}

// LayoutView is the layout of one issue type in one project.
type LayoutView struct {
	IssueTypeID uuid.UUID
	Configured  bool
	Items       []LayoutEntry
}

// CustomFieldService is the in-port that manages definitions, contexts and project layouts.
type CustomFieldService interface {
	ListCustomFields(ctx context.Context, actorID, orgID uuid.UUID) ([]*domain.CustomField, error)
	CreateCustomField(ctx context.Context, actorID, orgID uuid.UUID, in CustomFieldInput) (*domain.CustomField, error)
	UpdateCustomField(ctx context.Context, actorID, orgID, fieldID uuid.UUID, patch CustomFieldPatch) (*domain.CustomField, error)
	DeleteCustomField(ctx context.Context, actorID, orgID, fieldID uuid.UUID) error
	ReplaceCustomFieldContexts(ctx context.Context, actorID, orgID, fieldID uuid.UUID, contexts []ContextInput) (*domain.CustomField, error)
	ProjectLayout(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueTypeID uuid.UUID) (LayoutView, error)
	ReplaceProjectLayout(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueTypeID uuid.UUID, items []domain.LayoutItem) (LayoutView, error)
	// ScopeFor implements CustomFieldScopeReader.
	ScopeFor(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]domain.ScopedField, error)
}
