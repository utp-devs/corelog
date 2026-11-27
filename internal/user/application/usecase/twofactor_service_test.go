package usecase

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/crypto"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type fakeTwoFactorRepo struct {
	setups     map[uuid.UUID]*port.TwoFactorSetup
	codes      map[uuid.UUID][]fakeCode
	challenges map[uuid.UUID]*port.LoginChallenge
}

type fakeCode struct {
	id   uuid.UUID
	hash []byte
	used bool
}

func newFakeTwoFactorRepo() *fakeTwoFactorRepo {
	return &fakeTwoFactorRepo{
		setups:     map[uuid.UUID]*port.TwoFactorSetup{},
		codes:      map[uuid.UUID][]fakeCode{},
		challenges: map[uuid.UUID]*port.LoginChallenge{},
	}
}

func (r *fakeTwoFactorRepo) FindSetup(_ context.Context, userID uuid.UUID) (*port.TwoFactorSetup, error) {
	s, ok := r.setups[userID]
	if !ok {
		return nil, nil
	}
	copied := *s
	return &copied, nil
}

func (r *fakeTwoFactorRepo) SaveSetup(_ context.Context, userID uuid.UUID, sealed []byte) (bool, error) {
	if s, ok := r.setups[userID]; ok && s.EnabledAt != nil {
		return false, nil
	}
	r.setups[userID] = &port.TwoFactorSetup{UserID: userID, SecretEncrypted: sealed}
	return true, nil
}

func (r *fakeTwoFactorRepo) MarkEnabled(_ context.Context, userID uuid.UUID, at time.Time, step int64) (bool, error) {
	s, ok := r.setups[userID]
	if !ok || s.EnabledAt != nil {
		return false, nil
	}
	s.EnabledAt = &at
	s.LastUsedStep = &step
	return true, nil
}

func (r *fakeTwoFactorRepo) AdvanceLastUsedStep(_ context.Context, userID uuid.UUID, step int64) (bool, error) {
	s := r.setups[userID]
	if s.LastUsedStep != nil && *s.LastUsedStep >= step {
		return false, nil
	}
	s.LastUsedStep = &step
	return true, nil
}

func (r *fakeTwoFactorRepo) ListUnusedRecoveryCodes(_ context.Context, userID uuid.UUID) ([]port.RecoveryCodeRecord, error) {
	var out []port.RecoveryCodeRecord
	for _, c := range r.codes[userID] {
		if !c.used {
			out = append(out, port.RecoveryCodeRecord{ID: c.id, CodeHash: c.hash})
		}
	}
	return out, nil
}

func (r *fakeTwoFactorRepo) CountUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) (int, error) {
	list, _ := r.ListUnusedRecoveryCodes(ctx, userID)
	return len(list), nil
}

