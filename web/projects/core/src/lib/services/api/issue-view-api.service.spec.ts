import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { IssueView } from '../../models/issue-view.model';
import { IssueViewApiService, toPatchBody } from './issue-view-api.service';

const BASE = {
  id: 'i1',
  key: 'COR-1',
  project_id: 'p1',
  project_key: 'COR',
  summary: 'Titulo',
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
};

describe('toPatchBody', () => {
  it('sends only the keys that are present, with snake_case names', () => {
    expect(toPatchBody({ summary: 'Nuevo', dueDate: null, expectedUpdatedAt: 't' })).toEqual({
      summary: 'Nuevo',
      due_date: null,
      expected_updated_at: 't',
    });
  });

  it('keeps explicit nulls (clear) and drops undefined values', () => {
    const body = toPatchBody({ assigneeId: null, storyPoints: undefined });
    expect(body).toEqual({ assignee_id: null });
    expect('story_points' in body).toBe(false);
  });

  it('maps the list fields', () => {
    expect(toPatchBody({ fixVersions: ['v1'], components: [], labels: ['a'] })).toEqual({
      fix_versions: ['v1'],
      components: [],
      labels: ['a'],
    });
  });
});

describe('IssueViewApiService', () => {
  let service: IssueViewApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(IssueViewApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('maps the view with permissions, counters, breadcrumb and custom fields', () => {
    let view: IssueView | undefined;
    service.getView('COR-1').subscribe((value) => (view = value));
    http.expectOne('/api/v1/issues/COR-1').flush({
      data: {
        ...BASE,
        flagged: true,
        original_estimate_seconds: 3600,
        fix_versions: [{ id: 'v1', name: '1.0' }],
        custom_fields: [
          { field_id: 'f1', name: 'Cliente', type: 'text', options: [], required: true },
        ],
        custom: { f1: 'Acme' },
        permissions: ['edit_issue', 'log_work'],
        counts: { comments: 2, attachments: 0, children: 1, links: 0, watchers: 3, votes: 4 },
        breadcrumb: { project: { id: 'p1', key: 'COR', name: 'Core' }, parent: null },
        viewer_has_voted: true,
      },
    });
    expect(view?.permissions).toEqual(['edit_issue', 'log_work']);
    expect(view?.counts.votes).toBe(4);
    expect(view?.flagged).toBe(true);
    expect(view?.originalEstimateSeconds).toBe(3600);
    expect(view?.fixVersions).toEqual([{ id: 'v1', name: '1.0' }]);
    expect(view?.customFields[0]).toMatchObject({ fieldId: 'f1', required: true });
    expect(view?.custom['f1']).toBe('Acme');
    expect(view?.viewerHasVoted).toBe(true);
    expect(view?.breadcrumb.project.name).toBe('Core');
  });

  it('patches with the expected updated_at and reads the applied changes', () => {
    let result: { changes: readonly { field: string }[]; batchId: string | null } | undefined;
    service.patch('i1', { summary: 'X', expectedUpdatedAt: 't0' }).subscribe((r) => (result = r));
    const request = http.expectOne('/api/v1/issues/i1');
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({ summary: 'X', expected_updated_at: 't0' });
    request.flush({
      data: { ...BASE, batch_id: 'b1', changes: [{ field: 'summary', from: 'A', to: 'X' }] },
    });
    expect(result?.batchId).toBe('b1');
    expect(result?.changes).toEqual([{ field: 'summary', from: 'A', to: 'X' }]);
  });

  it('votes with PUT and removes the vote with DELETE', () => {
    service.vote('i1', true).subscribe();
    const put = http.expectOne('/api/v1/issues/i1/vote');
    expect(put.request.method).toBe('PUT');
    put.flush(null);
    service.vote('i1', false).subscribe();
    const del = http.expectOne('/api/v1/issues/i1/vote');
    expect(del.request.method).toBe('DELETE');
    del.flush(null);
  });

  it('creates a link with target and kind', () => {
    service.createLink('i1', 'COR-2', 'blocks').subscribe();
    const request = http.expectOne('/api/v1/issues/i1/links');
    expect(request.request.body).toEqual({ target: 'COR-2', kind: 'blocks' });
    request.flush({
      data: {
        id: 'l1',
        kind: 'blocks',
        direction: 'outward',
        label: 'bloquea',
        issue: { id: 'i2', key: 'COR-2' },
      },
    });
  });

  it('sends bulk actions with the issue ids and parameters, and reads per-issue failures', () => {
    let failed: readonly { issue: string; code: string }[] = [];
    service
      .bulk(['COR-1', 'COR-2'], 'set_priority', { priority_id: 'pr' })
      .subscribe((r) => (failed = r.failed));
    const request = http.expectOne('/api/v1/issues/bulk');
    expect(request.request.body).toEqual({
      issue_ids: ['COR-1', 'COR-2'],
      action: 'set_priority',
      params: { priority_id: 'pr' },
    });
    request.flush({
      data: {
        succeeded: [{ issue: BASE }],
        failed: [{ issue: 'COR-2', code: 'issue.forbidden', message: 'Sin permiso' }],
      },
    });
    expect(failed).toEqual([{ issue: 'COR-2', code: 'issue.forbidden', message: 'Sin permiso' }]);
  });

  it('reads the label names of an issue through the search endpoint', () => {
    let labels: string[] = [];
    service.labelsOf('COR-1').subscribe((value) => (labels = value));
    const request = http.expectOne((req) => req.url === '/api/v1/issues/search');
    expect(request.request.params.get('jql')).toBe('key = "COR-1"');
    request.flush({ data: [{ id: 'i1', key: 'COR-1', labels: ['bug', 'ui'] }] });
    expect(labels).toEqual(['bug', 'ui']);
  });
});
