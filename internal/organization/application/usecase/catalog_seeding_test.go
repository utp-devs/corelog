package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
)

// recordingSeeder records the organizations it seeded and can fail on demand.
type recordingSeeder struct {
	seeded []uuid.UUID
	err    error
}

func (s *recordingSeeder) SeedDefaults(_ context.Context, orgID uuid.UUID) error {
	if s.err != nil {
		return s.err
	}
	s.seeded = append(s.seeded, orgID)
	return nil
}

func TestCreateOrganizationSeedsDefaultCatalogs(t *testing.T) {
	f := newFixture(t)
	seeder := &recordingSeeder{}
	f.svc.WithCatalogSeeder(seeder)

	org, err := f.svc.CreateOrganization(context.Background(), uuid.New(), port.CreateInput{Name: "Seeded Org"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(seeder.seeded) != 1 || seeder.seeded[0] != org.ID {
		t.Fatalf("seeded = %v, want [%s]", seeder.seeded, org.ID)
	}
}

func TestCreateOrganizationFailsWhenCatalogSeedingFails(t *testing.T) {
	f := newFixture(t)
	boom := errors.New("seed failed")
	f.svc.WithCatalogSeeder(&recordingSeeder{err: boom})

	if _, err := f.svc.CreateOrganization(context.Background(), uuid.New(), port.CreateInput{Name: "Broken Org"}); !errors.Is(err, boom) {
		t.Fatalf("create error = %v, want the seeding error", err)
	}
}
