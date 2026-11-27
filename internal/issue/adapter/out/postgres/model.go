package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// issueColumns is the column list every issue read selects. The project key comes from the project row and
// the catalog references from the catalog rows, through scalar subqueries that read by primary key.
const issueColumns = `id, project_id, summary, description, status, priority, issue_type, parent_id,
		       due_date, story_points, rank, closed_at, reporter_id, assignee_id, key_number,
		       (SELECT p.key FROM projects p WHERE p.id = issues.project_id) AS project_key, created_at, updated_at,
		       issue_type_id, priority_id, resolution_id, resolved_at, custom,
		       (SELECT it.name FROM issue_types it WHERE it.id = issues.issue_type_id) AS issue_type_name,
		       (SELECT it.icon FROM issue_types it WHERE it.id = issues.issue_type_id) AS issue_type_icon,
		       (SELECT it.color FROM issue_types it WHERE it.id = issues.issue_type_id) AS issue_type_color,
		       (SELECT it.hierarchy_level FROM issue_types it WHERE it.id = issues.issue_type_id) AS issue_type_level,
		       (SELECT pr.name FROM priorities pr WHERE pr.id = issues.priority_id) AS priority_name,
		       (SELECT pr.icon FROM priorities pr WHERE pr.id = issues.priority_id) AS priority_icon,
		       (SELECT pr.color FROM priorities pr WHERE pr.id = issues.priority_id) AS priority_color,
		       (SELECT rs.name FROM resolutions rs WHERE rs.id = issues.resolution_id) AS resolution_name`

type issueRow struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Summary     string     `db:"summary"`
	Description string     `db:"description"`
	Status      string     `db:"status"`
	Priority    string     `db:"priority"`
	IssueType   string     `db:"issue_type"`
	ParentID    *uuid.UUID `db:"parent_id"`
	DueDate     *time.Time `db:"due_date"`
	StoryPoints *float64   `db:"story_points"`
	Rank        string     `db:"rank"`
	ClosedAt    *time.Time `db:"closed_at"`
	ReporterID  uuid.UUID  `db:"reporter_id"`
	KeyNumber   int        `db:"key_number"`
	ProjectKey  string     `db:"project_key"`
	AssigneeID  *uuid.UUID `db:"assignee_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`

	IssueTypeID    uuid.UUID  `db:"issue_type_id"`
	PriorityID     uuid.UUID  `db:"priority_id"`
	ResolutionID   *uuid.UUID `db:"resolution_id"`
	ResolvedAt     *time.Time `db:"resolved_at"`
	Custom         []byte     `db:"custom"`
	IssueTypeName  *string    `db:"issue_type_name"`
	IssueTypeIcon  *string    `db:"issue_type_icon"`
	IssueTypeColor *string    `db:"issue_type_color"`
	IssueTypeLevel *int       `db:"issue_type_level"`
	PriorityName   *string    `db:"priority_name"`
	PriorityIcon   *string    `db:"priority_icon"`
	PriorityColor  *string    `db:"priority_color"`
	ResolutionName *string    `db:"resolution_name"`
}

func (r issueRow) toDomain() *domain.Issue {
	issue := &domain.Issue{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		Summary:      r.Summary,
		Description:  r.Description,
		Status:       domain.Status(r.Status),
		Priority:     domain.Priority(r.Priority),
		IssueType:    domain.IssueType(r.IssueType),
		ParentID:     r.ParentID,
		DueDate:      r.DueDate,
		StoryPoints:  r.StoryPoints,
		Rank:         r.Rank,
		ClosedAt:     r.ClosedAt,
		ReporterID:   r.ReporterID,
		KeyNumber:    r.KeyNumber,
		ProjectKey:   r.ProjectKey,
		AssigneeID:   r.AssigneeID,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
		IssueTypeID:  r.IssueTypeID,
		PriorityID:   r.PriorityID,
		ResolutionID: r.ResolutionID,
		ResolvedAt:   r.ResolvedAt,
		Custom:       decodeCustom(r.Custom),
	}
	if r.IssueTypeName != nil {
		issue.IssueTypeRef = &domain.CatalogRef{
			ID: r.IssueTypeID, Name: *r.IssueTypeName, Icon: deref(r.IssueTypeIcon),
			Color: deref(r.IssueTypeColor), HierarchyLevel: derefInt(r.IssueTypeLevel),
		}
	}
	if r.PriorityName != nil {
		issue.PriorityRef = &domain.CatalogRef{
			ID: r.PriorityID, Name: *r.PriorityName, Icon: deref(r.PriorityIcon), Color: deref(r.PriorityColor),
		}
	}
	if r.ResolutionID != nil && r.ResolutionName != nil {
		issue.ResolutionRef = &domain.CatalogRef{ID: *r.ResolutionID, Name: *r.ResolutionName}
	}
	return issue
}

func fromDomain(t *domain.Issue) issueRow {
	return issueRow{
		ID:           t.ID,
		ProjectID:    t.ProjectID,
		Summary:      t.Summary,
		Description:  t.Description,
		Status:       string(t.Status),
		Priority:     string(t.Priority),
		IssueType:    string(t.IssueType),
		ParentID:     t.ParentID,
		DueDate:      t.DueDate,
		StoryPoints:  t.StoryPoints,
		Rank:         t.Rank,
		ClosedAt:     t.ClosedAt,
		ReporterID:   t.ReporterID,
		KeyNumber:    t.KeyNumber,
		ProjectKey:   t.ProjectKey,
		AssigneeID:   t.AssigneeID,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
		IssueTypeID:  t.IssueTypeID,
		PriorityID:   t.PriorityID,
		ResolutionID: t.ResolutionID,
		ResolvedAt:   t.ResolvedAt,
		Custom:       encodeCustom(t.Custom),
	}
}

// decodeCustom reads the custom values column. The column is a JSON object by constraint; anything else reads
// as no values, so a corrupt row cannot break the issue read path.
func decodeCustom(raw []byte) map[string]any {
	values := map[string]any{}
	if len(raw) == 0 {
		return values
	}
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return map[string]any{}
	}
	return values
}

// encodeCustom writes the custom values as a JSON object. The values were normalized by the use cases, so
// marshalling them cannot fail; a nil map is stored as an empty object.
func encodeCustom(values map[string]any) []byte {
	if len(values) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
