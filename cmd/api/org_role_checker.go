package main

import (
	"context"

	"github.com/google/uuid"

	orgport "github.com/jhonsferg/corelog/internal/organization/application/port"
	orgdomain "github.com/jhonsferg/corelog/internal/organization/domain"
)

// orgRoleChecker adapts the organization service to the narrow role check
// the logo handler needs.
type orgRoleChecker struct {
	svc orgport.Service
}

func (c orgRoleChecker) RequireRole(ctx context.Context, userID, orgID uuid.UUID, min string) error {
	_, err := c.svc.RequireRole(ctx, userID, orgID, orgdomain.Role(min))
	return err
}
