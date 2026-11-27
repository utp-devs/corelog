import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, Routes } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { IssueActivity, IssueComment, IssueWatcherState, NotificationService } from '@corelog/core';
import { guardConsole, installViewport } from '@corelog/core/testing';
import { NEVER, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ISSUE_ID,
  ISSUE_KEY,
  memberOf,
  projectOf,
  setupIssues,
  issueOf,
  userOf,
} from '../testing/issue-test-kit';
import { IssueDetailPage } from './issue-detail-page';

@Component({ selector: 'app-list-stub', template: '<p>Listado de incidencias</p>' })
class ListStub {}

const ROUTES: Routes = [
  { path: 'browse/:issueKey', component: IssueDetailPage },
  { path: 'issues', component: ListStub },
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

function rowOf(label: string): HTMLElement {
  const found = Array.from(document.querySelectorAll<HTMLElement>('.field-row__label')).find(
    (item) => item.textContent?.trim() === label,
  );
  const row = found?.closest<HTMLElement>('.field-row');
  if (!row) throw new Error(`no field ${label}`);
  return row;
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

async function renderDetail(ref: string = ISSUE_ID): Promise<RouterTestingHarness> {
  TestBed.inject(Router).resetConfig(ROUTES);
  const harness = await RouterTestingHarness.create(`/browse/${ref}`);
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
  return harness;
}

/** Renders without waiting for pending timers, for tests that drive the timers themselves. */
async function renderDetailWithoutWaiting(): Promise<RouterTestingHarness> {
  TestBed.inject(Router).resetConfig(ROUTES);
  const harness = await RouterTestingHarness.create(`/browse/${ISSUE_ID}`);
  harness.fixture.detectChanges();
  return harness;
}

/** Lets pending effects and requests settle, then re-renders. */
async function settle(harness: RouterTestingHarness): Promise<void> {
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

describe('IssueDetailPage', () => {
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

  it('shows description, details, files, links and activity as sections', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    const harness = await renderDetail();

    for (const heading of ['Detalles', 'Más información', 'Vínculos', 'Adjuntos', 'Actividad']) {
      expect(sectionOf(heading)).toBeDefined();
    }
    expect(document.body.textContent).toContain('Aún no hay archivos');
    expect(document.body.textContent).toContain('Aún no hay comentarios');
    expect(document.body.textContent).toContain('Aún no hay vínculos');
    harness.fixture.detectChanges();
  });

  it('shows the priority and the type with the catalog name, and the resolution when set', async () => {
    setupIssues({
      user: userOf({ role: 'admin' }),
      issueApi: {
        getById: () =>
          of(
            issueOf({
              status: 'resolved',
              resolution: { id: 'res-1', name: 'No se reproduce' },
              resolvedAt: AT,
            }),
          ),
      },
    });
    await renderDetail();

    expect(rowOf('Prioridad').textContent).toContain('Media');
    expect(rowOf('Prioridad').querySelector('app-catalog-icon')?.getAttribute('data-color')).toBe(
      'amber',
    );
    expect(document.querySelector('[data-testid="issue-resolution"]')?.textContent).toContain(
      'No se reproduce',
    );
    expect(sectionOf('Más información').textContent).toContain('Error');
  });

  it('does not show a resolution for an open issue', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();
    expect(document.querySelector('[data-testid="issue-resolution"]')).toBeNull();
  });

  it('stacks into one column on tablets and puts the details block right after the title', async () => {
    restoreViewport();
    restoreViewport = installViewport(800);
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    const headings = Array.from(document.querySelectorAll('h2')).map((heading) =>
      heading.textContent?.trim(),
    );
    const position = (name: string) => headings.indexOf(name);
    expect(position('Detalles')).toBeGreaterThanOrEqual(0);
    expect(position('Detalles')).toBeLessThan(position('Descripción'));
    expect(position('Descripción')).toBeLessThan(position('Adjuntos'));
    expect(position('Adjuntos')).toBeLessThan(position('Vínculos'));
    expect(position('Vínculos')).toBeLessThan(position('Actividad'));

    const page = document.querySelector('[data-page="issue-detail"]');
    expect(page?.getAttribute('data-has-sidebar')).toBe('false');
  });

  it('collapses the details block on phones until it is opened', async () => {
    restoreViewport();
    restoreViewport = installViewport(390);
    setupIssues({ user: userOf({ role: 'admin' }) });
    const harness = await renderDetail();

    expect(buttonIn(document, 'Mostrar detalles')).toBeDefined();
    expect(document.body.textContent).not.toContain('Más información');
    clickIn(document, 'Mostrar detalles');
    harness.fixture.detectChanges();
    expect(buttonIn(document, 'Ocultar detalles')).toBeDefined();
    expect(document.body.textContent).toContain('Más información');
  });

  it('keeps the side panel beside the main column on desktop', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    expect(document.querySelector('[data-page="issue-detail"]')?.getAttribute('data-stacked')).toBe(
      'false',
    );
  });

  it('shows a loading skeleton while the issue is loading', async () => {
    setupIssues({ issueApi: { getById: () => NEVER } });
    await renderDetail();

    expect(document.querySelector('[role="status"][aria-label="Cargando"]')).not.toBeNull();
  });

  it('shows an error with retry when the issue cannot be loaded', async () => {
    const getById = vi.fn(() => throwError(() => new Error('boom')));
    setupIssues({ issueApi: { getById } });
    const harness = await renderDetail();

    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'No se pudo cargar la información',
    );
    clickIn(document, 'Reintentar');
    harness.fixture.detectChanges();
    expect(getById).toHaveBeenCalledTimes(2);
  });

  it('renders without console warnings or errors', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
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
    setupIssues({
      user: userOf({ role: 'admin' }),
      memberSource: { listMembers: () => of([LUIS]) },
      issueApi: {
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

    expect(sectionOf('Actividad').querySelector('.issue-activity__author')?.textContent).toBe(
      'Luis Pérez',
    );
    expect(document.body.textContent).not.toMatch(/Usuario [0-9a-f]{8}/);
    expect(document.body.textContent).toContain('Subido por Usuario desconocido');
  });

  it('assigns the issue as soon as a teammate is picked, without an extra button', async () => {
    vi.useFakeTimers();
    const ctx = setupIssues({
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
    expect(ctx.issueApi.assign).toHaveBeenCalledWith(ISSUE_ID, 'user-2');
    expect(notify).toHaveBeenCalledWith('Incidencia asignada');
    expect(rowOf('Responsable').querySelector('[role="status"]')?.textContent).toContain(
      'Responsable actualizado',
    );
  });

  it('clears the assignee from the detail page and announces the change', async () => {
    const assigned = issueOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    const ctx = setupIssues({
      user: userOf({ role: 'admin' }),
      issueApi: { getById: () => of(assigned), assign: () => of(issueOf()) },
    });
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'success');
    const harness = await renderDetail();

    clickIn(rowOf('Responsable'), 'Quitar responsable');
    await settle(harness);

    expect(ctx.issueApi.assign).toHaveBeenCalledWith(ISSUE_ID, null);
    expect(notify).toHaveBeenCalledWith('Responsable quitado');
    expect(rowOf('Responsable').querySelector('[role="status"]')?.textContent).toContain(
      'Sin asignar',
    );
  });

  it('hides the clear option when the issue has no assignee', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    expect(buttonIn(rowOf('Responsable'), 'Quitar responsable')).toBeUndefined();
  });

  it('keeps the assignee and reports the error when clearing fails', async () => {
    const assigned = issueOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    setupIssues({
      user: userOf({ role: 'admin' }),
      issueApi: {
        getById: () => of(assigned),
        assign: () => throwError(() => new Error('network down')),
      },
    });
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'error');
    const harness = await renderDetail();

    clickIn(rowOf('Responsable'), 'Quitar responsable');
    await settle(harness);

    expect(notify).toHaveBeenCalledWith('No se pudo quitar el responsable');
    expect(rowOf('Responsable').querySelector('[role="status"]')?.textContent).not.toContain(
      'Sin asignar',
    );
  });

  it('hides the clear option and shows the assignee read-only for members who cannot edit', async () => {
    const assigned = issueOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    setupIssues({
      user: userOf({ role: 'user' }),
      issueApi: { getById: () => of(assigned) },
      projectApi: { listMembers: () => of([memberOf('user-1', 'viewer')]) },
    });
    await renderDetail();

    expect(buttonIn(rowOf('Responsable'), 'Quitar responsable')).toBeUndefined();
    expect(rowOf('Responsable').querySelector('input')?.value).toBe('Luis Pérez');
    expect(rowOf('Responsable').textContent).toContain(
      'No tienes permiso para cambiar el responsable.',
    );
  });

  it('reads the project members silently so a failure never raises a toast', async () => {
    const ctx = setupIssues({
      user: userOf({ role: 'user' }),
      issueApi: { getById: () => of(issueOf({})) },
      projectApi: { listMembers: () => of([memberOf('user-1', 'viewer')]) },
    });
    await renderDetail();

    expect(ctx.projectApi.listMembers).toHaveBeenCalledWith(expect.any(String), { silent: true });
  });

  it('says the project is archived and offers no edit controls', async () => {
    setupIssues({
      user: userOf({ role: 'user' }),
      issueApi: { getById: () => of(issueOf({})) },
      projectApi: {
        listMembers: () => of([memberOf('user-1', 'contributor')]),
        get: () => of(projectOf({ archivedAt: '2026-03-01T00:00:00Z' })),
      },
    });
    await renderDetail();

    expect(document.body.textContent).toContain(
      'Este proyecto está archivado y es de solo lectura',
    );
    expect(buttonIn(document, 'Editar')).toBeUndefined();
    expect(buttonIn(document, 'Vincular')).toBeUndefined();
    expect(buttonIn(document, 'Cargar archivo (máximo 10 MB)')?.disabled).toBe(true);
  });

  it('shows the clear option to a project contributor who is not a platform admin', async () => {
    const assigned = issueOf({ assigneeId: 'user-2', assigneeName: 'Luis Pérez' });
    setupIssues({
      user: userOf({ role: 'user' }),
      issueApi: { getById: () => of(assigned) },
      projectApi: { listMembers: () => of([memberOf('user-1', 'contributor')]) },
    });
    await renderDetail();

    expect(buttonIn(rowOf('Responsable'), 'Quitar responsable')).toBeDefined();
  });

  it('records the issue in the recent items once it is shown', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as {
      kind: string;
      id: string;
      link: string;
    }[];
    expect(recent.find((item) => item.kind === 'issue')).toMatchObject({
      kind: 'issue',
      id: ISSUE_ID,
      link: `/browse/${ISSUE_KEY}`,
    });
  });

  it('shows an error state when the attachment list cannot be loaded', async () => {
    setupIssues({
      user: userOf({ role: 'admin' }),
      attachmentApi: { list: () => throwError(() => new Error('network failure')) },
    });
    await renderDetail();

    expect(document.body.textContent).toContain('No se pudieron cargar los archivos');
  });

  it('lets a project member follow an issue and updates the follower count', async () => {
    const watching: IssueWatcherState = { isWatching: true, count: 1 };
    const ctx = setupIssues({
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
    setupIssues({ user: userOf({ role: 'admin' }) });
    const harness = await renderDetail();

    const section = sectionOf('Descripción');
    expect(section.textContent).toContain('Descripción de la incidencia');
    clickIn(section, 'Editar');
    harness.fixture.detectChanges();
    expect(buttonIn(sectionOf('Descripción'), 'Guardar')).toBeDefined();
  });
  it('shows one friendly page-level error for a forbidden issue and loads nothing else', async () => {
    const forbidden = {
      status: 403,
      code: 'http.403',
      message: 'forbidden project operation',
      details: [],
    };
    const ctx = setupIssues({
      user: userOf({ role: 'user' }),
      issueApi: { getById: () => throwError(() => forbidden) },
    });
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'error');
    await renderDetail();

    const alerts = document.querySelectorAll('[role="alert"]');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toContain('No tienes acceso a este contenido');
    expect(document.body.textContent).not.toContain('forbidden');
    expect(buttonIn(document, 'Reintentar')).toBeUndefined();
    expect(buttonIn(document, 'Volver a incidencias')).toBeDefined();
    expect(notify).not.toHaveBeenCalled();
    expect(ctx.issueApi.listComments).not.toHaveBeenCalled();
    expect(ctx.projectApi.listMembers).not.toHaveBeenCalled();
  });

  it('explains a missing issue without offering a retry', async () => {
    const missing = { status: 404, code: 'http.404', message: 'not found', details: [] };
    setupIssues({ issueApi: { getById: () => throwError(() => missing) } });
    await renderDetail();

    expect(document.body.textContent).toContain('No encontramos lo que buscas');
    expect(buttonIn(document, 'Reintentar')).toBeUndefined();
  });

  it('offers a retry when the server cannot be reached', async () => {
    const offline = {
      status: 0,
      code: 'network.unreachable',
      message: 'No se pudo conectar',
      details: [],
    };
    setupIssues({ issueApi: { getById: () => throwError(() => offline) } });
    await renderDetail();

    expect(document.body.textContent).toContain('Sin conexión');
    expect(buttonIn(document, 'Reintentar')).toBeDefined();
  });

  it('records the project of the issue in the recent items', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as {
      kind: string;
      id: string;
      title: string;
    }[];
    expect(recent.find((item) => item.kind === 'project')).toMatchObject({
      id: 'project-1',
      title: 'Proyecto demo',
    });
  });

  it('shows the project and the issue key in the breadcrumbs', async () => {
    setupIssues({ user: userOf({ role: 'admin' }) });
    await renderDetail();

    const trail = document.querySelector('nav[aria-label="Ruta de navegación"]');
    expect(trail?.textContent).toContain('Proyecto demo');
    expect(trail?.querySelector('[aria-current="page"]')?.textContent?.trim()).toBe(ISSUE_KEY);
  });

  it('opens the issue by its key and shows the key in the header', async () => {
    const getById = vi.fn(() => of(issueOf()));
    setupIssues({ user: userOf({ role: 'admin' }), issueApi: { getById } });
    await renderDetail(ISSUE_KEY);

    expect(getById).toHaveBeenCalledWith(ISSUE_KEY);
    expect(document.querySelector('.issue-key__text')?.textContent?.trim()).toBe(ISSUE_KEY);
    expect(document.querySelector('h1')?.textContent).toContain('Error de prueba');
  });

  it('opens the assignee picker with the organization members already listed', async () => {
    setupIssues({
      user: userOf({ role: 'admin' }),
      memberSource: { listMembers: () => of([LUIS, { userId: 'user-3', name: 'Ana Gómez' }]) },
    });
    const harness = await renderDetail();
    const picker = harness.fixture.debugElement.query((el) => el.name === 'app-user-picker');

    const names = (picker.componentInstance.users() as { name: string }[]).map((user) => user.name);
    expect(names).toEqual(['Ana Gómez', 'Luis Pérez']);
  });

  it('assigns the issue to the signed-in user with one click', async () => {
    const ctx = setupIssues({ user: userOf({ role: 'admin' }) });
    const harness = await renderDetail();

    clickIn(rowOf('Responsable'), 'Asignarme a mí');
    await settle(harness);

    expect(ctx.issueApi.assign).toHaveBeenCalledWith(ISSUE_ID, 'user-1');
  });

  it('explains why the assignee cannot be changed instead of showing a helper line', async () => {
    setupIssues({
      user: userOf({ role: 'user' }),
      projectApi: { listMembers: () => of([memberOf('user-1', 'viewer')]) },
    });
    await renderDetail();

    expect(document.body.textContent).not.toContain('Buscar compañero');
    expect(document.body.textContent).not.toContain('Actual:');
    expect(rowOf('Responsable').textContent).toContain(
      'No tienes permiso para cambiar el responsable.',
    );
    expect(buttonIn(rowOf('Responsable'), 'Asignarme a mí')).toBeUndefined();
  });

  it('writes the history in Spanish with the actor name', async () => {
    const activities: IssueActivity[] = [
      {
        id: 'a1',
        issueId: ISSUE_ID,
        actorId: 'user-2',
        action: 'created',
        fieldName: null,
        oldValue: null,
        newValue: null,
        createdAt: AT,
      },
      {
        id: 'a2',
        issueId: ISSUE_ID,
        actorId: 'user-2',
        action: 'status_changed',
        fieldName: 'status',
        oldValue: 'open',
        newValue: 'in_progress',
        createdAt: '2026-01-02T11:00:00Z',
      },
    ];
    setupIssues({
      user: userOf({ role: 'admin' }),
      memberSource: { listMembers: () => of([LUIS]) },
      issueApi: { listActivities: () => of(activities) },
    });
    const harness = await renderDetail();
    const tab = Array.from(document.querySelectorAll<HTMLElement>('[role="tab"]')).find((item) =>
      item.textContent?.includes('Historial'),
    );
    tab?.click();
    harness.fixture.detectChanges();

    const text = sectionOf('Actividad').textContent ?? '';
    expect(text).toContain('Luis Pérez creó la incidencia');
    expect(text).toContain('Luis Pérez cambió el estado de Abierto a En progreso');
    expect(text).not.toContain('created');
    expect(text).not.toContain('status_changed');
  });

  it('keeps the comment composer to one line until it is focused and clears it after sending', async () => {
    const saved: IssueComment = {
      id: 'c9',
      issueId: ISSUE_ID,
      authorId: 'user-1',
      body: 'Mi comentario',
      createdAt: AT,
      updatedAt: AT,
      deletedAt: null,
    };
    const ctx = setupIssues({
      user: userOf({ role: 'admin' }),
      issueApi: { createComment: () => of(saved) },
    });
    const harness = await renderDetail();

    expect(document.querySelector('app-issue-activity textarea')).toBeNull();
    document.querySelector<HTMLButtonElement>('.issue-activity__collapsed')?.click();
    harness.fixture.detectChanges();

    const textarea = document.querySelector<HTMLTextAreaElement>('app-issue-activity textarea');
    expect(textarea).not.toBeNull();
    if (!textarea) return;
    textarea.value = 'Mi comentario';
    textarea.dispatchEvent(new Event('input'));
    harness.fixture.detectChanges();
    clickIn(document, 'Comentar');
    await settle(harness);

    expect(ctx.issueApi.createComment).toHaveBeenCalledWith(ISSUE_ID, 'Mi comentario');
    expect(document.querySelector('app-issue-activity textarea')).toBeNull();
  });

  it('lets the author edit a comment and asks before deleting it', async () => {
    const own: IssueComment = {
      id: 'c1',
      issueId: ISSUE_ID,
      authorId: 'user-1',
      body: 'Primer texto',
      createdAt: AT,
      updatedAt: AT,
      deletedAt: null,
    };
    const other: IssueComment = { ...own, id: 'c2', authorId: 'user-2', body: 'Ajeno' };
    const ctx = setupIssues({
      user: userOf({ role: 'admin' }),
      issueApi: { listComments: () => of([own, other]) },
    });
    const harness = await renderDetail();

    const items = Array.from(document.querySelectorAll<HTMLElement>('.issue-activity__item'));
    expect(items).toHaveLength(2);
    const mine = items.find((item) => item.textContent?.includes('Primer texto'));
    const theirs = items.find((item) => item.textContent?.includes('Ajeno'));
    if (!mine || !theirs) throw new Error('comments not rendered');
    expect(buttonIn(theirs, 'Editar')).toBeUndefined();

    clickIn(mine, 'Editar');
    harness.fixture.detectChanges();
    const editor = document.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Editar comentario"]',
    );
    if (!editor) throw new Error('no editor');
    editor.value = 'Texto corregido';
    editor.dispatchEvent(new Event('input'));
    harness.fixture.detectChanges();
    clickIn(document, 'Guardar');
    await settle(harness);
    expect(ctx.issueApi.updateComment).toHaveBeenCalledWith(ISSUE_ID, 'c1', 'Texto corregido');
    expect(document.body.textContent).toContain('Texto corregido');

    const edited = Array.from(document.querySelectorAll<HTMLElement>('.issue-activity__item')).find(
      (item) => item.textContent?.includes('Texto corregido'),
    );
    if (!edited) throw new Error('edited comment missing');
    clickIn(edited, 'Eliminar');
    harness.fixture.detectChanges();
    expect(ctx.issueApi.deleteComment).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain('Eliminar comentario');
  });
});
