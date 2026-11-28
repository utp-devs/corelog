package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// fakeDetacher records the cascade calls and answers the sole-owner count.
type fakeDetacher struct {
	soleOwned int
	detached  []uuid.UUID
	err       error
}

func (d *fakeDetacher) SoleOwnedOrganizations(_ context.Context, _ uuid.UUID) (int, error) {
	return d.soleOwned, d.err
}

func (d *fakeDetacher) DetachMemberships(_ context.Context, userID uuid.UUID) error {
	if d.err != nil {
		return d.err
	}
	d.detached = append(d.detached, userID)
	return nil
}

func TestUserService_DeactivateBlockedForSoleOwner(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	tx := &database.FakeTxManager{}
	detacher := &fakeDetacher{soleOwned: 1}
	svc := NewUserService(repo, sessions).WithAccountClosure(tx, detacher)
	ctx := context.Background()
	userID := registerUser(t, svc, "owner@example.com", "clave-segura-Z9")

	err := svc.DeactivateAccount(ctx, userID, "clave-segura-Z9")
	if !errors.Is(err, domain.ErrSoleOwner) {
		t.Fatalf("want ErrSoleOwner, got %v", err)
	}
	if apperror.KindOf(err) != apperror.KindConflict || apperror.CodeOf(err) != "user.sole_owner" {
		t.Fatalf("kind/code = %v/%s, want conflict/user.sole_owner", apperror.KindOf(err), apperror.CodeOf(err))
	}
	if len(detacher.detached) != 0 {
		t.Fatalf("memberships must not be removed when the closure is blocked")
	}
	user, err := repo.FindByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !user.IsActive() {
		t.Fatal("account must stay active when the closure is blocked")
	}
}

func TestUserService_DeactivateCascadesMemberships(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	tx := &database.FakeTxManager{}
	detacher := &fakeDetacher{}
	svc := NewUserService(repo, sessions).WithAccountClosure(tx, detacher)
	ctx := context.Background()
	userID := registerUser(t, svc, "member@example.com", "clave-segura-Z9")
	if _, err := svc.OpenSession(ctx, userID, "test", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeactivateAccount(ctx, userID, "clave-segura-Z9"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if tx.Calls != 1 {
		t.Fatalf("deactivation must run in one transaction, InTx calls = %d", tx.Calls)
	}
	if len(detacher.detached) != 1 || detacher.detached[0] != userID {
		t.Fatalf("detached = %v, want [%s]", detacher.detached, userID)
	}
	user, err := repo.FindByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if user.IsActive() {
		t.Fatal("account must be deactivated")
	}
}

func TestUserService_DeactivatePropagatesCascadeError(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	detacher := &fakeDetacher{err: errors.New("boom")}
	svc := NewUserService(repo, sessions).WithAccountClosure(&database.FakeTxManager{}, detacher)
	userID := registerUser(t, svc, "fail@example.com", "clave-segura-Z9")

	if err := svc.DeactivateAccount(context.Background(), userID, "clave-segura-Z9"); err == nil {
		t.Fatal("cascade error must be returned")
	}
}
