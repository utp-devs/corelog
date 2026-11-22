package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

// TestSessionRepositoryPostgres valida el ciclo de vida de las sesiones contra PostgreSQL: creacion,
// revocacion individual, cerrar las demas, vencimiento y desactivacion de la cuenta.
func TestSessionRepositoryPostgres(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	schema := "session_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(120) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role VARCHAR(20) NOT NULL,
			team_id UUID,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			deactivated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE team_members (team_id UUID NOT NULL, user_id UUID NOT NULL, role VARCHAR(20) NOT NULL DEFAULT 'member', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (team_id, user_id))`,
		`CREATE TABLE user_sessions (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			user_agent VARCHAR(300) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	users := NewUserRepository(db)
	sessions := NewSessionRepository(db)

	user, err := domain.NewUser("Ana Prueba", "ana@example.com", "hash", domain.RoleRequester)
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
