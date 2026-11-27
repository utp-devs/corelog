package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type UserService struct {
	repo     port.Repository
	sessions port.SessionRepository
	tx       database.TxManager
	consents port.ConsentGate
	detacher port.MembershipDetacher
	clock    clock.Clock
	phones   port.PhoneVerificationRepository
	sms      port.SMSSender
	// codeSource returns a numeric code of the given length; replaced in tests.
	codeSource func(digits int) (string, error)
}

func NewUserService(repo port.Repository, sessions port.SessionRepository) *UserService {
	return &UserService{repo: repo, sessions: sessions, clock: clock.RealClock{}, codeSource: randomCode}
}

// WithAccountClosure enables the cascade of account deactivation: the sole-owner check, the removal of
// organization, team and project memberships, and the session revocation run in one transaction. Without
// it the use case only marks the account and revokes its sessions.
func (s *UserService) WithAccountClosure(tx database.TxManager, detacher port.MembershipDetacher) *UserService {
	s.tx = tx
	s.detacher = detacher
	return s
}

// WithClock replaces the time source (tests).
func (s *UserService) WithClock(c clock.Clock) *UserService {
	s.clock = c
	return s
}

// WithPhoneVerification enables SMS verification of phone numbers. Without it the verification
// operations answer a conflict because there is no way to deliver the code.
func (s *UserService) WithPhoneVerification(repo port.PhoneVerificationRepository, sender port.SMSSender) *UserService {
	s.phones = repo
	s.sms = sender
	return s
}

var _ port.Service = (*UserService)(nil)

// Register creates an account. The password confirmation is compared first (exact, untrimmed), before the
// names, the policy and any repository access, so a mismatch never creates a user. A password reset flow, if
// added, must apply the same rule through domain.CheckPasswordConfirmation.
func (s *UserService) Register(ctx context.Context, in port.RegisterInput) (*domain.User, error) {
	if err := domain.CheckPasswordConfirmation(in.Password, in.PasswordConfirmation, "password_confirmation"); err != nil {
		return nil, err
	}
	first, last, err := resolveNames(in.FirstName, in.LastName, in.Name)
	if err != nil {
		return nil, err
	}
	var phone *domain.Phone
	if in.Phone != nil && !in.Phone.Clear {
		if phone, err = domain.NewPhone(in.Phone.Country, in.Phone.Number); err != nil {
			return nil, err
		}
	}
	locale := in.Locale
	if locale == "" {
		locale = domain.DefaultLocale
	}
	if err := domain.ValidateLocale(locale); err != nil {
		return nil, err
	}

	if err := s.validateConsents(ctx, in); err != nil {
		return nil, err
	}

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

	user, err := domain.NewUser(domain.JoinNames(first, last), in.Email, string(hash), role)
	if err != nil {
		return nil, err
	}
	user.FirstName, user.LastName = first, last
	user.Phone = phone
	user.Locale = locale
	now := s.clock.Now()
	user.CreatedAt, user.UpdatedAt = now, now

	if err := s.saveWithConsents(ctx, user, in); err != nil {
		return nil, err
	}

	return user, nil
}

// resolveNames validates the identity of the input: first_name and last_name when first_name is given,
// otherwise the legacy single name, which is split.
func resolveNames(first, last, legacy string) (string, string, error) {
	switch {
	case strings.TrimSpace(first) != "" || strings.TrimSpace(last) != "":
		return domain.ParseNames(first, last)
	case strings.TrimSpace(legacy) != "":
		return domain.ParseLegacyName(legacy)
	default:
		return "", "", domain.ErrInvalidName
	}
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

	now := s.clock.Now()
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

	email := user.Email
	if in.Email != nil {
		email = *in.Email
	}

	if email == "" {
		return nil, domain.ErrInvalidEmail
	}
	if email != user.Email {
		if existing, err := s.repo.FindByEmail(ctx, email); err == nil && existing.ID != userID {
			return nil, domain.ErrEmailAlreadyTaken
		} else if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			return nil, err
		}
	}

	if in.FirstName != nil || in.LastName != nil {
		first, last := user.FirstName, user.LastName
		if in.FirstName != nil {
			first = *in.FirstName
		}
		if in.LastName != nil {
			last = *in.LastName
		}
		first, last, err := domain.ParseNames(first, last)
		if err != nil {
			return nil, err
		}
		user.SetNames(first, last)
	} else if in.Name != nil {
		first, last, err := domain.ParseLegacyName(*in.Name)
		if err != nil {
			return nil, err
		}
		user.SetNames(first, last)
	}
	user.Email = email
	user.UpdatedAt = s.clock.Now()
	if in.Phone != nil {
		if in.Phone.Clear {
			user.SetPhone(nil)
		} else {
			p, err := domain.NewPhone(in.Phone.Country, in.Phone.Number)
			if err != nil {
				return nil, err
			}
			user.SetPhone(p)
		}
	}
	if in.Locale != nil {
		if err := user.SetLocale(*in.Locale); err != nil {
			return nil, err
		}
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
	if err := domain.CheckPasswordConfirmation(in.NewPassword, in.NewPasswordConfirmation, "new_password_confirmation"); err != nil {
		return err
	}
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
