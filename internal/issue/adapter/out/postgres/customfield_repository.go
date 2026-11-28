package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// constraintCustomFieldName and constraintCustomFieldContext are the unique indexes mapped to domain errors.
const (
	constraintCustomFieldName    = "uq_custom_fields_org_name"
	constraintCustomFieldContext = "uq_custom_field_contexts_scope"
)

// CustomFieldRepository stores custom field definitions, their contexts, project layouts and issue values.
type CustomFieldRepository struct {
	db *sqlx.DB
}

var _ port.CustomFieldStore = (*CustomFieldRepository)(nil)

func NewCustomFieldRepository(db *sqlx.DB) *CustomFieldRepository {
	return &CustomFieldRepository{db: db}
}

const customFieldColumns = `id, organization_id, name, description, field_type, options, settings, position, created_at, updated_at`

type customFieldRow struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	FieldType      string    `db:"field_type"`
	Options        []byte    `db:"options"`
	Settings       []byte    `db:"settings"`
	Position       int       `db:"position"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type customContextRow struct {
	ID          uuid.UUID  `db:"id"`
	FieldID     uuid.UUID  `db:"field_id"`
	ProjectID   *uuid.UUID `db:"project_id"`
	IssueTypeID *uuid.UUID `db:"issue_type_id"`
	Required    bool       `db:"required"`
	Default     []byte     `db:"default_value"`
	Position    int        `db:"position"`
}

// optionJSON and settingsJSON are the stored shapes of the options and settings columns.
type optionJSON struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Color    string `json:"color,omitempty"`
	Position int    `json:"position"`
	Archived bool   `json:"archived"`
}

type settingsJSON struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// layoutJSON is one stored layout entry.
type layoutJSON struct {
	Kind     string `json:"kind"`
	Key      string `json:"key,omitempty"`
	FieldID  string `json:"field_id,omitempty"`
	Required bool   `json:"required"`
}

// ListFields returns the definitions of the organization with their contexts. It runs two queries whatever
// the number of fields.
func (r *CustomFieldRepository) ListFields(ctx context.Context, orgID uuid.UUID) ([]*domain.CustomField, error) {
	exec := database.Executor(ctx, r.db)
	var rows []customFieldRow
	if err := sqlx.SelectContext(ctx, exec, &rows, `
		SELECT `+customFieldColumns+`
		FROM custom_fields
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY position, lower(name)`, orgID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list custom fields", err)
	}
	if len(rows) == 0 {
		return []*domain.CustomField{}, nil
	}
	var contexts []customContextRow
	if err := sqlx.SelectContext(ctx, exec, &contexts, `
		SELECT c.id, c.field_id, c.project_id, c.issue_type_id, c.required, c.default_value, c.position
		FROM custom_field_contexts c
		JOIN custom_fields f ON f.id = c.field_id
		WHERE f.organization_id = $1 AND f.deleted_at IS NULL
		ORDER BY c.position, c.id`, orgID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list custom field contexts", err)
	}
	fields := make([]*domain.CustomField, 0, len(rows))
	byID := make(map[uuid.UUID]*domain.CustomField, len(rows))
	for _, row := range rows {
		field, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		byID[field.ID] = field
	}
	for _, ctxRow := range contexts {
		field, ok := byID[ctxRow.FieldID]
		if !ok {
			continue
		}
		field.Contexts = append(field.Contexts, ctxRow.toDomain())
	}
	return fields, nil
}

// FindField returns one definition of the organization. Definitions of other organizations are not found.
func (r *CustomFieldRepository) FindField(ctx context.Context, orgID, fieldID uuid.UUID) (*domain.CustomField, error) {
	exec := database.Executor(ctx, r.db)
	var row customFieldRow
	err := sqlx.GetContext(ctx, exec, &row, `
		SELECT `+customFieldColumns+`
		FROM custom_fields
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`, fieldID, orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomFieldNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find custom field", err)
	}
	field, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	var contexts []customContextRow
	if err := sqlx.SelectContext(ctx, exec, &contexts, `
		SELECT id, field_id, project_id, issue_type_id, required, default_value, position
		FROM custom_field_contexts
		WHERE field_id = $1
		ORDER BY position, id`, fieldID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load custom field contexts", err)
	}
	for _, ctxRow := range contexts {
		field.Contexts = append(field.Contexts, ctxRow.toDomain())
	}
	return field, nil
}

// InsertField stores a new definition and sets its id.
func (r *CustomFieldRepository) InsertField(ctx context.Context, field *domain.CustomField) error {
	options, settings, err := encodeFieldJSON(field)
	if err != nil {
		return err
	}
	var id uuid.UUID
	err = database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		INSERT INTO custom_fields (organization_id, name, description, field_type, options, settings, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CAST($5 AS JSONB), CAST($6 AS JSONB), $7, $8, $9)
		RETURNING id`,
		field.OrganizationID, field.Name, field.Description, string(field.Type), options, settings,
		field.Position, field.CreatedAt, field.UpdatedAt).Scan(&id)
	if err != nil {
		if isUniqueViolation(err, constraintCustomFieldName) {
			return domain.ErrCustomFieldNameTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save custom field", err)
	}
	field.ID = id
	return nil
}

