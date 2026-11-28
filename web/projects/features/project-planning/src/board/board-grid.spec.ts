import { BoardView, UNMAPPED_COLUMN_ID } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { boardViewOf, cardOf } from '../testing/agile-fixtures';
import { boardHeaders, boardRows, estimateText, moveAnnouncement } from './board-grid';

function withLanes(): BoardView {
  const base = boardViewOf();
  return {
    ...base,
    board: { ...base.board, swimlaneMode: 'assignee' },
    swimlanes: [
      { id: 'u1', name: 'Ada', kind: 'assignee', count: 2 },
      { id: 'unassigned', name: 'Sin asignar', kind: 'assignee', count: 1 },
    ],
    columns: base.columns.map((column) => ({
      ...column,
      cards: column.cards.map((card, index) => ({
        ...card,
        swimlaneId: card.id === '3' || index === 0 ? 'u1' : 'unassigned',
      })),
    })),
  };
}

describe('boardHeaders', () => {
  it('lists the columns with their WIP data and the estimate under the chosen estimation', () => {
    const headers = boardHeaders(boardViewOf(), 'story_points');
    expect(headers.map((h) => h.name)).toEqual(['Por hacer', 'En curso', 'Hecho']);
    expect(headers[0].estimate).toBe(4);
    expect(headers[1]).toMatchObject({ max: 1, wipState: 'at_max' });
    expect(boardHeaders(boardViewOf(), 'issue_count')[0].estimate).toBe(2);
  });

  it('adds the unmapped column only when it holds cards and the board shows it', () => {
    const base = boardViewOf();
    const withUnmapped: BoardView = {
      ...base,
      unmappedColumn: {
        id: UNMAPPED_COLUMN_ID,
        name: 'Sin asignar',
        statusIds: ['x'],
        count: 1,
        total: 1,
        estimate: 0,
        cards: [cardOf('9')],
      },
    };
    expect(boardHeaders(withUnmapped).map((h) => h.id)).toEqual([
      'col-todo',
      'col-doing',
      'col-done',
      UNMAPPED_COLUMN_ID,
    ]);
    const hidden = { ...withUnmapped, board: { ...withUnmapped.board, showUnmappedLane: false } };
    expect(boardHeaders(hidden)).toHaveLength(3);
  });
});

describe('boardRows', () => {
  it('returns one row without header and a cell per column when there are no swimlanes', () => {
    const rows = boardRows(boardViewOf());
    expect(rows).toHaveLength(1);
    expect(rows[0].hasHeader).toBe(false);
    expect(rows[0].cells.map((cell) => cell.cards.length)).toEqual([2, 1, 0]);
    expect(rows[0].count).toBe(3);
  });

  it('groups the cards by swimlane, one row per lane', () => {
    const rows = boardRows(withLanes());
    expect(rows.map((row) => row.name)).toEqual(['Ada', 'Sin asignar']);
    expect(rows[0].hasHeader).toBe(true);
    expect(rows[0].cells[0].cards.map((card) => card.id)).toEqual(['1']);
    expect(rows[1].cells[0].cards.map((card) => card.id)).toEqual(['2']);
    expect(rows[0].cells[1].laneId).toBe('u1');
  });

  it('collects cards of unknown lanes in an extra row', () => {
    const view = withLanes();
    const stray: BoardView = {
      ...view,
      columns: [
        {
          ...view.columns[0],
          cards: [...view.columns[0].cards, cardOf('7', { swimlaneId: 'ghost' })],
        },
        ...view.columns.slice(1),
      ],
    };
    const rows = boardRows(stray);
    expect(rows.at(-1)?.name).toBe('Otras');
    expect(rows.at(-1)?.cells[0].cards.map((card) => card.id)).toEqual(['7']);
  });
});

describe('texts', () => {
  it('writes the estimate and the announcement in Spanish', () => {
    expect(estimateText(8, 'story_points')).toBe('8 pts');
    expect(estimateText(2.5, 'time')).toBe('2,5 h');
    expect(estimateText(3, 'issue_count')).toBe('');
    expect(moveAnnouncement('CORE-1', 'En curso')).toBe('CORE-1 movida a En curso');
  });
});
