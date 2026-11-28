package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
	"github.com/jhonsferg/corelog/internal/platform/ratelimit"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type memTokenRow struct {
	userID  uuid.UUID
	expires time.Time
	used    bool
}

// memoryTokenStore mimics the table: one active token per user, single use, expiry.
type memoryTokenStore struct{ rows map[string]*memTokenRow }

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{rows: map[string]*memTokenRow{}}
}

func (m *memoryTokenStore) Replace(_ context.Context, userID uuid.UUID, hash []byte, exp, _ time.Time) error {
	for k, r := range m.rows {
		if r.userID == userID && !r.used {
			delete(m.rows, k)
		}
	}
	m.rows[string(hash)] = &memTokenRow{userID: userID, expires: exp}
	return nil
}

func (m *memoryTokenStore) Consume(_ context.Context, hash []byte, now time.Time) (uuid.UUID, bool, error) {
	r, ok := m.rows[string(hash)]
	if !ok || r.used || !r.expires.After(now) {
		return uuid.Nil, false, nil
	}
	r.used = true
	return r.userID, true, nil
}

func (m *memoryTokenStore) Find(_ context.Context, hash []byte, now time.Time) (uuid.UUID, bool, error) {
	r, ok := m.rows[string(hash)]
	if !ok || r.used || !r.expires.After(now) {
		return uuid.Nil, false, nil
	}
	return r.userID, true, nil
}

func (m *memoryTokenStore) PurgeExpired(context.Context, time.Time) (int64, error) { return 0, nil }

type memoryLockoutStore struct {
	states map[string]domain.LockoutState
}

func (m *memoryLockoutStore) Get(_ context.Context, k string) (domain.LockoutState, error) {
	return m.states[k], nil
}
func (m *memoryLockoutStore) Save(_ context.Context, k string, s domain.LockoutState, _ time.Time) error {
	m.states[k] = s
	return nil
}
func (m *memoryLockoutStore) Delete(_ context.Context, k string) error {
	delete(m.states, k)
	return nil
}
func (m *memoryLockoutStore) PurgeStale(context.Context, time.Time) (int64, error) { return 0, nil }

type securityEnv struct {
	router   *chi.Mux
	secret   string
	repo     *memoryUserRepo
	sessions *memorySessionRepo
	mail     *mailer.Recorder
	clk      *clock.FakeClock
	resets   *memoryTokenStore
	verifies *memoryTokenStore
	lockouts *memoryLockoutStore
	service  *usecase.UserService
}

func newSecurityEnv(t *testing.T, limits *userhttp.RateLimits) *securityEnv {
	t.Helper()
	env := &securityEnv{
		secret:   "test-secret-key-that-is-long-enough-32-bytes",
		repo:     newMemoryUserRepo(),
		sessions: newMemorySessionRepo(),
		mail:     &mailer.Recorder{},
		clk:      &clock.FakeClock{T: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)},
		resets:   newMemoryTokenStore(),
		verifies: newMemoryTokenStore(),
		lockouts: &memoryLockoutStore{states: map[string]domain.LockoutState{}},
	}
	env.service = usecase.NewUserService(env.repo, env.sessions).WithClock(env.clk)
	recovery := usecase.NewRecoveryService(env.repo, env.sessions, env.resets, env.verifies, env.lockouts, nil, env.mail, nil,
		env.clk, "https://app.test", nil).WithSyncDispatch()
	guard := usecase.NewLoginGuard(env.lockouts, domain.DefaultLockoutPolicy(), env.clk, nil)
	handler := userhttp.NewHandler(env.service, env.secret, 24*time.Hour).WithSecurity(userhttp.Security{
		Recovery: recovery, Guard: guard, Limits: limits,
	})
	env.router = chi.NewRouter()
	handler.RegisterRoutes(env.router)
	return env
}

const goodPassword = "Str0ng-Passw0rd!"

