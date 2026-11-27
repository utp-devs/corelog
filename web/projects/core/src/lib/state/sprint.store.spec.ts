import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { SessionService } from '../services/session.service';
import { SprintStore } from './sprint.store';

const API = '/api/v1';
const SPRINTS_URL = `${API}/sprints?project_id=project-1`;
const VELOCITY_URL = `${API}/sprints/velocity?project_id=project-1&limit=5`;
const BACKLOG_URL = `${API}/backlog?project_id=project-1`;

function sprint(id: string, status: string, name = id) {
  return {
    id,
    project_id: 'project-1',
    name,
    goal: '',
    status,
    start_date: null,
    end_date: null,
    closed_at: null,
  };
}

function issue(id: string, storyPoints: number | null = null) {
  return {
    id,
    title: `Incidencia ${id}`,
    status: 'open',
    priority: 'low',
    story_points: storyPoints,
    closed_at: null,
  };
}

function backlogItem(id: string) {
  return {
    id,
    project_id: 'project-1',
    summary: `Pendiente ${id}`,
    type: 'task',
    status: 'open',
    priority: 'low',
    rank: id,
  };
}

function setup(): { store: SprintStore; backend: HttpTestingController; session: SessionService } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(SprintStore),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
  };
}

/** Loads the project: sprints (planned, active, closed), velocity and the issues of the selected sprint. */
function loadProject(
  store: SprintStore,
  backend: HttpTestingController,
  sprintIssues = [issue('issue-1', 3)],
): void {
  store.load('project-1');
  backend.expectOne(SPRINTS_URL).flush({
    data: [
      sprint('sprint-1', 'planned'),
      sprint('sprint-2', 'active'),
      sprint('sprint-0', 'closed'),
    ],
  });
  backend.expectOne(VELOCITY_URL).flush({
    data: [{ sprint_id: 'sprint-0', name: 'Sprint 0', points: 13 }],
  });
  flushDetail(backend, sprintIssues);
}

function flushDetail(
  backend: HttpTestingController,
  sprintIssues = [issue('issue-1', 3)],
  sprintId = 'sprint-2',
): void {
  backend
    .expectOne(`${API}/sprints/${sprintId}/issues?project_id=project-1`)
    .flush({ data: sprintIssues });
  backend
    .expectOne(`${API}/sprints/${sprintId}/burndown?project_id=project-1`)
    .flush({ data: [{ date: '2026-11-02', ideal: 3, remaining: 3 }] });
  backend.expectOne(BACKLOG_URL).flush({ data: [backlogItem('issue-1'), backlogItem('issue-9')] });
}

