package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func TestLogoRepositorySetGetClear(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, postgres.NewRepository(db), owner, "Logo Org", "logo-org-"+uuid.NewString()[:8])
	repo := postgres.NewLogoRepository(db)

	key1 := "44444444444444444444444444444444"
	key2 := "55555555555555555555555555555555"

	if got, err := repo.GetLogoKey(ctx, org.ID); err != nil || got != nil {
		t.Fatalf("fresh org: key = %v, err = %v; want nil", got, err)
	}
	if old, err := repo.SetLogoKey(ctx, org.ID, &key1); err != nil || old != nil {
		t.Fatalf("first set: old = %v, err = %v", old, err)
	}
	if old, err := repo.SetLogoKey(ctx, org.ID, &key2); err != nil || old == nil || *old != key1 {
		t.Fatalf("replace: old = %v, err = %v; want %s", old, err, key1)
	}
	if got, err := repo.GetLogoKey(ctx, org.ID); err != nil || got == nil || *got != key2 {
		t.Fatalf("get after replace: key = %v, err = %v; want %s", got, err, key2)
	}
	if old, err := repo.SetLogoKey(ctx, org.ID, nil); err != nil || old == nil || *old != key2 {
		t.Fatalf("clear: old = %v, err = %v; want %s", old, err, key2)
	}
	if got, _ := repo.GetLogoKey(ctx, org.ID); got != nil {
		t.Fatalf("after clear: key = %v, want nil", *got)
	}
}

func TestLogoRepositoryUnknownAndDeletedOrganizations(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	owner := newUser(t, db)
	repo := postgres.NewLogoRepository(db)
	key := "66666666666666666666666666666666"

	unknown := uuid.New()
	if got, err := repo.GetLogoKey(ctx, unknown); err != nil || got != nil {
		t.Fatalf("get unknown: key = %v, err = %v; want nil, nil", got, err)
	}
	if _, err := repo.SetLogoKey(ctx, unknown, &key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("set unknown: err = %v, want ErrNotFound", err)
	}

	orgs := postgres.NewRepository(db)
	org := newOrg(t, orgs, owner, "Borrada", "borrada-"+uuid.NewString()[:8])
	if err := orgs.SoftDelete(ctx, org.ID, testNow); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.SetLogoKey(ctx, org.ID, &key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("set on deleted: err = %v, want ErrNotFound", err)
	}
	if got, err := repo.GetLogoKey(ctx, org.ID); err != nil || got != nil {
		t.Fatalf("get on deleted: key = %v, err = %v; want nil, nil", got, err)
	}
}
