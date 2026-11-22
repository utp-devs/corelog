package sqlite_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	usersqlite "github.com/jhonsferg/corelog/internal/user/adapter/out/sqlite"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func TestSQLiteUserRepository(t *testing.T) {
	ctx := context.Background()
	dbName := "test_user_repo_" + uuid.New().String()
	dbPath := filepath.Join("data", dbName+".db")
	defer func() { _ = os.Remove(dbPath) }()

	cfg := config.DatabaseConfig{
		Name:            dbName,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := database.NewSQLite(cfg)
	if err != nil {
		t.Fatalf("failed to initialize sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := usersqlite.NewUserRepository(db)

	user, err := domain.NewUser("Ada Lovelace", "ada@example.com", "$2a$10$hash", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("failed to create user domain object: %v", err)
	}

	t.Run("Save, FindByID, and FindByEmail", func(t *testing.T) {
		err := repo.Save(ctx, user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		found, err := repo.FindByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("failed to find user by ID: %v", err)
		}
		if found.ID != user.ID {
			t.Errorf("got ID %v, want %v", found.ID, user.ID)
		}
		if found.Email != "ada@example.com" {
			t.Errorf("got email %s, want 'ada@example.com'", found.Email)
		}

		byEmail, err := repo.FindByEmail(ctx, "ada@example.com")
		if err != nil {
			t.Fatalf("failed to find user by email: %v", err)
		}
		if byEmail.ID != user.ID {
			t.Errorf("got ID %v, want %v", byEmail.ID, user.ID)
		}
	})

	t.Run("Count returns total users", func(t *testing.T) {
		count, err := repo.Count(ctx)
		if err != nil {
			t.Fatalf("failed to count users: %v", err)
		}
		if count != 1 {
			t.Errorf("got count %d, want 1", count)
		}
	})

	t.Run("Update modifies user fields", func(t *testing.T) {
		_ = user.UpdateProfile("Ada Byron", "ada.byron@example.com")
		err := repo.Update(ctx, user)
		if err != nil {
			t.Fatalf("failed to update user: %v", err)
		}

		found, err := repo.FindByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("failed to find updated user: %v", err)
		}
		if found.Name != "Ada Byron" {
			t.Errorf("got name %s, want 'Ada Byron'", found.Name)
		}
	})

	t.Run("Search filters users by query", func(t *testing.T) {
		res, err := repo.Search(ctx, userport.SearchFilter{Query: "byron"})
		if err != nil {
			t.Fatalf("failed to search users: %v", err)
		}
		if len(res) != 1 {
			t.Errorf("got %d users, want 1", len(res))
		}
	})

	t.Run("FindByID returns ErrUserNotFound for missing ID", func(t *testing.T) {
		_, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}
