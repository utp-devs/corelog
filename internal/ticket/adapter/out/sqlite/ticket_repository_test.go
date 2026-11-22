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
	ticketsqlite "github.com/jhonsferg/corelog/internal/ticket/adapter/out/sqlite"
	ticketport "github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
	usersqlite "github.com/jhonsferg/corelog/internal/user/adapter/out/sqlite"
	userdomain "github.com/jhonsferg/corelog/internal/user/domain"
)

func TestSQLiteTicketRepository(t *testing.T) {
	ctx := context.Background()
	dbName := "test_ticket_repo_" + uuid.New().String()
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

	userRepo := usersqlite.NewUserRepository(db)
	user, err := userdomain.NewUser("Requester User", "req@example.com", "$2a$10$hash", userdomain.RoleRequester)
	if err != nil {
		t.Fatalf("failed to create user domain object: %v", err)
	}
	if err := userRepo.Save(ctx, user); err != nil {
		t.Fatalf("failed to save requester user: %v", err)
	}

	repo := ticketsqlite.NewTicketRepository(db)

	ticket, err := domain.NewTicket("Database latency", "Queries taking too long", domain.PriorityHigh, user.ID)
	if err != nil {
		t.Fatalf("failed to create ticket domain object: %v", err)
	}

	t.Run("Save and FindByID", func(t *testing.T) {
		err := repo.Save(ctx, ticket)
		if err != nil {
			t.Fatalf("failed to save ticket: %v", err)
		}

		found, err := repo.FindByID(ctx, ticket.ID)
		if err != nil {
			t.Fatalf("failed to find ticket by ID: %v", err)
		}
		if found.ID != ticket.ID {
			t.Errorf("got ID %v, want %v", found.ID, ticket.ID)
		}
		if found.Title != "Database latency" {
			t.Errorf("got title %s, want 'Database latency'", found.Title)
		}
	})

	t.Run("FindByID returns ErrTicketNotFound for missing ID", func(t *testing.T) {
		_, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})

	t.Run("FindAll with filters", func(t *testing.T) {
		res, err := repo.FindAll(ctx, ticketport.ListFilter{
			Status:   domain.StatusOpen,
			Priority: domain.PriorityHigh,
			Page:     1,
			PageSize: 20,
		})
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}
		if len(res) != 1 {
			t.Errorf("got %d tickets, want 1", len(res))
		}
	})

	t.Run("Update modifies ticket status", func(t *testing.T) {
		_ = ticket.ChangeStatus(domain.StatusInProgress)
		err := repo.Update(ctx, ticket)
		if err != nil {
			t.Fatalf("failed to update ticket: %v", err)
		}

		found, err := repo.FindByID(ctx, ticket.ID)
		if err != nil {
			t.Fatalf("failed to find updated ticket: %v", err)
		}
		if found.Status != domain.StatusInProgress {
			t.Errorf("got status %v, want %v", found.Status, domain.StatusInProgress)
		}
	})
}
