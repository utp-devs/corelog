package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var ErrInvalidCatalogItem = apperror.New(apperror.KindValidation, "name is required")

type Label struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Color     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewLabel(projectID uuid.UUID, name, color string) (*Label, error) {
	name = strings.TrimSpace(name)
	if projectID == uuid.Nil || name == "" || len(name) > 40 {
		return nil, ErrInvalidCatalogItem
	}
	color = strings.TrimSpace(color)
	if color == "" {
		color = "blue"
	}
	now := time.Now().UTC()
	return &Label{ProjectID: projectID, Name: name, Color: color, CreatedAt: now, UpdatedAt: now}, nil
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
