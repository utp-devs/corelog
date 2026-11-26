package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func insertAvatarTestUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(), `
INSERT INTO users (id, name, display_name, email, password_hash, role)
VALUES ($1, 'Usuario avatar', 'Usuario avatar', $2, 'not-a-real-hash', 'user')`,
		id, id.String()+"@example.test")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func TestAvatarRepositorySetGetClear(t *testing.T) {
	db := testdb.New(t)
	repo := NewAvatarRepository(db)
	ctx := context.Background()
	userID := insertAvatarTestUser(t, db)

	key1 := "11111111111111111111111111111111"
	key2 := "22222222222222222222222222222222"

	got, err := repo.GetAvatarKey(ctx, userID)
	if err != nil || got != nil {
		t.Fatalf("fresh user: key = %v, err = %v; want nil", got, err)
	}

	old, err := repo.SetAvatarKey(ctx, userID, &key1)
	if err != nil || old != nil {
		t.Fatalf("first set: old = %v, err = %v; want nil old", old, err)
	}
	if got, _ := repo.GetAvatarKey(ctx, userID); got == nil || *got != key1 {
		t.Fatalf("after set: key = %v, want %s", got, key1)
	}

	old, err = repo.SetAvatarKey(ctx, userID, &key2)
	if err != nil || old == nil || *old != key1 {
		t.Fatalf("replace: old = %v, err = %v; want %s", old, err, key1)
	}

	old, err = repo.SetAvatarKey(ctx, userID, nil)
	if err != nil || old == nil || *old != key2 {
		t.Fatalf("clear: old = %v, err = %v; want %s", old, err, key2)
	}
	if got, _ := repo.GetAvatarKey(ctx, userID); got != nil {
		t.Fatalf("after clear: key = %v, want nil", *got)
	}

	// The profile row keeps the rest of its data.
	var name string
	if err := db.GetContext(ctx, &name, `SELECT name FROM users WHERE id = $1`, userID); err != nil || name != "Usuario avatar" {
		t.Fatalf("user row changed: name=%q err=%v", name, err)
	}
}

func TestAvatarRepositoryUnknownUser(t *testing.T) {
	db := testdb.New(t)
	repo := NewAvatarRepository(db)
	ctx := context.Background()
	unknown := uuid.New()
	key := "33333333333333333333333333333333"

	if got, err := repo.GetAvatarKey(ctx, unknown); err != nil || got != nil {
		t.Fatalf("get unknown: key = %v, err = %v; want nil, nil", got, err)
	}
	_, err := repo.SetAvatarKey(ctx, unknown, &key)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("set unknown: err = %v, want ErrUserNotFound", err)
	}
}
