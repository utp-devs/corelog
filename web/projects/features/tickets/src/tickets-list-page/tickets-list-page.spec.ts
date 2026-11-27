import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { ApiError, SavedFilter, Ticket } from '@corelog/core';
import { guardConsole, installViewport } from '@corelog/core/testing';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { pageOf, setupTickets, ticketOf } from '../testing/ticket-test-kit';
import { TicketsListPage } from './tickets-list-page';

const SAVED_FILTER: SavedFilter = {
  id: 'filter-1',
  name: 'Open bugs',
  criteria: { status: 'open', issueType: 'bug', query: 'timeout' },
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const SEARCH_PLACEHOLDER = 'Buscar incidencia por título o descripción';

const sampleTicket: Ticket = ticketOf({
  id: 'a1b2c3d4-0000-0000-0000-000000000001',
  title: 'Fallo al guardar el perfil',
  status: 'in_progress',
  priority: 'high',
  issueType: 'bug',
  assigneeId: 'user-2',
  assigneeName: 'Luis Pérez',
  updatedAt: '2026-01-02T10:00:00Z',
});

function render(): ComponentFixture<TicketsListPage> {
  const fixture = TestBed.createComponent(TicketsListPage);
  fixture.detectChanges();
  return fixture;
}

/** Finds a button by its visible text, searching the whole document (dialogs render outside the host). */
function buttonByText(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === text,
  );
}

function clickButton(text: string): void {
  const button = buttonByText(text);
  if (!button) throw new Error(`no button named ${text}`);
  button.click();
}

function searchInput(): HTMLInputElement {
  const input = document.querySelector<HTMLInputElement>(
    `input[placeholder="${SEARCH_PLACEHOLDER}"]`,
  );
  if (!input) throw new Error('no search input');
  return input;
}

function typeInSearch(value: string): void {
  const input = searchInput();
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

const SERVER_ERROR: ApiError = {
  status: 0,
  code: 'network.unreachable',
  message: 'Sin conexión con el servidor',
  details: [],
};

describe('TicketsListPage saved filters', () => {
  const guard = guardConsole();
  let restoreViewport: () => void;

  beforeEach(() => {
    restoreViewport = installViewport(1280);
  });

  afterEach(() => {
    restoreViewport();
  });

  it('loads saved criteria into the issue filters and deletes the selected saved filter', async () => {
    const setup = setupTickets({
      savedFilterApi: { list: () => of([SAVED_FILTER]) },
    });
    const fixture = render();
    clickButton('Filtros avanzados');
    fixture.detectChanges();
    await fixture.whenStable();
    expect(setup.savedFilterApi.list).toHaveBeenCalled();

    const savedSelect = fixture.debugElement.query(By.css('.saved-row p-select'));
    savedSelect.triggerEventHandler('ngModelChange', SAVED_FILTER.id);
    fixture.detectChanges();

    expect(searchInput().value).toBe('timeout');
    clickButton('Eliminar filtro');
    fixture.detectChanges();
    await fixture.whenStable();
    expect(setup.savedFilterApi.delete).toHaveBeenCalledWith('filter-1');
    guard.expectNoConsoleOutput();
  });

  it('saves the active issue criteria with a personal name', async () => {
    const setup = setupTickets({ savedFilterApi: { list: () => of([]) } });
    const fixture = render();
    clickButton('Filtros avanzados');
    fixture.detectChanges();
    clickButton('Guardar filtro actual');
    fixture.detectChanges();

    const nameInput = document.querySelector<HTMLInputElement>(
      'input[placeholder="Nombre del filtro"]',
    );
    if (!nameInput) throw new Error('no name input');
    nameInput.value = 'My current view';
    nameInput.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    clickButton('Guardar');
    await fixture.whenStable();

    expect(setup.savedFilterApi.create).toHaveBeenCalledWith('My current view', {
      status: undefined,
      priority: undefined,
      issueType: undefined,
      query: undefined,
      dueFrom: undefined,
      dueTo: undefined,
    });
    guard.expectNoConsoleOutput();
  });
});

describe('TicketsListPage states', () => {
  const guard = guardConsole();
  let restoreViewport: () => void;

  beforeEach(() => {
    restoreViewport = installViewport(1280);
  });

  afterEach(() => {
    restoreViewport();
    vi.useRealTimers();
  });

  it('shows the error with a retry action that reloads the list', () => {
    const setup = setupTickets({
      ticketApi: { list: () => throwError(() => SERVER_ERROR) },
    });
    const fixture = render();

    const alert = document.querySelector('[role="alert"]');
    expect(alert?.textContent).toContain('Sin conexión con el servidor');
    clickButton('Reintentar');
    fixture.detectChanges();
    expect(setup.ticketApi.list).toHaveBeenCalledTimes(2);
  });

  it('shows pagination based on the total across pages', () => {
    setupTickets({ ticketApi: { list: () => of(pageOf([sampleTicket], 45)) } });
    render();

    expect(document.body.textContent).toContain('Página 1 de 3');
    expect(document.body.textContent).toContain('45 elementos');
  });

  it('hides pagination when there are no results', () => {
    setupTickets();
    render();

    expect(document.querySelector('[aria-label="Página siguiente"]')).toBeNull();
  });

  it('shows an empty state inside the table area when there are no issues', () => {
    setupTickets();
    render();

    // The list fills the remaining height of the page (PageContainer, Section and DataTable fill).
    expect(document.querySelector('[data-fill="true"]')).not.toBeNull();
    expect(document.body.textContent).toContain('Aún no hay incidencias');
    expect(buttonByText('Nueva incidencia')).toBeDefined();
  });

  it('offers to clear filters when the active filters match nothing', () => {
    vi.useFakeTimers();
    setupTickets();
    const fixture = render();

    typeInSearch('zzz');
    vi.advanceTimersByTime(300);
    fixture.detectChanges();

    expect(document.body.textContent).toContain('Ninguna incidencia coincide con los filtros');
    expect(buttonByText('Limpiar filtros')).toBeDefined();
  });

  it('shows a loading skeleton while the first page loads', () => {
    vi.useFakeTimers();
    setupTickets({
      ticketApi: { list: () => new Observable<never>(() => undefined) },
    });
    const fixture = render();
    // The table shows its skeleton only after a short delay, so quick loads do not flicker.
    vi.advanceTimersByTime(2000);
    fixture.detectChanges();

    expect(document.querySelector('[role="status"][aria-label="Cargando"]')).not.toBeNull();
  });

  it('renders the table with the key, summary, status and assignee columns on desktop', () => {
    setupTickets({ ticketApi: { list: () => of(pageOf([sampleTicket])) } });
    render();

    expect(document.querySelector('table')).not.toBeNull();
    expect(document.body.textContent).toContain('Fallo al guardar el perfil');
    expect(document.body.textContent).toContain('En progreso');
    expect(document.body.textContent).toContain('Luis Pérez');
  });

  it('renders cards instead of a table on mobile, with no clipped columns', () => {
    restoreViewport();
    restoreViewport = installViewport(390);
    setupTickets({ ticketApi: { list: () => of(pageOf([sampleTicket])) } });
    render();

    expect(document.querySelector('table')).toBeNull();
    expect(document.body.textContent).toContain('Fallo al guardar el perfil');
    expect(document.body.textContent).toContain('En progreso');
    expect(document.body.textContent).toContain('Luis Pérez');
  });

  it('renders without console warnings or errors', () => {
    setupTickets({ ticketApi: { list: () => of(pageOf([sampleTicket])) } });
    render();
    guard.expectNoConsoleOutput();
  });
});
