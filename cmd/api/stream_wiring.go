package main

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification"
	notificationstream "github.com/jhonsferg/corelog/internal/notification/adapter/in/stream"
	notificationpostgres "github.com/jhonsferg/corelog/internal/notification/adapter/out/postgres"
	notificationusecase "github.com/jhonsferg/corelog/internal/notification/application/usecase"
	notificationdomain "github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/sse"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
)

// streamRuntime is the live notification channel of the process: the hub of open streams and the bus that
// connects the API instances.
type streamRuntime struct {
	hub *sse.Hub
	bus *sse.Bus
}

// currentStream is set by buildRouter; main starts its listener and closes its hub on shutdown.
var currentStream atomic.Pointer[streamRuntime]

// wireStream mounts GET /notifications/stream, pushes every saved notification through the bus, raises the
// new login alert and closes the streams of revoked sessions.
func wireStream(cfg *config.Config, db *sqlx.DB, log *slog.Logger, m *notification.Module, users *userhttp.Handler) {
	hub := sse.NewHub(sse.Options{MaxPerUser: 5, Buffer: 32})
	bus := sse.NewBus(hub, db, cfg.Database.DSN(), sse.DefaultChannel, log)
	rt := &streamRuntime{hub: hub, bus: bus}
	currentStream.Store(rt)

	notes := notificationpostgres.NewNotificationRepository(db)
	// Single choke point: every saved in-app notification is announced after its transaction commits.
	notificationpostgres.SetSaveHook(db, func(ctx context.Context, n *notificationdomain.Notification) {
		bus.Publish(ctx, sse.Signal{UserID: n.UserID, Kind: sse.KindNotification, ID: n.ID.String()})
	})
	m.Handler.WithStream(notificationstream.NewHandler(hub, notes, notificationstream.Options{
		Validate: userpostgres.NewSessionRepository(db).Validate,
		Logger:   log,
	}))

	alerts := notificationusecase.NewSecurityAlertService(notes, notificationpostgres.NewFingerprintRepository(db), clock.RealClock{}).
		WithEmail(notificationpostgres.NewFactsRepository(db), notificationpostgres.NewOutboxRepository(db), cfg.Security.AppPublicURL)
	users.WithSessionEvents(sessionEvents{alerts: alerts, bus: bus, log: log})
}

// sessionEvents adapts the security notifications to the port of the user module.
type sessionEvents struct {
	alerts *notificationusecase.SecurityAlertService
	bus    *sse.Bus
	log    *slog.Logger
}

var _ userport.SessionEvents = sessionEvents{}

func (s sessionEvents) NewLogin(ctx context.Context, info userport.LoginInfo) {
	err := s.alerts.NewLogin(ctx, notificationdomain.NewLogin{
		UserID: info.UserID, SessionID: info.SessionID, UserAgent: info.UserAgent, IP: info.IP,
	})
	if err != nil {
		s.log.Warn("login alert failed", "error", err)
	}
}

func (s sessionEvents) SessionRevoked(ctx context.Context, userID, sessionID uuid.UUID) {
	s.bus.Publish(ctx, sse.Signal{UserID: userID, Kind: sse.KindSessionRevoked, ID: sessionID.String()})
}

func (s sessionEvents) OthersRevoked(ctx context.Context, userID, keepID uuid.UUID) {
	id := ""
	if keepID != uuid.Nil {
		id = keepID.String()
	}
	s.bus.Publish(ctx, sse.Signal{UserID: userID, Kind: sse.KindSessionsRevokedExcept, ID: id})
}
