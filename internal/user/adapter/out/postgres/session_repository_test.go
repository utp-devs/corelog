package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// TestSessionRepositoryPostgres valida el ciclo de vida de las sesiones contra PostgreSQL: creacion,
// revocacion individual, cerrar las demas, vencimiento y desactivacion de la cuenta.
func TestSessionRepositoryPostgres(t *testing.T) {
	db := testdb.New(t)

	ctx := context.Background()
	users := NewUserRepository(db)
	sessions := NewSessionRepository(db)

	user, err := domain.NewUser("Ana Prueba", "ana@example.com", "hash", domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.Save(ctx, user); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	newSession := func(expires time.Time) uuid.UUID {
		t.Helper()
		s := &domain.Session{ID: uuid.New(), UserID: user.ID, UserAgent: "navegador", CreatedAt: now, LastSeenAt: now, ExpiresAt: expires}
		if err := sessions.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	current := newSession(now.Add(time.Hour))
	other := newSession(now.Add(time.Hour))
	expired := newSession(now.Add(-time.Minute))

	if err := sessions.Validate(ctx, current, user.ID); err != nil {
		t.Fatalf("active session must validate: %v", err)
	}
	if err := sessions.Validate(ctx, expired, user.ID); err == nil {
		t.Fatal("expired session must not validate")
	}
	if err := sessions.Validate(ctx, current, uuid.New()); err == nil {
		t.Fatal("session must not validate for another user")
	}

	active, err := sessions.ListActive(ctx, user.ID, time.Now().UTC())
	if err != nil || len(active) != 2 {
		t.Fatalf("ListActive = %d sessions, %v; want 2 (expired excluded)", len(active), err)
	}

	if err := sessions.RevokeExcept(ctx, user.ID, &current, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := sessions.Validate(ctx, other, user.ID); err == nil {
		t.Fatal("session revoked by RevokeExcept must not validate")
	}
	if err := sessions.Validate(ctx, current, user.ID); err != nil {
		t.Fatalf("kept session must still validate: %v", err)
	}

	if err := sessions.Revoke(ctx, uuid.New(), current, time.Now().UTC()); err != domain.ErrSessionNotFound {
		t.Fatalf("revoking another user's session: want ErrSessionNotFound, got %v", err)
	}
	if err := sessions.Revoke(ctx, user.ID, current, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// Desactivar la cuenta persiste la fecha y invalida cualquier sesion, incluso una recien creada.
	fresh := newSession(now.Add(time.Hour))
	user.Deactivate(time.Now())
	if err := users.Update(ctx, user); err != nil {
		t.Fatal(err)
	}
	stored, err := users.FindByID(ctx, user.ID)
	if err != nil || stored.IsActive() {
		t.Fatalf("deactivated user must be persisted as inactive, got active=%v err=%v", stored != nil && stored.IsActive(), err)
	}
	if err := sessions.Validate(ctx, fresh, user.ID); err == nil {
		t.Fatal("session of a deactivated account must not validate")
	}
}
