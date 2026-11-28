package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type fakeSessionRepo struct {
	sessions map[uuid.UUID]*domain.Session
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: make(map[uuid.UUID]*domain.Session)}
}

func (r *fakeSessionRepo) Create(_ context.Context, s *domain.Session) error {
	copied := *s
	r.sessions[s.ID] = &copied
	return nil
}

func (r *fakeSessionRepo) ListActive(_ context.Context, userID uuid.UUID, now time.Time) ([]*domain.Session, error) {
	var out []*domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID && s.IsActive(now) {
			copied := *s
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (r *fakeSessionRepo) Revoke(_ context.Context, userID, sessionID uuid.UUID, now time.Time) error {
	s, ok := r.sessions[sessionID]
	if !ok || s.UserID != userID || s.RevokedAt != nil {
		return domain.ErrSessionNotFound
	}
	s.RevokedAt = &now
	return nil
}

func (r *fakeSessionRepo) RevokeExcept(_ context.Context, userID uuid.UUID, keepID *uuid.UUID, now time.Time) error {
	for id, s := range r.sessions {
		if s.UserID != userID || s.RevokedAt != nil || (keepID != nil && id == *keepID) {
			continue
		}
		s.RevokedAt = &now
	}
	return nil
}

func (r *fakeSessionRepo) Touch(_ context.Context, _ uuid.UUID, _ time.Time) error { return nil }

func (r *fakeSessionRepo) Validate(_ context.Context, sessionID, userID uuid.UUID) error {
	s, ok := r.sessions[sessionID]
	if !ok || s.UserID != userID || !s.IsActive(time.Now()) {
		return errors.New("session is no longer valid")
	}
	return nil
}

var _ port.SessionRepository = (*fakeSessionRepo)(nil)

func registerUser(t *testing.T, svc *UserService, email, password string) uuid.UUID {
	t.Helper()
	u, err := svc.Register(context.Background(), port.RegisterInput{Name: "Ana Prueba", Email: email, Password: password, PasswordConfirmation: password})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return u.ID
}

func TestUserService_DeactivatedAccountCannotAuthenticate(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	svc := NewUserService(repo, sessions)
	ctx := context.Background()
	userID := registerUser(t, svc, "ana@example.com", "clave-segura-Z9")

	if err := svc.DeactivateAccount(ctx, userID, "clave-segura-Z9"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	_, err := svc.Authenticate(ctx, port.AuthenticateInput{Email: "ana@example.com", Password: "clave-segura-Z9"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want invalid credentials for deactivated account, got %v", err)
	}
}

func TestUserService_DeactivateRequiresPasswordAndRevokesSessions(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	svc := NewUserService(repo, sessions)
	ctx := context.Background()
	userID := registerUser(t, svc, "ana@example.com", "clave-segura-Z9")
	if _, err := svc.OpenSession(ctx, userID, "navegador", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("open session: %v", err)
	}

	if err := svc.DeactivateAccount(ctx, userID, "otra-clave"); !errors.Is(err, domain.ErrIncorrectPassword) {
		t.Fatalf("wrong password: want ErrIncorrectPassword, got %v", err)
	}
	if u, _ := repo.FindByID(ctx, userID); !u.IsActive() {
		t.Fatal("account must stay active after a wrong password")
	}

	if err := svc.DeactivateAccount(ctx, userID, "clave-segura-Z9"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	active, _ := sessions.ListActive(ctx, userID, time.Now())
	if len(active) != 0 {
		t.Fatalf("all sessions must be revoked on deactivation, %d still active", len(active))
	}
}

func TestUserService_RevokeOtherSessionsKeepsCurrent(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	svc := NewUserService(repo, sessions)
	ctx := context.Background()
	userID := registerUser(t, svc, "ana@example.com", "clave-segura-Z9")
	expires := time.Now().Add(time.Hour)
	current, _ := svc.OpenSession(ctx, userID, "portatil", expires)
	other, _ := svc.OpenSession(ctx, userID, "movil", expires)

	views, err := svc.ListSessions(ctx, userID, current)
	if err != nil || len(views) != 2 {
		t.Fatalf("list: want 2 sessions, got %d (%v)", len(views), err)
	}
	if err := svc.RevokeOtherSessions(ctx, userID, current); err != nil {
		t.Fatalf("revoke others: %v", err)
	}
	views, _ = svc.ListSessions(ctx, userID, current)
	if len(views) != 1 || views[0].ID != current.String() || !views[0].Current {
		t.Fatalf("only the current session must remain, got %+v", views)
	}
	if err := sessions.Validate(ctx, other, userID); err == nil {
		t.Fatal("revoked session must not validate")
	}
}

func TestUserService_RevokeSessionOfAnotherUserIsNotFound(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	svc := NewUserService(repo, sessions)
	ctx := context.Background()
	owner := registerUser(t, svc, "ana@example.com", "clave-segura-Z9")
	intruder := registerUser(t, svc, "luis@example.com", "clave-segura-2")
	sid, _ := svc.OpenSession(ctx, owner, "portatil", time.Now().Add(time.Hour))

	if err := svc.RevokeSession(ctx, intruder, sid); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("want ErrSessionNotFound for another user's session, got %v", err)
	}
}
