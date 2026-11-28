package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func insertTokenUser(t *testing.T, db *sqlx.DB, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := db.Get(&id, `INSERT INTO users (name, display_name, email, password_hash, role) VALUES ('Ana', 'Ana', $1, 'x', 'user') RETURNING id`, email); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTokenStoreLifecycle(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	store := NewPasswordResetTokens(db)
	user := insertTokenUser(t, db, "tokens@example.com")
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	_, h1, _ := domain.NewToken()
	_, h2, _ := domain.NewToken()
	if err := store.Replace(ctx, user, h1, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	// A new request replaces the active token: the old hash stops working.
	if err := store.Replace(ctx, user, h2, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.Consume(ctx, h1, now); ok {
		t.Fatal("replaced token must not be consumable")
	}
	if id, ok, err := store.Find(ctx, h2, now); err != nil || !ok || id != user {
		t.Fatalf("Find = %v %v %v", id, ok, err)
	}
	if id, ok, err := store.Consume(ctx, h2, now); err != nil || !ok || id != user {
		t.Fatalf("Consume = %v %v %v", id, ok, err)
	}
	if _, ok, _ := store.Consume(ctx, h2, now); ok {
		t.Fatal("a token is single use")
	}
	// Expired tokens are rejected and can be purged.
	_, h3, _ := domain.NewToken()
	if err := store.Replace(ctx, user, h3, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.Consume(ctx, h3, now.Add(2*time.Hour)); ok {
		t.Fatal("expired token consumed")
	}
	if n, err := store.PurgeExpired(ctx, now.Add(3*time.Hour)); err != nil || n < 2 {
		t.Fatalf("purged %d, %v", n, err)
	}
	// Tokens of the verification table are independent.
	if _, ok, _ := NewEmailVerificationTokens(db).Find(ctx, h3, now); ok {
		t.Fatal("tables must be independent")
	}
}

func TestLockoutRepositoryPersistsState(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewLockoutRepository(db)
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	if s, err := repo.Get(ctx, "k"); err != nil || s.Failures != 0 || s.LockedUntil != nil {
		t.Fatalf("unknown key = %+v, %v", s, err)
	}
	until := now.Add(15 * time.Minute)
	state := domain.LockoutState{Failures: 0, LockCount: 1, LockedUntil: &until}
	if err := repo.Save(ctx, "k", state, now); err != nil {
		t.Fatal(err)
	}
	// A fresh repository over the same database sees the lock (it survives a restart).
	got, err := NewLockoutRepository(db).Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if locked, _ := got.Locked(now.Add(time.Minute)); !locked || got.LockCount != 1 {
		t.Fatalf("state after reload = %+v", got)
	}
	if n, _ := repo.PurgeStale(ctx, now.Add(time.Minute)); n != 0 {
		t.Fatal("a locked account must not be purged")
	}
	if n, _ := repo.PurgeStale(ctx, now.Add(time.Hour)); n != 1 {
		t.Fatalf("expired lock not purged: %d", n)
	}
	if err := repo.Save(ctx, "k2", domain.LockoutState{Failures: 2}, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, "k2"); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Get(ctx, "k2"); s.Failures != 0 {
		t.Fatal("Delete must clear the counter")
	}
}

func TestSecurityMigrationsRoundTrip(t *testing.T) {
	db := testdb.New(t)
	dir := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres")
	run := func(name string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	exists := func(table string) bool {
		var n int
		if err := db.Get(&n, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1`, table); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	run("000051_login_lockouts.down.sql")
	run("000050_security_tokens_audit.down.sql")
	if exists("account_lockouts") || exists("password_reset_tokens") || exists("email_verification_tokens") {
		t.Fatal("down migrations left tables behind")
	}
	run("000050_security_tokens_audit.up.sql")
	run("000051_login_lockouts.up.sql")
	run("000051_login_lockouts.up.sql") // idempotent
	if !exists("account_lockouts") || !exists("password_reset_tokens") || !exists("email_verification_tokens") {
		t.Fatal("up migrations did not recreate the tables")
	}
}
