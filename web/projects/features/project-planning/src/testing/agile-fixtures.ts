import type {
  AgileSprint,
  BacklogIssue,
  BacklogSection,
  BacklogView,
  BoardSummary,
  BoardView,
  BoardViewCard,
  BoardViewColumn,
} from '@corelog/core';

/** A card of a board view with sensible defaults. */
export function cardOf(id: string, extra: Partial<BoardViewCard> = {}): BoardViewCard {
  return {
    id,
    key: `CORE-${id}`,
    summary: `Incidencia ${id}`,
    type: { name: 'Tarea', key: 'task', icon: 'task', color: 'blue' },
    priority: { name: 'Media', key: 'medium', icon: 'minus', color: 'amber' },
    status: { id: 's-todo', name: 'Por hacer', category: 'todo' },
    assignee: null,
    labels: [],
    storyPoints: 2,
    remainingEstimateSeconds: null,
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

function columnOf(
  id: string,
  name: string,
  statusId: string,
  cards: BoardViewCard[],
  extra: Partial<BoardViewColumn> = {},
): BoardViewColumn {
  return {
    id,
    name,
    color: '',
    position: 0,
    minIssues: null,
    maxIssues: null,
    statusIds: [statusId],
    count: cards.length,
    total: cards.length,
    estimate: 0,
    wipState: 'ok',
    cards,
    ...extra,
  };
}

export const BOARD_SUMMARY: BoardSummary = {
  id: 'board-1',
  projectId: 'project-1',
  name: 'Tablero principal',
  type: 'kanban',
  isDefault: true,
  columnCount: 3,
  swimlaneMode: 'none',
};

/** A kanban board with three columns: two cards to do, one in progress (limit 1), none done. */
export function boardViewOf(
  overrides: { canMove?: boolean; canManage?: boolean; type?: 'scrum' | 'kanban' } = {},
): BoardView {
  return {
    board: {
      id: 'board-1',
      projectId: 'project-1',
      name: 'Tablero principal',
      type: overrides.type ?? 'kanban',
      isDefault: true,
      swimlaneMode: 'none',
      estimation: 'story_points',
      cardFields: ['key', 'type', 'priority', 'assignee', 'labels', 'points'],
      showUnmappedLane: true,
      doneRetentionDays: null,
      columns: [],
      quickFilters: [{ id: 'qf-1', name: 'Mis incidencias', query: 'assignee = currentUser()' }],
      swimlanes: [],
      workflowStatuses: [
        { id: 's-todo', name: 'Por hacer', category: 'todo' },
        { id: 's-doing', name: 'En curso', category: 'in_progress' },
        { id: 's-done', name: 'Hecho', category: 'done' },
      ],
      unmappedStatusIds: [],
      canManageBoard: overrides.canManage ?? true,
      canMoveCards: overrides.canMove ?? true,
    },
    sprint: null,
    columns: [
      columnOf('col-todo', 'Por hacer', 's-todo', [cardOf('1'), cardOf('2')]),
      columnOf(
        'col-doing',
        'En curso',
        's-doing',
        [cardOf('3', { status: { id: 's-doing', name: 'En curso', category: 'in_progress' } })],
        { maxIssues: 1, wipState: 'at_max' },
      ),
      columnOf('col-done', 'Hecho', 's-done', []),
    ],
    unmappedColumn: null,
    unmappedCount: 0,
    swimlanes: [],
  };
}

export function sprintOf(id: string, extra: Partial<AgileSprint> = {}): AgileSprint {
  return {
    id,
    projectId: 'project-1',
    boardId: null,
    name: `Sprint ${id}`,
    goal: '',
    status: 'planned',
    startDate: null,
    endDate: null,
    startedAt: null,
    completedAt: null,
    committedPoints: null,
    completedPoints: null,
    ...extra,
  };
}

export function backlogIssueOf(id: string, extra: Partial<BacklogIssue> = {}): BacklogIssue {
  return {
    id,
    key: `CORE-${id}`,
    summary: `Historia ${id}`,
    issueType: 'story',
    issueTypeName: 'Historia',
    issueTypeIcon: 'story',
    issueTypeColor: 'green',
    status: 'open',
    statusName: 'Por hacer',
    statusCategory: 'todo',
    priority: 'medium',
    priorityName: 'Media',
    priorityIcon: 'minus',
    priorityColor: 'amber',
    assigneeId: null,
    assigneeName: '',
    storyPoints: 3,
    originalEstimateSeconds: null,
    rank: id,
    epicId: null,
    epicKey: '',
    epicSummary: '',
    flagged: false,
    labels: [],
    ...extra,
  };
}

export function sectionOf(
  kind: BacklogSection['kind'],
  sprint: AgileSprint | null,
  issues: BacklogIssue[],
): BacklogSection {
  const sum = (list: BacklogIssue[]): number =>
    list.reduce((total, item) => total + (item.storyPoints ?? 0), 0);
  const by = (category: BacklogIssue['statusCategory']) => {
    const list = issues.filter((item) => item.statusCategory === category);
    return { issues: list.length, points: sum(list), originalEstimateSeconds: 0 };
  };
  return {
    kind,
    sprint,
    issues,
    totals: {
      issues: issues.length,
      points: sum(issues),
      originalEstimateSeconds: 0,
      byCategory: { todo: by('todo'), in_progress: by('in_progress'), done: by('done') },
    },
  };
}

export function backlogViewOf(
  overrides: { canMove?: boolean; canManage?: boolean } = {},
): BacklogView {
  return {
    projectId: 'project-1',
    sections: [
      sectionOf(
        'active',
        sprintOf('1', { status: 'active', name: 'Sprint 1', goal: 'Cerrar pagos' }),
        [backlogIssueOf('1'), backlogIssueOf('2', { statusCategory: 'done', statusName: 'Hecho' })],
      ),
      sectionOf('planned', sprintOf('2', { name: 'Sprint 2' }), []),
      sectionOf('backlog', null, [backlogIssueOf('3'), backlogIssueOf('4'), backlogIssueOf('5')]),
    ],
    epics: [
      {
        id: 'epic-1',
        key: 'CORE-9',
        summary: 'Pagos',
        status: 'in_progress',
        total: 4,
        done: 1,
        points: 13,
        donePoints: 3,
        progress: 0.25,
      },
    ],
    canManageSprints: overrides.canManage ?? true,
    canMoveCards: overrides.canMove ?? true,
    truncated: false,
  };
}