// UpdateField stores the mutable parts of a definition.
func (r *CustomFieldRepository) UpdateField(ctx context.Context, field *domain.CustomField) error {
	options, settings, err := encodeFieldJSON(field)
	if err != nil {
		return err
	}
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE custom_fields
		SET name = $3, description = $4, options = CAST($5 AS JSONB), settings = CAST($6 AS JSONB),
		    position = $7, updated_at = $8
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`,
		field.ID, field.OrganizationID, field.Name, field.Description, options, settings, field.Position, field.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, constraintCustomFieldName) {
			return domain.ErrCustomFieldNameTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to update custom field", err)
	}
	return requireAffected(result)
}

// DeleteField logically deletes a definition; its contexts and stored values stay for history and are hidden with it.
func (r *CustomFieldRepository) DeleteField(ctx context.Context, field *domain.CustomField) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE custom_fields SET deleted_at = now() WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`, field.ID, field.OrganizationID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete custom field", err)
	}
	return requireAffected(result)
}

// ReplaceContexts deletes the contexts of a field and inserts the given ones in one transaction.
func (r *CustomFieldRepository) ReplaceContexts(ctx context.Context, field *domain.CustomField, contexts []domain.FieldContext) error {
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		exec := database.Executor(ctx, r.db)
		if _, err := exec.ExecContext(ctx, `DELETE FROM custom_field_contexts WHERE field_id = $1`, field.ID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to replace custom field contexts", err)
		}
		for _, ctxValue := range contexts {
			var defaultJSON []byte
			if ctxValue.DefaultValue != nil {
				raw, err := json.Marshal(ctxValue.DefaultValue)
				if err != nil {
					return apperror.Wrap(apperror.KindInternal, "failed to encode default value", err)
				}
				defaultJSON = raw
			}
			_, err := exec.ExecContext(ctx, `
				INSERT INTO custom_field_contexts (field_id, project_id, issue_type_id, required, default_value, position)
				VALUES ($1, $2, $3, $4, CAST($5 AS JSONB), $6)`,
				field.ID, ctxValue.ProjectID, ctxValue.IssueTypeID, ctxValue.Required, nullableJSON(defaultJSON), ctxValue.Position)
			if err != nil {
				if isUniqueViolation(err, constraintCustomFieldContext) {
					return domain.ErrCustomFieldContextDup
				}
				return apperror.Wrap(apperror.KindInternal, "failed to save custom field context", err)
			}
		}
		return nil
	})
}

// RemoveValues deletes the stored values of a field from every issue of the organization.
func (r *CustomFieldRepository) RemoveValues(ctx context.Context, orgID, fieldID uuid.UUID) error {
	_, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE issues
		SET custom = custom - $2::text
		WHERE project_id IN (SELECT p.id FROM projects p WHERE p.organization_id = $1)
		  AND custom -> $2::text IS NOT NULL`, orgID, fieldID.String())
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove custom field values", err)
	}
	return nil
}

// CountOptionUsage counts the issues of the organization whose value holds the option. The containment test
// matches a scalar value (select) and a list (multiselect) alike.
func (r *CustomFieldRepository) CountOptionUsage(ctx context.Context, orgID, fieldID uuid.UUID, optionID string) (int, error) {
	var count int
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT COUNT(*)
		FROM issues i
		JOIN projects p ON p.id = i.project_id
		WHERE p.organization_id = $1 AND i.custom -> $2::text @> to_jsonb($3::text)`,
		orgID, fieldID.String(), optionID).Scan(&count)
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count option usage", err)
	}
	return count, nil
}

// ProjectOrganization returns the organization of a live project.
func (r *CustomFieldRepository) ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT organization_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, apperror.New(apperror.KindNotFound, "project not found")
		}
		return uuid.Nil, apperror.Wrap(apperror.KindInternal, "failed to load project", err)
	}
	return orgID, nil
}

