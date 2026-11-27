package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// CustomFieldService implements port.CustomFieldService: definitions and contexts (organization admins
// write, members read), and the project layouts per issue type (project editors write, viewers read).
type CustomFieldService struct {
	store      port.CustomFieldStore
	roles      port.OrgRoleChecker
	authorizer port.ProjectAuthorizer
	tx         database.TxManager
	clock      clock.Clock
	newID      func() string
}

var (
	_ port.CustomFieldService     = (*CustomFieldService)(nil)
	_ port.CustomFieldScopeReader = (*CustomFieldService)(nil)
)

// NewCustomFieldService wires the custom field use cases. All dependencies are required.
func NewCustomFieldService(store port.CustomFieldStore, roles port.OrgRoleChecker, authorizer port.ProjectAuthorizer, tx database.TxManager, clk clock.Clock) *CustomFieldService {
	return &CustomFieldService{store: store, roles: roles, authorizer: authorizer, tx: tx, clock: clk, newID: uuid.NewString}
}

// ListCustomFields returns the definitions of the organization with their contexts.
func (s *CustomFieldService) ListCustomFields(ctx context.Context, actorID, orgID uuid.UUID) ([]*domain.CustomField, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleGuest); err != nil {
		return nil, err
	}
	return s.store.ListFields(ctx, orgID)
}

// CreateCustomField adds a definition. Option ids are assigned here; the client does not send them.
func (s *CustomFieldService) CreateCustomField(ctx context.Context, actorID, orgID uuid.UUID, in port.CustomFieldInput) (*domain.CustomField, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	name, err := domain.NormalizeFieldName(in.Name)
	if err != nil {
		return nil, err
	}
	description, err := domain.NormalizeFieldDescription(in.Description)
	if err != nil {
		return nil, err
	}
	options, _, err := domain.MergeOptions(nil, in.Options, s.newID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	field := &domain.CustomField{
		OrganizationID: orgID,
		Name:           name,
		Description:    description,
		Type:           in.Type,
		Options:        options,
		NumberMin:      in.NumberMin,
		NumberMax:      in.NumberMax,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := domain.ValidateDefinition(field); err != nil {
		return nil, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		existing, err := s.store.ListFields(ctx, orgID)
		if err != nil {
			return err
		}
		field.Position = len(existing)
		return s.store.InsertField(ctx, field)
	})
	if err != nil {
		return nil, err
	}
	return s.store.FindField(ctx, orgID, field.ID)
}

// UpdateCustomField changes name, description, options or number bounds. The type never changes. Removed
// options that values still use are kept as archived options so those values keep resolving.
func (s *CustomFieldService) UpdateCustomField(ctx context.Context, actorID, orgID, fieldID uuid.UUID, patch port.CustomFieldPatch) (*domain.CustomField, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		field, err := s.store.FindField(ctx, orgID, fieldID)
		if err != nil {
			return err
		}
		if patch.Name != nil {
			name, err := domain.NormalizeFieldName(*patch.Name)
			if err != nil {
				return err
			}
			field.Name = name
		}
		if patch.Description != nil {
			description, err := domain.NormalizeFieldDescription(*patch.Description)
			if err != nil {
				return err
			}
			field.Description = description
		}
		if patch.Settings != nil {
			field.NumberMin = patch.Settings.Min
			field.NumberMax = patch.Settings.Max
		}
		if patch.Options != nil {
			if err := s.replaceOptions(ctx, field, *patch.Options); err != nil {
				return err
			}
		}
		if err := domain.ValidateDefinition(field); err != nil {
			return err
		}
		field.UpdatedAt = s.clock.Now().UTC()
		return s.store.UpdateField(ctx, field)
	})
	if err != nil {
		return nil, err
	}
	return s.store.FindField(ctx, orgID, fieldID)
}

