import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { DashboardGadget, SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { GadgetHost } from './gadget-host';

const DATA_URL = '/api/v1/dashboards/d-1/gadgets/g-1/data';

function gadgetOf(type: DashboardGadget['type'], config = {}): DashboardGadget {
  return { id: 'g-1', type, title: 'Mi gadget', config, columnIndex: 0, position: 0 };
}

@Component({
  imports: [GadgetHost],
  template: `<app-gadget-host
    dashboardId="d-1"
    [gadget]="gadget()"
    [editing]="editing()"
    [unsaved]="unsaved()"
    [column]="0"
    [columnCount]="3"
    [index]="1"
    [count]="3"
    (configure)="events.push('configure')"
    (remove)="events.push('remove')"
    (moveTo)="events.push('move:' + $event)"
    (nudge)="events.push('nudge:' + $event)"
  />`,
})
class Host {
  readonly gadget = signal<DashboardGadget>(gadgetOf('assigned_to_me'));
  readonly editing = signal(false);
  readonly unsaved = signal(false);
  readonly events: string[] = [];
}

function setup() {
  TestBed.configureTestingModule({
    imports: [Host],
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
    ],
  });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return {
    fixture,
    backend: TestBed.inject(HttpTestingController),
    el: fixture.nativeElement as HTMLElement,
  };
}

const ISSUE_DATA = {
  gadget_id: 'g-1',
  type: 'assigned_to_me',
  generated_at: '2026-11-27T10:00:00Z',
  data: {
    total: 2,
    columns: ['key', 'summary', 'status', 'due'],
    query: '',
    items: [
      {
        id: 'i-1',
        key: 'CL-1',
        summary: 'Arreglar el login',
        status: 'En curso',
        status_category: 'in_progress',
        priority: 'high',
        type: 'Tarea',
        assignee: 'Ana',
        project_key: 'CL',
        due: '2020-01-01',
        updated: '2026-11-27T09:00:00Z',
      },
    ],
  },
};

describe('GadgetHost', () => {
  it('shows a skeleton while loading, then the issues of the gadget', () => {
    const { fixture, backend, el } = setup();
    expect(el.querySelector('app-loading-skeleton')).not.toBeNull();

    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
    fixture.detectChanges();

    expect(el.querySelector('app-loading-skeleton')).toBeNull();
    expect(el.querySelector('[data-testid="gadget-issue-CL-1"]')?.textContent).toContain(
      'Arreglar el login',
    );
    expect(el.textContent).toContain('En curso');
    expect(el.querySelector('.gadget-issues__overdue')).not.toBeNull();
    expect(el.textContent).toContain('Mostrando 1 de 2 incidencias');
    expect(el.textContent).toContain('Actualizado a las');
  });

  it('shows an empty state when there are no issues', () => {
    const { fixture, backend, el } = setup();
    backend
      .expectOne(DATA_URL)
      .flush({ data: { ...ISSUE_DATA, data: { ...ISSUE_DATA.data, total: 0, items: [] } } });
    fixture.detectChanges();
    expect(el.textContent).toContain('No tienes incidencias abiertas.');
  });

  it('shows an error with retry, and reloads on retry', () => {
    const { fixture, backend, el } = setup();
    backend
      .expectOne(DATA_URL)
      .flush({ error: { message: 'x' } }, { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('No se pudo cargar');

    (el.querySelector('.gadget-frame__retry') as HTMLButtonElement).click();
    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });

  it('refreshes from its own button without hiding the content', () => {
    const { fixture, backend, el } = setup();
    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
    fixture.detectChanges();

    (el.querySelector('[aria-label="Actualizar Mi gadget"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="gadget-issue-CL-1"]')).not.toBeNull();
    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
  });

  it('draws a chart card for chart gadgets', () => {
    const { fixture, backend, el } = setup();
    fixture.componentInstance.gadget.set(gadgetOf('pie'));
    fixture.detectChanges();
    const pending = backend.match(DATA_URL);
    pending[pending.length - 1].flush({
      data: {
        gadget_id: 'g-1',
        type: 'pie',
        generated_at: '2026-11-27T10:00:00Z',
        data: {
          group_by: 'status',
          total: 3,
          groups: [{ key: 'a', label: 'Abiertas', count: 3 }],
          other: 0,
        },
      },
    });
    fixture.detectChanges();
    expect(el.querySelector('app-chart-card')).not.toBeNull();
    expect(el.textContent).toContain('Abiertas');
  });

  it('renders the text gadget from its configuration without asking the API', () => {
    const { fixture, backend, el } = setup();
    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
    fixture.componentInstance.gadget.set(gadgetOf('text', { markdown: '**Hola** equipo' }));
    fixture.detectChanges();
    backend.verify();
    expect(el.querySelector('app-rich-text-view strong')?.textContent).toBe('Hola');
  });

  it('shows a placeholder, and no request, for a gadget that is not saved yet', () => {
    const { fixture, backend, el } = setup();
    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
    fixture.componentInstance.unsaved.set(true);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="gadget-unsaved"]')?.textContent).toContain(
      'Guarda el panel',
    );
    backend.verify();
  });

  it('offers move, configure and remove in edit mode', () => {
    const { fixture, backend, el } = setup();
    backend.expectOne(DATA_URL).flush({ data: ISSUE_DATA });
    fixture.componentInstance.editing.set(true);
    fixture.detectChanges();

    expect(el.querySelector('.gadget-host__handle')).not.toBeNull();
    expect(el.querySelector('[aria-label="Actualizar Mi gadget"]')).toBeNull();
    const menu = el.querySelector('[data-testid="gadget-menu-g-1"]') as HTMLButtonElement;
    expect(menu.getAttribute('aria-label')).toBe('Opciones de Mi gadget');
    menu.click();
    fixture.detectChanges();

    const labels = [...document.querySelectorAll('.p-menu-item-label, .p-menu-item-link')].map(
      (node) => node.textContent?.trim(),
    );
    expect(labels).toEqual(
      expect.arrayContaining([
        'Configurar',
        'Subir',
        'Bajar',
        'Mover a la columna 2',
        'Mover a la columna 3',
        'Quitar del panel',
      ]),
    );
    expect(labels).not.toContain('Mover a la columna 1');
  });
});