func (r *fakeTwoFactorRepo) MarkRecoveryCodeUsed(_ context.Context, userID, codeID uuid.UUID, at time.Time) (bool, error) {
	for i := range r.codes[userID] {
		if r.codes[userID][i].id == codeID && !r.codes[userID][i].used {
			r.codes[userID][i].used = true
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTwoFactorRepo) ReplaceRecoveryCodes(_ context.Context, userID uuid.UUID, hashes [][]byte) error {
	list := make([]fakeCode, 0, len(hashes))
	for _, h := range hashes {
		list = append(list, fakeCode{id: uuid.New(), hash: h})
	}
	r.codes[userID] = list
	return nil
}

func (r *fakeTwoFactorRepo) Delete(_ context.Context, userID uuid.UUID) error {
	delete(r.setups, userID)
	delete(r.codes, userID)
	return nil
}

func (r *fakeTwoFactorRepo) CreateChallenge(_ context.Context, userID uuid.UUID, expiresAt time.Time) (uuid.UUID, error) {
	id := uuid.New()
	r.challenges[id] = &port.LoginChallenge{ID: id, UserID: userID, ExpiresAt: expiresAt}
	return id, nil
}

func (r *fakeTwoFactorRepo) FindChallenge(_ context.Context, id uuid.UUID) (*port.LoginChallenge, error) {
	c, ok := r.challenges[id]
	if !ok {
		return nil, nil
	}
	copied := *c
	return &copied, nil
}

func (r *fakeTwoFactorRepo) RecordFailedChallengeAttempt(_ context.Context, id uuid.UUID, max int, at time.Time) error {
	c := r.challenges[id]
	if c.ConsumedAt != nil {
		return nil
	}
	c.FailedAttempts++
	if c.FailedAttempts >= max {
		c.ConsumedAt = &at
	}
	return nil
}

func (r *fakeTwoFactorRepo) ConsumeChallenge(_ context.Context, id uuid.UUID, at time.Time, max int) (bool, error) {
	c := r.challenges[id]
	if c.ConsumedAt != nil || !at.Before(c.ExpiresAt) || c.FailedAttempts >= max {
		return false, nil
	}
	c.ConsumedAt = &at
	return true, nil
}

type fakeEventPublisher struct {
	events []events.Event
}

func (p *fakeEventPublisher) Publish(_ context.Context, e events.Event) error {
	p.events = append(p.events, e)
	return nil
}

func (p *fakeEventPublisher) types() []string {
	out := make([]string, 0, len(p.events))
	for _, e := range p.events {
		out = append(out, e.Type)
	}
	return out
}

type twoFactorFixture struct {
	svc       *TwoFactorService
	users     *fakeUserRepo
	repo      *fakeTwoFactorRepo
	pub       *fakeEventPublisher
	tx        *database.FakeTxManager
	clk       *clock.FakeClock
	userID    uuid.UUID
	password  string
	secretRaw domain.TOTPSecret
}

func newTwoFactorFixture(t *testing.T) *twoFactorFixture {
	t.Helper()
	users := newFakeUserRepo()
	password := "contrasena-segura-1"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	u, err := domain.NewUser("Ana", "ana@example.com", string(hash), domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.Save(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	key := bytes.Repeat([]byte{9}, crypto.KeySize)
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	f := &twoFactorFixture{
		users:    users,
		repo:     newFakeTwoFactorRepo(),
		pub:      &fakeEventPublisher{},
		tx:       &database.FakeTxManager{},
		clk:      &clock.FakeClock{T: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)},
		userID:   u.ID,
		password: password,
	}
	f.svc = NewTwoFactorService(users, f.repo, cipher, f.tx, f.pub, f.clk)
	return f
}

// enroll starts enrollment and confirms it with the current code. It returns the recovery codes.
func (f *twoFactorFixture) enroll(t *testing.T) []string {
	t.Helper()
	enrollment, err := f.svc.Enroll(context.Background(), f.userID)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	secret, err := domain.ParseTOTPSecret(enrollment.Secret)
	if err != nil {
		t.Fatalf("parse secret: %v", err)
	}
	f.secretRaw = secret
	codes, err := f.svc.ConfirmEnrollment(context.Background(), f.userID, f.code())
	if err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}
	f.clk.Advance(domain.TOTPPeriod) // a new step, so the confirmation code cannot be reused
	return codes
}

// code returns the TOTP code of the current instant.
func (f *twoFactorFixture) code() string {
	return domain.TOTPAt(f.secretRaw, domain.StepOf(f.clk.Now()))
}

func assertAppCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error %s, got nil", want)
	}
	if got := apperror.CodeOf(err); got != want {
		t.Fatalf("code %q, want %q (%v)", got, want, err)
	}
}

func TestEnrollReturnsSecretOnceAndStoresItEncrypted(t *testing.T) {
	f := newTwoFactorFixture(t)
	enrollment, err := f.svc.Enroll(context.Background(), f.userID)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if enrollment.Secret == "" || enrollment.OTPAuthURI == "" {
		t.Fatal("enrollment must return the secret and the otpauth URI")
	}
	stored := f.repo.setups[f.userID]
	if stored == nil || stored.EnabledAt != nil {
		t.Fatal("enrollment must store a pending setup")
	}
	if bytes.Contains(stored.SecretEncrypted, []byte(enrollment.Secret)) {
		t.Fatal("stored secret must not contain the plaintext base32 value")
	}
	enabled, err := f.svc.IsEnabled(context.Background(), f.userID)
	if err != nil || enabled {
		t.Fatalf("pending enrollment must not enable the second factor: enabled=%v err=%v", enabled, err)
	}
}

