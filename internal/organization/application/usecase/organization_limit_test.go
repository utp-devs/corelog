package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestCreateOrganizationLimitOfOwnedOrganizations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	creator := uuid.New()
	for i := 0; i < domain.MaxOwnedOrganizations; i++ {
		name := "Org " + string(rune('A'+i))
		if _, err := f.svc.CreateOrganization(ctx, creator, port.CreateInput{Name: name}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	_, err := f.svc.CreateOrganization(ctx, creator, port.CreateInput{Name: "Sexta"})
	requireErr(t, err, apperror.KindConflict, "org.limit_reached")

	// The limit counts ownership, not membership: another user may still create organizations.
	other := uuid.New()
	if _, err := f.svc.CreateOrganization(ctx, other, port.CreateInput{Name: "Ajena"}); err != nil {
		t.Fatalf("other user create: %v", err)
	}
}

func TestUpdateDetailsChangesNameAndKeepsUnsetFields(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	org, err := f.svc.CreateOrganization(ctx, owner, port.CreateInput{Name: "Antes", Description: "Desc"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newName := "  Despues  "
	updated, err := f.svc.UpdateOrganization(ctx, owner, org.ID, port.UpdateInput{Name: &newName, Settings: fullPatch(org.Settings)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Despues" || updated.Description != "Desc" {
		t.Fatalf("name/description = %q/%q", updated.Name, updated.Description)
	}

	empty := "   "
	_, err = f.svc.UpdateOrganization(ctx, owner, org.ID, port.UpdateInput{Name: &empty, Settings: fullPatch(org.Settings)})
	requireErr(t, err, apperror.KindValidation, "org.name_required")
}

// fullPatch expresses a complete settings value as a patch that sets every field.
func fullPatch(s domain.Settings) *domain.SettingsPatch {
	return &domain.SettingsPatch{
		WhoCanCreateProjects:     &s.WhoCanCreateProjects,
		WhoCanCreateTeams:        &s.WhoCanCreateTeams,
		DefaultProjectVisibility: &s.DefaultProjectVisibility,
		Require2FA:               &s.Require2FA,
		MaxUploadBytes:           &s.MaxUploadBytes,
		HoursPerDay:              &s.HoursPerDay,
		DaysPerWeek:              &s.DaysPerWeek,
		Timezone:                 &s.Timezone,
	}
}
