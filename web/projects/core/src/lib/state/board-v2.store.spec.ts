import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { NEVER, Subject, of, throwError } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { provideCorelogTesting } from '../../testing';
import type { ApiError } from '../models/api-error.model';
import type { BoardMoveResult, BoardSummary, BoardView } from '../models/board-v2.model';
import { BoardV2ApiService } from '../services/api/board-v2-api.service';
import { BoardViewStore, boardMoveErrorMessage, boardSettingsErrorMessage } from './board-v2.store';
import { viewOf } from './board-v2-fixtures';

const SUMMARY: BoardSummary = {
  id: 'b1',
  projectId: 'p1',
  name: 'Tablero',
  type: 'kanban',
  isDefault: true,
  columnCount: 3,
  swimlaneMode: 'none',
};

function errorOf(status: number, code: string, details: ApiError['details'] = []): ApiError {
  return { status, code, message: 'x', details };
}

function resultOf(exceeded = false): BoardMoveResult {
  return {
    issueId: 'a',
    columnId: 'c2',
    status: { id: 's-doing', name: 'En curso', category: 'in_progress' },
    transitionId: 't',
    transitioned: true,
    rank: 'n',
    wip: {
      columnId: 'c2',
      count: 3,
      minIssues: null,
      maxIssues: 2,
      state: exceeded ? 'over_max' : 'ok',
      exceeded,
    },
  };
}

interface Fake {
  list: ReturnType<typeof vi.fn>;
  view: ReturnType<typeof vi.fn>;
  moveCard: ReturnType<typeof vi.fn>;
}

function setup(view: BoardView = viewOf()): { store: BoardViewStore; api: Fake } {
  const api: Fake = {
    list: vi.fn(() => of([SUMMARY])),
    view: vi.fn(() => of(view)),
    moveCard: vi.fn(),
  };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: BoardV2ApiService, useValue: api },
    ],
  });
  const store = TestBed.inject(BoardViewStore);
  store.open('p1');
  return { store, api };
}

const idsOf = (store: BoardViewStore, column: number): string[] =>
  (store.view.value()?.columns[column].cards ?? []).map((card) => card.id);

