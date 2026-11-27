import { APP_BASE_HREF } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { Provider } from '@angular/core';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  AuthStore,
  BacklogApiService,
  BacklogItem,
  ProjectApiService,
  ProjectMember,
  SessionService,
  Sprint,
  SprintApiService,
  SprintIssue,
} from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  PROJECT_ID,
  ME_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  projectRoutes,
  signedInAuth,
  signedInSession,
} from '../testing/planning-test-kit';
import { ProjectBacklogPage } from './project-backlog-page';

ensureMatchMedia();

const backlogPath = `/projects/${PROJECT_ID}/backlog`;

const items: BacklogItem[] = [
  {
    id: 'issue-1',
    projectId: PROJECT_ID,
    summary: 'Primera',
    type: 'task',
    status: 'open',
    priority: 'high',
    rank: '000001',
  },
  {
    id: 'issue-2',
    projectId: PROJECT_ID,
    summary: 'Segunda',
    type: 'bug',
    status: 'in_progress',
    priority: 'low',
    rank: '000002',
  },
];

interface SprintFixture {
  readonly sprint: Sprint;
  readonly issues: SprintIssue[];
}

async function settle(harness: RouterTestingHarness): Promise<void> {
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

async function renderPage(
  list: Observable<BacklogItem[]> = of(items),
  extraProviders: Provider[] = [],
  members: ProjectMember[] = [],
  sprints: SprintFixture[] = [],
) {
  TestBed.resetTestingModule();
  const backlogApi = {
    list: vi.fn(() => list),
    move: vi.fn((_projectId: string, _issueId: string, _position: number) =>
      of([items[1], items[0]]),
    ),
  };
  const sprintApi = {
    list: vi.fn(() => of(sprints.map((entry) => entry.sprint))),
    listIssues: vi.fn((_projectId: string, sprintId: string) =>
      of(sprints.find((entry) => entry.sprint.id === sprintId)?.issues ?? []),
    ),
  };
  const projectApi = {
    get: vi.fn(() => of(projectOf())),
    listMembers: vi.fn(() => of(members)),
  };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectBacklogPage, 'backlog')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: BacklogApiService, useValue: backlogApi },
      { provide: SprintApiService, useValue: sprintApi },
      { provide: ProjectApiService, useValue: projectApi },
      { provide: AuthStore, useValue: signedInAuth() },
      ...extraProviders,
    ],
  });
  const harness = await RouterTestingHarness.create(backlogPath);
  await settle(harness);
  return { backlogApi, sprintApi, harness };
}

function buttonNamed(label: string): HTMLButtonElement | null {
  return document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
}

function bodyText(): string {
  return document.body.textContent ?? '';
}

function alertText(): string {
  return document.querySelector('[role="alert"]')?.textContent ?? '';
}

