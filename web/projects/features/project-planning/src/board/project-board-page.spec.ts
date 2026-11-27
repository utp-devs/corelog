import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  AuthStore,
  BoardApiService,
  ProjectApiService,
  ProjectBoard,
  ProjectMember,
  SessionService,
} from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  PROJECT_ID,
  byAriaLabel,
  ensureMatchMedia,
  ME_ID,
  failWith,
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
  members: ProjectMember[] = [],
) {
  TestBed.resetTestingModule();
  const boardApi = {
    getProjectBoard: vi.fn(() => board),
    moveCard: vi.fn(() => of(movedBoard)),
    updateColumnWip: vi.fn(() => of(baseBoard())),
  };
  const projectApi = {
    get: vi.fn(() => projectRead),
    listMembers: vi.fn(() => of(members)),
  };
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

/** Opens the "Mover a" menu of a card and picks a column. */
function moveWithMenu(summary: string, column: string, harness: RouterTestingHarness): void {
  byAriaLabel(`Mover ${summary} a columna`)?.click();
  harness.fixture.detectChanges();
  const item = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find(
    (el) => el.textContent?.trim().startsWith(column),
  );
  if (!item) throw new Error(`no menu item ${column}`);
  item.click();
  harness.fixture.detectChanges();
}

function viewerMember(): ProjectMember {
  return {
    projectId: PROJECT_ID,
    userId: ME_ID,
    role: 'viewer',
    createdAt: '2026-01-01T00:00:00Z',
  };
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
    expect(document.querySelector('[data-testid="board-card-issue-1"] app-avatar')).not.toBeNull();
    expect(bodyText()).toContain('3 pts');
    expect(bodyText()).toContain('INC-ISSUE-');
    expect(boardApi.getProjectBoard).toHaveBeenCalledWith(PROJECT_ID);
    expect(bodyText()).toContain('Sin incidencias');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('moves a card to another column with the card menu alternative', async () => {
    const { boardApi, harness } = await renderPage();

    moveWithMenu('No carga la pantalla', 'En curso', harness);
    await settle(harness);

    expect(boardApi.moveCard).toHaveBeenCalledWith(PROJECT_ID, 'issue-1', 'column-2');
    expect(document.querySelector('[role="status"]')?.textContent).toContain(
      'Incidencia movida a En curso',
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
    expect(document.querySelector('section.project-board__column--full')).not.toBeNull();
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

    expect(byAriaLabel('Límite WIP de En curso')).toBeNull();
    byAriaLabel('Opciones de En curso')?.click();
    harness.fixture.detectChanges();
    const edit = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(
      (el) => el.textContent?.includes('Editar límite WIP'),
    );
    edit?.click();
    harness.fixture.detectChanges();
    expect(byAriaLabel('Límite WIP de En curso')).not.toBeNull();
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

    expect(bodyText()).toContain('En curso');
    expect(byAriaLabel('Opciones de En curso')).toBeNull();
    harness.fixture.destroy();
  });

  it('names the columns in Spanish by status code, never with the raw API text', async () => {
    const board = baseBoard({
      columns: [
        { id: 'c1', name: 'In progress', status: 'in_progress', position: 0, count: 0, cards: [] },
        { id: 'c2', name: 'Resolved', status: 'resolved', position: 1, count: 0, cards: [] },
      ],
    });
    const { harness } = await renderPage(of(board));

    expect(bodyText()).toContain('En curso');
    expect(bodyText()).toContain('Resuelto');
    expect(bodyText()).not.toContain('In progress');
    harness.fixture.destroy();
  });

  it('gives viewers a read-only board without move controls', async () => {
    const { harness } = await renderPage(of(baseBoard()), of(projectOf()), [viewerMember()]);

    expect(bodyText()).toContain('No carga la pantalla');
    expect(bodyText()).toContain('Solo lectura');
    expect(byAriaLabel('Mover No carga la pantalla a columna')).toBeNull();
    expect(document.querySelector('.project-board__card-row--draggable')).toBeNull();
    harness.fixture.destroy();
  });

  it('shows the archived notice and removes the move controls of an archived project', async () => {
    const archived = { ...projectOf(), archivedAt: '2026-03-01T00:00:00Z' };
    const { harness } = await renderPage(of(baseBoard()), of(archived));

    expect(bodyText()).toContain('Este proyecto está archivado y es de solo lectura');
    expect(bodyText()).toContain('No carga la pantalla');
    expect(byAriaLabel('Mover No carga la pantalla a columna')).toBeNull();
    expect(document.querySelector('.project-board__card-row--draggable')).toBeNull();
    harness.fixture.destroy();
  });

  it('does not show the archived notice on a live project', async () => {
    const { harness } = await renderPage();

    expect(document.querySelector('[data-testid="archived-notice"]')).toBeNull();
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
    // One column at a time, chosen with the switcher that shows each count.
    expect(mobileBoard?.querySelectorAll('section')).toHaveLength(1);
    const tabs = document.querySelectorAll('[role="tablist"] [role="tab"]');
    expect(tabs).toHaveLength(2);
    expect(tabs[0].textContent).toContain('Abierto');
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

    moveWithMenu('No carga la pantalla', 'En curso', harness);
    await settle(harness);

    expect(alertText()).toContain('límite WIP');
    expect(bodyText()).toContain('No carga la pantalla');
    harness.fixture.destroy();
  });
});
