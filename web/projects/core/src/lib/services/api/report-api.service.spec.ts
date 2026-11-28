import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { ReportApiService, reportParams } from './report-api.service';

function setup(): { service: ReportApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(ReportApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('reportParams', () => {
  it('maps the query to snake_case parameters and drops empty values', () => {
    expect(
      reportParams({
        from: '2026-11-01',
        to: '2026-11-30',
        groupBy: 'issues',
        filter: '',
        sprint: undefined,
        interval: 'week',
        limit: 10,
      }),
    ).toEqual({
      from: '2026-11-01',
      to: '2026-11-30',
      group_by: 'issues',
      interval: 'week',
      limit: '10',
    });
  });
});

describe('ReportApiService', () => {
  it('lists the catalog of a project', () => {
    const { service, backend } = setup();
    let kinds: string[] = [];
    service.list('p-1').subscribe((list) => (kinds = list.map((entry) => entry.kind)));
    backend.expectOne('/api/v1/projects/p-1/reports').flush({
      data: [{ kind: 'burndown', title: 'Burndown', description: 'd', params: ['sprint'] }],
    });
    expect(kinds).toEqual(['burndown']);
  });

  it('runs a report with its parameters and maps the payload', () => {
    const { service, backend } = setup();
    let title = '';
    let generatedAt = '';
    let rows = 0;
    service.get('p-1', 'velocity', { limit: 6, sprint: 's-1' }).subscribe((report) => {
      title = report.title;
      generatedAt = report.generatedAt;
      rows = report.table.rows.length;
    });
    const request = backend.expectOne((req) => req.url === '/api/v1/projects/p-1/reports/velocity');
    expect(request.request.params.get('limit')).toBe('6');
    expect(request.request.params.get('sprint')).toBe('s-1');
    request.flush({
      data: {
        kind: 'velocity',
        title: 'Velocidad',
        generated_at: '2026-11-27T10:00:00Z',
        series: [{ name: 'Comprometido', points: [{ x: 'S1', y: 5 }] }],
        table: { columns: [{ key: 'sprint', label: 'Sprint', type: 'text' }], rows: [['S1']] },
        summary: { sprints: 1 },
      },
    });
    expect(title).toBe('Velocidad');
    expect(generatedAt).toBe('2026-11-27T10:00:00Z');
    expect(rows).toBe(1);
  });

  it('tolerates a report without series, table or summary', () => {
    const { service, backend } = setup();
    let series = -1;
    let columns = -1;
    service.get('p-1', 'workload').subscribe((report) => {
      series = report.series.length;
      columns = report.table.columns.length;
    });
    backend
      .expectOne('/api/v1/projects/p-1/reports/workload')
      .flush({ data: { kind: 'workload', title: 'Carga', generated_at: 'x' } });
    expect([series, columns]).toEqual([0, 0]);
  });

  it('downloads the csv as a blob from the .csv endpoint', () => {
    const { service, backend } = setup();
    let blob: Blob | null = null;
    service.csv('p-1', 'burndown', { from: '2026-11-01' }).subscribe((value) => (blob = value));
    const request = backend.expectOne(
      (req) => req.url === '/api/v1/projects/p-1/reports/burndown.csv',
    );
    expect(request.request.responseType).toBe('blob');
    expect(request.request.params.get('from')).toBe('2026-11-01');
    request.flush(new Blob(['a;b']));
    expect(blob).toBeInstanceOf(Blob);
  });

  it('keeps the JSON error body of a failed csv request', async () => {
    const { service, backend } = setup();
    const failure = new Promise<HttpErrorResponse>((resolve) =>
      service.csv('p-1', 'burndown').subscribe({ error: resolve }),
    );
    backend
      .expectOne('/api/v1/projects/p-1/reports/burndown.csv')
      .flush(new Blob([JSON.stringify({ error: { code: 'report.range_too_large' } })]), {
        status: 422,
        statusText: 'Unprocessable Entity',
      });
    const error = await failure;
    expect(error.status).toBe(422);
    expect((error.error as { error: { code: string } }).error.code).toBe('report.range_too_large');
  });
});
