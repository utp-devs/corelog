package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// newCatalogIssueService wires the issue service with the in-memory catalog and an authorizer that allows all.
func newCatalogIssueService(f *catalogFixture) (*IssueService, *fakeIssueRepo) {
	repo := newFakeIssueRepo()
	svc := NewIssueService(repo, newFakeUserDirectory(), denyAuthorizer{}).WithCatalog(f.store)
	return svc, repo
}

func TestCreateUsesCatalogDefaultsAndLegacyValues(t *testing.T) {
	f := newCatalogFixture(t)
	svc, _ := newCatalogIssueService(f)
	ctx := context.Background()
	reporter := uuid.New()

	// No type and no priority: the organization defaults apply.
	issue, err := svc.Create(ctx, port.CreateInput{Summary: "Por defecto", Description: "Descripcion", ReporterID: reporter, ProjectID: &f.project})
	if err != nil {
		t.Fatalf("create with defaults: %v", err)
	}
	if issue.IssueTypeID != f.types["Tarea"].ID || issue.IssueTypeRef == nil || issue.IssueTypeRef.Name != "Tarea" {
		t.Errorf("default type = %v, want Tarea", issue.IssueTypeID)
	}
	if issue.PriorityID != f.prios["Media"].ID || issue.PriorityRef == nil || issue.PriorityRef.Name != "Media" {
		t.Errorf("default priority = %v, want Media", issue.PriorityID)
	}
	if issue.Priority != domain.PriorityMedium || issue.IssueType != domain.IssueTypeTask {
		t.Errorf("legacy mirror = %s/%s, want medium/task", issue.Priority, issue.IssueType)
	}

	// Legacy strings map to the catalog item that carries the same legacy key.
	legacy, err := svc.Create(ctx, port.CreateInput{
		Summary: "Legado", Description: "Descripcion", Priority: domain.PriorityCritical,
		IssueType: domain.IssueTypeBug, ReporterID: reporter, ProjectID: &f.project,
	})
	if err != nil {
		t.Fatalf("create with legacy values: %v", err)
	}
	if legacy.PriorityID != f.prios["Máxima"].ID || legacy.IssueTypeID != f.types["Error"].ID {
		t.Errorf("legacy mapping priority=%v type=%v", legacy.PriorityID, legacy.IssueTypeID)
	}

	// Explicit ids win over legacy values.
	explicit, err := svc.Create(ctx, port.CreateInput{
		Summary: "Por id", Description: "Descripcion", ReporterID: reporter, ProjectID: &f.project,
		IssueTypeID: idPtr(f.types["Historia"].ID), PriorityID: idPtr(f.prios["Alta"].ID),
	})
	if err != nil {
		t.Fatalf("create with ids: %v", err)
	}
	if explicit.IssueTypeRef.Name != "Historia" || explicit.PriorityRef.Name != "Alta" {
		t.Errorf("explicit refs = %s/%s", explicit.IssueTypeRef.Name, explicit.PriorityRef.Name)
	}
}

func idPtr(id uuid.UUID) *uuid.UUID { return &id }

func TestCreateRejectsUnknownAndDisabledValues(t *testing.T) {
	f := newCatalogFixture(t)
	svc, _ := newCatalogIssueService(f)
	ctx := context.Background()
	reporter := uuid.New()

	_, err := svc.Create(ctx, port.CreateInput{Summary: "X", Description: "Descripcion", ReporterID: reporter,
		ProjectID: &f.project, IssueTypeID: idPtr(uuid.New())})
	codeOf(t, err, apperror.KindValidation, "issue.catalog_value_unknown")

	// The project enables only the epic and subtask types plus Historia: a Tarea is disabled.
	f.store.projectTypes[f.project] = []uuid.UUID{f.types["Épica"].ID, f.types["Subtarea"].ID, f.types["Historia"].ID}
	_, err = svc.Create(ctx, port.CreateInput{Summary: "X", Description: "Descripcion", ReporterID: reporter,
		ProjectID: &f.project, IssueTypeID: idPtr(f.types["Tarea"].ID)})
	codeOf(t, err, apperror.KindValidation, "issue.type_not_enabled")

	// Without an explicit type the default falls to a type the project enables.
	issue, err := svc.Create(ctx, port.CreateInput{Summary: "X", Description: "Descripcion", ReporterID: reporter, ProjectID: &f.project})
	if err != nil {
		t.Fatalf("create with default on restricted project: %v", err)
	}
	if issue.IssueTypeID != f.types["Épica"].ID && issue.IssueTypeID != f.types["Historia"].ID && issue.IssueTypeID != f.types["Subtarea"].ID {
		t.Errorf("default type %v is not enabled for the project", issue.IssueTypeID)
	}
}
