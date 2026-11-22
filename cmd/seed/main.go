package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	teampostgres "github.com/jhonsferg/corelog/internal/team/adapter/out/postgres"
	teamport "github.com/jhonsferg/corelog/internal/team/application/port"
	teamdomain "github.com/jhonsferg/corelog/internal/team/domain"
	ticketpostgres "github.com/jhonsferg/corelog/internal/ticket/adapter/out/postgres"
	ticketport "github.com/jhonsferg/corelog/internal/ticket/application/port"
	ticketdomain "github.com/jhonsferg/corelog/internal/ticket/domain"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
	userdomain "github.com/jhonsferg/corelog/internal/user/domain"
)

func main() {
	if err := runSeed(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func runSeed() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var db *sqlx.DB
	db, err = database.NewPostgres(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	var userRepo userport.Repository
	var teamRepo teamport.Repository
	var ticketRepo ticketport.Repository
	userRepo = userpostgres.NewUserRepository(db)
	teamRepo = teampostgres.NewTeamRepository(db)
	ticketRepo = ticketpostgres.NewTicketRepository(db)

	ctx := context.Background()

	// 1. Seed Team
	teamName := "Core Development"
	team, err := teamRepo.FindByName(ctx, teamName)
	if err != nil {
		team, err = teamdomain.NewTeam(teamName)
		if err != nil {
			return fmt.Errorf("create team domain: %w", err)
		}
		if err := teamRepo.Save(ctx, team); err != nil {
			return fmt.Errorf("save team: %w", err)
		}
		fmt.Printf("✓ Created team: %s (%s)\n", team.Name, team.ID)
	} else {
		fmt.Printf("• Existing team: %s (%s)\n", team.Name, team.ID)
	}

	// 2. Seed Users
	defaultPassword := "Password123!"
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	passwordHash := string(hashedBytes)

	usersToSeed := []struct {
		name  string
		email string
		role  userdomain.Role
	}{
		{"Admin User", "admin@corelog.local", userdomain.RoleAdmin},
		{"Agent User", "agent@corelog.local", userdomain.RoleAgent},
		{"Requester User", "requester@corelog.local", userdomain.RoleRequester},
	}

	userMap := make(map[string]*userdomain.User)
	for _, u := range usersToSeed {
		user, err := userRepo.FindByEmail(ctx, u.email)
		if err != nil {
			user, err = userdomain.NewUser(u.name, u.email, passwordHash, u.role)
			if err != nil {
				return fmt.Errorf("create user %s: %w", u.email, err)
			}
			user.AssignTeam(&team.ID)
			if err := userRepo.Save(ctx, user); err != nil {
				return fmt.Errorf("save user %s: %w", u.email, err)
			}
			fmt.Printf("✓ Created user: %s [%s] (password: %s)\n", user.Email, user.Role, defaultPassword)
		} else {
			if user.TeamID == nil {
				user.AssignTeam(&team.ID)
				_ = userRepo.Update(ctx, user)
			}
			fmt.Printf("• Existing user: %s [%s]\n", user.Email, user.Role)
		}
		userMap[u.email] = user
	}

	// 3. Seed Tickets
	adminUser := userMap["admin@corelog.local"]
	agentUser := userMap["agent@corelog.local"]
	requesterUser := userMap["requester@corelog.local"]

	ticketsToSeed := []struct {
		title       string
		description string
		priority    ticketdomain.Priority
		status      ticketdomain.Status
		requesterID uuid.UUID
		assigneeID  *uuid.UUID
	}{
		{
			title:       "Database latency on high concurrent requests",
			description: "P99 latency spikes above 2000ms when more than 500 requests per second are handled.",
			priority:    ticketdomain.PriorityCritical,
			status:      ticketdomain.StatusOpen,
			requesterID: requesterUser.ID,
			assigneeID:  nil,
		},
		{
			title:       "Improve dark mode contrast for status badges",
			description: "Certain badge colors are hard to read on dark backgrounds, causing accessibility issues.",
			priority:    ticketdomain.PriorityHigh,
			status:      ticketdomain.StatusInProgress,
			requesterID: requesterUser.ID,
			assigneeID:  &agentUser.ID,
		},
		{
			title:       "Add automated integration tests for team module",
			description: "Ensure team creation, member association and deletion are tested against PostgreSQL.",
			priority:    ticketdomain.PriorityMedium,
			status:      ticketdomain.StatusResolved,
			requesterID: adminUser.ID,
			assigneeID:  &agentUser.ID,
		},
		{
			title:       "Fix typo in API documentation header",
			description: "The title in Swagger UI states CorLog instead of CoreLog.",
			priority:    ticketdomain.PriorityLow,
			status:      ticketdomain.StatusClosed,
			requesterID: requesterUser.ID,
			assigneeID:  &adminUser.ID,
		},
	}

	for _, t := range ticketsToSeed {
		ticket, err := ticketdomain.NewTicket(t.title, t.description, t.priority, t.requesterID)
		if err != nil {
			return fmt.Errorf("create ticket %s: %w", t.title, err)
		}
		if t.status != ticketdomain.StatusOpen {
			if err := ticket.ChangeStatus(t.status); err != nil {
				// If status is resolved/closed, transition through in_progress first if needed
				if t.status == ticketdomain.StatusResolved {
					_ = ticket.ChangeStatus(ticketdomain.StatusInProgress)
					_ = ticket.ChangeStatus(ticketdomain.StatusResolved)
				} else if t.status == ticketdomain.StatusClosed {
					_ = ticket.ChangeStatus(ticketdomain.StatusClosed)
				}
			}
		}
		if t.assigneeID != nil {
			ticket.AssigneeID = t.assigneeID
		}
		if err := ticketRepo.Save(ctx, ticket); err != nil {
			return fmt.Errorf("save ticket %s: %w", t.title, err)
		}
		fmt.Printf("✓ Created ticket: %s [%s - %s]\n", ticket.Title, ticket.Status, ticket.Priority)
	}

	fmt.Println("\n✨ Database seeding completed successfully.")
	return nil
}
