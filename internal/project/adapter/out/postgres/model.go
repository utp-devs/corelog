package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

type projectRow struct {
	ID          uuid.UUID  `db:"id"`
	Key         string     `db:"key"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	Type        string     `db:"type"`
	LeadID      uuid.UUID  `db:"lead_id"`
	Visibility  string     `db:"visibility"`
	ArchivedAt  *time.Time `db:"archived_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (r projectRow) toDomain() *domain.Project {
	return &domain.Project{
		ID: r.ID, Key: r.Key, Name: r.Name, Description: r.Description,
		Type: domain.ProjectType(r.Type), LeadID: r.LeadID, Visibility: domain.Visibility(r.Visibility),
		ArchivedAt: r.ArchivedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromDomain(p *domain.Project) projectRow {
	return projectRow{
		ID: p.ID, Key: p.Key, Name: p.Name, Description: p.Description,
		Type: string(p.Type), LeadID: p.LeadID, Visibility: string(p.Visibility),
		ArchivedAt: p.ArchivedAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
