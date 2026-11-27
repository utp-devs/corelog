package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// projectBrowseChecker adapts the project authorizer to the check that user field values need: the named
// user must be able to view the project. The check runs without a global role, so a platform administrator
// who is not a member of the project is reported as unable to browse it.
type projectBrowseChecker struct {
	authorizer projectAuthorizerAdapter
}

func (c projectBrowseChecker) CanBrowse(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	err := c.authorizer.Authorize(ctx, "view_project", userID, "", projectID)
	if err == nil {
		return true, nil
	}
	switch apperror.KindOf(err) {
	case apperror.KindForbidden, apperror.KindNotFound:
		return false, nil
	default:
		return false, err
	}
}