// replaceOptions applies a client options list. Options removed from the list are dropped when no value uses
// them and archived otherwise.
func (s *CustomFieldService) replaceOptions(ctx context.Context, field *domain.CustomField, incoming []domain.OptionInput) error {
	kept, removed, err := domain.MergeOptions(field.Options, incoming, s.newID)
	if err != nil {
		return err
	}
	for _, option := range removed {
		used, err := s.store.CountOptionUsage(ctx, field.OrganizationID, field.ID, option.ID)
		if err != nil {
			return err
		}
		if used > 0 {
			option.Archived = true
			kept = append(kept, option)
		}
	}
	for index := range kept {
		kept[index].Position = index
	}
	field.Options = kept
	return nil
}

// DeleteCustomField removes a definition and the values stored for it in the organization's issues.
func (s *CustomFieldService) DeleteCustomField(ctx context.Context, actorID, orgID, fieldID uuid.UUID) error {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		field, err := s.store.FindField(ctx, orgID, fieldID)
		if err != nil {
			return err
		}
		if err := s.store.RemoveValues(ctx, orgID, fieldID); err != nil {
			return err
		}
		return s.store.DeleteField(ctx, field)
	})
}

// ReplaceCustomFieldContexts replaces every context of a definition. Projects and issue types must belong to
// the organization; each project and type pair appears once; defaults must be valid values of the field.
func (s *CustomFieldService) ReplaceCustomFieldContexts(ctx context.Context, actorID, orgID, fieldID uuid.UUID, inputs []port.ContextInput) (*domain.CustomField, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	field, err := s.store.FindField(ctx, orgID, fieldID)
	if err != nil {
		return nil, err
	}
	contexts := make([]domain.FieldContext, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for index, in := range inputs {
		if err := s.requireContextScope(ctx, orgID, in); err != nil {
			return nil, err
		}
		key := contextKey(in.ProjectID, in.IssueTypeID)
		if _, dup := seen[key]; dup {
			return nil, domain.ErrCustomFieldContextDup
		}
		seen[key] = struct{}{}
		var defaultValue any
		if len(in.Default) > 0 {
			defaultValue, err = domain.NormalizeDefault(field, in.Default)
			if err != nil {
				return nil, err
			}
		}
		contexts = append(contexts, domain.FieldContext{
			FieldID:      fieldID,
			ProjectID:    in.ProjectID,
			IssueTypeID:  in.IssueTypeID,
			Required:     in.Required,
			DefaultValue: defaultValue,
			Position:     index,
		})
	}
	if err := s.store.ReplaceContexts(ctx, field, contexts); err != nil {
		return nil, err
	}
	return s.store.FindField(ctx, orgID, fieldID)
}

func (s *CustomFieldService) requireContextScope(ctx context.Context, orgID uuid.UUID, in port.ContextInput) error {
	if in.ProjectID != nil {
		owner, err := s.store.ProjectOrganization(ctx, *in.ProjectID)
		if err != nil {
			if apperror.KindOf(err) == apperror.KindNotFound {
				return domain.ErrCustomFieldContext
			}
			return err
		}
		if owner != orgID {
			return domain.ErrCustomFieldContext
		}
	}
	if in.IssueTypeID != nil {
		ok, err := s.store.IssueTypeInOrganization(ctx, orgID, *in.IssueTypeID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrCustomFieldContext
		}
	}
	return nil
}

func contextKey(projectID, issueTypeID *uuid.UUID) string {
	key := "all/all"
	if projectID != nil {
		key = projectID.String()
	}
	if issueTypeID != nil {
		key += "/" + issueTypeID.String()
	} else {
		key += "/all"
	}
	return key
}

// ProjectLayout returns the ordered fields of an issue type in a project. Viewers may read it.
func (s *CustomFieldService) ProjectLayout(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueTypeID uuid.UUID) (port.LayoutView, error) {
	if err := s.authorizer.Authorize(ctx, actionViewProject, actorID, globalRole, projectID); err != nil {
		return port.LayoutView{}, err
	}
	return s.layoutView(ctx, projectID, issueTypeID)
}

// ReplaceProjectLayout stores the ordered layout of an issue type in a project. Project editors only.
func (s *CustomFieldService) ReplaceProjectLayout(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueTypeID uuid.UUID, items []domain.LayoutItem) (port.LayoutView, error) {
	if err := s.authorizer.Authorize(ctx, actionEditProject, actorID, globalRole, projectID); err != nil {
		return port.LayoutView{}, err
	}
	orgID, err := s.store.ProjectOrganization(ctx, projectID)
	if err != nil {
		return port.LayoutView{}, err
	}
	ok, err := s.store.IssueTypeInOrganization(ctx, orgID, issueTypeID)
	if err != nil {
		return port.LayoutView{}, err
	}
	if !ok {
		return port.LayoutView{}, domain.ErrCatalogValueUnknown
	}
	fields, err := s.store.ListFields(ctx, orgID)
	if err != nil {
		return port.LayoutView{}, err
	}
	known := make(map[uuid.UUID]bool, len(fields))
	for _, field := range fields {
		known[field.ID] = true
	}
	if problems := domain.ValidateLayout(items, known); len(problems) > 0 {
		return port.LayoutView{}, layoutInvalid(problems)
	}
	if err := s.store.SaveLayout(ctx, projectID, issueTypeID, items); err != nil {
		return port.LayoutView{}, err
	}
	return s.layoutView(ctx, projectID, issueTypeID)
}

// layoutInvalid builds a fresh layout error carrying the problems. The shared error value must not be
// extended, because WithDetails mutates it.
func layoutInvalid(problems []apperror.FieldError) error {
	return apperror.WithCode(apperror.KindValidation, "project.field_layout_invalid", "la disposicion de campos no es valida").WithDetails(problems...)
}

// layoutView builds the layout a project shows for an issue type.
func (s *CustomFieldService) layoutView(ctx context.Context, projectID, issueTypeID uuid.UUID) (port.LayoutView, error) {
	stored, configured, scope, err := s.resolve(ctx, projectID, issueTypeID)
	if err != nil {
		return port.LayoutView{}, err
	}
	byID := make(map[uuid.UUID]domain.ScopedField, len(scope))
	for _, entry := range scope {
		byID[entry.Field.ID] = entry
	}
	view := port.LayoutView{IssueTypeID: issueTypeID, Configured: configured}
	for _, item := range domain.ResolveLayout(stored, scope) {
		entry := port.LayoutEntry{Item: item}
		if item.Kind == domain.LayoutCustom {
			entry.Field = byID[item.FieldID].Field
		}
		view.Items = append(view.Items, entry)
	}
	return view, nil
}

// ScopeFor returns the custom fields that apply to issues of the project and issue type, with the effective
// required flag and default. It does not check the caller; use cases call it after their own checks.
func (s *CustomFieldService) ScopeFor(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]domain.ScopedField, error) {
	_, _, scope, err := s.resolve(ctx, projectID, issueTypeID)
	return scope, err
}

// resolve loads the fields of the project's organization, the stored layout and the fields in scope.
func (s *CustomFieldService) resolve(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]domain.LayoutItem, bool, []domain.ScopedField, error) {
	orgID, err := s.store.ProjectOrganization(ctx, projectID)
	if err != nil {
		return nil, false, nil, err
	}
	fields, err := s.store.ListFields(ctx, orgID)
	if err != nil {
		return nil, false, nil, err
	}
	stored, configured, err := s.store.FindLayout(ctx, projectID, issueTypeID)
	if err != nil {
		return nil, false, nil, err
	}
	layoutRequired := make(map[uuid.UUID]bool)
	for _, item := range stored {
		if item.Kind == domain.LayoutCustom && item.Required {
			layoutRequired[item.FieldID] = true
		}
	}
	return stored, configured, domain.BuildScope(fields, layoutRequired, projectID, issueTypeID), nil
}