describe('BoardViewStore', () => {
  beforeEach(() => localStorage.clear());

  it('opens the default board and loads its view', () => {
    const { store, api } = setup();
    expect(store.boardId()).toBe('b1');
    expect(api.view).toHaveBeenCalledWith('b1', { quickFilterIds: [] });
    expect(store.view.value()?.columns).toHaveLength(3);
    expect(store.canMove()).toBe(true);
  });

  it('remembers the board the person picked for the project', () => {
    const { store } = setup();
    store.select('b2');
    expect(localStorage.getItem('corelog.board.p1')).toBe('b2');
  });

  it('shows a move at once and keeps it when the server accepts it', () => {
    const { store, api } = setup();
    const answer = new Subject<BoardMoveResult>();
    api.moveCard.mockReturnValue(answer);

    store.move('a', 'c2', { beforeIssueId: 'd' });
    expect(idsOf(store, 0)).toEqual(['b', 'c']);
    expect(idsOf(store, 1)).toEqual(['a', 'd']);
    expect(api.moveCard).toHaveBeenCalledWith('b1', 'a', {
      columnId: 'c2',
      beforeIssueId: 'd',
      afterIssueId: undefined,
      swimlane: undefined,
      transitionId: undefined,
    });

    answer.next(resultOf());
    answer.complete();
    expect(store.announcement()).toContain('movida a c2');
    expect(store.moveError()).toBeNull();
    expect(api.view).toHaveBeenCalledTimes(2);
  });

  it('warns, without blocking, when the move leaves the column over its limit', () => {
    const { store, api } = setup();
    api.view.mockReturnValue(NEVER);
    api.moveCard.mockReturnValue(of(resultOf(true)));
    store.move('a', 'c2');
    expect(store.wipWarning()).toContain('límite');
    expect(idsOf(store, 1)).toContain('a');
  });

  it('rolls the board back and explains a move the workflow does not allow', () => {
    const { store, api } = setup();
    api.moveCard.mockReturnValue(throwError(() => errorOf(409, 'issue.transition_unavailable')));
    store.move('a', 'c2');
    expect(idsOf(store, 0)).toEqual(['a', 'b', 'c']);
    expect(idsOf(store, 1)).toEqual(['d']);
    expect(store.moveError()).toContain('transición');
  });

  it('asks which transition to use when several lead to the column', () => {
    const { store, api } = setup();
    api.moveCard.mockReturnValueOnce(
      throwError(() =>
        errorOf(409, 'board.ambiguous_transition', [
          { field: 'start', message: 'Empezar' },
          { field: 'review', message: 'Revisar' },
        ]),
      ),
    );
    store.move('a', 'c2');
    expect(idsOf(store, 0)).toEqual(['a', 'b', 'c']);
    expect(store.choice()?.candidates.map((c) => c.id)).toEqual(['start', 'review']);

    api.view.mockReturnValue(NEVER);
    api.moveCard.mockReturnValueOnce(of(resultOf()));
    store.resolveChoice('review');
    expect(api.moveCard.mock.calls[1][2]).toMatchObject({ columnId: 'c2', transitionId: 'review' });
    expect(store.choice()).toBeNull();
    expect(idsOf(store, 1)).toContain('a');
  });

  it('asks for the data a transition needs instead of failing', () => {
    const { store, api } = setup();
    api.moveCard.mockReturnValue(
      throwError(() =>
        errorOf(422, 'issue.required_field_missing', [
          { field: 'transition_id', message: 'finish' },
          { field: 'comment', message: 'obligatorio' },
        ]),
      ),
    );
    store.move('a', 'c3');
    expect(store.inputRequest()).toEqual({ cardId: 'a', transitionId: 'finish' });
    expect(store.moveError()).toBeNull();
    expect(idsOf(store, 0)).toEqual(['a', 'b', 'c']);
  });

  it('does not move anything when the person cannot move cards', () => {
    const base = viewOf();
    const { store, api } = setup({ ...base, board: { ...base.board, canMoveCards: false } });
    store.move('a', 'c2');
    expect(api.moveCard).not.toHaveBeenCalled();
    expect(idsOf(store, 0)).toEqual(['a', 'b', 'c']);
  });

  it('reads the view again with the quick filters that are on', () => {
    const { store, api } = setup();
    store.toggleFilter('f1');
    store.toggleFilter('f2');
    expect(api.view).toHaveBeenLastCalledWith('b1', { quickFilterIds: ['f1', 'f2'] });
    store.toggleFilter('f1');
    expect(store.activeFilterIds()).toEqual(['f2']);
    store.clearFilters();
    expect(api.view).toHaveBeenLastCalledWith('b1', { quickFilterIds: [] });
  });

  it('toggles swimlane collapse and the estimation shown', () => {
    const { store } = setup();
    store.toggleLane('lane');
    expect(store.collapsedLanes().has('lane')).toBe(true);
    store.toggleLane('lane');
    expect(store.collapsedLanes().has('lane')).toBe(false);
    expect(store.estimation()).toBe('story_points');
    store.setEstimation('issue_count');
    expect(store.estimation()).toBe('issue_count');
  });
});

describe('board error messages', () => {
  it('maps the move codes to Spanish', () => {
    expect(boardMoveErrorMessage(errorOf(409, 'project.archived'))).toContain('archivado');
    expect(boardMoveErrorMessage(errorOf(409, 'issue.status_conflict'))).toContain('Otra persona');
    expect(boardMoveErrorMessage(errorOf(403, 'forbidden'))).toContain('permiso');
    expect(boardMoveErrorMessage(errorOf(500, 'x'))).toContain('No se pudo mover');
  });

  it('maps the settings codes to Spanish', () => {
    expect(boardSettingsErrorMessage(errorOf(409, 'board.name_taken'))).toContain('Ya existe');
    expect(boardSettingsErrorMessage(errorOf(422, 'board.invalid_query'))).toContain('consulta');
    expect(boardSettingsErrorMessage(errorOf(403, 'x'))).toContain('administra');
  });
});
