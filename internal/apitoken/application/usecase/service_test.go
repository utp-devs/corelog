package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/apitoken/application/port"
	"github.com/jhonsferg/corelog/internal/apitoken/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

type storedToken struct {
	tok  domain.Token
	hash string
}

type fakeRepo struct {
	tokens  []storedToken
	owners  map[uuid.UUID]port.Owner
	touches int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{owners: map[uuid.UUID]port.Owner{}} }

func (r *fakeRepo) Insert(_ context.Context, t *domain.Token, hash string) error {
	t.ID = uuid.New()
	r.tokens = append(r.tokens, storedToken{*t, hash})
	return nil
}

func (r *fakeRepo) CountActive(_ context.Context, userID uuid.UUID, now time.Time) (int, error) {
	n := 0
	for _, s := range r.tokens {
		if s.tok.UserID == userID && s.tok.Active(now) {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepo) List(_ context.Context, userID uuid.UUID, includeRevoked bool) ([]domain.Token, error) {
	var out []domain.Token
	for _, s := range r.tokens {
		if s.tok.UserID == userID && (includeRevoked || s.tok.RevokedAt == nil) {
			out = append(out, s.tok)
		}
	}
	return out, nil
}

func (r *fakeRepo) Revoke(_ context.Context, userID, id uuid.UUID, at time.Time) (domain.Token, error) {
	for i, s := range r.tokens {
		if s.tok.ID == id && s.tok.UserID == userID {
			if r.tokens[i].tok.RevokedAt == nil {
				r.tokens[i].tok.RevokedAt = &at
			}
			return r.tokens[i].tok, nil
		}
	}
	return domain.Token{}, domain.ErrNotFound
}

func (r *fakeRepo) FindByHash(_ context.Context, hash string) (domain.Token, port.Owner, string, error) {
	for _, s := range r.tokens {
		if s.hash == hash {
			return s.tok, r.owners[s.tok.UserID], s.hash, nil
		}
	}
	return domain.Token{}, port.Owner{}, "", domain.ErrNotFound
}

func (r *fakeRepo) Touch(context.Context, uuid.UUID, time.Time, time.Time) error {
	r.touches++
	return nil
}

type recorder struct{ events []events.Event }

func (p *recorder) Publish(_ context.Context, e events.Event) error {
	p.events = append(p.events, e)
	return nil
}

type fixedLimiter struct{ allow bool }

func (l fixedLimiter) Allow(string) (bool, time.Duration) { return l.allow, time.Second }

func newService(repo *fakeRepo, pub events.Publisher, limiter port.Limiter) (*Service, *clock.FakeClock) {
	clk := &clock.FakeClock{T: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)}
	return NewService(repo, &database.FakeTxManager{}, pub, clk, limiter), clk
}

func TestCreateShowsPlaintextOnceAndStoresOnlyTheHash(t *testing.T) {
	repo, pub := newFakeRepo(), &recorder{}
	svc, _ := newService(repo, pub, nil)
	user := uuid.New()
	tok, plain, err := svc.Create(context.Background(), user, port.CreateInput{Name: " CI ", Scopes: []string{"write", "read"}, GlobalRole: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if !domain.WellFormed(plain) || tok.Name != "CI" || tok.Prefix != plain[:11] || tok.ID == uuid.Nil {
		t.Fatalf("token %+v plaintext %q", tok, plain)
	}
	if len(repo.tokens) != 1 || repo.tokens[0].hash != domain.Hash(plain) || strings.Contains(repo.tokens[0].hash, plain) {
		t.Fatal("only the hash may be stored")
	}
	if len(pub.events) != 1 || pub.events[0].Type != "auth.token_created" {
		t.Fatalf("events %v", pub.events)
	}
	for k, v := range pub.events[0].Payload {
		if s, ok := v.(string); ok && strings.Contains(s, plain) {
			t.Fatalf("audit payload %s leaks the token", k)
		}
	}
	listed, _ := svc.List(context.Background(), user, false)
	if len(listed) != 1 {
		t.Fatalf("list = %v", listed)
	}
}

func TestCreateValidationAndLimits(t *testing.T) {
	repo := newFakeRepo()
	svc, clk := newService(repo, nil, nil)
	user := uuid.New()
	ctx := context.Background()
	past := clk.T.Add(-time.Hour)
	for name, in := range map[string]port.CreateInput{
		"no name":       {Scopes: []string{"read"}},
		"no scopes":     {Name: "x"},
		"bad scope":     {Name: "x", Scopes: []string{"root"}},
		"past expiry":   {Name: "x", Scopes: []string{"read"}, ExpiresAt: &past},
		"admin as user": {Name: "x", Scopes: []string{"admin"}, GlobalRole: "user"},
	} {
		if _, _, err := svc.Create(ctx, user, in); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, _, err := svc.Create(ctx, user, port.CreateInput{Name: "x", Scopes: []string{"admin"}, GlobalRole: "admin"}); err != nil {
		t.Fatalf("admin scope for admin: %v", err)
	}
	for i := 1; i < domain.MaxActivePerUser; i++ {
		if _, _, err := svc.Create(ctx, user, port.CreateInput{Name: "t", Scopes: []string{"read"}}); err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
	}
	_, _, err := svc.Create(ctx, user, port.CreateInput{Name: "one more", Scopes: []string{"read"}})
	if apperror.KindOf(err) != apperror.KindConflict || apperror.CodeOf(err) != "api_token.limit_reached" {
		t.Fatalf("21st token: %v", err)
	}
	// Revoked and expired tokens do not count.
	if err := svc.Revoke(ctx, user, repo.tokens[0].tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Create(ctx, user, port.CreateInput{Name: "after revoke", Scopes: []string{"read"}}); err != nil {
		t.Fatalf("create after revoke: %v", err)
	}
}

func TestRevoke(t *testing.T) {
	repo, pub := newFakeRepo(), &recorder{}
	svc, _ := newService(repo, pub, nil)
	owner, other := uuid.New(), uuid.New()
	tok, _, _ := svc.Create(context.Background(), owner, port.CreateInput{Name: "x", Scopes: []string{"read"}})
	if err := svc.Revoke(context.Background(), other, tok.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign revoke: %v", err)
	}
	if err := svc.Revoke(context.Background(), owner, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown revoke: %v", err)
	}
	if err := svc.Revoke(context.Background(), owner, tok.ID); err != nil {
		t.Fatal(err)
	}
	if got := pub.events[len(pub.events)-1].Type; got != "auth.token_revoked" {
		t.Fatalf("event %s", got)
	}
}

func TestAuthenticateMatrix(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc, clk := newService(repo, nil, nil)
	user := uuid.New()
	repo.owners[user] = port.Owner{Role: "admin"}

	create := func(scopes []string, expires *time.Time) (domain.Token, string) {
		tok, plain, err := svc.Create(ctx, user, port.CreateInput{Name: "t", Scopes: scopes, ExpiresAt: expires, GlobalRole: "admin"})
		if err != nil {
			t.Fatal(err)
		}
		return tok, plain
	}
	valid, validPlain := create([]string{"read"}, nil)
	revoked, revokedPlain := create([]string{"read"}, nil)
	_ = svc.Revoke(ctx, user, revoked.ID)
	soon := clk.T.Add(time.Minute)
	_, expiringPlain := create([]string{"read"}, &soon)
	_, adminPlain := create([]string{"admin"}, nil)

	id, err := svc.Authenticate(ctx, validPlain)
	if err != nil || id.UserID != user || id.TokenID != valid.ID || id.Role != "user" {
		t.Fatalf("valid: %+v %v (a read token never carries the admin role)", id, err)
	}
	if id, err = svc.Authenticate(ctx, adminPlain); err != nil || id.Role != "admin" {
		t.Fatalf("admin scope: %+v %v", id, err)
	}
	if repo.touches == 0 {
		t.Fatal("last use must be recorded")
	}
	for name, plain := range map[string]string{
		"revoked":                       revokedPlain,
		"unknown":                       "cl_pat_" + strings.Repeat("Z", 40),
		"malformed":                     "cl_pat_short",
		"empty":                         "",
		"same prefix, different secret": validPlain[:11] + strings.Repeat("Q", 36),
	} {
		if _, err := svc.Authenticate(ctx, plain); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := svc.Authenticate(ctx, expiringPlain); err != nil {
		t.Fatalf("before expiry: %v", err)
	}
	clk.Advance(2 * time.Minute)
	if _, err := svc.Authenticate(ctx, expiringPlain); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expired: %v", err)
	}
	repo.owners[user] = port.Owner{Role: "admin", Deactivated: true}
	if _, err := svc.Authenticate(ctx, validPlain); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("deactivated owner: %v", err)
	}
}

func TestAuthenticateRateLimit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	user := uuid.New()
	svc, _ := newService(repo, nil, fixedLimiter{allow: false})
	_, plain, _ := svc.Create(ctx, user, port.CreateInput{Name: "x", Scopes: []string{"read"}})
	if _, err := svc.Authenticate(ctx, plain); apperror.KindOf(err) != apperror.KindRateLimited {
		t.Fatalf("limited: %v", err)
	}
	svc, _ = newService(repo, nil, fixedLimiter{allow: true})
	if _, err := svc.Authenticate(ctx, plain); err != nil {
		t.Fatalf("allowed: %v", err)
	}
}