func (e *securityEnv) register(t *testing.T, email string) *domain.User {
	t.Helper()
	u, err := e.service.Register(context.Background(), userport.RegisterInput{
		FirstName: "Ana", LastName: "Prueba", Email: email, Password: goodPassword, PasswordConfirmation: goodPassword,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return u
}

func (e *securityEnv) post(path string, body any, headers ...string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.RemoteAddr = "203.0.113.9:4000"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *securityEnv) login(email, password string) *httptest.ResponseRecorder {
	return e.post("/auth/login", map[string]string{"email": email, "password": password})
}

func lastToken(t *testing.T, m *mailer.Recorder) string {
	t.Helper()
	if len(m.Messages) == 0 {
		t.Fatal("no email was sent")
	}
	text := m.Messages[len(m.Messages)-1].Text
	i := strings.Index(text, "token=")
	if i < 0 {
		t.Fatalf("no token link in %q", text)
	}
	rest := text[i+len("token="):]
	if end := strings.IndexAny(rest, " \n\r"); end >= 0 {
		rest = rest[:end]
	}
	token, err := url.QueryUnescape(rest)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error.Code
}

func TestForgotPasswordDoesNotEnumerateAccounts(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	env.register(t, "ana@example.com")

	known := env.post("/auth/password/forgot", map[string]string{"email": "ana@example.com"})
	unknown := env.post("/auth/password/forgot", map[string]string{"email": "ghost@example.com"})
	if known.Code != http.StatusAccepted || unknown.Code != http.StatusAccepted {
		t.Fatalf("status known=%d unknown=%d, want 202 both", known.Code, unknown.Code)
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("bodies differ:\n%s\n%s", known.Body, unknown.Body)
	}
	if len(env.mail.Messages) != 1 || env.mail.Messages[0].To != "ana@example.com" {
		t.Fatalf("emails = %+v, want exactly one to the real account", env.mail.Messages)
	}
	// Malformed addresses are a plain validation error, the same for every input.
	if rec := env.post("/auth/password/forgot", map[string]string{"email": "nope"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid email status = %d", rec.Code)
	}
	if rec := env.post("/auth/password/forgot", map[string]any{"email": "a@b.co", "extra": 1}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field status = %d", rec.Code)
	}
}

func TestPasswordResetLifecycle(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	user := env.register(t, "ana@example.com")
	sessionID, err := env.service.OpenSession(context.Background(), user.ID, "ua", env.clk.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	env.post("/auth/password/forgot", map[string]string{"email": "ana@example.com"})
	first := lastToken(t, env.mail)
	// A second request replaces the first token.
	env.post("/auth/password/forgot", map[string]string{"email": "ana@example.com"})
	second := lastToken(t, env.mail)
	if first == second {
		t.Fatal("tokens must be unique")
	}
	reset := func(token, pw, conf string) *httptest.ResponseRecorder {
		return env.post("/auth/password/reset", map[string]string{"token": token, "password": pw, "password_confirmation": conf})
	}
	if rec := reset(first, "Nuev0-Passw0rd!x", "Nuev0-Passw0rd!x"); rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "auth.reset_token_invalid" {
		t.Fatalf("replaced token: %d %s", rec.Code, rec.Body)
	}
	if rec := reset(second, "Nuev0-Passw0rd!x", "different"); errorCode(t, rec) != "user.password_mismatch" {
		t.Fatalf("mismatch: %d %s", rec.Code, rec.Body)
	}
	if rec := reset(second, "short", "short"); errorCode(t, rec) != "user.password_weak" {
		t.Fatalf("weak: %d %s", rec.Code, rec.Body)
	}
	// Lock the account first so the reset can be seen clearing the lockout.
	for i := 0; i < 5; i++ {
		env.login("ana@example.com", "wrong")
	}
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status = %d", rec.Code)
	}
	// The weak attempts above did not burn the token.
	if rec := reset(second, "Nuev0-Passw0rd!x", "Nuev0-Passw0rd!x"); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	if rec := reset(second, "Otro-Passw0rd!99", "Otro-Passw0rd!99"); errorCode(t, rec) != "auth.reset_token_invalid" {
		t.Fatalf("token reuse: %d %s", rec.Code, rec.Body)
	}
	if s := env.sessions.sessions[sessionID]; s.RevokedAt == nil {
		t.Fatal("reset must revoke every session")
	}
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password status = %d, want 401", rec.Code)
	}
	if rec := env.login("ana@example.com", "Nuev0-Passw0rd!x"); rec.Code != http.StatusOK {
		t.Fatalf("new password login = %d %s (lockout must be cleared)", rec.Code, rec.Body)
	}
}

func TestPasswordResetTokenExpires(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	env.register(t, "ana@example.com")
	env.post("/auth/password/forgot", map[string]string{"email": "ana@example.com"})
	token := lastToken(t, env.mail)
	env.clk.Advance(domain.PasswordResetTTL + time.Second)
	rec := env.post("/auth/password/reset", map[string]string{"token": token, "password": "Nuev0-Passw0rd!x", "password_confirmation": "Nuev0-Passw0rd!x"})
	if errorCode(t, rec) != "auth.reset_token_invalid" {
		t.Fatalf("expired token: %d %s", rec.Code, rec.Body)
	}
	if rec := env.post("/auth/password/reset", map[string]string{"token": "garbage", "password": "Nuev0-Passw0rd!x", "password_confirmation": "Nuev0-Passw0rd!x"}); errorCode(t, rec) != "auth.reset_token_invalid" {
		t.Fatalf("garbage token: %d %s", rec.Code, rec.Body)
	}
}

func TestLoginLockoutAndRecovery(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	env.register(t, "ana@example.com")
	for i := 0; i < 4; i++ {
		if rec := env.login("ana@example.com", "wrong"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", i+1, rec.Code)
		}
	}
	rec := env.login("ana@example.com", "wrong")
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "auth.account_locked" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("fifth failure: %d %s retry=%q", rec.Code, rec.Body, rec.Header().Get("Retry-After"))
	}
	// The right password does not help while locked, and the answer is the same as for a wrong one.
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login = %d", rec.Code)
	}
	// An address without an account locks the same way (no enumeration through the lockout).
	for i := 0; i < 5; i++ {
		rec = env.login("ghost@example.com", "wrong")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ghost lockout = %d", rec.Code)
	}
	env.clk.Advance(16 * time.Minute)
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("after the lockout: %d %s", rec.Code, rec.Body)
	}
	// Success cleared the counter: four new failures do not lock.
	for i := 0; i < 4; i++ {
		env.login("ana@example.com", "wrong")
	}
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("counter was not cleared: %d", rec.Code)
	}
}