describe('SprintStore', () => {
  beforeEach(() => localStorage.clear());

  it('selects the active sprint first and loads its details', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    expect(store.selectedId()).toBe('sprint-2');
    expect(store.selected()?.status).toBe('active');
    expect(store.issues().map((item) => item.id)).toEqual(['issue-1']);
    expect(store.burndown()).toHaveLength(1);
    expect(store.velocity()[0].points).toBe(13);
    expect(store.plannedTargets().map((item) => item.id)).toEqual(['sprint-1']);
    backend.verify();
  });

  it('hides backlog items that are already in the sprint and totals the points', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    expect(store.assignableBacklog().map((item) => item.id)).toEqual(['issue-9']);
    expect(store.issueTotals()).toEqual({ count: 1, points: 3 });
    backend.verify();
  });

  it('falls back to the planned sprint, then to the first one', () => {
    const { store, backend } = setup();
    store.load('project-1');
    backend
      .expectOne(SPRINTS_URL)
      .flush({ data: [sprint('sprint-1', 'planned'), sprint('sprint-0', 'closed')] });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    flushDetail(backend, [], 'sprint-1');

    expect(store.selectedId()).toBe('sprint-1');
    backend.verify();
  });

  it('keeps the burndown empty when it fails but still shows the issues', () => {
    const { store, backend } = setup();
    store.load('project-1');
    backend.expectOne(SPRINTS_URL).flush({ data: [sprint('sprint-2', 'active')] });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    backend
      .expectOne(`${API}/sprints/sprint-2/issues?project_id=project-1`)
      .flush({ data: [issue('issue-1')] });
    backend
      .expectOne(`${API}/sprints/sprint-2/burndown?project_id=project-1`)
      .flush({}, { status: 422, statusText: 'Unprocessable' });
    backend.expectOne(BACKLOG_URL).flush({ data: [] });

    expect(store.burndown()).toEqual([]);
    expect(store.issues()).toHaveLength(1);
    expect(store.actionError()).toBeNull();
    backend.verify();
  });

  it('reports a failure loading the sprint issues in Spanish', () => {
    const { store, backend } = setup();
    store.load('project-1');
    backend.expectOne(SPRINTS_URL).flush({ data: [sprint('sprint-2', 'active')] });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    backend
      .expectOne(`${API}/sprints/sprint-2/issues?project_id=project-1`)
      .flush({}, { status: 500, statusText: 'Server Error' });

    // The failed request cancels the burndown and backlog requests that were started with it.
    expect(store.actionError()).toBe('No se pudieron cargar las incidencias del sprint.');
    backend.verify({ ignoreCancelled: true });
  });

  it('does not show the issues of the previous sprint while the new one loads', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.selectSprint('sprint-1');

    expect(store.selectedId()).toBe('sprint-1');
    expect(store.issues()).toEqual([]);
    flushDetail(backend, [issue('issue-7')], 'sprint-1');
    expect(store.issues().map((item) => item.id)).toEqual(['issue-7']);
    backend.verify();
  });

  it('reports a failed overview load', () => {
    const { store, backend } = setup();
    store.load('project-1');
    backend.expectOne(SPRINTS_URL).flush({}, { status: 403, statusText: 'Forbidden' });

    expect(store.loadErrorMessage()).toBe(
      'No se pudieron cargar los sprints. Verifica que tengas acceso al proyecto e intenta de nuevo.',
    );
    expect(store.selectedId()).toBeUndefined();
    backend.verify({ ignoreCancelled: true });
  });

  it('requires a name before planning a sprint', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.setDraft({ name: '   ' });
    store.create();

    expect(store.actionError()).toBe('El nombre del sprint es obligatorio.');
    backend.verify();
  });

  it('plans a sprint with empty dates as null, clears the draft and selects it', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.setDraft({ name: 'Sprint 3', goal: 'Exportar' });
    store.create();
    const create = backend.expectOne(`${API}/sprints?project_id=project-1`);
    expect(create.request.body).toEqual({
      name: 'Sprint 3',
      goal: 'Exportar',
      start_date: null,
      end_date: null,
    });
    create.flush({ data: sprint('sprint-3', 'planned', 'Sprint 3') });
    backend.expectOne(SPRINTS_URL).flush({
      data: [
        sprint('sprint-1', 'planned'),
        sprint('sprint-2', 'active'),
        sprint('sprint-3', 'planned'),
      ],
    });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    flushDetail(backend, [], 'sprint-3');

    expect(store.draft().name).toBe('');
    expect(store.selectedId()).toBe('sprint-3');
    backend.verify();
  });

  it('keeps the draft and explains a validation failure when planning fails', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.setDraft({ name: 'Sprint 3', startDate: '2026-12-10', endDate: '2026-12-01' });
    store.create();
    backend
      .expectOne(`${API}/sprints?project_id=project-1`)
      .flush({}, { status: 422, statusText: 'Unprocessable' });

    expect(store.draft().name).toBe('Sprint 3');
    expect(store.actionError()).toBe(
      'Revisa los datos del sprint: fechas en formato AAAA-MM-DD y fin posterior al inicio.',
    );
    backend.verify();
  });

  it('explains starting a sprint while another one is active', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.start('sprint-1');
    backend
      .expectOne(`${API}/sprints/sprint-1/start?project_id=project-1`)
      .flush({}, { status: 409, statusText: 'Conflict' });

    expect(store.actionError()).toBe(
      'Esa acción no está permitida en el estado actual del sprint o ya hay otro sprint activo.',
    );
    backend.verify();
  });

  it('starts a sprint and reloads the overview on success', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.start('sprint-1');
    backend
      .expectOne(`${API}/sprints/sprint-1/start?project_id=project-1`)
      .flush({ data: sprint('sprint-1', 'active') });
    backend
      .expectOne(SPRINTS_URL)
      .flush({ data: [sprint('sprint-1', 'active'), sprint('sprint-2', 'closed')] });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    flushDetail(backend, [], 'sprint-1');

    expect(store.selectedId()).toBe('sprint-1');
    backend.verify();
  });

  it('cannot close into another sprint until a planned target is chosen', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.setCloseMode('sprint');
    expect(store.canClose()).toBe(false);
    store.close('sprint-2');
    backend.verify();

    store.setCloseTarget('sprint-1');
    expect(store.canClose()).toBe(true);
    store.close('sprint-2');
    const close = backend.expectOne(`${API}/sprints/sprint-2/close?project_id=project-1`);
    expect(close.request.body).toEqual({ unfinished: 'sprint', target_sprint_id: 'sprint-1' });
    close.flush({ data: sprint('sprint-2', 'closed') });
    backend.expectOne(SPRINTS_URL).flush({ data: [] });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    backend.verify();
  });

  it('closes back to the backlog without a target', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.close('sprint-2');
    const close = backend.expectOne(`${API}/sprints/sprint-2/close?project_id=project-1`);
    expect(close.request.body).toEqual({ unfinished: 'backlog', target_sprint_id: null });
    close.flush({ data: sprint('sprint-2', 'closed') });
    backend.expectOne(SPRINTS_URL).flush({ data: [] });
    backend.expectOne(VELOCITY_URL).flush({ data: [] });
    backend.verify();
  });

  it('adds the chosen backlog item and clears the choice even when the request fails', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.setIssueToAdd('issue-9');
    store.addChosenIssue('sprint-2');
    backend
      .expectOne(`${API}/sprints/sprint-2/issues/issue-9?project_id=project-1`)
      .flush({}, { status: 409, statusText: 'Conflict' });

    expect(store.issueToAdd()).toBeUndefined();
    expect(store.actionError()).toBe(
      'Esa acción no está permitida en el estado actual del sprint o ya hay otro sprint activo.',
    );
    backend.verify();
  });

  it('removes an issue and explains a forbidden removal', () => {
    const { store, backend } = setup();
    loadProject(store, backend);

    store.removeIssue('sprint-2', 'issue-1');
    backend
      .expectOne(`${API}/sprints/sprint-2/issues/issue-1?project_id=project-1`)
      .flush({}, { status: 403, statusText: 'Forbidden' });

    expect(store.actionError()).toBe(
      'Solo el lead del proyecto o el administrador pueden gestionar sprints.',
    );
    backend.verify();
  });

  it('clears the whole state when the session ends', () => {
    const { store, backend, session } = setup();
    loadProject(store, backend);

    session.startSession('token');
    session.clear();
    TestBed.flushEffects();

    expect(store.overview.hasValue()).toBe(false);
    expect(store.selectedId()).toBeUndefined();
  });
});