func TestConfirmEnrollmentEnablesAndReturnsRecoveryCodes(t *testing.T) {
	f := newTwoFactorFixture(t)
	ctx := context.Background()
	enrollment, err := f.svc.Enroll(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	f.secretRaw, err = domain.ParseTOTPSecret(enrollment.Secret)
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.svc.ConfirmEnrollment(ctx, f.userID, f.wrongCode())
	assertAppCode(t, err, "auth.two_factor_invalid")

	codes, err := f.svc.ConfirmEnrollment(ctx, f.userID, f.code())
	if err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}
	if len(codes) != domain.RecoveryCodeCount {
		t.Fatalf("got %d recovery codes, want %d", len(codes), domain.RecoveryCodeCount)
	}
	status, err := f.svc.Status(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.RecoveryCodesRemaining != domain.RecoveryCodeCount {
		t.Fatalf("unexpected status %+v", status)
	}
	if got := f.pub.types(); len(got) != 1 || got[0] != EventTwoFactorEnabled {
		t.Fatalf("expected one %s event, got %v", EventTwoFactorEnabled, got)
	}
	if f.tx.Calls == 0 {
		t.Fatal("confirmation must run in a transaction")
	}
}

// wrongCode returns a six digit code that no step in the accepted window would match.
func (f *twoFactorFixture) wrongCode() string {
	valid := map[string]bool{}
	step := domain.StepOf(f.clk.Now())
	for d := int64(-1); d <= 1; d++ {
		valid[domain.TOTPAt(f.secretRaw, step+d)] = true
	}
	for _, c := range []string{"000000", "000001", "000002", "000003"} {
		if !valid[c] {
			return c
		}
	}
	return "000004"
}

func TestConfirmEnrollmentErrors(t *testing.T) {
	f := newTwoFactorFixture(t)
	_, err := f.svc.ConfirmEnrollment(context.Background(), f.userID, "123456")
	assertAppCode(t, err, "auth.two_factor_not_enrolled")

	f.enroll(t)
	_, err = f.svc.ConfirmEnrollment(context.Background(), f.userID, f.code())
	assertAppCode(t, err, "auth.two_factor_already_enabled")

	_, err = f.svc.Enroll(context.Background(), f.userID)
	assertAppCode(t, err, "auth.two_factor_already_enabled")
}

func TestLoginChallengeAcceptsCurrentCodeOnce(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	ctx := context.Background()

	challengeID, expiresAt, err := f.svc.StartChallenge(ctx, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if got := expiresAt.Sub(f.clk.Now()); got != domain.ChallengeTTL {
		t.Fatalf("challenge TTL %v, want %v", got, domain.ChallengeTTL)
	}
	user, err := f.svc.CompleteChallenge(ctx, challengeID, f.userID, port.CompleteLoginInput{Code: f.code()})
	if err != nil {
		t.Fatalf("CompleteChallenge: %v", err)
	}
	if user.ID != f.userID {
		t.Fatal("challenge returned another user")
	}

	second, _, _ := f.svc.StartChallenge(ctx, f.userID)
	_, err = f.svc.CompleteChallenge(ctx, second, f.userID, port.CompleteLoginInput{Code: f.code()})
	assertAppCode(t, err, "auth.two_factor_invalid")
}

func TestLoginChallengeRejectsReplayAcrossChallenges(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	ctx := context.Background()
	code := f.code()

	first, _, _ := f.svc.StartChallenge(ctx, f.userID)
	if _, err := f.svc.CompleteChallenge(ctx, first, f.userID, port.CompleteLoginInput{Code: code}); err != nil {
		t.Fatalf("first use: %v", err)
	}
	second, _, _ := f.svc.StartChallenge(ctx, f.userID)
	_, err := f.svc.CompleteChallenge(ctx, second, f.userID, port.CompleteLoginInput{Code: code})
	assertAppCode(t, err, "auth.two_factor_invalid")
}

func TestLoginChallengeWindowTolerance(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	ctx := context.Background()

	// The enrollment consumed the step of the current instant; move on so that the previous step is unused.
	f.clk.Advance(domain.TOTPPeriod)
	previous := domain.TOTPAt(f.secretRaw, domain.StepOf(f.clk.Now())-1)
	id, _, _ := f.svc.StartChallenge(ctx, f.userID)
	if _, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{Code: previous}); err != nil {
		t.Fatalf("code from the previous step must be accepted: %v", err)
	}

	f.clk.Advance(domain.TOTPPeriod * 3)
	stale := domain.TOTPAt(f.secretRaw, domain.StepOf(f.clk.Now())-3)
	id, _, _ = f.svc.StartChallenge(ctx, f.userID)
	_, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{Code: stale})
	assertAppCode(t, err, "auth.two_factor_invalid")
}

