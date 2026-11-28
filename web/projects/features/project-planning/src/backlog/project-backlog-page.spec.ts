import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  BacklogV2ApiService,
  BacklogView,
  ProjectApiService,
  SessionService,
  SprintV2ApiService,
} from '@corelog/core';
import { guardConsole, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { backlogViewOf, sectionOf, sprintOf } from '../testing/agile-fixtures';
import {
  PROJECT_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  projectRoutes,
  signedInSession,
} from '../testing/planning-test-kit';
import { ProjectBacklogPage } from './project-backlog-page';

ensureMatchMedia();

async function settle(harness: RouterTestingHarness): Promise<void> {
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

interface Options {
  backlog?: Observable<BacklogView>;
  project?: Observable<unknown>;
}

async function renderPage(options: Options = {}) {
  TestBed.resetTestingModule();
  const backlogApi = {
    get: vi.fn(() => options.backlog ?? of(backlogViewOf())),
    rank: vi.fn((_input: unknown) => of(undefined)),
    setPoints: vi.fn(() => of(undefined)),
  };
  const sprintApi = {
    list: vi.fn(() => of([])),
    create: vi.fn((_projectId: string, input: { name: string }) =>
      of(sprintOf('9', { name: input.name })),
    ),
    start: vi.fn(() => of(sprintOf('2', { status: 'active' }))),
    complete: vi.fn(() =>
      of({
        sprint: sprintOf('1', { status: 'closed' }),
        nextSprint: null,
        completedIssues: 1,
        movedIssues: 1,
        completedPoints: 3,
        committedPoints: 6,
      }),
    ),
    update: vi.fn(),
    delete: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectBacklogPage, 'backlog')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: BacklogV2ApiService, useValue: backlogApi },
      { provide: SprintV2ApiService, useValue: sprintApi },
      {
        provide: ProjectApiService,
        useValue: {
          get: vi.fn(() => options.project ?? of(projectOf())),
          listMembers: vi.fn(() => of([])),
        },
      },
    ],
  });
  const harness = await RouterTestingHarness.create(`/projects/${PROJECT_ID}/backlog`);
  await settle(harness);
  return { backlogApi, sprintApi, harness };
}

function byTestId<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.querySelector<T>(`[data-testid="${id}"]`);
}

function showEpics(harness: RouterTestingHarness): void {
  const toggle = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find((el) =>
    el.textContent?.includes('Mostrar épicas'),
  );
  toggle?.click();
  harness.fixture.detectChanges();
}

function menuItem(label: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find(
    (el) => el.textContent?.trim() === label,
  );
}

