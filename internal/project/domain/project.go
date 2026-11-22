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
	VisibilityPrivate Visibility = "private"
	VisibilityTeam    Visibility = "team"
)

var (
	ErrProjectNotFound = apperror.New(apperror.KindNotFound, "project not found")
	ErrInvalidProject  = apperror.New(apperror.KindValidation, "invalid project")
	ErrProjectKeyTaken = apperror.New(apperror.KindConflict, "a project with this key already exists")
)

type Project struct {
	ID          uuid.UUID
	Key         string
	Name        string
	Description string
	Type        ProjectType
	LeadID      uuid.UUID
	Visibility  Visibility
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UpdateInput struct {
	Name        string
	Description string
	Type        ProjectType
	LeadID      uuid.UUID
	Visibility  Visibility
}

func NewProject(key, name, description string, projectType ProjectType, leadID uuid.UUID, visibility Visibility) (*Project, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	name = strings.TrimSpace(name)

	if key == "" || len(key) > 20 || name == "" || len(name) > 120 {
		return nil, ErrInvalidProject
	}
	if projectType != ProjectTypeKanban && projectType != ProjectTypeScrum {
		return nil, ErrInvalidProject
	}
	if visibility != VisibilityPrivate && visibility != VisibilityTeam {
		return nil, ErrInvalidProject
	}
	if leadID == uuid.Nil {
		return nil, ErrInvalidProject
	}

	now := time.Now().UTC()
	return &Project{
		ID:          uuid.New(),
		Key:         key,
		Name:        name,
		Description: strings.TrimSpace(description),
		Type:        projectType,
		LeadID:      leadID,
		Visibility:  visibility,
		CreatedAt:   now,
		UpdatedAt:   now,
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
	if input.Visibility != VisibilityPrivate && input.Visibility != VisibilityTeam {
		return ErrInvalidProject
	}
	if input.LeadID == uuid.Nil {
		return ErrInvalidProject
	}

	p.Name = input.Name
	p.Description = strings.TrimSpace(input.Description)
	p.Type = input.Type
	p.LeadID = input.LeadID
	p.Visibility = input.Visibility
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *Project) Archive() {
	now := time.Now().UTC()
	p.ArchivedAt = &now
	p.UpdatedAt = now
}
