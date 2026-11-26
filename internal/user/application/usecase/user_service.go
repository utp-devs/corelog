package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type UserService struct {
	repo     port.Repository
	sessions port.SessionRepository
	tx       database.TxManager
	detacher port.MembershipDetacher
}

func NewUserService(repo port.Repository, sessions port.SessionRepository) *UserService {
	return &UserService{repo: repo, sessions: sessions}
}

// WithAccountClosure enables the cascade of account deactivation: the sole-owner check, the removal of
// organization, team and project memberships, and the session revocation run in one transaction. Without
// it the use case only marks the account and revokes its sessions.
func (s *UserService) WithAccountClosure(tx database.TxManager, detacher port.MembershipDetacher) *UserService {
	s.tx = tx
	s.detacher = detacher
	return s
}

var _ port.Service = (*UserService)(nil)

func (s *UserService) Register(ctx context.Context, in port.RegisterInput) (*domain.User, error) {
	if _, err := s.repo.FindByEmail(ctx, in.Email); err == nil {
		return nil, domain.ErrEmailAlreadyTaken
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to hash password", err)
	}

	role := in.Role
	if role == "" {
		role = domain.RoleUser
	}

	count, err := s.repo.Count(ctx)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		role = domain.RoleAdmin
	}

	user, err := domain.NewUser(in.Name, in.Email, string(hash), role)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) Authenticate(ctx context.Context, in port.AuthenticateInput) (*domain.User, error) {
	user, err := s.repo.FindByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// Una cuenta desactivada responde igual que una contrasena incorrecta: no revela que existio.
	if !user.IsActive() {
		return nil, domain.ErrInvalidCredentials
	}

	now := time.Now().UTC()
	if err := s.repo.RecordLogin(ctx, user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now

	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, in port.UpdateProfileInput) (*domain.User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	name, email := user.Name, user.Email
	if in.Name != nil {
		name = *in.Name
	}
	if in.Email != nil {
		email = *in.Email
	}

	if email != user.Email {
		if existing, err := s.repo.FindByEmail(ctx, email); err == nil && existing.ID != userID {
			return nil, domain.ErrEmailAlreadyTaken
		} else if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			return nil, err
		}
	}

	if err := user.UpdateProfile(name, email); err != nil {
		return nil, err
	}
	if in.DisplayName != nil {
		if err := user.SetDisplayName(*in.DisplayName); err != nil {
			return nil, err
		}
	}
	if in.JobTitle != nil {
		user.SetJobTitle(*in.JobTitle)
	}
	if in.Department != nil {
		user.SetDepartment(*in.Department)
	}
	if in.Timezone != nil {
		if err := user.SetTimezone(*in.Timezone); err != nil {
			return nil, err
		}
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID uuid.UUID, in port.ChangePasswordInput) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.CurrentPassword)); err != nil {
		return domain.ErrIncorrectPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to hash password", err)
	}

	user.ChangePasswordHash(string(hash))

	return s.repo.Update(ctx, user)
}

func (s *UserService) SetTeam(ctx context.Context, userID uuid.UUID, teamID *uuid.UUID) (*domain.User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.AssignTeam(teamID)

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) SearchTeammates(ctx context.Context, actorID uuid.UUID, query string) ([]*domain.User, error) {
	actor, err := s.repo.FindByID(ctx, actorID)
	if err != nil {
		return nil, err
	}

	if actor.TeamID == nil {
		return []*domain.User{}, nil
	}

	return s.repo.Search(ctx, port.SearchFilter{Query: query, TeamID: actor.TeamID})
}

func (s *UserService) SearchInOrganization(ctx context.Context, actorID, orgID uuid.UUID, query string) ([]*domain.User, error) {
	role, found, err := s.repo.FindOrganizationRole(ctx, orgID, actorID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, domain.ErrNotOrganizationMember
	}

	return s.repo.SearchOrganizationMembers(ctx, port.OrganizationSearchFilter{
		OrganizationID: orgID,
		ActorID:        actorID,
		ActorIsGuest:   role == orgRoleGuest,
		Query:          query,
	})
}

// orgRoleGuest is the organization_members.role value of guests (see migration 000023).
const orgRoleGuest = "guest"

func (s *UserService) Search(ctx context.Context, filter port.SearchFilter) ([]*domain.User, error) {
	return s.repo.Search(ctx, filter)
}

const maxUserAgentLength = 300

func (s *UserService) OpenSession(ctx context.Context, userID uuid.UUID, userAgent string, expiresAt time.Time) (uuid.UUID, error) {
	now := time.Now().UTC()
	if ua := []rune(userAgent); len(ua) > maxUserAgentLength {
		userAgent = string(ua[:maxUserAgentLength])
	}
	session := &domain.Session{
		ID:         uuid.New(),
		UserID:     userID,
		UserAgent:  userAgent,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt,
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return uuid.Nil, err
	}
	return session.ID, nil
}

func (s *UserService) ListSessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]port.SessionView, error) {
	sessions, err := s.sessions.ListActive(ctx, userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	views := make([]port.SessionView, 0, len(sessions))
	for _, session := range sessions {
		views = append(views, port.SessionView{
			ID:         session.ID.String(),
			UserAgent:  session.UserAgent,
			CreatedAt:  session.CreatedAt.Format(time.RFC3339),
			LastSeenAt: session.LastSeenAt.Format(time.RFC3339),
			Current:    session.ID == currentSessionID,
		})
	}
	return views, nil
}

func (s *UserService) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return s.sessions.Revoke(ctx, userID, sessionID, time.Now().UTC())
}

func (s *UserService) RevokeOtherSessions(ctx context.Context, userID, currentSessionID uuid.UUID) error {
	return s.sessions.RevokeExcept(ctx, userID, &currentSessionID, time.Now().UTC())
}

// DeactivateAccount cierra la cuenta tras confirmar la contrasena y revoca todas sus sesiones.
func (s *UserService) DeactivateAccount(ctx context.Context, userID uuid.UUID, password string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.ErrIncorrectPassword
	}

	run := func(ctx context.Context) error {
		if s.detacher != nil {
			sole, err := s.detacher.SoleOwnedOrganizations(ctx, userID)
			if err != nil {
				return err
			}
			if sole > 0 {
				return domain.ErrSoleOwner
			}
		}

		now := time.Now().UTC()
		user.Deactivate(now)
		// Update re-syncs team_members from user.TeamID, so the detach below must run after it.
		if err := s.repo.Update(ctx, user); err != nil {
			return err
		}
		if s.detacher != nil {
			if err := s.detacher.DetachMemberships(ctx, userID); err != nil {
				return err
			}
		}
		return s.sessions.RevokeExcept(ctx, userID, nil, now)
	}

	if s.tx == nil {
		return run(ctx)
	}
	return s.tx.InTx(ctx, run)
}
