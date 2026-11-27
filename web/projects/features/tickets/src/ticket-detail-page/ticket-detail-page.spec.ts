import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, Routes } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { IssueActivity, IssueComment, IssueWatcherState, NotificationService } from '@corelog/core';
import { guardConsole, installViewport } from '@corelog/core/testing';
import { NEVER, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ISSUE_ID, memberOf, setupTickets, ticketOf, userOf } from '../testing/ticket-test-kit';
import { TicketDetailPage } from './ticket-detail-page';

@Component({ selector: 'app-list-stub', template: '<p>Listado de incidencias</p>' })
class ListStub {}

const ROUTES: Routes = [
  { path: 'workspace/issues/:id', component: TicketDetailPage },
  { path: 'workspace/issues', component: ListStub },
];

const AT = '2026-01-02T10:00:00Z';
const LUIS = { userId: 'user-2', name: 'Luis Pérez' };

function sectionOf(heading: string): HTMLElement {
  const found = Array.from(document.querySelectorAll<HTMLElement>('h2')).find(
    (item) => item.textContent?.trim() === heading,
  );
  const section = found?.closest('section');
  if (!section) throw new Error(`no section ${heading}`);
  return section as HTMLElement;
}

function buttonIn(root: ParentNode, text: string): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === text,
  );
}

function clickIn(root: ParentNode, text: string): void {
  const button = buttonIn(root, text);
  if (!button) throw new Error(`no button ${text}`);
  button.click();
}

async function renderDetail(): Promise<RouterTestingHarness> {
  TestBed.inject(Router).resetConfig(ROUTES);
  const harness = await RouterTestingHarness.create(`/workspace/issues/${ISSUE_ID}`);
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
  return harness;
}

/** Renders without waiting for pending timers, for tests that drive the timers themselves. */
async function renderDetailWithoutWaiting(): Promise<RouterTestingHarness> {
  TestBed.inject(Router).resetConfig(ROUTES);
  const harness = await RouterTestingHarness.create(`/workspace/issues/${ISSUE_ID}`);
  harness.fixture.detectChanges();
  return harness;
}

