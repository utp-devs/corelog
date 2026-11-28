import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { ISSUE_STALE_CODE, IssueViewStore } from './issue-view.store';

const ISSUE = {
  id: 'i1',
  key: 'COR-1',
  project_id: 'p1',
  project_key: 'COR',
  summary: 'Original',
  description: '',
  status: 'open',
  priority: { id: 'pr', name: 'Media', icon: 'minus', color: 'amber' },
  issue_type: { id: 't', name: 'Tarea', icon: 'task', color: 'blue', hierarchy_level: 0 },
  parent_id: null,
  due_date: null,
  story_points: null,
  rank: 'r1',
  closed_at: null,
  reporter_id: 'u1',
  reporter_name: 'Ana',
  assignee_id: null,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-02T00:00:00Z',
  permissions: ['edit_issue'],
  counts: { comments: 1, attachments: 0, children: 0, links: 0, watchers: 0, votes: 0 },
  breadcrumb: { project: { id: 'p1', key: 'COR', name: 'Core' }, parent: null },
};

/** Lets pending promise callbacks run so the next request is issued. */
async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) await Promise.resolve();
}

describe('IssueViewStore', () => {
  let store: IssueViewStore;
  let http: HttpTestingController;

  async function open(): Promise<void> {
    const loading = store.load('COR-1');
    http.expectOne('/api/v1/issues/COR-1').flush({ data: ISSUE });
    await loading;
    await settle();
    http
      .match((req) => req.url === '/api/v1/issues/search')
      .forEach((req) => req.flush({ data: [{ id: 'i1', key: 'COR-1', labels: ['bug'] }] }));
    await settle();
  }

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideHttpClient(),
        provideHttpClientTesting(),
        IssueViewStore,
      ],
    });
    store = TestBed.inject(IssueViewStore);
    http = TestBed.inject(HttpTestingController);
  });

  it('loads the view and exposes the permissions of the viewer', async () => {
    await open();
    expect(store.view()?.key).toBe('COR-1');
    expect(store.labels()).toEqual(['bug']);
    expect(store.can('edit_issue')).toBe(true);
    expect(store.can('delete_issue')).toBe(false);
  });

  it('keeps the error of a refused issue and no view', async () => {
    const loading = store.load('COR-9');
    http
      .expectOne('/api/v1/issues/COR-9')
      .flush({ error: { message: 'No existe' } }, { status: 404, statusText: 'Not Found' });
    await loading;
    expect(store.view()).toBeNull();
    expect(store.error()?.status).toBe(404);
  });

  it('applies the preview at once and sends expected_updated_at', async () => {
    await open();
    const saving = store.patch({ summary: 'Nuevo' }, { summary: 'Nuevo' });
    expect(store.view()?.summary).toBe('Nuevo');
    const patch = http.expectOne('/api/v1/issues/i1');
    expect(patch.request.body).toEqual({ summary: 'Nuevo', expected_updated_at: ISSUE.updated_at });
    const updated = { ...ISSUE, summary: 'Nuevo', updated_at: '2026-11-03T00:00:00Z' };
    patch.flush({ data: updated });
    await settle();
    http.expectOne('/api/v1/issues/i1').flush({ data: updated });
    const view = await saving;
    expect(view.summary).toBe('Nuevo');
    expect(store.saving()).toBe(false);
  });

  it('rolls the preview back when the server refuses the change', async () => {
    await open();
    const saving = store.patch({ summary: 'Nuevo' }, { summary: 'Nuevo' });
    http
      .expectOne('/api/v1/issues/i1')
      .flush(
        { error: { message: 'Sin permiso', code: 'issue.forbidden' } },
        { status: 403, statusText: 'Forbidden' },
      );
    await expect(saving).rejects.toMatchObject({ status: 403 });
    expect(store.view()?.summary).toBe('Original');
  });

  it('reloads the issue when the update is stale and still reports the conflict', async () => {
    await open();
    const saving = store.patch({ summary: 'Nuevo' }, { summary: 'Nuevo' });
    http
      .expectOne('/api/v1/issues/i1')
      .flush(
        { error: { message: 'Cambió', code: ISSUE_STALE_CODE } },
        { status: 409, statusText: 'Conflict' },
      );
    await settle();
    http.expectOne('/api/v1/issues/i1').flush({
      data: { ...ISSUE, summary: 'De otra persona', updated_at: '2026-11-04T00:00:00Z' },
    });
    await expect(saving).rejects.toMatchObject({ code: ISSUE_STALE_CODE });
    expect(store.view()?.summary).toBe('De otra persona');
  });

  it('toggles the vote locally and adjusts the counter', async () => {
    await open();
    store.setVoted(true);
    expect(store.view()?.viewerHasVoted).toBe(true);
    expect(store.view()?.counts.votes).toBe(1);
    store.setVoted(false);
    expect(store.view()?.counts.votes).toBe(0);
  });
});
