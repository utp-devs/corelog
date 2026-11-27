import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { ApiError } from '../models';
import { addStatus, removeStatus, updateTransition } from '../utils/workflow-draft';
import { WorkflowStore } from './workflow.store';

const URL = '/api/v1/projects/p1/workflow';
const VIEW = {
  statuses: [
    { id: 's1', name: 'Por hacer', category: 'todo', position: 0, issue_count: 2 },
    { id: 's2', name: 'Hecho', category: 'done', position: 1, issue_count: 0 },
  ],
  initial_status_id: 's1',
  transitions: [
    {
      id: 'finish',
      name: 'Terminar',
      from: ['s1'],
      to: 's2',
      conditions: [],
      validators: [],
      post_functions: [],
      required_fields: [],
    },
  ],
  version: 2,
  updated_at: '2026-11-27T00:00:00Z',
  is_template_default: false,
};

describe('WorkflowStore', () => {
  let store: WorkflowStore;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    store = TestBed.inject(WorkflowStore);
    http = TestBed.inject(HttpTestingController);
    store.load('p1');
    http.expectOne(URL).flush({ data: VIEW });
  });

  afterEach(() => http.verify());

  it('starts clean and becomes dirty with an edit, clean again when discarded', () => {
    expect(store.dirty()).toBe(false);
    store.edit((draft) => updateTransition(draft, 'finish', { name: 'Completar' }));
    expect(store.dirty()).toBe(true);
    store.discard();
    expect(store.dirty()).toBe(false);
  });

  it('is not dirty when an edit is undone by hand', () => {
    store.edit((draft) => updateTransition(draft, 'finish', { name: 'Completar' }));
    store.edit((draft) => updateTransition(draft, 'finish', { name: 'Terminar' }));
    expect(store.dirty()).toBe(false);
  });

  it('lists the removed statuses that still hold issues', () => {
    store.edit((draft) => removeStatus(draft, 's1'));
    expect(store.removedWithIssues().map((status) => status.id)).toEqual(['s1']);
  });

  it('validates the draft and drops the result when the draft changes', () => {
    store.edit((draft) => addStatus(draft, { name: 'Bloqueado', category: 'in_progress' }));
    store.validate().subscribe();
    const request = http.expectOne(`${URL}/validate`);
    expect(request.request.body.statuses[2]).toEqual({
      ref: 'new-1',
      id: undefined,
      name: 'Bloqueado',
      category: 'in_progress',
    });
    request.flush({ data: { valid: true, errors: [], warnings: [], mapping_required: [] } });
    expect(store.validation()?.valid).toBe(true);
    store.edit((draft) => updateTransition(draft, 'finish', { name: 'X' }));
    expect(store.validation()).toBeNull();
  });

  it('saves with the loaded version and adopts the saved workflow as the new baseline', () => {
    store.edit((draft) => updateTransition(draft, 'finish', { name: 'Completar' }));
    store.save().subscribe();
    const request = http.expectOne(URL);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body.expected_version).toBe(2);
    request.flush({
      data: {
        ...VIEW,
        version: 3,
        transitions: [{ ...VIEW.transitions[0], name: 'Completar' }],
        warnings: [{ path: 'p', code: 'c', message: 'Aviso' }],
      },
    });
    expect(store.dirty()).toBe(false);
    expect(store.version()).toBe(3);
    expect(store.warnings()).toHaveLength(1);
    expect(store.saving()).toBe(false);
  });

  it('keeps the draft and reports the conflict when the version is stale', () => {
    store.edit((draft) => updateTransition(draft, 'finish', { name: 'Completar' }));
    let error: ApiError | undefined;
    store.save().subscribe({ error: (raw: ApiError) => (error = raw) });
    http
      .expectOne(URL)
      .flush(
        { error: { code: 'workflow.version_conflict', message: 'cambió' } },
        { status: 409, statusText: 'Conflict' },
      );
    expect(error?.code).toBe('workflow.version_conflict');
    expect(store.dirty()).toBe(true);
    expect(store.saving()).toBe(false);
  });

  it('follows a rename of a saved status without becoming dirty', () => {
    store.applyRename('s1', 'Pendiente');
    expect(store.draft()?.statuses[0].name).toBe('Pendiente');
    expect(store.dirty()).toBe(false);
  });

  it('applies a template and replaces the draft', () => {
    store.applyTemplate('kanban').subscribe();
    const request = http.expectOne(`${URL}/reset`);
    expect(request.request.body).toEqual({
      template: 'kanban',
      expected_version: 2,
      status_mapping: {},
    });
    request.flush({ data: { ...VIEW, version: 3 } });
    expect(store.version()).toBe(3);
  });
});
