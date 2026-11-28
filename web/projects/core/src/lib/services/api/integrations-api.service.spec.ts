import { HttpHeaders, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { ApiTokenApiService } from './api-token-api.service';
import { AutomationApiService } from './automation-api.service';
import { ImportExportApiService } from './import-export-api.service';
import { WebhookApiService } from './webhook-api.service';

function setup(): HttpTestingController {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return TestBed.inject(HttpTestingController);
}

describe('ApiTokenApiService', () => {
  it('creates a token and returns the plain value apart from the stored token', () => {
    const http = setup();
    let created: { secret: string; token: { prefix: string } } | undefined;
    TestBed.inject(ApiTokenApiService)
      .create({ name: 'CI', scopes: ['read', 'write'], expiresAt: '2030-01-01T00:00:00Z' })
      .subscribe((value) => (created = value));
    const request = http.expectOne({ method: 'POST', url: '/api/v1/me/api-tokens' });
    expect(request.request.body).toEqual({
      name: 'CI',
      scopes: ['read', 'write'],
      expires_at: '2030-01-01T00:00:00Z',
    });
    request.flush({
      data: {
        id: 't1',
        name: 'CI',
        prefix: 'cl_pat_ab',
        scopes: ['read'],
        created_at: 'x',
        token: 'cl_pat_secret',
      },
    });
    expect(created?.secret).toBe('cl_pat_secret');
    expect(created?.token.prefix).toBe('cl_pat_ab');
    expect(JSON.stringify(created?.token)).not.toContain('cl_pat_secret');
  });

  it('lists tokens and revokes by id', () => {
    const http = setup();
    const service = TestBed.inject(ApiTokenApiService);
    let names: string[] = [];
    service.list().subscribe((tokens) => (names = tokens.map((t) => t.name)));
    http
      .expectOne('/api/v1/me/api-tokens')
      .flush({ data: [{ id: 't1', name: 'A', created_at: 'x', revoked_at: 'y' }] });
    expect(names).toEqual(['A']);
    service.list(true).subscribe();
    const all = http.expectOne(
      (r) => r.url === '/api/v1/me/api-tokens' && r.params.has('include_revoked'),
    );
    expect(all.request.params.get('include_revoked')).toBe('true');
    all.flush({ data: [] });
    service.revoke('t1').subscribe();
    http.expectOne({ method: 'DELETE', url: '/api/v1/me/api-tokens/t1' }).flush(null);
  });
});

describe('WebhookApiService', () => {
  it('creates with the filter and project only when set and returns the secret', () => {
    const http = setup();
    let secret = '';
    TestBed.inject(WebhookApiService)
      .create('org1', {
        name: 'n',
        url: 'https://e.com',
        events: ['*'],
        filterQuery: ' ',
        projectId: null,
        enabled: true,
      })
      .subscribe((value) => (secret = value.secret));
    const request = http.expectOne({ method: 'POST', url: '/api/v1/organizations/org1/webhooks' });
    expect(request.request.body).toEqual({
      name: 'n',
      url: 'https://e.com',
      events: ['*'],
      enabled: true,
    });
    request.flush({
      data: {
        id: 'w',
        organization_id: 'org1',
        name: 'n',
        url: 'https://e.com',
        enabled: true,
        created_at: 'x',
        updated_at: 'x',
        secret: 's3',
      },
    });
    expect(secret).toBe('s3');
  });

  it('maps deliveries and posts the redelivery', () => {
    const http = setup();
    const service = TestBed.inject(WebhookApiService);
    let status = '';
    service.deliveries('o', 'w', 10).subscribe((list) => (status = list[0].status));
    http
      .expectOne(
        (r) =>
          r.url === '/api/v1/organizations/o/webhooks/w/deliveries' &&
          r.params.get('limit') === '10',
      )
      .flush({
        data: [
          {
            id: 'd',
            webhook_id: 'w',
            event: 'ping',
            status: 'dead',
            created_at: 'x',
            payload: { a: 1 },
          },
        ],
      });
    expect(status).toBe('dead');
    service.redeliver('o', 'w', 'd').subscribe();
    http
      .expectOne({
        method: 'POST',
        url: '/api/v1/organizations/o/webhooks/w/deliveries/d/redeliver',
      })
      .flush({
        data: { id: 'd2', webhook_id: 'w', event: 'ping', status: 'pending', created_at: 'x' },
      });
  });
});

describe('AutomationApiService', () => {
  it('sends only the fields of each part and the version on update', () => {
    const http = setup();
    TestBed.inject(AutomationApiService)
      .update('r1', 4, {
        name: 'R',
        trigger: { type: 'issue_transitioned', to: 'Hecho', from: '' },
        conditions: [{ type: 'field_in', field: 'status', values: ['a'] }],
        actions: [{ type: 'assign_issue', who: 'user', userId: 'u1', text: '' }],
      })
      .subscribe();
    const request = http.expectOne({ method: 'PATCH', url: '/api/v1/automation-rules/r1' });
    expect(request.request.body).toEqual({
      name: 'R',
      trigger: { type: 'issue_transitioned', to: 'Hecho' },
      conditions: [{ type: 'field_in', field: 'status', values: ['a'] }],
      actions: [{ type: 'assign_issue', who: 'user', user_id: 'u1' }],
      version: 4,
    });
    request.flush({
      data: {
        id: 'r1',
        name: 'R',
        trigger: { type: 'comment_added' },
        version: 5,
        created_at: 'x',
        updated_at: 'x',
      },
    });
  });

  it('maps a rule and its validation', () => {
    const http = setup();
    const service = TestBed.inject(AutomationApiService);
    let count = -1;
    service.list('p1').subscribe((rules) => (count = rules[0].runCount));
    http.expectOne('/api/v1/projects/p1/automation-rules').flush({
      data: [
        {
          id: 'r',
          name: 'n',
          enabled: true,
          trigger: { type: 'scheduled', every_minutes: 10, query: 'q' },
          version: 1,
          run_count: 7,
          created_at: 'x',
          updated_at: 'x',
        },
      ],
    });
    expect(count).toBe(7);
    let planned: readonly string[] = [];
    service
      .validate('p1', {
        name: 'n',
        enabled: true,
        trigger: { type: 'issue_created' },
        conditions: [],
        actions: [{ type: 'add_label', label: 'x' }],
      })
      .subscribe((v) => (planned = v.plannedActions));
    const request = http.expectOne('/api/v1/automation-rules/validate');
    expect(request.request.body.project_id).toBe('p1');
    expect(request.request.body.enabled).toBeUndefined();
    request.flush({ data: { valid: true, planned_actions: ['Añadir etiqueta x'] } });
    expect(planned).toEqual(['Añadir etiqueta x']);
  });
});

describe('ImportExportApiService', () => {
  it('uploads the file as multipart and maps the preview', () => {
    const http = setup();
    let columns: readonly string[] = [];
    TestBed.inject(ImportExportApiService)
      .upload('p1', new File(['a,b\n1,2'], 'x.csv', { type: 'text/csv' }))
      .subscribe((preview) => (columns = preview.columns));
    const request = http.expectOne({ method: 'POST', url: '/api/v1/projects/p1/import/csv' });
    expect(request.request.body instanceof FormData).toBe(true);
    request.flush({
      data: {
        job_id: 'j',
        columns: ['a', 'b'],
        sample_rows: [['1', '2']],
        suggested_mapping: {},
        total_rows: 1,
        delimiter: ',',
      },
    });
    expect(columns).toEqual(['a', 'b']);
  });

  it('starts a dry run with the mapping', () => {
    const http = setup();
    TestBed.inject(ImportExportApiService).start('p1', 'j', { summary: 'a' }, true).subscribe();
    const request = http.expectOne({
      method: 'POST',
      url: '/api/v1/projects/p1/import/csv/j/start',
    });
    expect(request.request.body).toEqual({ mapping: { summary: 'a' }, dry_run: true });
    request.flush({ data: { id: 'j', status: 'queued', created_at: 'x' } });
  });

  it('downloads with the proposed name and the filter', async () => {
    const http = setup();
    let name = '';
    TestBed.inject(ImportExportApiService)
      .exportIssues('p1', ' priority = High ')
      .subscribe((file) => (name = file.fileName));
    const request = http.expectOne((r) => r.url === '/api/v1/projects/p1/export/issues.csv');
    expect(request.request.params.get('filter')).toBe('priority = High');
    request.flush(new Blob(['x']), {
      headers: new HttpHeaders({ 'Content-Disposition': 'attachment; filename="p1.csv"' }),
    });
    expect(name).toBe('p1.csv');
  });

  it('turns a failed download body into a readable error', async () => {
    const http = setup();
    let status = 0;
    let message = '';
    TestBed.inject(ImportExportApiService)
      .exportProject('p1')
      .subscribe({
        error: (error: { status: number; error: { error: { message: string } } }) => {
          status = error.status;
          message = error.error.error.message;
        },
      });
    http
      .expectOne('/api/v1/projects/p1/export/project.json')
      .flush(new Blob([JSON.stringify({ error: { message: 'Demasiado grande' } })]), {
        status: 422,
        statusText: 'Unprocessable',
      });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(status).toBe(422);
    expect(message).toBe('Demasiado grande');
  });
});