func TestLoginChallengeAttemptLimit(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	ctx := context.Background()
	f.clk.Advance(domain.TOTPPeriod)

	id, _, _ := f.svc.StartChallenge(ctx, f.userID)
	for i := 0; i < domain.ChallengeMaxAttempts; i++ {
		_, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{Code: f.wrongCode()})
		assertAppCode(t, err, "auth.two_factor_invalid")
	}
	// Even the right code no longer works once the attempts are exhausted.
	_, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{Code: f.code()})
	assertAppCode(t, err, "auth.challenge_expired")
}

func TestLoginChallengeExpires(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	ctx := context.Background()

	id, _, _ := f.svc.StartChallenge(ctx, f.userID)
	code := f.code()
	f.clk.Advance(domain.ChallengeTTL)
	_, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{Code: code})
	assertAppCode(t, err, "auth.challenge_expired")
}

func TestLoginChallengeRejectsOtherUser(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	ctx := context.Background()
	id, _, _ := f.svc.StartChallenge(ctx, f.userID)
	_, err := f.svc.CompleteChallenge(ctx, id, uuid.New(), port.CompleteLoginInput{Code: f.code()})
	assertAppCode(t, err, "auth.challenge_expired")
}

func TestRecoveryCodeIsSingleUse(t *testing.T) {
	f := newTwoFactorFixture(t)
	codes := f.enroll(t)
	ctx := context.Background()

	id, _, _ := f.svc.StartChallenge(ctx, f.userID)
	if _, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{RecoveryCode: codes[0]}); err != nil {
		t.Fatalf("first use of a recovery code: %v", err)
	}
	status, _ := f.svc.Status(ctx, f.userID)
	if status.RecoveryCodesRemaining != domain.RecoveryCodeCount-1 {
		t.Fatalf("remaining %d, want %d", status.RecoveryCodesRemaining, domain.RecoveryCodeCount-1)
	}

	again, _, _ := f.svc.StartChallenge(ctx, f.userID)
	_, err := f.svc.CompleteChallenge(ctx, again, f.userID, port.CompleteLoginInput{RecoveryCode: codes[0]})
	assertAppCode(t, err, "auth.two_factor_invalid")

	// Normalization: lower case and dashes are accepted for the other codes.
	typed := []byte(codes[1])
	typed = append(typed[:5], append([]byte{'-'}, typed[5:]...)...)
	third, _, _ := f.svc.StartChallenge(ctx, f.userID)
	if _, err := f.svc.CompleteChallenge(ctx, third, f.userID, port.CompleteLoginInput{RecoveryCode: string(typed)}); err != nil {
		t.Fatalf("recovery code with a dash: %v", err)
	}
}

func TestLoginChallengeRequiresSecondFactorInput(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.enroll(t)
	id, _, _ := f.svc.StartChallenge(context.Background(), f.userID)
	_, err := f.svc.CompleteChallenge(context.Background(), id, f.userID, port.CompleteLoginInput{})
	assertAppCode(t, err, "auth.two_factor_required")
}

