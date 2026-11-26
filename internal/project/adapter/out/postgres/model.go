package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

// storedOpen is the stored spelling of
// domain.VisibilityOpen (migration 000025 widened the CHECK from private|team to private|open).
const storedOpen = "open"

type projectRow struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	Key            string     `db:"key"`
	Name           string     `db:"name"`
	Description    string     `db:"description"`
	Type           string     `db:"type"`
	LeadID         uuid.UUID  `db:"lead_id"`
	Visibility     string     `db:"visibility"`
	ArchivedAt     *time.Time `db:"archived_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// projectColumns is the explicit column list of every project read (no SELECT *).
const projectColumns = `id, organization_id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at`

func (r projectRow) toDomain() *domain.Project {
	visibility := domain.Visibility(r.Visibility)
	if visibility == storedOpen {
		visibility = domain.VisibilityOpen
	}
	return &domain.Project{
		ID: r.ID, OrganizationID: r.OrganizationID, Key: r.Key, Name: r.Name, Description: r.Description,
		Type: domain.ProjectType(r.Type), LeadID: r.LeadID, Visibility: visibility,
		ArchivedAt: r.ArchivedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromDomain(p *domain.Project) projectRow {
	visibility := string(p.Visibility)
	if p.Visibility == domain.VisibilityOpen {
		visibility = storedOpen
	}
	return projectRow{
		ID: p.ID, OrganizationID: p.OrganizationID, Key: p.Key, Name: p.Name, Description: p.Description,
		Type: string(p.Type), LeadID: p.LeadID, Visibility: visibility,
		ArchivedAt: p.ArchivedAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
