package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type fakeUserRepo struct {
	users       map[uuid.UUID]*domain.User
	logins      map[uuid.UUID]time.Time
	memberships map[membershipKey]string
	// sharedProjects lists user pairs that have a project in common (same organization).
	sharedProjects map[[2]uuid.UUID]bool
}

type membershipKey struct{ org, user uuid.UUID }

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:          make(map[uuid.UUID]*domain.User),
		logins:         make(map[uuid.UUID]time.Time),
		memberships:    make(map[membershipKey]string),
		sharedProjects: make(map[[2]uuid.UUID]bool),
	}
}

func (r *fakeUserRepo) RecordLogin(_ context.Context, userID uuid.UUID, at time.Time) error {
	if _, ok := r.users[userID]; !ok {
		return domain.ErrUserNotFound
	}
	r.logins[userID] = at
	return nil
}

func (r *fakeUserRepo) FindOrganizationRole(_ context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	role, ok := r.memberships[membershipKey{orgID, userID}]
	return role, ok, nil
}

// SearchOrganizationMembers mirrors the SQL rule: guests only see non-guest members or users sharing a project.
func (r *fakeUserRepo) SearchOrganizationMembers(_ context.Context, filter port.OrganizationSearchFilter) ([]*domain.User, error) {
	var list []*domain.User
	for key, role := range r.memberships {
		if key.org != filter.OrganizationID {
			continue
		}
		if filter.ActorIsGuest && role == "guest" && !r.sharedProjects[[2]uuid.UUID{filter.ActorID, key.user}] {
			continue
		}
		if u, ok := r.users[key.user]; ok {
			copied := *u
			list = append(list, &copied)
		}
	}
	return list, nil
}

