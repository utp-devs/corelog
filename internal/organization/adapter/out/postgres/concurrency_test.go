package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/application/usecase"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// newRealService wires the use cases over PostgreSQL, as cmd/api does.
func newRealService(db *sqlx.DB) (*usecase.OrganizationService, *postgres.Repository) {
	repo := postgres.NewRepository(db)
	pub := events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) })
	return usecase.NewOrganizationService(repo, database.NewTxManager(db), pub, clock.RealClock{}), repo
}

// TestConcurrentCreatesRespectOwnedLimit starts eight creations for the same owner at once. The advisory
// lock must let exactly MaxOwnedOrganizations through; the rest get org.limit_reached.
func TestConcurrentCreatesRespectOwnedLimit(t *testing.T) {
	db := testdb.New(t)
	svc, repo := newRealService(db)
	ctx := context.Background()
	owner := newUser(t, db)

	const attempts = 8
	start := make(chan struct{})
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.CreateOrganization(ctx, owner, port.CreateInput{Name: fmt.Sprintf("Concurrente %d", i)})
		}(i)
	}
	close(start)
	wg.Wait()

	succeeded, limited := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrLimitReached):
			limited++
		default:
			t.Errorf("attempt %d: unexpected error %v", i, err)
		}
	}
	if succeeded != domain.MaxOwnedOrganizations || limited != attempts-domain.MaxOwnedOrganizations {
		t.Fatalf("succeeded %d, limited %d; want %d and %d", succeeded, limited,
			domain.MaxOwnedOrganizations, attempts-domain.MaxOwnedOrganizations)
	}
	owned, err := repo.CountOwnedBy(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if owned != domain.MaxOwnedOrganizations {
		t.Fatalf("stored owned organizations = %d, want %d", owned, domain.MaxOwnedOrganizations)
	}
}

// TestConcurrentPatchesOnDifferentSettingsKeepBothChanges runs two patches at once, each touching another
// settings key. Both changes must be present afterwards: the row lock serialises the read-merge-write.
func TestConcurrentPatchesOnDifferentSettingsKeepBothChanges(t *testing.T) {
	db := testdb.New(t)
	svc, repo := newRealService(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org, err := svc.CreateOrganization(ctx, owner, port.CreateInput{Name: "Parches"})
	if err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 10; round++ {
		off := false
		if _, err := svc.UpdateOrganization(ctx, owner, org.ID, port.UpdateInput{
			Settings: &domain.SettingsPatch{Require2FA: &off},
		}); err != nil {
			t.Fatalf("reset: %v", err)
		}

		on := true
		hours := float64(round)
		patches := []port.UpdateInput{
			{Settings: &domain.SettingsPatch{Require2FA: &on}},
			{Settings: &domain.SettingsPatch{HoursPerDay: &hours}},
		}
		start := make(chan struct{})
		errs := make([]error, len(patches))
		var wg sync.WaitGroup
		for i := range patches {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = svc.UpdateOrganization(ctx, owner, org.ID, patches[i])
			}(i)
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d patch %d: %v", round, i, err)
			}
		}

		stored, err := repo.GetByID(ctx, org.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !stored.Settings.Require2FA || stored.Settings.HoursPerDay != hours {
			t.Fatalf("round %d: lost update, require_2fa=%v hours_per_day=%v (want true and %v)",
				round, stored.Settings.Require2FA, stored.Settings.HoursPerDay, hours)
		}
	}
}
