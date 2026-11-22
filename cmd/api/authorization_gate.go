package main

import (
	"context"

	"github.com/google/uuid"

	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	boardport "github.com/jhonsferg/corelog/internal/board/application/port"
	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
	ticketport "github.com/jhonsferg/corelog/internal/ticket/application/port"
	ticketdomain "github.com/jhonsferg/corelog/internal/ticket/domain"
)

// projectAuthorizerAdapter adapta el caso de uso de autorizacion al puerto ProjectAuthorizer que
// declaran los modulos ticket y project. Vive en el root de composicion, el unico sitio que conoce ambos hexagonos.
type projectAuthorizerAdapter struct {
	authorizer *authusecase.Authorizer
}

var (
	_ ticketport.ProjectAuthorizer  = projectAuthorizerAdapter{}
	_ projectport.ProjectAuthorizer = projectAuthorizerAdapter{}
	_ boardport.Authorizer          = projectAuthorizerAdapter{}
)

func (a projectAuthorizerAdapter) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return a.authorizer.Authorize(ctx, authport.Action(action), actorID, globalRole, projectID)
}

// boardIssueMover adapta el servicio de tickets al puerto IssueMover del tablero.
type boardIssueMover struct {
	issues ticketport.Service
}

var _ boardport.IssueMover = boardIssueMover{}

func (m boardIssueMover) MoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID uuid.UUID, status string) error {
	return m.issues.MoveIssueToStatus(ctx, actorID, globalRole, projectID, issueID, ticketdomain.Status(status))
}