func (r *fakeUserRepo) Save(_ context.Context, u *domain.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copied := *u
	r.users[u.ID] = &copied
	return nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copied := *u
	return &copied, nil
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			copied := *u
			return &copied, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeUserRepo) Update(_ context.Context, u *domain.User) error {
	if _, ok := r.users[u.ID]; !ok {
		return domain.ErrUserNotFound
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copied := *u
	r.users[u.ID] = &copied
	return nil
}

func (r *fakeUserRepo) Count(_ context.Context) (int, error) {
	return len(r.users), nil
}

func (r *fakeUserRepo) Search(_ context.Context, filter port.SearchFilter) ([]*domain.User, error) {
	var list []*domain.User
	for _, u := range r.users {
		if filter.TeamID != nil {
			if u.TeamID == nil || *u.TeamID != *filter.TeamID {
				continue
			}
		}
		if filter.Query != "" {
			q := strings.ToLower(filter.Query)
			if !strings.Contains(strings.ToLower(u.Name), q) && !strings.Contains(strings.ToLower(u.Email), q) {
				continue
			}
		}
		copied := *u
		list = append(list, &copied)
	}
	return list, nil
}

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())

	t.Run("first registered user becomes admin", func(t *testing.T) {
		user, err := svc.Register(ctx, port.RegisterInput{
			Name:     "Admin User",
			Email:    "admin@example.com",
			Password: "Secure-Pass-123x", PasswordConfirmation: "Secure-Pass-123x",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != domain.RoleAdmin {
			t.Errorf("got role %v, want %v", user.Role, domain.RoleAdmin)
		}
	})

	t.Run("second registered user defaults to user", func(t *testing.T) {
		user, err := svc.Register(ctx, port.RegisterInput{
			Name:     "Second User",
			Email:    "second@example.com",
			Password: "Secure-Pass-123x", PasswordConfirmation: "Secure-Pass-123x",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != domain.RoleUser {
			t.Errorf("got role %v, want %v", user.Role, domain.RoleUser)
		}
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		_, err := svc.Register(ctx, port.RegisterInput{
			Name:     "Duplicate",
			Email:    "admin@example.com",
			Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x",
		})
		if !errors.Is(err, domain.ErrEmailAlreadyTaken) {
			t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
		}
	})
}

func TestUserService_Authenticate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())

	user, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x",
	})

	t.Run("successful authentication", func(t *testing.T) {
		authenticated, err := svc.Authenticate(ctx, port.AuthenticateInput{
			Email:    "alice@example.com",
			Password: "Tr1cky-Pass9x",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authenticated.ID != user.ID {
			t.Errorf("got user ID %v, want %v", authenticated.ID, user.ID)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := svc.Authenticate(ctx, port.AuthenticateInput{
			Email:    "alice@example.com",
			Password: "wrongpassword",
		})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, err := svc.Authenticate(ctx, port.AuthenticateInput{
			Email:    "unknown@example.com",
			Password: "Tr1cky-Pass9x",
		})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestUserService_ProfileAndPassword(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())

	user1, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "User One",
		Email:    "one@example.com",
		Password: "Old-Pass-1234x", PasswordConfirmation: "Old-Pass-1234x",
	})
	user2, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "User Two",
		Email:    "two@example.com",
		Password: "Old-Pass-1234x", PasswordConfirmation: "Old-Pass-1234x",
	})

	t.Run("updates profile successfully", func(t *testing.T) {
		updated, err := svc.UpdateProfile(ctx, user1.ID, port.UpdateProfileInput{
			Name:  strPtr("User One Renamed"),
			Email: strPtr("one_new@example.com"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Name != "User One Renamed" {
			t.Errorf("got name %s, want 'User One Renamed'", updated.Name)
		}
	})

	t.Run("rejects email taken by another user", func(t *testing.T) {
		_, err := svc.UpdateProfile(ctx, user1.ID, port.UpdateProfileInput{
			Name:  strPtr("User One"),
			Email: strPtr(user2.Email),
		})
		if !errors.Is(err, domain.ErrEmailAlreadyTaken) {
			t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
		}
	})

	t.Run("changes password with valid current password", func(t *testing.T) {
		err := svc.ChangePassword(ctx, user1.ID, port.ChangePasswordInput{
			CurrentPassword:         "Old-Pass-1234x",
			NewPassword:             "New-Pass-4567x",
			NewPasswordConfirmation: "New-Pass-4567x",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Re-authenticate with new password
		_, err = svc.Authenticate(ctx, port.AuthenticateInput{
			Email:    "one_new@example.com",
			Password: "New-Pass-4567x",
		})
		if err != nil {
			t.Errorf("failed to authenticate with new password: %v", err)
		}
	})

	t.Run("rejects password change with wrong current password", func(t *testing.T) {
		err := svc.ChangePassword(ctx, user1.ID, port.ChangePasswordInput{
			CurrentPassword:         "wrongpassword",
			NewPassword:             "Another-Pass-77x",
			NewPasswordConfirmation: "Another-Pass-77x",
		})
		if !errors.Is(err, domain.ErrIncorrectPassword) {
			t.Errorf("expected ErrIncorrectPassword, got %v", err)
		}
	})
}

func TestUserService_TeamAndSearch(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo, newFakeSessionRepo())

	teamID := uuid.New()
	u1, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "Bob Dylan",
		Email:    "bob@example.com",
		Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x",
	})
	u2, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "Bob Marley",
		Email:    "marley@example.com",
		Password: "Tr1cky-Pass9x", PasswordConfirmation: "Tr1cky-Pass9x",
	})

	t.Run("SearchTeammates returns empty when user has no team", func(t *testing.T) {
		results, err := svc.SearchTeammates(ctx, u1.ID, "Bob")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})

	t.Run("SetTeam assigns team to user", func(t *testing.T) {
		updated, err := svc.SetTeam(ctx, u1.ID, &teamID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.TeamID == nil || *updated.TeamID != teamID {
			t.Errorf("got teamID %v, want %v", updated.TeamID, teamID)
		}
		_, _ = svc.SetTeam(ctx, u2.ID, &teamID)
	})

	t.Run("SearchTeammates returns teammates in same team matching query", func(t *testing.T) {
		results, err := svc.SearchTeammates(ctx, u1.ID, "Dylan")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].ID != u1.ID {
			t.Errorf("got result ID %v, want %v", results[0].ID, u1.ID)
		}
	})
}

func strPtr(s string) *string { return &s }