/** Lets pending effects and requests settle, then re-renders. */
async function settle(harness: RouterTestingHarness): Promise<void> {
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

describe('TicketDetailPage', () => {
  const guard = guardConsole();
  let restoreViewport: () => void;

  beforeEach(() => {
    restoreViewport = installViewport(1280);
    window.localStorage.clear();
  });

  afterEach(() => {
    restoreViewport();
    vi.useRealTimers();
  });

  it('shows description, details, links, files, comments and history as sections', async () => {
    setupTickets({ user: userOf({ role: 'admin' }) });
    const harness = await renderDetail();

    for (const heading of ['Detalles', 'Vínculos', 'Adjuntos', 'Comentarios', 'Historial']) {
      expect(sectionOf(heading)).toBeDefined();
    }
    expect(document.body.textContent).toContain('Aún no hay archivos');
    expect(document.body.textContent).toContain('Aún no hay comentarios');
    expect(document.body.textContent).toContain('Aún no hay actividad');
    expect(document.body.textContent).toContain('Aún no hay vínculos');
    harness.fixture.detectChanges();
  });

  it('lists main content before links and the side panel after the main column on mobile', async () => {
    restoreViewport();
    restoreViewport = installViewport(800);
    setupTickets({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    const headings = Array.from(document.querySelectorAll('h2')).map((heading) =>
      heading.textContent?.trim(),
    );
    const position = (name: string) => headings.indexOf(name);
    expect(position('Descripción')).toBeGreaterThanOrEqual(0);
    expect(position('Descripción')).toBeLessThan(position('Detalles'));
    expect(position('Detalles')).toBeLessThan(position('Vínculos'));
    expect(position('Vínculos')).toBeLessThan(position('Historial'));
    expect(position('Historial')).toBeLessThan(position('Estado y prioridad'));

    const page = document.querySelector('[data-page="issue-detail"]');
    expect(page?.getAttribute('data-stacked')).toBe('true');
    expect(page?.getAttribute('data-has-sidebar')).toBe('true');
  });

  it('keeps the side panel beside the main column on desktop', async () => {
    setupTickets({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    expect(document.querySelector('[data-page="issue-detail"]')?.getAttribute('data-stacked')).toBe(
      'false',
    );
  });

  it('shows a loading skeleton while the issue is loading', async () => {
    setupTickets({ ticketApi: { getById: () => NEVER } });
    await renderDetail();

    expect(document.querySelector('[role="status"][aria-label="Cargando"]')).not.toBeNull();
  });

  it('shows an error with retry when the issue cannot be loaded', async () => {
    const getById = vi.fn(() => throwError(() => new Error('boom')));
    setupTickets({ ticketApi: { getById } });
    const harness = await renderDetail();

    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'No se pudo encontrar la incidencia solicitada.',
    );
    clickIn(document, 'Reintentar');
    harness.fixture.detectChanges();
    expect(getById).toHaveBeenCalledTimes(2);
  });

  it('renders without console warnings or errors', async () => {
    setupTickets({ user: userOf({ role: 'admin' }) });
    await renderDetail();
    guard.expectNoConsoleOutput();
  });

  it('shows the member name of comment authors, history actors and attachment uploaders', async () => {
    const comment: IssueComment = {
      id: 'c1',
      issueId: ISSUE_ID,
      authorId: 'user-2',
      body: 'Hola equipo',
      createdAt: AT,
      updatedAt: AT,
      deletedAt: null,
    };
    const activity: IssueActivity = {
      id: 'a1',
      issueId: ISSUE_ID,
      actorId: 'user-2',
      action: 'status_changed',
      fieldName: 'status',
      oldValue: 'open',
      newValue: 'in_progress',
      createdAt: AT,
    };
    setupTickets({
      user: userOf({ role: 'admin' }),
      memberSource: { listMembers: () => of([LUIS]) },
      ticketApi: {
        listComments: () => of([comment]),
        listActivities: () => of([activity]),
      },
      attachmentApi: {
        list: () =>
          of([
            {
              id: 'f1',
              issueId: ISSUE_ID,
              uploaderId: 'user-9',
              fileName: 'captura.png',
              contentType: 'image/png',
              sizeBytes: 2048,
              createdAt: AT,
            },
          ]),
      },
    });
    await renderDetail();

    expect(sectionOf('Comentarios').querySelector('.author')?.textContent).toBe('Luis Pérez');
    expect(document.body.textContent).not.toMatch(/Usuario [0-9a-f]{8}/);
    expect(document.body.textContent).toContain('Subido por Usuario desconocido');
  });

  it('assigns the issue as soon as a teammate is picked, without an extra button', async () => {
    vi.useFakeTimers();
    const ctx = setupTickets({
      user: userOf({ role: 'admin' }),
      directoryApi: {
        searchTeammates: () =>
          of([{ id: 'user-2', name: 'Luis Pérez', email: 'luis@example.test' }]),
      },
    });
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'success');
    const harness = await renderDetailWithoutWaiting();
    const picker = harness.fixture.debugElement.query((el) => el.name === 'app-user-picker');

    picker.triggerEventHandler('search', 'Luis');
    vi.advanceTimersByTime(300);
    harness.fixture.detectChanges();
    picker.triggerEventHandler('valueChange', 'user-2');

    harness.fixture.detectChanges();
    expect(ctx.ticketApi.assign).toHaveBeenCalledWith(ISSUE_ID, 'user-2');
    expect(notify).toHaveBeenCalledWith('Incidencia asignada');
    expect(sectionOf('Responsable').querySelector('[role="status"]')?.textContent).toContain(
      'Responsable actualizado',
    );
  });

  it('clears the assignee from the detail page and announces the change', async () => {
    const assigned = ticketOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    const ctx = setupTickets({
      user: userOf({ role: 'admin' }),
      ticketApi: { getById: () => of(assigned), assign: () => of(ticketOf()) },
    });
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'success');
    const harness = await renderDetail();

    clickIn(sectionOf('Responsable'), 'Quitar responsable');
    await settle(harness);

    expect(ctx.ticketApi.assign).toHaveBeenCalledWith(ISSUE_ID, null);
    expect(notify).toHaveBeenCalledWith('Responsable quitado');
    expect(sectionOf('Responsable').querySelector('[role="status"]')?.textContent).toContain(
      'Sin asignar',
    );
  });

  it('hides the clear option when the issue has no assignee', async () => {
    setupTickets({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    expect(buttonIn(sectionOf('Responsable'), 'Quitar responsable')).toBeUndefined();
  });

  it('keeps the assignee and reports the error when clearing fails', async () => {
    const assigned = ticketOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    setupTickets({
      user: userOf({ role: 'admin' }),
      ticketApi: {
        getById: () => of(assigned),
        assign: () => throwError(() => new Error('network down')),
      },
    });
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'error');
    const harness = await renderDetail();

    clickIn(sectionOf('Responsable'), 'Quitar responsable');
    await settle(harness);

    expect(notify).toHaveBeenCalledWith('No se pudo quitar el responsable');
    expect(sectionOf('Responsable').querySelector('[role="status"]')?.textContent).not.toContain(
      'Sin asignar',
    );
  });

  it('hides the clear option and shows the assignee read-only for members who cannot edit', async () => {
    const assigned = ticketOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    setupTickets({
      user: userOf({ role: 'user' }),
      ticketApi: { getById: () => of(assigned) },
      projectApi: { listMembers: () => of([memberOf('user-1', 'viewer')]) },
    });
    await renderDetail();

    expect(buttonIn(sectionOf('Responsable'), 'Quitar responsable')).toBeUndefined();
    expect(sectionOf('Responsable').textContent).toContain('Luis Pérez');
    expect(sectionOf('Responsable').textContent).toContain(
      'No tienes permiso para cambiar el responsable.',
    );
  });

  it('shows the clear option to a project contributor who is not a platform admin', async () => {
    const assigned = ticketOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    setupTickets({
      user: userOf({ role: 'user' }),
      ticketApi: { getById: () => of(assigned) },
      projectApi: { listMembers: () => of([memberOf('user-1', 'contributor')]) },
    });
    await renderDetail();

    expect(buttonIn(sectionOf('Responsable'), 'Quitar responsable')).toBeDefined();
  });

  it('records the issue in the recent items once it is shown', async () => {
    setupTickets({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as {
      kind: string;
      id: string;
      link: string;
    }[];
    expect(recent[0]).toMatchObject({
      kind: 'issue',
      id: ISSUE_ID,
      link: `/workspace/issues/${ISSUE_ID}`,
    });
  });

  it('shows an error state when the attachment list cannot be loaded', async () => {
    setupTickets({
      user: userOf({ role: 'admin' }),
      attachmentApi: { list: () => throwError(() => new Error('network failure')) },
    });
    await renderDetail();

    expect(document.body.textContent).toContain('No se pudieron cargar los archivos');
  });

  it('lets a project member follow an issue and updates the follower count', async () => {
    const watching: IssueWatcherState = { isWatching: true, count: 1 };
    const ctx = setupTickets({
      user: userOf({ role: 'admin' }),
      watcherApi: { watch: () => of(watching) },
    });
    const harness = await renderDetail();

    clickIn(document, 'Seguir incidencia');
    await settle(harness);

    expect(ctx.watcherApi.watch).toHaveBeenCalledWith(ISSUE_ID);
    expect(buttonIn(document, 'Dejar de seguir')).toBeDefined();
    expect(document.body.textContent).toContain('1 persona sigue esta incidencia');
  });

  it('shows the description in view mode and opens the editor from the section action', async () => {
    setupTickets({ user: userOf({ role: 'admin' }) });
    const harness = await renderDetail();

    const section = sectionOf('Descripción');
    expect(section.textContent).toContain('Descripción de la incidencia');
    clickIn(section, 'Editar');
    harness.fixture.detectChanges();
    expect(buttonIn(sectionOf('Descripción'), 'Guardar')).toBeDefined();
  });
});
