package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// projectBrowsable is the project visibility rule, read-only, with $1 the user and p the project row.
// It mirrors the rule used for favorites: organization owners and admins, open projects for organization
// members, explicit project members, members of granted teams, and platform administrators.
const projectBrowsable = `
	(
	    EXISTS (SELECT 1 FROM users u WHERE u.id = $1 AND u.role = 'admin')
	 OR EXISTS (SELECT 1 FROM organization_members om
	            WHERE om.organization_id = p.organization_id AND om.user_id = $1
	              AND om.role IN ('owner', 'admin'))
	 OR (p.visibility = 'open' AND EXISTS (
	            SELECT 1 FROM organization_members om
	            WHERE om.organization_id = p.organization_id AND om.user_id = $1
	              AND om.role = 'member'))
	 OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = $1)
	 OR EXISTS (SELECT 1 FROM project_team_roles ptr
	            JOIN team_members tm ON tm.team_id = ptr.team_id
	            WHERE ptr.project_id = p.id AND tm.user_id = $1)
	)`

// LinkAccess answers browse questions for note links with read-only queries over projects and issues.
// Deleted projects are never browsable, and an issue is browsable when its project is.
type LinkAccess struct {
	db *sqlx.DB
}

var _ port.LinkAccess = (*LinkAccess)(nil)

func NewLinkAccess(db *sqlx.DB) *LinkAccess {
	return &LinkAccess{db: db}
}

func (a *LinkAccess) CanBrowseProject(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	var ok bool
	if err := sqlx.GetContext(ctx, database.Executor(ctx, a.db), &ok, `
		SELECT EXISTS (
		    SELECT 1 FROM projects p
		    WHERE p.id = $2 AND p.deleted_at IS NULL AND `+projectBrowsable+`
		)`, userID.String(), projectID.String()); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check project access", err)
	}
	return ok, nil
}

func (a *LinkAccess) CanBrowseIssue(ctx context.Context, userID, issueID uuid.UUID) (bool, error) {
	var ok bool
	if err := sqlx.GetContext(ctx, database.Executor(ctx, a.db), &ok, `
		SELECT EXISTS (
		    SELECT 1 FROM issues i
		    JOIN projects p ON p.id = i.project_id
		    WHERE i.id = $2 AND p.deleted_at IS NULL AND `+projectBrowsable+`
		)`, userID.String(), issueID.String()); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check issue access", err)
	}
	return ok, nil
}
