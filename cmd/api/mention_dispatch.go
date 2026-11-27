package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authpostgres "github.com/jhonsferg/corelog/internal/authorization/adapter/out/postgres"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	notificationpostgres "github.com/jhonsferg/corelog/internal/notification/adapter/out/postgres"
	notificationport "github.com/jhonsferg/corelog/internal/notification/application/port"
	notificationusecase "github.com/jhonsferg/corelog/internal/notification/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// mentionBrowser adapts the project permission checks to the browse question of the mention consumer. It asks as a
// plain user, so platform admin support access does not count.
type mentionBrowser struct {
	perms issuePermissionChecker
}

var _ notificationport.ProjectBrowser = mentionBrowser{}

func (b mentionBrowser) CanBrowse(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	return b.perms.Can(ctx, userID, "user", projectID, "browse_project")
}

// startCommentMentionDispatcher delivers comment events to the mention consumer until ctx is done. The consumer
// keeps its own cursor, so restarts resume where they stopped.
func startCommentMentionDispatcher(ctx context.Context, db *sqlx.DB, log *slog.Logger) error {
	authorizer := authusecase.NewAuthorizer(authpostgres.NewRoleLookup(db)).WithSchemes(authpostgres.NewSchemeRepository(db))
	consumer := notificationusecase.NewMentionConsumer(
		notificationusecase.NewNotificationService(notificationpostgres.NewNotificationRepository(db)),
		mentionBrowser{perms: issuePermissionChecker{authorizer: authorizer}},
		notificationpostgres.NewMentionRepository(db),
	)
	dispatcher, err := events.NewDispatcher(db, []events.Consumer{consumer}, events.Options{Logger: log})
	if err != nil {
		return err
	}
	return dispatcher.Run(ctx, time.Second)
}
