import { describe, expect, it } from 'vitest';

import type { BacklogIssue, BacklogSection, BacklogView } from '../models/backlog-v2.model';
import {
  anchorAt,
  moveIssuesInBacklog,
  rangeBetween,
  setPointsInBacklog,
  totalsOf,
} from './backlog-view-ops';

function issue(id: string, extra: Partial<BacklogIssue> = {}): BacklogIssue {
  return {
    id,
    key: `K-${id}`,
    summary: id,
    issueType: 'task',
    issueTypeName: 'Tarea',
    issueTypeIcon: 'task',
    issueTypeColor: 'blue',
    status: 'open',
    statusName: 'Por hacer',
    statusCategory: 'todo',
    priority: 'medium',
    priorityName: 'Media',
    priorityIcon: 'minus',
    priorityColor: 'amber',
    assigneeId: null,
    assigneeName: '',
    storyPoints: 1,
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

function section(
  kind: BacklogSection['kind'],
  sprintId: string | null,
  issues: BacklogIssue[],
): BacklogSection {
  return {
    kind,
    sprint: sprintId
      ? {
          id: sprintId,
          projectId: 'p',
          boardId: null,
          name: sprintId,
          goal: '',
          status: kind === 'active' ? 'active' : 'planned',
          startDate: null,
          endDate: null,
          startedAt: null,
          completedAt: null,
          committedPoints: null,
          completedPoints: null,
        }
      : null,
    issues,
    totals: totalsOf(issues),
  };
}

function viewOf(): BacklogView {
  return {
    projectId: 'p',
    sections: [
      section('active', 's1', [issue('a'), issue('b', { statusCategory: 'done', storyPoints: 3 })]),
      section('planned', 's2', [issue('c')]),
      section('backlog', null, [issue('d'), issue('e'), issue('f')]),
    ],
    epics: [],
    canManageSprints: true,
    canMoveCards: true,
    truncated: false,
  };
}

const idsOf = (view: BacklogView, index: number): string[] =>
  view.sections[index].issues.map((item) => item.id);

describe('totalsOf', () => {
  it('adds points and issues overall and by category', () => {
    const totals = totalsOf([issue('a'), issue('b', { statusCategory: 'done', storyPoints: 3 })]);
    expect(totals.issues).toBe(2);
    expect(totals.points).toBe(4);
    expect(totals.byCategory.done).toEqual({ issues: 1, points: 3, originalEstimateSeconds: 0 });
    expect(totals.byCategory.in_progress?.issues).toBe(0);
  });
});

describe('moveIssuesInBacklog', () => {
  it('moves one issue to the end of another section and recomputes totals', () => {
    const view = moveIssuesInBacklog(viewOf(), ['d'], 's2');
    expect(idsOf(view, 1)).toEqual(['c', 'd']);
    expect(idsOf(view, 2)).toEqual(['e', 'f']);
    expect(view.sections[1].totals.issues).toBe(2);
    expect(view.sections[2].totals.points).toBe(2);
  });

  it('places the issues before or after an anchor, keeping their order', () => {
    const before = moveIssuesInBacklog(viewOf(), ['f', 'd'], null, { beforeIssueId: 'e' });
    expect(idsOf(before, 2)).toEqual(['f', 'd', 'e']);
    const after = moveIssuesInBacklog(viewOf(), ['a', 'b'], null, { afterIssueId: 'd' });
    expect(idsOf(after, 2)).toEqual(['d', 'a', 'b', 'e', 'f']);
    expect(idsOf(after, 0)).toEqual([]);
  });

  it('reorders inside the same section', () => {
    const view = moveIssuesInBacklog(viewOf(), ['d'], null, { afterIssueId: 'f' });
    expect(idsOf(view, 2)).toEqual(['e', 'f', 'd']);
  });

  it('ignores unknown ids and unknown targets', () => {
    const base = viewOf();
    expect(moveIssuesInBacklog(base, ['zzz'], 's2')).toBe(base);
    expect(moveIssuesInBacklog(base, ['a'], 'nope')).toBe(base);
  });
});

describe('setPointsInBacklog', () => {
  it('changes the points of one issue and the section total', () => {
    const view = setPointsInBacklog(viewOf(), 'd', 8);
    expect(view.sections[2].issues[0].storyPoints).toBe(8);
    expect(view.sections[2].totals.points).toBe(10);
    expect(setPointsInBacklog(view, 'd', null).sections[2].issues[0].storyPoints).toBeNull();
  });
});

describe('rangeBetween', () => {
  it('selects the ids between two rows in list order, either direction', () => {
    const ids = ['a', 'b', 'c', 'd'];
    expect(rangeBetween(ids, 'b', 'd')).toEqual(['b', 'c', 'd']);
    expect(rangeBetween(ids, 'd', 'b')).toEqual(['b', 'c', 'd']);
    expect(rangeBetween(ids, 'zzz', 'c')).toEqual(['c']);
  });
});

describe('anchorAt', () => {
  it('skips moving issues when it looks for the anchor', () => {
    expect(anchorAt(['a', 'b', 'c'], 1, new Set(['b']))).toEqual({ beforeIssueId: 'c' });
    expect(anchorAt(['a', 'b', 'c'], 2, new Set(['c']))).toEqual({ afterIssueId: 'b' });
    expect(anchorAt(['a'], 0, new Set(['a']))).toEqual({});
    expect(anchorAt([], 0, new Set())).toEqual({});
  });
});
