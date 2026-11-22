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
	teamsqlite "github.com/jhonsferg/corelog/internal/team/adapter/out/sqlite"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

func TestSQLiteTeamRepository(t *testing.T) {
	ctx := context.Background()
	dbName := "test_team_repo_" + uuid.New().String()
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

	repo := teamsqlite.NewTeamRepository(db)

	team, err := domain.NewTeam("Engineering")
	if err != nil {
		t.Fatalf("failed to create team domain object: %v", err)
	}

	t.Run("Save and FindByID", func(t *testing.T) {
		err := repo.Save(ctx, team)
		if err != nil {
			t.Fatalf("failed to save team: %v", err)
		}

		found, err := repo.FindByID(ctx, team.ID)
		if err != nil {
			t.Fatalf("failed to find team: %v", err)
		}
		if found.ID != team.ID {
			t.Errorf("got ID %v, want %v", found.ID, team.ID)
		}
		if found.Name != "Engineering" {
			t.Errorf("got name %s, want 'Engineering'", found.Name)
		}
	})

	t.Run("FindByID returns ErrTeamNotFound for missing id", func(t *testing.T) {
		_, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrTeamNotFound) {
			t.Errorf("expected ErrTeamNotFound, got %v", err)
		}
	})

	t.Run("FindByName finds existing team", func(t *testing.T) {
		found, err := repo.FindByName(ctx, "Engineering")
		if err != nil {
			t.Fatalf("failed to find team by name: %v", err)
		}
		if found.ID != team.ID {
			t.Errorf("got ID %v, want %v", found.ID, team.ID)
		}
	})

	t.Run("FindAll returns all saved teams", func(t *testing.T) {
		all, err := repo.FindAll(ctx)
		if err != nil {
			t.Fatalf("failed to list teams: %v", err)
		}
		if len(all) == 0 {
			t.Errorf("expected at least 1 team")
		}
	})

	t.Run("Update modifies team name", func(t *testing.T) {
		_ = team.Rename("Engineering Core")
		err := repo.Update(ctx, team)
		if err != nil {
			t.Fatalf("failed to update team: %v", err)
		}

		found, err := repo.FindByID(ctx, team.ID)
		if err != nil {
			t.Fatalf("failed to find updated team: %v", err)
		}
		if found.Name != "Engineering Core" {
			t.Errorf("got name %s, want 'Engineering Core'", found.Name)
		}
	})

	t.Run("Delete removes team", func(t *testing.T) {
		err := repo.Delete(ctx, team.ID)
		if err != nil {
			t.Fatalf("failed to delete team: %v", err)
		}

		_, err = repo.FindByID(ctx, team.ID)
		if !errors.Is(err, domain.ErrTeamNotFound) {
			t.Errorf("expected ErrTeamNotFound after delete, got %v", err)
		}
	})
}
