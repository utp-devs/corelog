package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// DashboardReader calcula el resumen con consultas acotadas al usuario.
type DashboardReader struct {
	db *sqlx.DB
}

func NewDashboardReader(db *sqlx.DB) *DashboardReader {
	return &DashboardReader{db: db}
}

var _ port.Reader = (*DashboardReader)(nil)

type statusCount struct {
	Status string `db:"status"`
	Count  int    `db:"count"`
}

type sprintRow struct {
	ID         uuid.UUID  `db:"id"`
	Name       string     `db:"name"`
	ProjectKey string     `db:"project_key"`
	EndDate    *time.Time `db:"end_date"`
}

type issueRow struct {
	ID         uuid.UUID  `db:"id"`
	Title      string     `db:"title"`
	Status     string     `db:"status"`
	Priority   string     `db:"priority"`
	ProjectKey string     `db:"project_key"`
	DueDate    *time.Time `db:"due_date"`
}

func (r *DashboardReader) Summary(ctx context.Context, userID uuid.UUID, now time.Time) (port.Summary, error) {
	summary := port.Summary{AssignedByStatus: map[string]int{}}

	var byStatus []statusCount
	if err := r.db.SelectContext(ctx, &byStatus, `
		SELECT i.status, COUNT(*) AS count FROM issues i
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		WHERE i.assignee_id = $1 GROUP BY i.status`, userID); err != nil {
		return summary, apperror.Wrap(apperror.KindInternal, "failed to count assigned issues", err)
	}
	for _, item := range byStatus {
		summary.AssignedByStatus[item.Status] = item.Count
		if item.Status != "closed" {
			summary.AssignedOpen += item.Count
		}
	}

	if err := r.db.GetContext(ctx, &summary.Overdue, `
		SELECT COUNT(*) FROM issues i
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		WHERE i.assignee_id = $1 AND i.status <> 'closed' AND i.due_date < $2`, userID, now); err != nil {
		return summary, apperror.Wrap(apperror.KindInternal, "failed to count overdue issues", err)
	}

	if err := r.db.GetContext(ctx, &summary.ProjectCount, `
		SELECT COUNT(*) FROM project_members pm
		JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL
		WHERE pm.user_id = $1`, userID); err != nil {
		return summary, apperror.Wrap(apperror.KindInternal, "failed to count projects", err)
	}

	var sprints []sprintRow
	if err := r.db.SelectContext(ctx, &sprints, `
		SELECT s.id, s.name, p.key AS project_key, s.end_date
		FROM sprints s
		JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL
		JOIN project_members pm ON pm.project_id = s.project_id AND pm.user_id = $1
		WHERE s.status = 'active'
		ORDER BY s.end_date ASC NULLS LAST, s.id
		LIMIT 10`, userID); err != nil {
		return summary, apperror.Wrap(apperror.KindInternal, "failed to list active sprints", err)
	}
	for _, sprint := range sprints {
		summary.ActiveSprints = append(summary.ActiveSprints, port.SprintRef{ID: sprint.ID, Name: sprint.Name, ProjectKey: sprint.ProjectKey, EndDate: sprint.EndDate})
	}

	var issues []issueRow
	if err := r.db.SelectContext(ctx, &issues, `
		SELECT i.id, i.summary AS title, i.status, i.priority, p.key AS project_key, i.due_date
		FROM issues i
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		WHERE i.assignee_id = $1 AND i.status <> 'closed'
		ORDER BY i.due_date ASC NULLS LAST, i.updated_at DESC
		LIMIT 5`, userID); err != nil {
		return summary, apperror.Wrap(apperror.KindInternal, "failed to list recent issues", err)
	}
	for _, issue := range issues {
		summary.RecentIssues = append(summary.RecentIssues, port.IssueRef{ID: issue.ID, Title: issue.Title, Status: issue.Status, Priority: issue.Priority, ProjectKey: issue.ProjectKey, DueDate: issue.DueDate})
	}
	return summary, nil
}
