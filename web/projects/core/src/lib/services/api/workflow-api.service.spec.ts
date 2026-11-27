import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import { asApiError } from '../../utils/api-error';
import { WorkflowApiService } from './workflow-api.service';

const PROJECT = 'proj-1';
const VIEW_DTO = {
  statuses: [{ id: 's1', name: 'Por hacer', category: 'todo', position: 0, issue_count: 3 }],
  initial_status_id: 's1',
  transitions: [
    {
      id: 'start',
      name: 'Iniciar',
      from: null,
      to: 's1',
      conditions: [{ type: 'is_assignee' }],
      validators: null,
      post_functions: [{ type: 'assign', params: { to: 'current_user' } }],
      required_fields: ['due_date'],
    },
  ],
  version: 4,
  updated_at: '2026-11-27T00:00:00Z',
  is_template_default: true,
};

describe('WorkflowApiService', () => {
  let service: WorkflowApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(WorkflowApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('maps the workflow view to camelCase with rules defaulted', () => {
    let version = 0;
    let transition: unknown;
    service.get(PROJECT).subscribe((view) => {
      version = view.version;
      transition = view.transitions[0];
    });
    const request = http.expectOne(`/api/v1/projects/${PROJECT}/workflow`);
    expect(request.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    request.flush({ data: VIEW_DTO });
    expect(version).toBe(4);
    expect(transition).toEqual({
      id: 'start',
      name: 'Iniciar',
      from: [],
      to: 's1',
      conditions: [{ type: 'is_assignee', params: {} }],
      validators: [],
      postFunctions: [{ type: 'assign', params: { to: 'current_user' } }],
      requiredFields: ['due_date'],
    });
  });

  it('puts the definition with the expected version and the status mapping', () => {
    service
      .save(PROJECT, {
        expectedVersion: 4,
        initialRef: 's1',
        statuses: [
          { ref: 's1', id: 's1' },
          { ref: 'new-1', name: 'Bloqueado', category: 'in_progress' },
        ],
        transitions: [],
        statusMapping: { old: 's1' },
      })
      .subscribe();
    const request = http.expectOne(`/api/v1/projects/${PROJECT}/workflow`);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body.expected_version).toBe(4);
    expect(request.request.body.initial_ref).toBe('s1');
    expect(request.request.body.status_mapping).toEqual({ old: 's1' });
    request.flush({
      data: { ...VIEW_DTO, warnings: [{ path: 'x', code: 'w', message: 'Aviso' }] },
    });
  });

  it('keeps the path, code and message of a refused definition and the mapping it needs', () => {
    let error: ReturnType<typeof asApiError> | undefined;
    service
      .save(PROJECT, {
        expectedVersion: 1,
        initialRef: 's1',
        statuses: [],
        transitions: [],
        statusMapping: {},
      })
      .subscribe({ error: (raw: unknown) => (error = asApiError(raw)) });
    http.expectOne(`/api/v1/projects/${PROJECT}/workflow`).flush(
      {
        error: {
          code: 'workflow.status_mapping_required',
          message: 'falta',
          details: [
            {
              path: 'transitions[3].to',
              code: 'workflow.unknown_status',
              message: 'Estado desconocido',
            },
          ],
          mapping_required: [{ status_id: 's9', name: 'Viejo', issue_count: 2 }],
        },
      },
      { status: 422, statusText: 'Unprocessable' },
    );
    expect(error?.code).toBe('workflow.status_mapping_required');
    expect(error?.details).toEqual([
      {
        field: 'transitions[3].to',
        code: 'workflow.unknown_status',
        message: 'Estado desconocido',
      },
    ]);
    expect(error?.mappingRequired).toEqual([{ statusId: 's9', name: 'Viejo', issueCount: 2 }]);
  });

  it('validates, resets and migrates', () => {
    let valid: boolean | undefined;
    service
      .validate(PROJECT, {
        expectedVersion: 1,
        initialRef: 's1',
        statuses: [],
        transitions: [],
        statusMapping: {},
      })
      .subscribe((result) => (valid = result.valid));
    http.expectOne(`/api/v1/projects/${PROJECT}/workflow/validate`).flush({
      data: {
        valid: false,
        errors: [{ path: 'a', code: 'b', message: 'c' }],
        mapping_required: null,
      },
    });
    expect(valid).toBe(false);

    service.reset(PROJECT, 'kanban', 4, { a: 'b' }).subscribe();
    const reset = http.expectOne(`/api/v1/projects/${PROJECT}/workflow/reset`);
    expect(reset.request.body).toEqual({
      template: 'kanban',
      expected_version: 4,
      status_mapping: { a: 'b' },
    });
    reset.flush({ data: VIEW_DTO });

    let moved = 0;
    service.migrateIssues(PROJECT, { a: 'b' }).subscribe((result) => (moved = result.moved));
    http
      .expectOne(`/api/v1/projects/${PROJECT}/workflow/migrate-issues`)
      .flush({ data: { moved: 5 } });
    expect(moved).toBe(5);
  });

  it('reads the templates in the editing format', () => {
    let first: unknown;
    service.listTemplates().subscribe((items) => (first = items[0]));
    http.expectOne('/api/v1/workflow-templates').flush({
      data: [
        {
          id: 'kanban',
          name: 'Kanban',
          description: 'd',
          definition: {
            initial_ref: 'todo',
            statuses: [{ ref: 'todo', name: 'Por hacer', category: 'todo' }],
            transitions: [{ id: 'a', name: 'A', from: ['todo'], to: 'todo' }],
          },
        },
      ],
    });
    expect(first).toMatchObject({ id: 'kanban', initialRef: 'todo' });
  });

  it('manages the organization statuses', () => {
    service.createStatus('org', { name: 'Bloqueado', category: 'in_progress' }).subscribe();
    const request = http.expectOne('/api/v1/organizations/org/statuses');
    expect(request.request.method).toBe('POST');
    request.flush({ data: {} });
  });

  it('deletes with the status that receives the issues and reorders by id', () => {
    let reassigned = -1;
    service.deleteStatus('org', 's1', 's2').subscribe((count) => (reassigned = count));
    const del = http.expectOne('/api/v1/organizations/org/statuses/s1?reassign_to=s2');
    del.flush({ data: { id: 's1', reassigned_issues: 4 } });
    expect(reassigned).toBe(4);

    service.reorderStatuses('org', ['b', 'a']).subscribe();
    const put = http.expectOne('/api/v1/organizations/org/statuses/reorder');
    expect(put.request.body).toEqual({ status_ids: ['b', 'a'] });
    put.flush({ data: [] });
  });
});
