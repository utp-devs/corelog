package memory

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

// TestMemoryRepositorySaveFindUpdateAndFilter recorre el repositorio en memoria que usa el modo de desarrollo:
// guardado con id asignado, copia defensiva al leer, filtros por proyecto, estado y prioridad, y paginacion.
func TestMemoryRepositorySaveFindUpdateAndFilter(t *testing.T) {
	ctx := context.Background()
	repo := NewTicketRepository()

	project := uuid.New()
	ticket := &domain.Ticket{Title: "Nueva", Description: "x", Status: domain.StatusOpen, Priority: domain.PriorityHigh, ProjectID: project}
	if err := repo.Save(ctx, ticket); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if ticket.ID == uuid.Nil {
		t.Fatal("Save must assign an id when it is missing")
	}

	found, err := repo.FindByID(ctx, ticket.ID)
	if err != nil || found.Title != "Nueva" {
		t.Fatalf("FindByID = %+v, %v", found, err)
	}
	found.Title = "Modificada fuera"
	again, _ := repo.FindByID(ctx, ticket.ID)
	if again.Title != "Nueva" {
		t.Fatal("reads must return copies, not the stored value")
	}
	if _, err := repo.FindByID(ctx, uuid.New()); err == nil {
		t.Fatal("unknown id must fail")
	}

	ticket.Title = "Actualizada"
	if err := repo.Update(ctx, ticket); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := repo.Update(ctx, &domain.Ticket{ID: uuid.New()}); err == nil {
		t.Fatal("updating an unknown ticket must fail")
	}

	inProject, err := repo.FindAll(ctx, port.ListFilter{ProjectIDs: []uuid.UUID{project}, Page: 1, PageSize: 10})
	if err != nil || len(inProject) != 1 || inProject[0].Title != "Actualizada" {
		t.Fatalf("FindAll by project = %d, %v", len(inProject), err)
	}
	high, _ := repo.FindAll(ctx, port.ListFilter{Priority: domain.PriorityHigh, Page: 1, PageSize: 10})
	if len(high) == 0 {
		t.Fatal("priority filter returned nothing")
	}
	open, _ := repo.Count(ctx, port.ListFilter{Status: domain.StatusOpen})
	if open == 0 {
		t.Fatal("status count returned nothing")
	}
	beyond, _ := repo.FindAll(ctx, port.ListFilter{Page: 1000, PageSize: 10})
	if len(beyond) != 0 {
		t.Fatalf("page beyond the end returned %d items", len(beyond))
	}
}
