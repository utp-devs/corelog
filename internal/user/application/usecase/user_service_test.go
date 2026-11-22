package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type fakeUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[uuid.UUID]*domain.User)}
}

func (r *fakeUserRepo) Save(_ context.Context, u *domain.User) error {
	copied := *u
	r.users[u.ID] = &copied
	return nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
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
	svc := NewUserService(repo)

	t.Run("first registered user becomes admin", func(t *testing.T) {
		user, err := svc.Register(ctx, port.RegisterInput{
			Name:     "Admin User",
			Email:    "admin@example.com",
			Password: "securepassword",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != domain.RoleAdmin {
			t.Errorf("got role %v, want %v", user.Role, domain.RoleAdmin)
		}
	})

	t.Run("second registered user defaults to requester", func(t *testing.T) {
		user, err := svc.Register(ctx, port.RegisterInput{
			Name:     "Second User",
			Email:    "second@example.com",
			Password: "securepassword",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != domain.RoleRequester {
			t.Errorf("got role %v, want %v", user.Role, domain.RoleRequester)
		}
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		_, err := svc.Register(ctx, port.RegisterInput{
			Name:     "Duplicate",
			Email:    "admin@example.com",
			Password: "password",
		})
		if !errors.Is(err, domain.ErrEmailAlreadyTaken) {
			t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
		}
	})
}

func TestUserService_Authenticate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo)

	user, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "password123",
	})

	t.Run("successful authentication", func(t *testing.T) {
		authenticated, err := svc.Authenticate(ctx, port.AuthenticateInput{
			Email:    "alice@example.com",
			Password: "password123",
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
			Password: "password123",
		})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestUserService_ProfileAndPassword(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo)

	user1, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "User One",
		Email:    "one@example.com",
		Password: "oldpassword",
	})
	user2, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "User Two",
		Email:    "two@example.com",
		Password: "oldpassword",
	})

	t.Run("updates profile successfully", func(t *testing.T) {
		updated, err := svc.UpdateProfile(ctx, user1.ID, port.UpdateProfileInput{
			Name:  "User One Renamed",
			Email: "one_new@example.com",
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
			Name:  "User One",
			Email: user2.Email,
		})
		if !errors.Is(err, domain.ErrEmailAlreadyTaken) {
			t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
		}
	})

	t.Run("changes password with valid current password", func(t *testing.T) {
		err := svc.ChangePassword(ctx, user1.ID, port.ChangePasswordInput{
			CurrentPassword: "oldpassword",
			NewPassword:     "newpassword123",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Re-authenticate with new password
		_, err = svc.Authenticate(ctx, port.AuthenticateInput{
			Email:    "one_new@example.com",
			Password: "newpassword123",
		})
		if err != nil {
			t.Errorf("failed to authenticate with new password: %v", err)
		}
	})

	t.Run("rejects password change with wrong current password", func(t *testing.T) {
		err := svc.ChangePassword(ctx, user1.ID, port.ChangePasswordInput{
			CurrentPassword: "wrongpassword",
			NewPassword:     "anotherpass",
		})
		if !errors.Is(err, domain.ErrIncorrectPassword) {
			t.Errorf("expected ErrIncorrectPassword, got %v", err)
		}
	})
}

func TestUserService_TeamAndSearch(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewUserService(repo)

	teamID := uuid.New()
	u1, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "Bob Dylan",
		Email:    "bob@example.com",
		Password: "password",
	})
	u2, _ := svc.Register(ctx, port.RegisterInput{
		Name:     "Bob Marley",
		Email:    "marley@example.com",
		Password: "password",
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
