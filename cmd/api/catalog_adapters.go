package main

import (
	"context"

	"github.com/google/uuid"

	orgport "github.com/jhonsferg/corelog/internal/organization/application/port"
	orgdomain "github.com/jhonsferg/corelog/internal/organization/domain"
)

// orgCatalogRoleChecker adapts the organization service to the role check the catalog use cases need.
type orgCatalogRoleChecker struct {
	svc orgport.Service
}

func (c orgCatalogRoleChecker) RequireOrgRole(ctx context.Context, userID, orgID uuid.UUID, min string) error {
	_, err := c.svc.RequireRole(ctx, userID, orgID, orgdomain.Role(min))
	return err
}
