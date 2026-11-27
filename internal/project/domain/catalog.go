package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/palette"
)

var ErrInvalidCatalogItem = apperror.WithCode(apperror.KindValidation, "project.catalog_item_invalid", "name is required")

// ErrInvalidLabelColor reports a label color outside the palette.
var ErrInvalidLabelColor = apperror.WithCode(apperror.KindValidation, "project.label_color_invalid",
	"el color de la etiqueta no pertenece a la paleta")

type Label struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Color     string
	// UsageCount is the number of live issues that carry the label. Only the list query fills it.
	UsageCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewLabel(projectID uuid.UUID, name, color string) (*Label, error) {
	name = strings.TrimSpace(name)
	if projectID == uuid.Nil || name == "" || len(name) > 40 {
		return nil, ErrInvalidCatalogItem
	}
	color = strings.TrimSpace(color)
	if color == "" {
		color = palette.Default
	}
	if !palette.IsKey(color) {
		return nil, ErrInvalidLabelColor
	}
	now := time.Now().UTC()
	return &Label{ProjectID: projectID, Name: name, Color: color, CreatedAt: now, UpdatedAt: now}, nil
}

// Update changes the name and/or the color. A nil argument keeps the current value.
func (l *Label) Update(name, color *string) error {
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" || len(trimmed) > 40 {
			return ErrInvalidCatalogItem
		}
		l.Name = trimmed
	}
	if color != nil {
		trimmed := strings.TrimSpace(*color)
		if !palette.IsKey(trimmed) {
			return ErrInvalidLabelColor
		}
		l.Color = trimmed
	}
	return nil
}

type Component struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewComponent(projectID uuid.UUID, name, description string) (*Component, error) {
	name = strings.TrimSpace(name)
	if projectID == uuid.Nil || name == "" || len(name) > 80 {
		return nil, ErrInvalidCatalogItem
	}
	now := time.Now().UTC()
	return &Component{
		ProjectID: projectID, Name: name,
		Description: strings.TrimSpace(description), CreatedAt: now, UpdatedAt: now,
	}, nil
}
