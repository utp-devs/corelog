import type { BoardView, BoardViewCard, BoardViewColumn } from '../models/board-v2.model';

export function cardOf(id: string, extra: Partial<BoardViewCard> = {}): BoardViewCard {
  return {
    id,
    key: `K-${id}`,
    summary: `Resumen ${id}`,
    type: { name: 'Tarea', key: 'task', icon: 'task', color: 'blue' },
    priority: { name: 'Media', key: 'medium', icon: 'minus', color: 'amber' },
    status: { id: 's-todo', name: 'Por hacer', category: 'todo' },
    assignee: null,
    labels: [],
    storyPoints: 2,
    remainingEstimateSeconds: 7200,
    flagged: false,
    rank: id,
    sprintId: null,
    epic: null,
    parent: null,
    subtasks: { total: 0, done: 0 },
    swimlaneId: '',
    ...extra,
  };
}

export function columnOf(
  id: string,
  statusId: string,
  cards: BoardViewCard[],
  extra: Partial<BoardViewColumn> = {},
): BoardViewColumn {
  return {
    id,
    name: id,
    color: '',
    position: 0,
    minIssues: null,
    maxIssues: null,
    statusIds: [statusId],
    count: cards.length,
    total: cards.length,
    estimate: cards.reduce((sum, card) => sum + (card.storyPoints ?? 0), 0),
    wipState: 'ok',
    cards,
    ...extra,
  };
}

export function viewOf(): BoardView {
  return {
    board: {
      id: 'b1',
      projectId: 'p1',
      name: 'Tablero',
      type: 'kanban',
      isDefault: true,
      swimlaneMode: 'none',
      estimation: 'story_points',
      cardFields: ['key'],
      showUnmappedLane: true,
      doneRetentionDays: null,
      columns: [],
      quickFilters: [],
      swimlanes: [],
      workflowStatuses: [
        { id: 's-todo', name: 'Por hacer', category: 'todo' },
        { id: 's-doing', name: 'En curso', category: 'in_progress' },
        { id: 's-done', name: 'Hecho', category: 'done' },
      ],
      unmappedStatusIds: [],
      canManageBoard: true,
      canMoveCards: true,
    },
    sprint: null,
    columns: [
      columnOf('c1', 's-todo', [cardOf('a'), cardOf('b'), cardOf('c')]),
      columnOf('c2', 's-doing', [cardOf('d')], { maxIssues: 2, wipState: 'ok' }),
      columnOf('c3', 's-done', []),
    ],
    unmappedColumn: null,
    unmappedCount: 0,
    swimlanes: [],
  };
}