func TestUserWithoutTwoFactorIsNotChallenged(t *testing.T) {
	f := newTwoFactorFixture(t)
	enabled, err := f.svc.IsEnabled(context.Background(), f.userID)
	if err != nil || enabled {
		t.Fatalf("user without setup: enabled=%v err=%v", enabled, err)
	}
	status, err := f.svc.Status(context.Background(), f.userID)
	if err != nil || status.Enabled {
		t.Fatalf("status of user without setup: %+v err=%v", status, err)
	}
}

func TestDisableRequiresPasswordAndSecondFactor(t *testing.T) {
	f := newTwoFactorFixture(t)
	codes := f.enroll(t)
	ctx := context.Background()
	f.clk.Advance(domain.TOTPPeriod)

	err := f.svc.Disable(ctx, f.userID, "contrasena-incorrecta", port.CompleteLoginInput{Code: f.code()})
	if !errors.Is(err, domain.ErrIncorrectPassword) {
		t.Fatalf("wrong password: got %v", err)
	}
	err = f.svc.Disable(ctx, f.userID, f.password, port.CompleteLoginInput{})
	assertAppCode(t, err, "auth.two_factor_required")
	err = f.svc.Disable(ctx, f.userID, f.password, port.CompleteLoginInput{Code: f.wrongCode()})
	assertAppCode(t, err, "auth.two_factor_invalid")

	if err := f.svc.Disable(ctx, f.userID, f.password, port.CompleteLoginInput{RecoveryCode: codes[2]}); err != nil {
		t.Fatalf("disable with recovery code: %v", err)
	}
	enabled, _ := f.svc.IsEnabled(ctx, f.userID)
	if enabled {
		t.Fatal("second factor still enabled after disable")
	}
	if len(f.repo.codes[f.userID]) != 0 {
		t.Fatal("recovery codes must be removed on disable")
	}
	if got := f.pub.types(); got[len(got)-1] != EventTwoFactorDisabled {
		t.Fatalf("last event %s, want %s", got[len(got)-1], EventTwoFactorDisabled)
	}
}

func TestDisableWithoutSetupIsConflict(t *testing.T) {
	f := newTwoFactorFixture(t)
	err := f.svc.Disable(context.Background(), f.userID, f.password, port.CompleteLoginInput{Code: "123456"})
	assertAppCode(t, err, "auth.two_factor_not_enabled")
}

func TestRegenerateRecoveryCodesInvalidatesOldOnes(t *testing.T) {
	f := newTwoFactorFixture(t)
	old := f.enroll(t)
	ctx := context.Background()
	f.clk.Advance(domain.TOTPPeriod)

	_, err := f.svc.RegenerateRecoveryCodes(ctx, f.userID, f.wrongCode())
	assertAppCode(t, err, "auth.two_factor_invalid")

	fresh, err := f.svc.RegenerateRecoveryCodes(ctx, f.userID, f.code())
	if err != nil {
		t.Fatalf("RegenerateRecoveryCodes: %v", err)
	}
	if len(fresh) != domain.RecoveryCodeCount {
		t.Fatalf("got %d codes", len(fresh))
	}
	f.clk.Advance(domain.TOTPPeriod)
	id, _, _ := f.svc.StartChallenge(ctx, f.userID)
	_, err = f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{RecoveryCode: old[0]})
	assertAppCode(t, err, "auth.two_factor_invalid")

	id, _, _ = f.svc.StartChallenge(ctx, f.userID)
	if _, err := f.svc.CompleteChallenge(ctx, id, f.userID, port.CompleteLoginInput{RecoveryCode: fresh[0]}); err != nil {
		t.Fatalf("new recovery code: %v", err)
	}
}

func TestRegenerateRequiresEnabledSecondFactor(t *testing.T) {
	f := newTwoFactorFixture(t)
	_, err := f.svc.RegenerateRecoveryCodes(context.Background(), f.userID, "123456")
	assertAppCode(t, err, "auth.two_factor_not_enabled")
}