func TestRateLimitedEndpoints(t *testing.T) {
	t.Parallel()
	clk := &clock.FakeClock{T: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)}
	limits := &userhttp.RateLimits{
		Login:  ratelimit.New(ratelimit.PerMinute(2), clk),
		Forgot: ratelimit.New(ratelimit.PerHour(1), clk),
	}
	env := newSecurityEnv(t, limits)
	env.register(t, "ana@example.com")

	env.login("ana@example.com", goodPassword)
	env.login("ana@example.com", goodPassword)
	rec := env.login("ana@example.com", goodPassword)
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "auth.rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("third login: %d %s", rec.Code, rec.Body)
	}
	if rec := env.post("/auth/password/forgot", map[string]string{"email": "ana@example.com"}); rec.Code != http.StatusAccepted {
		t.Fatalf("first forgot = %d", rec.Code)
	}
	if rec := env.post("/auth/password/forgot", map[string]string{"email": "other@example.com"}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second forgot from the same address = %d, want 429", rec.Code)
	}
	clk.Advance(2 * time.Hour)
	if rec := env.post("/auth/password/forgot", map[string]string{"email": "ana@example.com"}); rec.Code != http.StatusAccepted {
		t.Fatalf("forgot after the window = %d", rec.Code)
	}
}

func TestEmailVerificationFlow(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	user := env.register(t, "ana@example.com")
	bearer, err := authtoken.Generate(env.secret, time.Hour, user.ID, string(user.Role))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/email/verify/request", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body)
	}
	token := lastToken(t, env.mail)
	if rec := env.post("/auth/email/verify", map[string]string{"token": "bad"}); errorCode(t, rec) != "auth.verification_token_invalid" {
		t.Fatalf("bad token: %d %s", rec.Code, rec.Body)
	}
	if rec := env.post("/auth/email/verify", map[string]string{"token": token}); rec.Code != http.StatusNoContent {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	if rec := env.post("/auth/email/verify", map[string]string{"token": token}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("token reuse: %d", rec.Code)
	}
	stored, _ := env.repo.FindByID(context.Background(), user.ID)
	if stored.EmailVerifiedAt == nil {
		t.Fatal("email_verified_at was not set")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/email/verify/request", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("already verified: %d", rec.Code)
	}
}

func TestLoginRequiresVerifiedEmailWhenEnabled(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	env.service.WithEmailVerification(true, nil)
	user := env.register(t, "ana@example.com")
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusForbidden || errorCode(t, rec) != "auth.email_not_verified" {
		t.Fatalf("unverified login: %d %s", rec.Code, rec.Body)
	}
	now := env.clk.Now()
	user.EmailVerifiedAt = &now
	if err := env.repo.Update(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	if rec := env.login("ana@example.com", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("verified login: %d %s", rec.Code, rec.Body)
	}
}

func TestRegisterAppliesPasswordPolicy(t *testing.T) {
	t.Parallel()
	env := newSecurityEnv(t, nil)
	for _, pw := range []string{"short1A!", "alllowercase1", "Password123!", "ana.prueba-Secret9"} {
		rec := env.post("/auth/register", map[string]string{
			"first_name": "Ana", "last_name": "Prueba", "email": "ana.prueba@example.com", "password": pw, "password_confirmation": pw,
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%q status = %d %s", pw, rec.Code, rec.Body)
		}
		if code := errorCode(t, rec); code != "user.password_weak" && code != "validation.failed" {
			t.Fatalf("%q code = %q", pw, code)
		}
	}
}
