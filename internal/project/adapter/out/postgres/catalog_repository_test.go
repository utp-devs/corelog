package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

func TestLabelUpdateRenamesRecolorsAndRejectsDuplicates(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	catalog := NewCatalogRepository(db)
	leadID := insertUser(t, db)
	orgID := uuid.MustParse(defaultOrganizationID)

	project := newDomainProject(t, orgID, leadID, "LBL", domain.TemplateKanban, domain.VisibilityOpen)
	if err := projects.CreateWithDefaults(ctx, project, port.Member{UserID: leadID, Role: "lead"}); err != nil {
		t.Fatal(err)
	}
	bug, err := domain.NewLabel(project.ID, "bug", "red")
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateLabel(ctx, bug); err != nil {
		t.Fatal(err)
	}
	ux, err := domain.NewLabel(project.ID, "ux", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateLabel(ctx, ux); err != nil {
		t.Fatal(err)
	}
	if ux.Color != "blue" {
		t.Fatalf("default color = %q, want blue", ux.Color)
	}

	found, err := catalog.FindLabel(ctx, project.ID, bug.ID)
	if err != nil {
		t.Fatal(err)
	}
	newName := "defecto"
	newColor := "teal"
	if err := found.Update(&newName, &newColor); err != nil {
		t.Fatal(err)
	}
	if err := catalog.UpdateLabel(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := catalog.FindLabel(ctx, project.ID, bug.ID)
	if err != nil || reloaded.Name != "defecto" || reloaded.Color != "teal" {
		t.Fatalf("reloaded = %+v, %v", reloaded, err)
	}

	clash := "ux"
	dup, err := catalog.FindLabel(ctx, project.ID, bug.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := dup.Update(&clash, nil); err != nil {
		t.Fatal(err)
	}
	if err := catalog.UpdateLabel(ctx, dup); apperror.KindOf(err) != apperror.KindConflict {
		t.Errorf("duplicate rename: kind = %v, want conflict", apperror.KindOf(err))
	}

	if _, err := catalog.FindLabel(ctx, uuid.New(), bug.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("label of another project: kind = %v, want not found", apperror.KindOf(err))
	}

	list, err := catalog.ListLabels(ctx, project.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %v, %v", list, err)
	}
	for _, label := range list {
		if label.UsageCount != 0 {
			t.Errorf("usage count of %s = %d, want 0", label.Name, label.UsageCount)
		}
	}
}
