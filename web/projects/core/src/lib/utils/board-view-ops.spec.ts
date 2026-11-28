import { describe, expect, it } from 'vitest';

import type { BoardView } from '../models/board-v2.model';
import { cardOf, viewOf } from '../state/board-v2-fixtures';
import {
  UNMAPPED_COLUMN_ID,
  adjacentColumnId,
  cardEstimate,
  dropNeighbors,
  estimateOf,
  moveCardInView,
  wipStateOf,
} from './board-view-ops';

describe('wipStateOf', () => {
  it('reports ok, at_max, over_max and below_min', () => {
    expect(wipStateOf(1, null, 3)).toBe('ok');
    expect(wipStateOf(3, null, 3)).toBe('at_max');
    expect(wipStateOf(4, null, 3)).toBe('over_max');
    expect(wipStateOf(1, 2, null)).toBe('below_min');
    expect(wipStateOf(5, null, null)).toBe('ok');
  });
});

describe('estimates', () => {
  it('adds points, counts issues or sums remaining hours', () => {
    const cards = [cardOf('a'), cardOf('b', { storyPoints: null })];
    expect(cardEstimate(cards[0], 'story_points')).toBe(2);
    expect(estimateOf(cards, 'story_points')).toBe(2);
    expect(estimateOf(cards, 'issue_count')).toBe(2);
    expect(estimateOf(cards, 'time')).toBe(4);
  });
});

describe('dropNeighbors', () => {
  it('anchors before the next card, else after the previous one, else nothing', () => {
    expect(dropNeighbors(['a', 'b'], 0)).toEqual({ beforeIssueId: 'a' });
    expect(dropNeighbors(['a', 'b'], 1)).toEqual({ beforeIssueId: 'b' });
    expect(dropNeighbors(['a', 'b'], 2)).toEqual({ afterIssueId: 'b' });
    expect(dropNeighbors([], 0)).toEqual({});
  });
});

describe('adjacentColumnId', () => {
  const columns = [{ id: 'x' }, { id: 'y' }, { id: 'z' }];

  it('returns the neighbor or null at the ends', () => {
    expect(adjacentColumnId(columns, 'y', 1)).toBe('z');
    expect(adjacentColumnId(columns, 'y', -1)).toBe('x');
    expect(adjacentColumnId(columns, 'z', 1)).toBeNull();
    expect(adjacentColumnId(columns, 'missing', 1)).toBeNull();
  });
});

describe('moveCardInView', () => {
  it('moves a card to another column and updates counts, estimate, status and WIP', () => {
    const view = moveCardInView(viewOf(), 'a', 'c2');
    const [c1, c2] = view.columns;
    expect(c1.cards.map((c) => c.id)).toEqual(['b', 'c']);
    expect(c1.count).toBe(2);
    expect(c1.estimate).toBe(4);
    expect(c2.cards.map((c) => c.id)).toEqual(['d', 'a']);
    expect(c2.total).toBe(2);
    expect(c2.wipState).toBe('at_max');
    expect(c2.cards[1].status.id).toBe('s-doing');
  });

  it('flags the column over its limit', () => {
    const view = moveCardInView(moveCardInView(viewOf(), 'a', 'c2'), 'b', 'c2');
    expect(view.columns[1].wipState).toBe('over_max');
  });

  it('places the card before or after a neighbor', () => {
    const before = moveCardInView(viewOf(), 'd', 'c1', { beforeIssueId: 'b' });
    expect(before.columns[0].cards.map((c) => c.id)).toEqual(['a', 'd', 'b', 'c']);
    const after = moveCardInView(viewOf(), 'd', 'c1', { afterIssueId: 'b' });
    expect(after.columns[0].cards.map((c) => c.id)).toEqual(['a', 'b', 'd', 'c']);
  });

  it('reorders inside the same column without changing its counts or the card status', () => {
    const view = moveCardInView(viewOf(), 'a', 'c1', { afterIssueId: 'c' });
    expect(view.columns[0].cards.map((c) => c.id)).toEqual(['b', 'c', 'a']);
    expect(view.columns[0].count).toBe(3);
    expect(view.columns[0].total).toBe(3);
    expect(view.columns[0].cards[2].status.id).toBe('s-todo');
  });

  it('takes a card out of the unmapped column', () => {
    const base = viewOf();
    const view: BoardView = {
      ...base,
      unmappedColumn: {
        id: UNMAPPED_COLUMN_ID,
        name: 'Sin asignar',
        statusIds: ['x'],
        count: 1,
        total: 1,
        estimate: 2,
        cards: [cardOf('u')],
      },
      unmappedCount: 1,
    };
    const moved = moveCardInView(view, 'u', 'c3');
    expect(moved.unmappedColumn?.cards).toEqual([]);
    expect(moved.unmappedCount).toBe(0);
    expect(moved.columns[2].cards.map((c) => c.id)).toEqual(['u']);
  });

  it('records the swimlane the card was dropped in', () => {
    const view = moveCardInView(viewOf(), 'a', 'c2', { swimlaneId: 'lane-1' });
    expect(view.columns[1].cards.find((c) => c.id === 'a')?.swimlaneId).toBe('lane-1');
  });

  it('returns the same view for unknown cards or columns', () => {
    const base = viewOf();
    expect(moveCardInView(base, 'zzz', 'c2')).toBe(base);
    expect(moveCardInView(base, 'a', 'zzz')).toBe(base);
  });
});
