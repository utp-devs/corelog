import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  BacklogV2ApiService,
  BoardV2ApiService,
  BoardView,
  ProjectApiService,
  SessionService,
  SprintV2ApiService,
} from '@corelog/core';
import { guardConsole, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { BOARD_SUMMARY, boardViewOf, sprintOf } from '../testing/agile-fixtures';
import {
  PROJECT_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  projectRoutes,
  signedInSession,
} from '../testing/planning-test-kit';
import { ProjectBoardPage } from './project-board-page';

ensureMatchMedia();

const boardPath = `/projects/${PROJECT_ID}/board`;

async function settle(harness: RouterTestingHarness): Promise<void> {
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

interface Options {
  view?: Observable<BoardView>;
  project?: Observable<unknown>;
}

async function renderPage(options: Options = {}) {
  TestBed.resetTestingModule();
  const boardApi = {
    list: vi.fn(() => of([BOARD_SUMMARY])),
    view: vi.fn(() => options.view ?? of(boardViewOf())),
    moveCard: vi.fn((_board: string, _issue: string, _input: unknown) =>
      of({
        issueId: '1',
        columnId: 'col-doing',
        status: { id: 's-doing', name: 'En curso', category: 'in_progress' },
        transitionId: 't',
        transitioned: true,
        rank: 'n',
        wip: null,
      }),
    ),
    update: vi.fn(),
  };
  const projectApi = {
    get: vi.fn(() => options.project ?? of(projectOf())),
    listMembers: vi.fn(() => of([])),
  };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectBoardPage, 'board')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: BoardV2ApiService, useValue: boardApi },
      { provide: ProjectApiService, useValue: projectApi },
      { provide: SprintV2ApiService, useValue: { list: vi.fn(() => of([sprintOf('1')])) } },
      { provide: BacklogV2ApiService, useValue: { get: vi.fn(() => NEVER) } },
    ],
  });
  const harness = await RouterTestingHarness.create(boardPath);
  await settle(harness);
  return { boardApi, harness };
}

function text(): string {
  return document.body.textContent ?? '';
}

describe('ProjectBoardPage', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => localStorage.clear());
  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('renders the columns with counts, WIP limits and the cards', async () => {
    const { boardApi, harness } = await renderPage();

    expect(boardApi.view).toHaveBeenCalledWith('board-1', { quickFilterIds: [] });
    expect(text()).toContain('Incidencia 1');
    expect(text()).toContain('CORE-3');
    const headers = Array.from(document.querySelectorAll('app-board-column-header')).map((el) =>
      el.querySelector('.board-column-header__name')?.textContent?.trim(),
    );
    expect(headers).toEqual(['Por hacer', 'En curso', 'Hecho']);
    expect(
      document.querySelector('[data-testid="column-header-col-doing"]')?.textContent,
    ).toContain('1/1');
    const cell = document.querySelector('[data-column="col-doing"]');
    expect(cell?.getAttribute('data-wip')).toBe('at_max');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('fills the width with as many equal columns as the board has', async () => {
    const { harness } = await renderPage();
    const grid = document.querySelector<HTMLElement>('.board-grid');
    expect(grid?.style.getPropertyValue('--board-cols')).toBe('3');
    harness.fixture.destroy();
  });

  it('moves a card with the menu, shows it at once and calls the move endpoint', async () => {
    const { boardApi, harness } = await renderPage();

    document
      .querySelector<HTMLButtonElement>('[aria-label="Mover CORE-1 a otra columna"]')
      ?.click();
    harness.fixture.detectChanges();
    const item = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find(
      (el) => el.textContent?.trim() === 'En curso',
    );
    item?.click();
    await settle(harness);

    expect(boardApi.moveCard).toHaveBeenCalledTimes(1);
    expect(boardApi.moveCard.mock.calls[0].slice(0, 2)).toEqual(['board-1', '1']);
    expect(boardApi.moveCard.mock.calls[0][2]).toMatchObject({ columnId: 'col-doing' });
    expect(document.querySelector('[role="status"]')?.textContent).toContain(
      'CORE-1 movida a En curso',
    );
    harness.fixture.destroy();
  });

  it('moves a card to the next column with Alt+ArrowRight', async () => {
    const { boardApi, harness } = await renderPage();
    const card = document.querySelector('[data-testid="kanban-card-CORE-1"]') as HTMLElement;
    card.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'ArrowRight', altKey: true, bubbles: true }),
    );
    await settle(harness);
    expect(boardApi.moveCard.mock.calls[0][2]).toMatchObject({ columnId: 'col-doing' });
    harness.fixture.destroy();
  });

  it('turns a quick filter on and reads the view with it', async () => {
    const { boardApi, harness } = await renderPage();
    const chip = document.querySelector<HTMLButtonElement>(
      '[data-testid="quick-filter-Mis incidencias"]',
    );
    expect(chip?.getAttribute('aria-pressed')).toBe('false');
    chip?.click();
    await settle(harness);
    expect(boardApi.view).toHaveBeenLastCalledWith('board-1', { quickFilterIds: ['qf-1'] });
    expect(
      document
        .querySelector('[data-testid="quick-filter-Mis incidencias"]')
        ?.getAttribute('aria-pressed'),
    ).toBe('true');
    harness.fixture.destroy();
  });

  it('gives viewers no way to move cards or create issues', async () => {
    const { harness } = await renderPage({
      view: of(boardViewOf({ canMove: false, canManage: false })),
    });
    expect(document.querySelector('[aria-label^="Mover CORE"]')).toBeNull();
    expect(document.querySelector('[data-testid="board-create-open"]')).toBeNull();
    expect(document.querySelector('.cdk-drag-disabled')).not.toBeNull();
    expect(document.querySelector('[data-testid="swimlane-select"]')).toBeNull();
    harness.fixture.destroy();
  });

  it('lets people who can move cards create an issue at the end of the first column', async () => {
    const { harness } = await renderPage();
    expect(document.querySelector('[data-testid="board-create-open"]')).not.toBeNull();
    document.querySelector<HTMLButtonElement>('[data-testid="board-create-open"]')?.click();
    harness.fixture.detectChanges();
    expect(document.querySelector('[data-testid="board-create-input"]')).not.toBeNull();
    harness.fixture.destroy();
  });

  it('explains that a scrum board needs an active sprint', async () => {
    const { harness } = await renderPage({ view: of(boardViewOf({ type: 'scrum' })) });
    expect(text()).toContain('No hay un sprint activo');
    expect(document.querySelector('.board-grid')).toBeNull();
    harness.fixture.destroy();
  });

  it('shows a retryable error when the board cannot load', async () => {
    const { harness } = await renderPage({ view: failWith(500) });
    expect(document.querySelector('app-error-state')).not.toBeNull();
    harness.fixture.destroy();
  });

  it('shows the board read-only in an archived project', async () => {
    const archived = { ...projectOf(), archivedAt: '2026-02-01T00:00:00Z' };
    const { harness } = await renderPage({ project: of(archived) });
    expect(document.querySelector('[data-testid="archived-notice"]')).not.toBeNull();
    expect(document.querySelector('[aria-label^="Mover CORE"]')).toBeNull();
    expect(document.querySelector('[data-testid="board-create-open"]')).toBeNull();
    harness.fixture.destroy();
  });

  it('shows a skeleton while the view loads', async () => {
    const { harness } = await renderPage({ view: NEVER });
    expect(document.querySelector('[aria-label="Cargando tablero"]')).not.toBeNull();
    harness.fixture.destroy();
  });
});
