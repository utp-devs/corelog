package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// commentLevelOf ranks the user in the project for comment visibility: lead when it manages comments, contributor
// when it can comment, viewer when it can browse, none otherwise. A nil checker grants nothing.
func commentLevelOf(ctx context.Context, perms port.CommentPermissions, userID uuid.UUID, globalRole string, projectID uuid.UUID) (domain.CommentLevel, error) {
	if perms == nil {
		return domain.CommentLevelNone, nil
	}
	browse, err := perms.Can(ctx, userID, globalRole, projectID, port.PermissionBrowseProject)
	if err != nil || !browse {
		return domain.CommentLevelNone, err
	}
	manage, err := perms.Can(ctx, userID, globalRole, projectID, port.PermissionManageComments)
	if err != nil {
		return domain.CommentLevelNone, err
	}
	if manage {
		return domain.CommentLevelLead, nil
	}
	comment, err := perms.Can(ctx, userID, globalRole, projectID, port.PermissionCommentIssue)
	if err != nil {
		return domain.CommentLevelNone, err
	}
	if comment {
		return domain.CommentLevelContributor, nil
	}
	return domain.CommentLevelViewer, nil
}

// commentIssueKey renders the key of an issue for notifications, for example CL-12.
func commentIssueKey(projectKey string, number int) string {
	if projectKey == "" || number <= 0 {
		return ""
	}
	return fmt.Sprintf("%s-%d", projectKey, number)
}
