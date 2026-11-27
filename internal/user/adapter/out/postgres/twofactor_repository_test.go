package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func TestTwoFactorRepositoryPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	users := NewUserRepository(db)
	repo := NewTwoFactorRepository(db)

	user, err := domain.NewUser("Ana Prueba", "ana.2fa@example.com", "hash", domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.Save(ctx, user); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)

	if setup, err := repo.FindSetup(ctx, user.ID); err != nil || setup != nil {
		t.Fatalf("no setup expected: %v %v", setup, err)
	}

	saved, err := repo.SaveSetup(ctx, user.ID, []byte("sealed-1"))
	if err != nil || !saved {
		t.Fatalf("SaveSetup: saved=%v err=%v", saved, err)
	}
	saved, err = repo.SaveSetup(ctx, user.ID, []byte("sealed-2"))
	if err != nil || !saved {
		t.Fatalf("a pending setup is replaced: saved=%v err=%v", saved, err)
	}
	setup, err := repo.FindSetup(ctx, user.ID)
	if err != nil || setup == nil || string(setup.SecretEncrypted) != "sealed-2" || setup.EnabledAt != nil {
		t.Fatalf("pending setup: %+v err=%v", setup, err)
	}

	enabled, err := repo.MarkEnabled(ctx, user.ID, now, 100)
	if err != nil || !enabled {
		t.Fatalf("MarkEnabled: %v %v", enabled, err)
	}
	if again, _ := repo.MarkEnabled(ctx, user.ID, now, 101); again {
		t.Fatal("MarkEnabled must succeed only once")
	}
	if saved, _ := repo.SaveSetup(ctx, user.ID, []byte("sealed-3")); saved {
		t.Fatal("an enabled setup must not be overwritten by SaveSetup")
	}

	if ok, _ := repo.AdvanceLastUsedStep(ctx, user.ID, 100); ok {
		t.Fatal("the step already used must not advance")
	}
	if ok, _ := repo.AdvanceLastUsedStep(ctx, user.ID, 101); !ok {
		t.Fatal("a later step must advance")
	}

	hashes := [][]byte{domain.HashRecoveryCode("AAAAA22222"), domain.HashRecoveryCode("BBBBB33333")}
	if err := repo.ReplaceRecoveryCodes(ctx, user.ID, hashes); err != nil {
		t.Fatalf("ReplaceRecoveryCodes: %v", err)
	}
	records, err := repo.ListUnusedRecoveryCodes(ctx, user.ID)
	if err != nil || len(records) != 2 {
		t.Fatalf("unused codes: %d %v", len(records), err)
	}
	if ok, _ := repo.MarkRecoveryCodeUsed(ctx, user.ID, records[0].ID, now); !ok {
		t.Fatal("first use must succeed")
	}
	if ok, _ := repo.MarkRecoveryCodeUsed(ctx, user.ID, records[0].ID, now); ok {
		t.Fatal("second use must fail")
	}
	if count, _ := repo.CountUnusedRecoveryCodes(ctx, user.ID); count != 1 {
		t.Fatalf("remaining %d, want 1", count)
	}
	other := uuid.New()
	if ok, _ := repo.MarkRecoveryCodeUsed(ctx, other, records[1].ID, now); ok {
		t.Fatal("a code of another user must not be consumed")
	}

	challengeID, err := repo.CreateChallenge(ctx, user.ID, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	for i := 0; i < domain.ChallengeMaxAttempts-1; i++ {
		if err := repo.RecordFailedChallengeAttempt(ctx, challengeID, domain.ChallengeMaxAttempts, now); err != nil {
			t.Fatal(err)
		}
	}
	ch, err := repo.FindChallenge(ctx, challengeID)
	if err != nil || ch == nil || ch.FailedAttempts != domain.ChallengeMaxAttempts-1 || ch.ConsumedAt != nil {
		t.Fatalf("challenge after %d failures: %+v %v", domain.ChallengeMaxAttempts-1, ch, err)
	}
	if err := repo.RecordFailedChallengeAttempt(ctx, challengeID, domain.ChallengeMaxAttempts, now); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.ConsumeChallenge(ctx, challengeID, now, domain.ChallengeMaxAttempts); ok {
		t.Fatal("an exhausted challenge must not be consumed")
	}

	fresh, _ := repo.CreateChallenge(ctx, user.ID, now.Add(5*time.Minute))
	if ok, _ := repo.ConsumeChallenge(ctx, fresh, now.Add(time.Minute), domain.ChallengeMaxAttempts); !ok {
		t.Fatal("a valid challenge must be consumed")
	}
	if ok, _ := repo.ConsumeChallenge(ctx, fresh, now.Add(time.Minute), domain.ChallengeMaxAttempts); ok {
		t.Fatal("a challenge must be consumed once")
	}
	expired, _ := repo.CreateChallenge(ctx, user.ID, now)
	if ok, _ := repo.ConsumeChallenge(ctx, expired, now.Add(time.Second), domain.ChallengeMaxAttempts); ok {
		t.Fatal("an expired challenge must not be consumed")
	}
	if missing, err := repo.FindChallenge(ctx, uuid.New()); err != nil || missing != nil {
		t.Fatalf("unknown challenge: %v %v", missing, err)
	}

	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if setup, _ := repo.FindSetup(ctx, user.ID); setup != nil {
		t.Fatal("setup must be removed")
	}
	if count, _ := repo.CountUnusedRecoveryCodes(ctx, user.ID); count != 0 {
		t.Fatalf("recovery codes left after delete: %d", count)
	}
}
