import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { SessionService } from '../services/session.service';
import { BacklogStore } from './backlog.store';

const LIST_URL = '/api/v1/backlog?project_id=project-1';

function item(id: string, summary: string) {
  return {
    id,
    project_id: 'project-1',
    summary,
    type: 'task',
    status: 'open',
    priority: 'medium',
    rank: id,
  };
}

const BACKLOG = [
  item('issue-1', 'Primera'),
  item('issue-2', 'Segunda'),
  item('issue-3', 'Tercera'),
];

function setup(): { store: BacklogStore; backend: HttpTestingController; session: SessionService } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(BacklogStore),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
  };
}

function sprint(id: string, status: string) {
  return { id, project_id: 'project-1', name: id, goal: '', status };
}

/** Loads the backlog and flushes the sprint reads: the sprints given and one issue list per open sprint. */
function loadBacklog(
  store: BacklogStore,
  backend: HttpTestingController,
  items = BACKLOG,
  sprints: { id: string; status: string; issues: string[] }[] = [],
): void {
  store.load('project-1');
  backend.expectOne(LIST_URL).flush({ data: items });
  backend
    .expectOne('/api/v1/sprints?project_id=project-1')
    .flush({ data: sprints.map((entry) => sprint(entry.id, entry.status)) });
  for (const entry of sprints.filter((candidate) => candidate.status !== 'closed')) {
    backend.expectOne(`/api/v1/sprints/${entry.id}/issues?project_id=project-1`).flush({
      data: entry.issues.map((id) => ({ id, title: id, status: 'open', priority: 'medium' })),
    });
  }
}

describe('BacklogStore', () => {
  beforeEach(() => localStorage.clear());

  it('keeps the rank order and reports an empty backlog', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend);
    expect(store.ordered().map((entry) => entry.id)).toEqual(['issue-1', 'issue-2', 'issue-3']);
    expect(store.isEmpty()).toBe(false);

    store.load('project-2');
    backend.expectOne('/api/v1/backlog?project_id=project-2').flush({ data: [] });
    backend.expectOne('/api/v1/sprints?project_id=project-2').flush({ data: [] });
    expect(store.isEmpty()).toBe(true);
    backend.verify();
  });

  it('disables move up on the first row and move down on the last row', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend);

    expect(store.canMoveUp(0)).toBe(false);
    expect(store.canMoveUp(1)).toBe(true);
    expect(store.canMoveDown(1)).toBe(true);
    expect(store.canMoveDown(2)).toBe(false);
    expect(store.canMoveDown(-1)).toBe(false);
    backend.verify();
  });

  it('places the item at the target position and replaces the list with the response', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend);

    store.move(2, 1);
    const request = backend.expectOne('/api/v1/backlog/issue-3?project_id=project-1');
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({ position: 1 });
    request.flush({
      data: [item('issue-1', 'Primera'), item('issue-3', 'Tercera'), item('issue-2', 'Segunda')],
    });

    expect(store.ordered().map((entry) => entry.id)).toEqual(['issue-1', 'issue-3', 'issue-2']);
    expect(store.moveError()).toBeNull();
    backend.verify();
  });

  it('moves among the unplanned items using their position in the whole backlog', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend, BACKLOG, [
      { id: 'sprint-1', status: 'active', issues: ['issue-2'] },
    ]);

    expect(store.canMoveDown(0)).toBe(true);
    expect(store.canMoveDown(1)).toBe(false);
    store.move(1, 0);
    const request = backend.expectOne('/api/v1/backlog/issue-3?project_id=project-1');
    expect(request.request.body).toEqual({ position: 0 });
    request.flush({
      data: [item('issue-3', 'Tercera'), item('issue-1', 'Primera'), item('issue-2', 'Segunda')],
    });
    expect(store.unplanned().map((entry) => entry.id)).toEqual(['issue-3', 'issue-1']);
    backend.verify();
  });

  it('ignores moves outside the list without a request', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend);

    store.move(0, -1);
    store.move(2, 3);
    store.move(7, 0);

    backend.verify();
  });

  it('explains a forbidden reorder and an invalid position', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend);

    store.move(1, 0);
    backend
      .expectOne('/api/v1/backlog/issue-2?project_id=project-1')
      .flush({}, { status: 403, statusText: 'Forbidden' });
    expect(store.moveError()).toBe('No tienes permiso para ordenar el backlog de este proyecto.');

    store.move(1, 0);
    backend
      .expectOne('/api/v1/backlog/issue-2?project_id=project-1')
      .flush({}, { status: 422, statusText: 'Unprocessable' });
    expect(store.moveError()).toBe('La posición de la incidencia no es válida.');

    store.move(1, 0);
    backend
      .expectOne('/api/v1/backlog/issue-2?project_id=project-1')
      .flush({}, { status: 500, statusText: 'Server Error' });
    expect(store.moveError()).toBe('No se pudo reordenar el backlog. Intenta de nuevo.');
    expect(store.ordered()).toHaveLength(3);
    backend.verify();
  });

  it('loads the open sprints, active first, and leaves out of the backlog what they already hold', () => {
    const { store, backend } = setup();
    loadBacklog(store, backend, BACKLOG, [
      { id: 'sprint-planned', status: 'planned', issues: ['issue-3'] },
      { id: 'sprint-old', status: 'closed', issues: ['issue-1'] },
      { id: 'sprint-active', status: 'active', issues: ['issue-2'] },
    ]);

    expect(store.sprints.value()?.map((entry) => entry.sprint.id)).toEqual([
      'sprint-active',
      'sprint-planned',
    ]);
    expect(store.unplanned().map((entry) => entry.id)).toEqual(['issue-1']);
    expect(store.ordered()).toHaveLength(3);
    backend.verify();
  });

  it('keeps the backlog usable when the sprints cannot be read', () => {
    const { store, backend } = setup();
    store.load('project-1');
    backend.expectOne(LIST_URL).flush({ data: BACKLOG });
    backend
      .expectOne('/api/v1/sprints?project_id=project-1')
      .flush({}, { status: 500, statusText: 'Server Error' });

    expect(store.sprints.value()).toEqual([]);
    expect(store.unplanned()).toHaveLength(3);
    backend.verify();
  });

  it('clears the list when the session ends', () => {
    const { store, backend, session } = setup();
    loadBacklog(store, backend);

    session.startSession('token');
    session.clear();
    TestBed.flushEffects();

    expect(store.items.hasValue()).toBe(false);
    expect(store.ordered()).toEqual([]);
  });
});
