package main

import (
	"context"

	"github.com/google/uuid"

	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
)

// issuePermissionChecker adapts the authorizer to the per-permission checks of the issue module.
type issuePermissionChecker struct {
	authorizer *authusecase.Authorizer
}

var _ issueport.IssuePermissionChecker = issuePermissionChecker{}

func (c issuePermissionChecker) Can(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, permission string, holders ...string) (bool, error) {
	special := make([]authdomain.Holder, 0, len(holders))
	for _, holder := range holders {
		special = append(special, authdomain.Holder(holder))
	}
	return c.authorizer.Can(ctx, actorID, globalRole, projectID, authdomain.Permission(permission), special...)
}

func (c issuePermissionChecker) Effective(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]string, error) {
	_, granted, err := c.authorizer.Effective(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(granted))
	for _, permission := range granted {
		names = append(names, string(permission))
	}
	return names, nil
}
