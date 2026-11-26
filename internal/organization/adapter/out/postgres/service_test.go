package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/application/usecase"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// newPostgresService wires the real use case to the Postgres repository and the outbox publisher.
func newPostgresService(db *sqlx.DB) *usecase.OrganizationService {
	pub := events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext {
		return database.Executor(ctx, db)
	})
	return usecase.NewOrganizationService(
		postgres.NewRepository(db),
		database.NewTxManager(db),
		pub,
		clock.NewFixed(testNow),
	)
}

func countEvents(t *testing.T, db *sqlx.DB, orgID uuid.UUID, eventType string) int {
	t.Helper()
	var n int
	err := db.GetContext(context.Background(), &n,
		`SELECT COUNT(*) FROM domain_events WHERE org_id = $1 AND type = $2`, orgID, eventType)
	if err != nil {
		t.Fatalf("count %s: %v", eventType, err)
	}
	return n
}

func requireAppErr(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s/%s, got nil", kind, code)
	}
	if apperror.KindOf(err) != kind || apperror.CodeOf(err) != code {
		t.Fatalf("got kind=%s code=%s (%v), want %s/%s", apperror.KindOf(err), apperror.CodeOf(err), err, kind, code)
	}
}

func TestServiceLifecycleThroughPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	svc := newPostgresService(db)
	repo := postgres.NewRepository(db)

	owner := newUser(t, db)
	member := newUser(t, db)

	// Create: org.created and member.added are written to the outbox.
	org, err := svc.CreateOrganization(ctx, owner, port.CreateInput{Name: "Equipo Ejemplo"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if org.Slug != "equipo-ejemplo" || org.ID == uuid.Nil {
		t.Fatalf("unexpected org: %+v", org)
	}
	if n := countEvents(t, db, org.ID, usecase.EventOrgCreated); n != 1 {
		t.Fatalf("org.created events = %d, want 1", n)
	}
	if n := countEvents(t, db, org.ID, usecase.EventMemberAdded); n != 1 {
		t.Fatalf("member.added events = %d, want 1", n)
	}

	// The sole owner cannot leave.
	requireAppErr(t, svc.LeaveOrganization(ctx, owner, org.ID), apperror.KindConflict, "org.last_owner")

	// Add a member through the repository (the service has no invitation flow yet).
	m, err := domain.NewMember(org.ID, member, domain.RoleMember, testNow)
	if err != nil {
		t.Fatalf("new member: %v", err)
	}
	if err := repo.AddMember(ctx, m); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Transfer ownership: the new owner takes the role, the old owner becomes admin.
	if err := svc.TransferOwnership(ctx, owner, org.ID, member); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if role, err := svc.RequireRole(ctx, member, org.ID, domain.RoleOwner); err != nil || role != domain.RoleOwner {
		t.Fatalf("new owner role = %s, %v", role, err)
	}
	if role, err := svc.RequireRole(ctx, owner, org.ID, domain.RoleAdmin); err != nil || role != domain.RoleAdmin {
		t.Fatalf("old owner role = %s, %v", role, err)
	}
	if n := countEvents(t, db, org.ID, usecase.EventMemberRoleChanged); n != 2 {
		t.Fatalf("member.role_changed events = %d, want 2", n)
	}

	// The former owner, now admin, may leave. Then the new owner is the last one.
	if err := svc.LeaveOrganization(ctx, owner, org.ID); err != nil {
		t.Fatalf("leave as admin: %v", err)
	}
	if n := countEvents(t, db, org.ID, usecase.EventMemberRemoved); n != 1 {
		t.Fatalf("member.removed events = %d, want 1", n)
	}
	requireAppErr(t, svc.LeaveOrganization(ctx, member, org.ID), apperror.KindConflict, "org.last_owner")

	// Delete needs the typed slug.
	requireAppErr(t, svc.DeleteOrganization(ctx, member, org.ID, "otro-slug"), apperror.KindValidation, "org.confirmation_mismatch")
	if err := svc.DeleteOrganization(ctx, member, org.ID, org.Slug); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := countEvents(t, db, org.ID, usecase.EventOrgDeleted); n != 1 {
		t.Fatalf("org.deleted events = %d, want 1", n)
	}
	_, err = svc.GetOrganization(ctx, member, org.ID)
	requireAppErr(t, err, apperror.KindNotFound, "org.not_found")

	// The slug stays reserved after deletion.
	_, err = svc.CreateOrganization(ctx, owner, port.CreateInput{Name: "Equipo Ejemplo"})
	requireAppErr(t, err, apperror.KindConflict, "org.slug_taken")
}
