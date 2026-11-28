import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { ReportStore } from './report.store';

function setup(): { store: ReportStore; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return { store: TestBed.inject(ReportStore), backend: TestBed.inject(HttpTestingController) };
}

const REPORT = {
  kind: 'workload',
  title: 'Carga de trabajo',
  generated_at: '2026-11-27T10:00:00Z',
  series: [],
  table: { columns: [], rows: [] },
  summary: {},
};

describe('ReportStore', () => {
  it('loads the catalog', () => {
    const { store, backend } = setup();
    store.loadCatalog('p-1');
    expect(store.catalog.loading()).toBe(true);
    backend.expectOne('/api/v1/projects/p-1/reports').flush({
      data: [{ kind: 'burndown', title: 'Burndown', description: 'd', params: [] }],
    });
    expect(store.catalog.value()?.length).toBe(1);
    expect(store.catalog.loading()).toBe(false);
  });

  it('runs a report and keeps its value', () => {
    const { store, backend } = setup();
    store.run('p-1', 'workload', {});
    backend.expectOne('/api/v1/projects/p-1/reports/workload').flush({ data: REPORT });
    expect(store.report.value()?.title).toBe('Carga de trabajo');
  });

  it('clears the previous report before running another and cancels the one in flight', () => {
    const { store, backend } = setup();
    store.run('p-1', 'workload', {});
    backend.expectOne('/api/v1/projects/p-1/reports/workload').flush({ data: REPORT });
    store.run('p-1', 'velocity', {});
    expect(store.report.value()).toBeUndefined();
    store.run('p-1', 'burndown', {});
    const cancelled = backend.expectOne('/api/v1/projects/p-1/reports/velocity');
    expect(cancelled.cancelled).toBe(true);
    backend.expectOne('/api/v1/projects/p-1/reports/burndown');
  });

  it('keeps the error of a failed report', () => {
    const { store, backend } = setup();
    store.run('p-1', 'burndown', {});
    backend
      .expectOne('/api/v1/projects/p-1/reports/burndown')
      .flush(
        { error: { code: 'report.range_too_large', message: 'x' } },
        { status: 422, statusText: 'Unprocessable' },
      );
    expect(store.report.error()?.code).toBe('report.range_too_large');
    expect(store.report.value()).toBeUndefined();
  });
});