describe('ProjectBacklogPage', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => localStorage.clear());
  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('shows the active sprint, the planned sprint and the backlog with their counts', async () => {
    const { backlogApi, harness } = await renderPage();
    expect(backlogApi.get).toHaveBeenCalledWith(PROJECT_ID, { epic: undefined, q: undefined });

    const titles = Array.from(document.querySelectorAll('.backlog-section__title')).map((el) =>
      el.textContent?.trim(),
    );
    expect(titles).toEqual(['Sprint 1', 'Sprint 2', 'Backlog']);
    expect(byTestId('backlog-section-1')?.textContent).toContain('2 incidencias');
    expect(byTestId('backlog-section-1')?.textContent).toContain('Cerrar pagos');
    expect(byTestId('backlog-section-backlog')?.textContent).toContain('3 incidencias');
    showEpics(harness);
    expect(byTestId('epics-panel')?.textContent).toContain('Pagos');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('moves an issue to a sprint with the row menu', async () => {
    const { backlogApi, harness } = await renderPage();
    document.querySelector<HTMLButtonElement>('[aria-label="Mover CORE-3"]')?.click();
    harness.fixture.detectChanges();
    menuItem('Sprint 2')?.click();
    await settle(harness);

    expect(backlogApi.rank).toHaveBeenCalledWith({
      issueIds: ['3'],
      sprintId: '2',
      beforeIssueId: undefined,
      afterIssueId: undefined,
    });
    harness.fixture.destroy();
  });

  it('selects several rows with Ctrl and moves them together', async () => {
    const { backlogApi, harness } = await renderPage();
    const row = (key: string) => byTestId(`backlog-row-${key}`) as HTMLElement;
    row('CORE-3').dispatchEvent(new MouseEvent('click', { bubbles: true }));
    row('CORE-5').dispatchEvent(new MouseEvent('click', { bubbles: true, ctrlKey: true }));
    harness.fixture.detectChanges();
    expect(byTestId('selection-count')?.textContent).toContain('2 seleccionadas');

    document.querySelector<HTMLButtonElement>('[aria-label="Mover CORE-3"]')?.click();
    harness.fixture.detectChanges();
    menuItem('Sprint 2')?.click();
    await settle(harness);
    expect(backlogApi.rank.mock.calls[0][0]).toMatchObject({ issueIds: ['3', '5'], sprintId: '2' });
    harness.fixture.destroy();
  });

  it('opens the compact panel of the selected issue with a link to the full one', async () => {
    const { harness } = await renderPage();
    (byTestId('backlog-row-CORE-4') as HTMLElement).dispatchEvent(
      new MouseEvent('click', { bubbles: true }),
    );
    await settle(harness);
    const panel = byTestId('backlog-issue-panel');
    expect(panel?.textContent).toContain('Historia 4');
    expect(byTestId('backlog-open-issue')?.getAttribute('href')).toBe('/browse/CORE-4');
    harness.fixture.destroy();
  });

  it('edits story points in place', async () => {
    const { backlogApi, harness } = await renderPage();
    byTestId('points-CORE-3')?.click();
    harness.fixture.detectChanges();
    const input = byTestId<HTMLInputElement>('points-input-CORE-3');
    expect(input).not.toBeNull();
    (input as HTMLInputElement).value = '8';
    (input as HTMLInputElement).dispatchEvent(new Event('input', { bubbles: true }));
    (input as HTMLInputElement).dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
    );
    await settle(harness);
    expect(backlogApi.setPoints).toHaveBeenCalledWith('3', 8);
    harness.fixture.destroy();
  });

  it('creates a sprint from the dialog', async () => {
    const { sprintApi, harness } = await renderPage();
    byTestId('create-sprint')?.querySelector('button')?.click();
    await settle(harness);
    expect(document.body.textContent).toContain('Crear sprint');
    const name = byTestId<HTMLInputElement>('sprint-name');
    expect(name?.value).toBe('Sprint 3');
    byTestId('sprint-confirm')?.querySelector('button')?.click();
    await settle(harness);
    expect(sprintApi.create).toHaveBeenCalledTimes(1);
    expect(sprintApi.create.mock.calls[0][1]).toMatchObject({ name: 'Sprint 3' });
    harness.fixture.destroy();
  });

  it('lets a planned sprint start only when no sprint is active', async () => {
    const withActive = await renderPage();
    expect(byTestId('backlog-section-2')?.querySelector('[data-testid="start-sprint"]')).toBeNull();
    withActive.harness.fixture.destroy();

    const view = backlogViewOf();
    const noActive: BacklogView = {
      ...view,
      sections: view.sections.filter((section) => section.kind !== 'active'),
    };
    const { harness } = await renderPage({ backlog: of(noActive) });
    expect(byTestId('start-sprint')).not.toBeNull();
    harness.fixture.destroy();
  });

  it('opens the complete dialog with the numbers of the active sprint', async () => {
    const { harness } = await renderPage();
    byTestId('backlog-section-1')
      ?.querySelector<HTMLElement>('[data-testid="complete-sprint"] button')
      ?.click();
    harness.fixture.detectChanges();
    expect(byTestId('complete-done')?.textContent).toContain('1 incidencia terminada (3 pts)');
    expect(byTestId('complete-open')?.textContent).toContain('1 incidencia sin terminar (3 pts)');
    harness.fixture.destroy();
  });

  it('gives viewers a read-only backlog', async () => {
    const { harness } = await renderPage({
      backlog: of(backlogViewOf({ canMove: false, canManage: false })),
    });
    expect(byTestId('create-sprint')).toBeNull();
    expect(document.querySelector('[aria-label="Mover CORE-3"]')).toBeNull();
    expect(byTestId('backlog-create-open')).toBeNull();
    expect(byTestId('points-CORE-3')?.hasAttribute('disabled')).toBe(true);
    harness.fixture.destroy();
  });

  it('explains an empty project and shows retry on failure', async () => {
    const empty: BacklogView = {
      ...backlogViewOf(),
      sections: [sectionOf('backlog', null, [])],
      epics: [],
    };
    const first = await renderPage({ backlog: of(empty) });
    expect(document.body.textContent).toContain('Todavía no hay incidencias');
    first.harness.fixture.destroy();

    const failed = await renderPage({ backlog: failWith(500) });
    expect(document.querySelector('app-error-state')).not.toBeNull();
    failed.harness.fixture.destroy();
  });

  it('shows a skeleton while loading and reads archived projects as read-only', async () => {
    const loading = await renderPage({ backlog: NEVER });
    expect(document.querySelector('[aria-label="Cargando backlog"]')).not.toBeNull();
    loading.harness.fixture.destroy();

    const archived = await renderPage({
      project: of({ ...projectOf(), archivedAt: '2026-02-01T00:00:00Z' }),
    });
    expect(byTestId('archived-notice')).not.toBeNull();
    expect(byTestId('create-sprint')).toBeNull();
    expect(byTestId('backlog-create-open')).toBeNull();
    archived.harness.fixture.destroy();
  });

  it('filters by epic when its card is clicked', async () => {
    const { backlogApi, harness } = await renderPage();
    showEpics(harness);
    byTestId('epic-CORE-9')?.click();
    await settle(harness);
    expect(backlogApi.get).toHaveBeenLastCalledWith(PROJECT_ID, { epic: 'epic-1', q: undefined });
    expect(byTestId('epic-filter-clear')).not.toBeNull();
    harness.fixture.destroy();
  });
});
