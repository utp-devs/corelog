package main

import (
	"context"

	"github.com/google/uuid"

	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
)

// ticketProjectAuthorizer adapta el caso de uso de autorizacion al puerto ProjectAuthorizer que
// declara el modulo ticket. Vive en el root de composicion, el unico sitio que conoce ambos hexagonos.
type ticketProjectAuthorizer struct {
	authorizer *authusecase.Authorizer
}

var _ port.ProjectAuthorizer = ticketProjectAuthorizer{}

func (a ticketProjectAuthorizer) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return a.authorizer.Authorize(ctx, authport.Action(action), actorID, globalRole, projectID)
}