describe('ProjectBacklogPage', () => {
  const consoleGuard = guardConsole();
  let restoreViewport: (() => void) | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    restoreViewport?.();
    restoreViewport = undefined;
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('renders the ordered rows and an empty state inside the table when there are no issues', async () => {
    const { harness } = await renderPage(of([]));

    expect(bodyText()).toContain('El backlog está vacío');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows the 403 page when the backlog is forbidden', async () => {
    const { harness } = await renderPage(failWith(403));

    expect(bodyText()).toContain('No tienes acceso a esta página');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('lists the issues in rank order with their position', async () => {
    const { harness } = await renderPage();

    expect(bodyText()).toContain('Primera');
    expect(bodyText()).toContain('Segunda');
    expect(buttonNamed('Subir Primera')?.disabled).toBe(true);
    expect(buttonNamed('Bajar Segunda')?.disabled).toBe(true);
    harness.fixture.destroy();
  });

  it('shows type and status in Spanish with the short key', async () => {
    const { harness } = await renderPage();

    expect(bodyText()).toContain('Tarea');
    expect(bodyText()).toContain('Error');
    expect(bodyText()).toContain('Abierto');
    expect(bodyText()).toContain('En curso');
    expect(bodyText()).not.toContain('Abierta');
    harness.fixture.destroy();
  });

  it('gives the arrow buttons a visible target and a start/end menu', async () => {
    const { backlogApi, harness } = await renderPage();

    expect(document.querySelector('.project-backlog__arrow')).not.toBeNull();
    buttonNamed('Más opciones de orden de Segunda')?.click();
    harness.fixture.detectChanges();
    const toStart = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(
      (el) => el.textContent?.includes('Mover al inicio'),
    );
    toStart?.click();
    await settle(harness);

    expect(backlogApi.move).toHaveBeenCalledWith(PROJECT_ID, 'issue-2', 0);
    harness.fixture.destroy();
  });

  it('hides the reorder controls from viewers', async () => {
    const { harness } = await renderPage(
      of(items),
      [],
      [{ projectId: PROJECT_ID, userId: ME_ID, role: 'viewer', createdAt: '2026-01-01T00:00:00Z' }],
    );

    expect(bodyText()).toContain('Primera');
    expect(buttonNamed('Bajar Primera')).toBeNull();
    expect(bodyText()).toContain('Solo lectura');
    harness.fixture.destroy();
  });

  it('moves an issue one position down and shows the reordered list', async () => {
    const { backlogApi, harness } = await renderPage();

    buttonNamed('Bajar Primera')?.click();
    await settle(harness);

    expect(backlogApi.move).toHaveBeenCalledWith(PROJECT_ID, 'issue-1', 1);
    expect(buttonNamed('Subir Primera')?.disabled).toBe(false);
    harness.fixture.destroy();
  });

  it('reports a forbidden reorder in Spanish', async () => {
    const { backlogApi, harness } = await renderPage();
    backlogApi.move.mockReturnValue(failWith(403));

    buttonNamed('Bajar Primera')?.click();
    await settle(harness);

    expect(alertText()).toContain('No tienes permiso para ordenar el backlog');
    harness.fixture.destroy();
  });

  it('shows the table skeleton with busy feedback while the backlog loads', async () => {
    const { harness } = await renderPage(NEVER);

    await vi.waitFor(() => {
      const loading = document.querySelector('[role="status"][aria-label="Cargando"]');
      expect(loading?.getAttribute('aria-busy')).toBe('true');
    });
    expect(bodyText()).not.toContain('El backlog está vacío');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows an error inside the table with retry when the backlog fails for another reason', async () => {
    const { backlogApi, harness } = await renderPage(failWith(500));

    expect(alertText()).toContain('No se pudo cargar el backlog');
    backlogApi.list.mockReturnValue(of(items));
    Array.from(document.querySelectorAll('button'))
      .find((button) => button.textContent?.trim() === 'Reintentar')
      ?.click();
    await settle(harness);

    expect(bodyText()).toContain('Primera');
    expect(backlogApi.list).toHaveBeenCalledTimes(2);
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('renders a table on desktop and issue cards with move buttons on mobile', async () => {
    restoreViewport = installViewport(1440);
    const desktop = await renderPage();

    expect(document.querySelectorAll('table').length).toBeGreaterThan(0);
    desktop.harness.fixture.destroy();

    restoreViewport();
    restoreViewport = installViewport(390);
    const mobile = await renderPage();

    expect(bodyText()).toContain('Primera');
    expect(document.querySelector('table')).toBeNull();
    const cards = document.querySelectorAll('li');
    expect(cards.length).toBe(2);
    expect(cards[0]?.contains(buttonNamed('Bajar Primera'))).toBe(true);
    consoleGuard.expectNoConsoleOutput();
    mobile.harness.fixture.destroy();
  });

  it('links each row summary to the issue under the workspace base without repeating it', async () => {
    const { harness } = await renderPage(of(items), [
      { provide: APP_BASE_HREF, useValue: '/workspace/' },
    ]);
    const hrefs = Array.from(
      document.querySelectorAll<HTMLAnchorElement>('a.project-backlog__summary'),
    ).map((link) => link.getAttribute('href'));
    expect(hrefs).toContain('/workspace/browse/issue-1');
    expect(hrefs.some((href) => href?.includes('/workspace/workspace/'))).toBe(false);
    harness.fixture.destroy();
  });

  describe('sprint sections', () => {
    const activeSprint: Sprint = {
      id: 'sprint-1',
      projectId: PROJECT_ID,
      name: 'Sprint 1',
      goal: 'Entregar el tablero',
      status: 'active',
      startDate: '2026-11-02',
      endDate: '2026-11-15',
    };
    const sprintIssue = (
      id: string,
      title: string,
      status: SprintIssue['status'],
    ): SprintIssue => ({
      id,
      title,
      status,
      priority: 'medium',
    });
    const withSprint: SprintFixture[] = [
      {
        sprint: activeSprint,
        issues: [
          sprintIssue('issue-1', 'Primera', 'resolved'),
          sprintIssue('issue-9', 'Novena', 'open'),
        ],
      },
    ];

    function toggleOf(name: string): HTMLButtonElement | null {
      return (
        Array.from(document.querySelectorAll<HTMLButtonElement>('button[aria-expanded]')).find(
          (button) => button.textContent?.includes(name),
        ) ?? null
      );
    }

    it('shows each open sprint with dates, goal, progress and its issues, apart from the backlog', async () => {
      const { harness } = await renderPage(of(items), [], [], withSprint);

      const block = toggleOf('Sprint 1')?.closest('section');
      expect(block?.textContent).toContain('02/11/2026 - 15/11/2026');
      expect(block?.textContent).toContain('Entregar el tablero');
      expect(block?.textContent).toContain('Activo');
      expect(block?.textContent).toContain('Primera');
      expect(block?.textContent).toContain('Novena');
      const bar = block?.querySelector('[role="progressbar"]');
      expect(bar?.getAttribute('aria-valuenow')).toBe('50');
      expect(block?.textContent).toContain('Resueltas 1 de 2');

      const table = document.querySelector('table');
      expect(table?.textContent).toContain('Segunda');
      expect(table?.textContent).not.toContain('Primera');
      expect(toggleOf('Backlog')?.textContent).toContain('1 sin planificar');
      harness.fixture.destroy();
    });

    it('collapses and expands each section and keeps aria-expanded in step', async () => {
      const { harness } = await renderPage(of(items), [], [], withSprint);
      const sprintToggle = toggleOf('Sprint 1') as HTMLButtonElement;
      expect(sprintToggle.getAttribute('aria-expanded')).toBe('true');
      const panelId = sprintToggle.getAttribute('aria-controls') as string;
      expect(document.getElementById(panelId)).not.toBeNull();

      sprintToggle.click();
      harness.fixture.detectChanges();
      expect(sprintToggle.getAttribute('aria-expanded')).toBe('false');
      expect(document.getElementById(panelId)).toBeNull();
      expect(bodyText()).toContain('Entregar el tablero');

      const backlogToggle = toggleOf('Backlog') as HTMLButtonElement;
      backlogToggle.click();
      harness.fixture.detectChanges();
      expect(backlogToggle.getAttribute('aria-expanded')).toBe('false');
      expect(document.querySelector('table')).toBeNull();

      sprintToggle.click();
      backlogToggle.click();
      harness.fixture.detectChanges();
      expect(sprintToggle.getAttribute('aria-expanded')).toBe('true');
      expect(backlogToggle.getAttribute('aria-expanded')).toBe('true');
      harness.fixture.destroy();
    });

    it('reorders only the unplanned issues, using their place in the whole backlog', async () => {
      const three: BacklogItem[] = [
        ...items,
        { ...items[0], id: 'issue-3', summary: 'Tercera', rank: '000003' },
      ];
      const { backlogApi, harness } = await renderPage(
        of(three),
        [],
        [],
        [
          {
            sprint: activeSprint,
            issues: [sprintIssue('issue-2', 'Segunda', 'open')],
          },
        ],
      );

      expect(buttonNamed('Bajar Primera')?.disabled).toBe(false);
      buttonNamed('Bajar Primera')?.click();
      await settle(harness);

      expect(backlogApi.move).toHaveBeenCalledWith(PROJECT_ID, 'issue-1', 2);
      harness.fixture.destroy();
    });

    it('says everything is planned when no issue is left outside a sprint', async () => {
      const { harness } = await renderPage(
        of(items),
        [],
        [],
        [
          {
            sprint: activeSprint,
            issues: [
              sprintIssue('issue-1', 'Primera', 'open'),
              sprintIssue('issue-2', 'Segunda', 'open'),
            ],
          },
        ],
      );

      expect(bodyText()).toContain('Todo está planificado');
      expect(bodyText()).not.toContain('El backlog está vacío');
      harness.fixture.destroy();
    });

    it('keeps the backlog when the sprints cannot be read', async () => {
      const failed = await renderPage(of(items), [
        {
          provide: SprintApiService,
          useValue: { list: () => failWith(500), listIssues: () => of([]) },
        },
      ]);

      expect(bodyText()).toContain('Primera');
      expect(bodyText()).toContain('2 sin planificar');
      failed.harness.fixture.destroy();
    });
  });
});