// IssueTypeInOrganization reports whether the issue type belongs to the organization.
func (r *CustomFieldRepository) IssueTypeInOrganization(ctx context.Context, orgID, issueTypeID uuid.UUID) (bool, error) {
	var exists bool
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM issue_types WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL)`, issueTypeID, orgID).Scan(&exists)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check issue type", err)
	}
	return exists, nil
}

// FindLayout returns the stored layout of a project and issue type. The flag is false when none is stored.
func (r *CustomFieldRepository) FindLayout(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]domain.LayoutItem, bool, error) {
	var raw []byte
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT items FROM project_field_layouts WHERE project_id = $1 AND issue_type_id = $2`,
		projectID, issueTypeID).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, apperror.Wrap(apperror.KindInternal, "failed to load field layout", err)
	}
	var stored []layoutJSON
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, false, apperror.Wrap(apperror.KindInternal, "failed to decode field layout", err)
	}
	items := make([]domain.LayoutItem, 0, len(stored))
	for _, entry := range stored {
		item := domain.LayoutItem{Kind: domain.LayoutKind(entry.Kind), Key: entry.Key, Required: entry.Required}
		if item.Kind == domain.LayoutCustom {
			id, err := uuid.Parse(entry.FieldID)
			if err != nil {
				return nil, false, apperror.Wrap(apperror.KindInternal, "failed to decode field layout", err)
			}
			item.FieldID = id
		}
		items = append(items, item)
	}
	return items, true, nil
}

// SaveLayout stores the layout of a project and issue type, replacing any previous one.
func (r *CustomFieldRepository) SaveLayout(ctx context.Context, projectID, issueTypeID uuid.UUID, items []domain.LayoutItem) error {
	stored := make([]layoutJSON, 0, len(items))
	for _, item := range items {
		entry := layoutJSON{Kind: string(item.Kind), Required: item.Required}
		if item.Kind == domain.LayoutCustom {
			entry.FieldID = item.FieldID.String()
		} else {
			entry.Key = item.Key
		}
		stored = append(stored, entry)
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to encode field layout", err)
	}
	_, err = database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO project_field_layouts (project_id, issue_type_id, items, updated_at)
		VALUES ($1, $2, CAST($3 AS JSONB), now())
		ON CONFLICT (project_id, issue_type_id) DO UPDATE SET items = EXCLUDED.items, updated_at = now()`,
		projectID, issueTypeID, string(raw))
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save field layout", err)
	}
	return nil
}

func (row customFieldRow) toDomain() (*domain.CustomField, error) {
	var options []optionJSON
	if err := json.Unmarshal(row.Options, &options); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to decode custom field options", err)
	}
	var settings settingsJSON
	if err := json.Unmarshal(row.Settings, &settings); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to decode custom field settings", err)
	}
	field := &domain.CustomField{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Description:    row.Description,
		Type:           domain.FieldType(row.FieldType),
		NumberMin:      settings.Min,
		NumberMax:      settings.Max,
		Position:       row.Position,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	for _, option := range options {
		field.Options = append(field.Options, domain.FieldOption{
			ID: option.ID, Label: option.Label, Color: option.Color, Position: option.Position, Archived: option.Archived,
		})
	}
	return field, nil
}

func (row customContextRow) toDomain() domain.FieldContext {
	ctx := domain.FieldContext{
		ID:          row.ID,
		FieldID:     row.FieldID,
		ProjectID:   row.ProjectID,
		IssueTypeID: row.IssueTypeID,
		Required:    row.Required,
		Position:    row.Position,
	}
	if len(row.Default) > 0 {
		// The default was normalized when it was stored, so it decodes into the same shapes as values.
		var value any
		if err := json.Unmarshal(row.Default, &value); err == nil {
			ctx.DefaultValue = value
		}
	}
	return ctx
}

// encodeFieldJSON serializes the options and settings columns of a definition.
func encodeFieldJSON(field *domain.CustomField) (options, settings string, err error) {
	stored := make([]optionJSON, 0, len(field.Options))
	for _, option := range field.Options {
		stored = append(stored, optionJSON{ID: option.ID, Label: option.Label, Color: option.Color, Position: option.Position, Archived: option.Archived})
	}
	rawOptions, err := json.Marshal(stored)
	if err != nil {
		return "", "", apperror.Wrap(apperror.KindInternal, "failed to encode custom field options", err)
	}
	rawSettings, err := json.Marshal(settingsJSON{Min: field.NumberMin, Max: field.NumberMax})
	if err != nil {
		return "", "", apperror.Wrap(apperror.KindInternal, "failed to encode custom field settings", err)
	}
	return string(rawOptions), string(rawSettings), nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to read affected rows", err)
	}
	if affected == 0 {
		return domain.ErrCustomFieldNotFound
	}
	return nil
}

// isUniqueViolation reports whether err is a unique violation of the named constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == constraint
}
