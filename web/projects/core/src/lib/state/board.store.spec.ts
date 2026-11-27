import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { SessionService } from '../services/session.service';
import { BoardStore } from './board.store';

const BOARD_URL = '/api/v1/projects/project-1/board';
const MOVE_URL = '/api/v1/projects/project-1/board/cards/issue-1/move';
const WIP_URL = '/api/v1/projects/project-1/board/columns/column-2';

function card(id: string, summary: string, extra: Record<string, unknown> = {}) {
  return {
    id,
    type: 'task',
    summary,
    status: 'open',
    priority: 'low',
    rank: id,
    ...extra,
  };
}

/** Two columns: "Por hacer" has room, "En curso" is full (limit 1, one card). */
function boardDto(overrides: { inProgressCount?: number } = {}) {
  return {
    id: 'board-1',
    project_id: 'project-1',
    name: 'Default',
    type: 'kanban',
    can_manage_board: true,
    columns: [
      {
        id: 'column-1',
        name: 'Por hacer',
        status: 'open',
        position: 0,
        count: 2,
        cards: [
          card('issue-1', 'Error al exportar', {
            type: 'bug',
            priority: 'high',
            assignee_id: 'user-1',
            assignee: 'Ada',
            story_points: 3,
          }),
          card('issue-3', 'Revisar login', { priority: 'low' }),
        ],
      },
      {
        id: 'column-2',
        name: 'En curso',
        status: 'in_progress',
        position: 1,
        wip_limit: 1,
        count: overrides.inProgressCount ?? 1,
        cards: [card('issue-2', 'Documentar API', { status: 'in_progress', priority: 'medium' })],
      },
    ],
  };
}

function setup(): { store: BoardStore; backend: HttpTestingController; session: SessionService } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(BoardStore),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
  };
}

function loadBoard(store: BoardStore, backend: HttpTestingController): void {
  store.load('project-1');
  backend.expectOne(BOARD_URL).flush({ data: boardDto() });
}

