import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  AuthStore,
  BoardApiService,
  ProjectApiService,
  ProjectBoard,
  SessionService,
} from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  PROJECT_ID,
  byAriaLabel,
  ensureMatchMedia,
  failWith,
  pickOption,
  projectOf,
  projectRoutes,
  signedInAuth,
  signedInSession,
} from '../testing/planning-test-kit';
import { ProjectBoardPage } from './project-board-page';

ensureMatchMedia();

const boardPath = `/projects/${PROJECT_ID}/board`;

function baseBoard(overrides: Partial<ProjectBoard> = {}): ProjectBoard {
  return {
    id: 'board-1',
    projectId: PROJECT_ID,
    name: 'Default',
    type: 'kanban',
    canManageBoard: false,
    columns: [
      {
        id: 'column-1',
        name: 'Abierto',
        status: 'open',
        position: 0,
        count: 1,
        cards: [
          {
            id: 'issue-1',
            type: 'bug',
            summary: 'No carga la pantalla',
            status: 'open',
            priority: 'high',
            assignee: 'Ada Lovelace',
            storyPoints: 3,
            rank: 'm',
          },
        ],
      },
      {
        id: 'column-2',
        name: 'En progreso',
        status: 'in_progress',
        position: 1,
        count: 0,
        cards: [],
      },
    ],
    ...overrides,
  };
}

const movedBoard: ProjectBoard = baseBoard({
  columns: [
    { id: 'column-1', name: 'Abierto', status: 'open', position: 0, count: 0, cards: [] },
    {
      id: 'column-2',
      name: 'En progreso',
      status: 'in_progress',
      position: 1,
      count: 1,
      cards: [],
    },
  ],
});

async function settle(harness: RouterTestingHarness): Promise<void> {
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

async function renderPage(
  board: Observable<ProjectBoard> = of(baseBoard()),
  projectRead: Observable<unknown> = of(projectOf()),
) {
  TestBed.resetTestingModule();
  const boardApi = {
    getProjectBoard: vi.fn(() => board),
    moveCard: vi.fn(() => of(movedBoard)),
    updateColumnWip: vi.fn(() => of(baseBoard())),
  };
  const projectApi = { get: vi.fn(() => projectRead) };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectBoardPage, 'board')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: BoardApiService, useValue: boardApi },
      { provide: ProjectApiService, useValue: projectApi },
      { provide: AuthStore, useValue: signedInAuth() },
    ],
  });
  const harness = await RouterTestingHarness.create(boardPath);
  await settle(harness);
  return { boardApi, harness };
}

function bodyText(): string {
  return document.body.textContent ?? '';
}

function alertText(): string {
  return document.querySelector('[role="alert"]')?.textContent ?? '';
}

