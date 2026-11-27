import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  BacklogApiService,
  ProjectApiService,
  BacklogItem,
  Sprint,
  SprintApiService,
  SprintIssue,
  SessionService,
} from '@corelog/core';
import { guardConsole, provideCorelogTesting } from '@corelog/core/testing';
import { Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  PROJECT_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  projectRoutes,
  signedInSession,
} from '../testing/planning-test-kit';
import { ProjectSprintsPage } from './project-sprints-page';

ensureMatchMedia();

const sprintsPath = `/projects/${PROJECT_ID}/sprints`;

const planned: Sprint = {
  id: 'sprint-2',
  projectId: PROJECT_ID,
  name: 'Sprint 2',
  goal: '',
  status: 'planned',
};
const active: Sprint = {
  id: 'sprint-1',
  projectId: PROJECT_ID,
  name: 'Sprint 1',
  goal: 'Entregar exportacion',
  status: 'active',
  startDate: '2026-11-02',
  endDate: '2026-11-08',
};

const sprintIssues: SprintIssue[] = [
  { id: 'issue-1', title: 'Exportar CSV', status: 'in_progress', priority: 'high', storyPoints: 3 },
];

async function settle(harness: RouterTestingHarness): Promise<void> {
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

async function renderPage(overrides: Partial<Record<'list', Observable<Sprint[]>>> = {}) {
  TestBed.resetTestingModule();
  const sprintApi = {
    list: vi.fn((): Observable<Sprint[]> => overrides.list ?? of([active, planned])),
    velocity: vi.fn(() => of([])),
    listIssues: vi.fn(() => of(sprintIssues)),
    burndown: vi.fn(() => of([{ date: '2026-11-02', ideal: 3, remaining: 3 }])),
    start: vi.fn(() => of(planned)),
    create: vi.fn(),
    assignIssue: vi.fn(() => of(undefined)),
    removeIssue: vi.fn(() => of(undefined)),
    close: vi.fn(() => of(undefined)),
  };
  const backlogApi = { list: vi.fn((): Observable<BacklogItem[]> => of([])) };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectSprintsPage, 'sprints')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: ProjectApiService, useValue: { get: vi.fn(() => of(projectOf())) } },
      { provide: SprintApiService, useValue: sprintApi },
      { provide: BacklogApiService, useValue: backlogApi },
    ],
  });
  const harness = await RouterTestingHarness.create(sprintsPath);
  await settle(harness);
  return { sprintApi, harness };
}

function bodyText(): string {
  return document.body.textContent ?? '';
}

function alertText(): string {
  return document.querySelector('[role="alert"]')?.textContent ?? '';
}

function headingNamed(text: string): boolean {
  return Array.from(document.querySelectorAll('h2')).some((heading) =>
    heading.textContent?.includes(text),
  );
}

function buttonNamed(name: string | RegExp): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find((button) => {
    const text = button.textContent?.trim() ?? '';
    return typeof name === 'string' ? text === name : name.test(text);
  });
}

describe('ProjectSprintsPage', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('shows the 403 page when the sprints are forbidden, without console output', async () => {
    const { harness } = await renderPage({ list: failWith(403) });

    expect(bodyText()).toContain('No tienes acceso a esta página');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows the active sprint in detail with its issues and burndown', async () => {
    const { harness } = await renderPage();

    expect(bodyText()).toContain('Exportar CSV');
    expect(headingNamed('Sprint 1')).toBe(true);
    expect(document.querySelector('app-burndown-chart')).not.toBeNull();
    // jsdom has no canvas context, so chart.js reports it; the chart is covered by the chart card spec.
    harness.fixture.destroy();
  });

  it('shows the sprint dates as dd/MM/yyyy with date fields instead of native date inputs', async () => {
    const { harness } = await renderPage();

    expect(bodyText()).toContain('02/11/2026 - 08/11/2026');
    expect(document.querySelector('input[type="date"]')).toBeNull();
    expect(document.querySelectorAll('app-date-field')).toHaveLength(2);
    harness.fixture.destroy();
  });

  it('asks for confirmation before closing the active sprint', async () => {
    const { sprintApi, harness } = await renderPage();

    buttonNamed('Cerrar sprint')?.click();
    await settle(harness);
    expect(sprintApi.close).not.toHaveBeenCalled();
    expect(bodyText()).toContain('Esta acción no se puede deshacer');

    const confirm = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).filter(
      (button) => button.textContent?.trim() === 'Cerrar sprint',
    );
    confirm[confirm.length - 1]?.click();
    await settle(harness);

    expect(sprintApi.close).toHaveBeenCalled();
    harness.fixture.destroy();
  });

  it('starts a planned sprint and reloads the list', async () => {
    const { sprintApi, harness } = await renderPage();

    buttonNamed(/Sprint 2/)?.click();
    await settle(harness);
    buttonNamed('Iniciar sprint')?.click();
    await settle(harness);

    expect(sprintApi.start).toHaveBeenCalledWith(PROJECT_ID, 'sprint-2');
    expect(sprintApi.list.mock.calls.length).toBeGreaterThan(1);
    harness.fixture.destroy();
  });

  it('reports a conflict when a second sprint cannot start, in Spanish', async () => {
    const { sprintApi, harness } = await renderPage();
    sprintApi.start.mockReturnValue(failWith(409));

    buttonNamed(/Sprint 2/)?.click();
    await settle(harness);
    buttonNamed('Iniciar sprint')?.click();
    await settle(harness);

    expect(alertText()).toContain('ya hay otro sprint activo');
    harness.fixture.destroy();
  });

  it('does not create a sprint without a name', async () => {
    const { harness } = await renderPage();

    buttonNamed('Planificar sprint')?.click();
    await settle(harness);

    expect(alertText()).toContain('El nombre del sprint es obligatorio');
    harness.fixture.destroy();
  });

  it('shows the skeleton while the sprints load, without empty or list content', async () => {
    const { harness } = await renderPage({ list: new Observable<Sprint[]>(() => undefined) });

    expect(document.querySelector('h1')?.textContent).toContain('Sprints');
    expect(bodyText()).not.toContain('Aún no hay sprints');
    expect(buttonNamed(/Sprint 1/)).toBeUndefined();
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows the empty state for sprints when the project has none', async () => {
    const { harness } = await renderPage({ list: of([]) });

    expect(bodyText()).toContain('Aún no hay sprints');
    expect(bodyText()).toContain('Planifica el primero con el formulario de abajo.');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows an error with retry when the sprints fail for another reason', async () => {
    const { sprintApi, harness } = await renderPage({ list: failWith(500) });

    expect(alertText()).not.toBe('');
    sprintApi.list.mockReturnValue(of([active, planned]));
    buttonNamed('Reintentar')?.click();
    await settle(harness);

    expect(headingNamed('Sprint 1')).toBe(true);
    expect(sprintApi.list).toHaveBeenCalledTimes(2);
    // jsdom has no canvas context, so chart.js reports it; the chart is covered by the chart card spec.
    harness.fixture.destroy();
  });
});
