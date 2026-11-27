package main

import (
	"context"

	"github.com/google/uuid"

	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
)

// projectKeyResolver resolves a project key for the workflow handler through the project module, so key lookups
// follow the same visibility rules as the project routes.
type projectKeyResolver struct {
	projects projectport.ProjectManager
}

func (p projectKeyResolver) ResolveProjectKey(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (uuid.UUID, error) {
	project, err := p.projects.FindByKey(ctx, actorID, globalRole, orgID, key)
	if err != nil {
		return uuid.Nil, err
	}
	return project.ID, nil
}