function regionNamed(name: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[role="region"][aria-label="${name}"]`);
}

describe('ProjectBoardPage', () => {
  const consoleGuard = guardConsole();
  let restoreViewport: (() => void) | undefined;

  afterEach(() => {
    restoreViewport?.();
    restoreViewport = undefined;
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('renders ordered columns, counts and issue card details', async () => {
    const { boardApi, harness } = await renderPage();

    expect(bodyText()).toContain('No carga la pantalla');
    expect(document.querySelector('h1')?.textContent).toContain('Tablero');
    expect(bodyText()).toContain('Responsable: Ada Lovelace');
    expect(bodyText()).toContain('3 puntos');
    expect(boardApi.getProjectBoard).toHaveBeenCalledWith(PROJECT_ID);
    expect(bodyText()).toContain('Sin incidencias');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('moves a card to another column with the keyboard select alternative', async () => {
    const { boardApi, harness } = await renderPage();

    pickOption(
      byAriaLabel('Mover No carga la pantalla a columna') as HTMLElement,
      'En progreso',
      harness.fixture,
    );
    await settle(harness);

    expect(boardApi.moveCard).toHaveBeenCalledWith(PROJECT_ID, 'issue-1', 'column-2');
    expect(document.querySelector('[role="status"]')?.textContent).toContain(
      'Incidencia movida a En progreso',
    );
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows a useful error when project access or board loading fails', async () => {
    const { harness } = await renderPage(failWith(500));

    expect(alertText()).toContain('No se pudo cargar el tablero');
    harness.fixture.destroy();
  });

  it('shows the wip limit next to the count and flags a full column', async () => {
    const board = baseBoard({
      columns: [
        {
          id: 'column-1',
          name: 'En curso',
          status: 'in_progress',
          position: 0,
          count: 2,
          wipLimit: 2,
          cards: [
            {
              id: 'issue-2',
              type: 'task',
              summary: 'Trabajo en curso',
              status: 'in_progress',
              priority: 'medium',
              assignee: 'Grace Hopper',
              rank: 'n',
            },
          ],
        },
      ],
    });
    const { harness } = await renderPage(of(board));

    expect(bodyText()).toContain('2 / 2');
    expect(bodyText()).toContain('Trabajo en curso');
    expect(document.querySelector('section.column-full')).not.toBeNull();
    harness.fixture.destroy();
  });

  it('lets a manager edit the wip limit of a column', async () => {
    const board = baseBoard({
      canManageBoard: true,
      columns: [
        {
          id: 'column-2',
          name: 'En progreso',
          status: 'in_progress',
          position: 1,
          count: 0,
          cards: [],
        },
      ],
    });
    const { harness } = await renderPage(of(board));

    expect(byAriaLabel('Límite WIP de En progreso')).not.toBeNull();
    harness.fixture.destroy();
  });

  it('hides the wip editor from users who cannot manage the board', async () => {
    const board = baseBoard({
      canManageBoard: false,
      columns: [
        {
          id: 'column-2',
          name: 'En progreso',
          status: 'in_progress',
          position: 1,
          count: 0,
          cards: [],
        },
      ],
    });
    const { harness } = await renderPage(of(board));

    expect(bodyText()).toContain('En progreso');
    expect(byAriaLabel('Límite WIP de En progreso')).toBeNull();
    harness.fixture.destroy();
  });

  it('shows skeleton columns with busy feedback while the board loads', async () => {
    const { harness } = await renderPage(NEVER);

    const loading = byAriaLabel('Cargando tablero');
    expect(loading?.getAttribute('aria-busy')).toBe('true');
    expect(regionNamed('Tablero Default')).toBeNull();
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows the 403 page when the board is forbidden', async () => {
    const { harness } = await renderPage(failWith(403));

    expect(bodyText()).toContain('No tienes acceso a esta página');
    expect(regionNamed('Tablero Default')).toBeNull();
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows an empty hint inside a column that has no issues', async () => {
    const { harness } = await renderPage();

    expect(regionNamed('Tablero Default')?.textContent).toContain('Arrastra incidencias aquí.');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows an empty state when the project has no columns', async () => {
    const { harness } = await renderPage(of(baseBoard({ columns: [] })));

    expect(bodyText()).toContain('El proyecto todavía no tiene columnas de tablero.');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('scrolls the columns inside the board region on mobile and lays them out on desktop', async () => {
    restoreViewport = installViewport(390);
    const mobile = await renderPage();
    const mobileBoard = regionNamed('Tablero Default');
    expect(mobileBoard?.getAttribute('data-scroll')).toBe('inside-board');
    expect(mobileBoard?.getAttribute('data-layout')).toBe('mobile-columns');
    expect(mobileBoard?.hasAttribute('data-scroll-x')).toBe(true);
    mobile.harness.fixture.destroy();

    restoreViewport();
    restoreViewport = installViewport(1440);
    const desktop = await renderPage();
    expect(regionNamed('Tablero Default')?.getAttribute('data-layout')).toBe('desktop-columns');
    consoleGuard.expectNoConsoleOutput();
    desktop.harness.fixture.destroy();
  });

  it('reports a move that the server rejects in Spanish, without changing the board', async () => {
    const { boardApi, harness } = await renderPage();
    boardApi.moveCard.mockReturnValue(failWith(409));

    pickOption(
      byAriaLabel('Mover No carga la pantalla a columna') as HTMLElement,
      'En progreso',
      harness.fixture,
    );
    await settle(harness);

    expect(alertText()).toContain('límite WIP');
    expect(bodyText()).toContain('No carga la pantalla');
    harness.fixture.destroy();
  });
});