describe('BoardStore', () => {
  beforeEach(() => localStorage.clear());

  it('loads the board and keeps every card visible without filters', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    expect(store.board.value()?.canManageBoard).toBe(true);
    expect(store.visibleColumns().map((column) => column.cards.length)).toEqual([2, 1]);
    expect(store.hasColumns()).toBe(true);
    expect(store.board.loading()).toBe(false);
    backend.verify();
  });

  it('filters cards locally by text, priority and assignee without requests', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);
    store.setCurrentUserId('user-1');

    store.setFilter({ search: '  EXPORT ' });
    expect(store.visibleColumns()[0].cards.map((item) => item.id)).toEqual(['issue-1']);

    store.setFilter({ search: '', priority: 'low' });
    expect(store.visibleColumns()[0].cards.map((item) => item.id)).toEqual(['issue-3']);

    store.setFilter({ priority: null, onlyMine: true });
    expect(store.visibleColumns()[0].cards.map((item) => item.id)).toEqual(['issue-1']);
    expect(store.visibleColumns()[1].cards).toEqual([]);
    backend.verify();
  });

  it('moves a card, replaces the board with the response and announces the move', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-2');
    const request = backend.expectOne(MOVE_URL);
    expect(request.request.body).toEqual({ column_id: 'column-2' });
    request.flush({ data: boardDto({ inProgressCount: 2 }) });

    expect(store.board.value()?.columns[1].count).toBe(2);
    expect(store.announcement()).toBe('Incidencia movida a En curso');
    expect(store.moveError()).toBeNull();
    backend.verify();
  });

  it('keeps the board untouched and explains a WIP or flow conflict', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-2');
    backend
      .expectOne(MOVE_URL)
      .flush(
        { error: { message: 'WIP', code: 'board.wip' } },
        { status: 409, statusText: 'Conflict' },
      );

    expect(store.moveError()).toBe(
      'La columna destino alcanzó su límite WIP o el flujo no permite ese cambio.',
    );
    expect(store.board.value()?.columns[0].cards).toHaveLength(2);
    expect(store.announcement()).toBe('');
    backend.verify();
  });

  it('explains that the workflow has no transition to the column', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-2');
    backend
      .expectOne(MOVE_URL)
      .flush(
        { error: { message: 'x', code: 'issue.transition_unavailable' } },
        { status: 409, statusText: 'Conflict' },
      );

    expect(store.moveError()).toContain('no tiene una transición');
    expect(store.pendingInput()).toBeNull();
    expect(store.board.value()?.columns[0].cards).toHaveLength(2);
    backend.verify();
  });

  it('keeps the move pending when the transition needs data from the person', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-2');
    backend
      .expectOne(MOVE_URL)
      .flush(
        { error: { message: 'faltan datos', code: 'workflow.validation_failed' } },
        { status: 422, statusText: 'Unprocessable' },
      );

    expect(store.moveError()).toBeNull();
    expect(store.pendingInput()).toMatchObject({ cardId: 'issue-1', targetColumnId: 'column-2' });
    store.clearPendingInput();
    expect(store.pendingInput()).toBeNull();
    backend.verify();
  });

  it('explains a move forbidden by permissions and a generic failure', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-2');
    backend.expectOne(MOVE_URL).flush({}, { status: 403, statusText: 'Forbidden' });
    expect(store.moveError()).toBe('No tienes permiso para mover incidencias en este proyecto.');

    store.clearMoveError();
    store.move('issue-1', 'column-2');
    backend.expectOne(MOVE_URL).flush({}, { status: 500, statusText: 'Server Error' });
    expect(store.moveError()).toBe('No se pudo mover la incidencia. Intenta de nuevo.');
    backend.verify();
  });

  it('ignores moves to the same column and cards that are not on the board', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-1');
    store.move('unknown', 'column-2');

    backend.verify();
  });

  it('offers every other column as a target and flags full ones as disabled', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    expect(store.moveTargetsFor('issue-1')).toEqual([
      { id: 'column-2', name: 'En curso', disabled: true },
    ]);
    expect(store.moveTargetsFor('missing')).toEqual([]);
  });

  it('saves a WIP limit typed by the user and sends null for an empty field', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.saveWipLimit('column-2', '3');
    const set = backend.expectOne(WIP_URL);
    expect(set.request.method).toBe('PATCH');
    expect(set.request.body).toEqual({ wip_limit: 3 });
    set.flush({ data: boardDto() });

    store.saveWipLimit('column-2', '   ');
    expect(backend.expectOne(WIP_URL).request.body).toEqual({ wip_limit: null });
    backend.verify();
  });

  it('rejects an invalid WIP limit without a request', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.saveWipLimit('column-2', '0');
    store.saveWipLimit('column-2', '2.5');
    store.saveWipLimit('column-2', 'abc');

    expect(store.moveError()).toBe('El límite WIP debe ser un número entero mayor que 0.');
    backend.verify();
  });

  it('makes no request when the WIP limit did not change', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.saveWipLimit('column-2', '1');

    backend.verify();
  });

  it('explains a WIP change forbidden to non-leads and a generic WIP failure', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.saveWipLimit('column-2', '4');
    backend.expectOne(WIP_URL).flush({}, { status: 403, statusText: 'Forbidden' });
    expect(store.moveError()).toBe(
      'Solo el lead del proyecto o el administrador pueden cambiar el límite WIP.',
    );

    store.clearMoveError();
    store.saveWipLimit('column-2', '4');
    backend.expectOne(WIP_URL).flush({}, { status: 500, statusText: 'Server Error' });
    expect(store.moveError()).toBe('No se pudo guardar el límite WIP. Intenta de nuevo.');
    backend.verify();
  });

  it('ignores a response that arrives after the project changed', () => {
    const { store, backend } = setup();
    loadBoard(store, backend);

    store.move('issue-1', 'column-2');
    const pending = backend.expectOne(MOVE_URL);
    store.load('project-2');
    backend.expectOne('/api/v1/projects/project-2/board').flush({
      data: { ...boardDto(), project_id: 'project-2', name: 'Otro' },
    });
    pending.flush({ data: boardDto({ inProgressCount: 2 }) });

    expect(store.board.value()?.name).toBe('Otro');
    backend.verify();
  });

  it('clears the board and filters when the session ends', () => {
    const { store, backend, session } = setup();
    loadBoard(store, backend);
    store.setFilter({ search: 'x' });

    session.startSession('token');
    session.clear();
    TestBed.flushEffects();

    expect(store.board.hasValue()).toBe(false);
    expect(store.filter().search).toBe('');
  });
});
