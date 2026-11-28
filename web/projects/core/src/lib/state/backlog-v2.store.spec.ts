import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { NEVER, Subject, of, throwError } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';

import { provideCorelogTesting } from '../../testing';
import type { ApiError } from '../models/api-error.model';
import type { BacklogIssue, BacklogSection, BacklogView } from '../models/backlog-v2.model';
import { BacklogV2ApiService } from '../services/api/backlog-v2-api.service';
import { totalsOf } from '../utils/backlog-view-ops';
import { BacklogViewStore, backlogErrorMessage } from './backlog-v2.store';

function issue(id: string): BacklogIssue {
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
    storyPoints: 2,
    originalEstimateSeconds: null,
    rank: id,
    epicId: null,
    epicKey: '',
    epicSummary: '',
    flagged: false,
    labels: [],
  };
}

function section(
  kind: BacklogSection['kind'],
  sprintId: string | null,
  ids: string[],
): BacklogSection {
  const issues = ids.map(issue);
  return {
    kind,
    sprint: sprintId
      ? {
          id: sprintId,
          projectId: 'p1',
          boardId: null,
          name: sprintId,
          goal: '',
          status: 'planned',
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

function viewOf(canMove = true): BacklogView {
  return {
    projectId: 'p1',
    sections: [section('planned', 's1', ['a', 'b']), section('backlog', null, ['c', 'd', 'e'])],
    epics: [],
    canManageSprints: true,
    canMoveCards: canMove,
    truncated: false,
  };
}

function errorOf(status: number, code: string): ApiError {
  return { status, code, message: 'x', details: [] };
}

function setup(view: BacklogView = viewOf()) {
  const api = {
    get: vi.fn(() => of(view)),
    rank: vi.fn(),
    setPoints: vi.fn(),
  };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: BacklogV2ApiService, useValue: api },
    ],
  });
  const store = TestBed.inject(BacklogViewStore);
  store.open('p1');
  return { store, api };
}

const idsOf = (store: BacklogViewStore, index: number): string[] =>
  (store.view.value()?.sections[index].issues ?? []).map((item) => item.id);

describe('BacklogViewStore', () => {
  it('loads the sections and the permissions', () => {
    const { store, api } = setup();
    expect(api.get).toHaveBeenCalledWith('p1', { epic: undefined, q: undefined });
    expect(store.canMove()).toBe(true);
    expect(store.canManageSprints()).toBe(true);
    expect(idsOf(store, 1)).toEqual(['c', 'd', 'e']);
  });

  it('selects one row, toggles with Ctrl and extends a range with Shift', () => {
    const { store } = setup();
    store.select('c', 'single');
    expect(store.selectedIds()).toEqual(['c']);
    expect(store.inspected()?.id).toBe('c');
    store.select('e', 'range');
    expect(store.selectedIds()).toEqual(['c', 'd', 'e']);
    expect(store.inspected()).toBeNull();
    store.select('d', 'toggle');
    expect(store.selectedIds()).toEqual(['c', 'e']);
    store.clearSelection();
    expect(store.selectedIds()).toEqual([]);
  });

  it('moves issues to a sprint at once and refreshes after the server accepts', () => {
    const { store, api } = setup();
    const answer = new Subject<void>();
    api.rank.mockReturnValue(answer);

    store.rank(['c', 'd'], 's1', { beforeIssueId: 'b' });
    expect(idsOf(store, 0)).toEqual(['a', 'c', 'd', 'b']);
    expect(idsOf(store, 1)).toEqual(['e']);
    expect(api.rank).toHaveBeenCalledWith({
      issueIds: ['c', 'd'],
      sprintId: 's1',
      beforeIssueId: 'b',
      afterIssueId: undefined,
    });

    answer.next();
    answer.complete();
    expect(store.announcement()).toBe('2 incidencias movidas');
    expect(api.get).toHaveBeenCalledTimes(2);
  });

  it('undoes the move and explains when the server refuses it', () => {
    const { store, api } = setup();
    api.rank.mockReturnValue(throwError(() => errorOf(409, 'sprint.closed')));
    store.rank(['c'], 's1');
    expect(idsOf(store, 0)).toEqual(['a', 'b']);
    expect(idsOf(store, 1)).toEqual(['c', 'd', 'e']);
    expect(store.changeError()).toContain('cerrado');
  });

  it('does not rank without permission to move cards', () => {
    const { store, api } = setup(viewOf(false));
    store.rank(['c'], 's1');
    expect(api.rank).not.toHaveBeenCalled();
  });

  it('edits points in place and restores them on failure', () => {
    const { store, api } = setup();
    api.get.mockReturnValue(NEVER);
    api.setPoints.mockReturnValueOnce(of(undefined));
    store.setPoints('c', 8);
    expect(store.findIssue('c')?.storyPoints).toBe(8);
    expect(api.setPoints).toHaveBeenCalledWith('c', 8);

    api.setPoints.mockReturnValueOnce(throwError(() => errorOf(500, 'x')));
    store.setPoints('d', 13);
    expect(store.findIssue('d')?.storyPoints).toBe(2);
    expect(store.changeError()).toContain('No se pudo');
  });

  it('filters by epic and by text, and clicking the same epic clears the filter', () => {
    const { store, api } = setup();
    store.setEpicFilter('epic-1');
    expect(api.get).toHaveBeenLastCalledWith('p1', { epic: 'epic-1', q: undefined });
    store.setSearch('login');
    expect(api.get).toHaveBeenLastCalledWith('p1', { epic: 'epic-1', q: 'login' });
    store.setEpicFilter('epic-1');
    expect(store.epicFilter()).toBeNull();
  });
});

describe('backlogErrorMessage', () => {
  it('maps archived and forbidden answers to Spanish', () => {
    expect(backlogErrorMessage(errorOf(409, 'project.archived'))).toContain('archivado');
    expect(backlogErrorMessage(errorOf(403, 'x'))).toContain('permiso');
  });
});
