package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// ProjectVisibility applies the project visibility rules to favorites and recents with one read-only query.
// A user sees a live project when one of these holds:
//   - owner or admin of the organization;
//   - member of the organization and the project is open;
//   - explicit project member;
//   - member of a team granted on the project.
//
// Deleted projects are never visible. Platform administrators without an organization role are not
// covered by this query.
type ProjectVisibility struct {
	db *sqlx.DB
}

var _ port.EntityVisibility = (*ProjectVisibility)(nil)

func NewProjectVisibility(db *sqlx.DB) *ProjectVisibility {
	return &ProjectVisibility{db: db}
}

func (v *ProjectVisibility) CanView(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	var visible bool
	if err := sqlx.GetContext(ctx, database.Executor(ctx, v.db), &visible, `
		SELECT EXISTS (
		    SELECT 1 FROM projects p
		    WHERE p.id = $2
		      AND p.deleted_at IS NULL
		      AND (
		          EXISTS (SELECT 1 FROM organization_members om
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
		      ))`,
		userID, projectID,
	); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check project visibility", err)
	}
	return visible, nil
}
