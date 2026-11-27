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
	ID                    uuid.UUID  `db:"id"`
	OrganizationID        uuid.UUID  `db:"organization_id"`
	Key                   string     `db:"key"`
	Name                  string     `db:"name"`
	Description           string     `db:"description"`
	Type                  string     `db:"type"`
	Template              string     `db:"template"`
	LeadID                uuid.UUID  `db:"lead_id"`
	Visibility            string     `db:"visibility"`
	Color                 string     `db:"color"`
	Icon                  string     `db:"icon"`
	DefaultAssigneePolicy string     `db:"default_assignee_policy"`
	NextIssueNumber       int        `db:"next_issue_number"`
	ArchivedAt            *time.Time `db:"archived_at"`
	DeletedAt             *time.Time `db:"deleted_at"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
}

// projectColumns is the explicit column list of every project read (no SELECT *).
const projectColumns = `id, organization_id, key, name, description, type, template, lead_id, visibility,
	color, icon, default_assignee_policy, next_issue_number, archived_at, deleted_at, created_at, updated_at`

func (r projectRow) toDomain() *domain.Project {
	visibility := domain.Visibility(r.Visibility)
	if visibility == storedOpen {
		visibility = domain.VisibilityOpen
	}
	return &domain.Project{
		ID: r.ID, OrganizationID: r.OrganizationID, Key: r.Key, Name: r.Name, Description: r.Description,
		Type: domain.ProjectType(r.Type), Template: domain.Template(r.Template),
		LeadID: r.LeadID, Visibility: visibility,
		Color: r.Color, Icon: r.Icon, DefaultAssigneePolicy: domain.AssigneePolicy(r.DefaultAssigneePolicy),
		NextIssueNumber: r.NextIssueNumber,
		ArchivedAt:      r.ArchivedAt, DeletedAt: r.DeletedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromDomain(p *domain.Project) projectRow {
	visibility := string(p.Visibility)
	if p.Visibility == domain.VisibilityOpen {
		visibility = storedOpen
	}
	return projectRow{
		ID: p.ID, OrganizationID: p.OrganizationID, Key: p.Key, Name: p.Name, Description: p.Description,
		Type: string(p.Type), Template: string(p.Template),
		LeadID: p.LeadID, Visibility: visibility,
		Color: p.Color, Icon: p.Icon, DefaultAssigneePolicy: string(p.DefaultAssigneePolicy),
		NextIssueNumber: p.NextIssueNumber,
		ArchivedAt:      p.ArchivedAt, DeletedAt: p.DeletedAt,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
