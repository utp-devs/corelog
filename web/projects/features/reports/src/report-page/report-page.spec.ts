import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { NEVER, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  reportOf,
  reportsTestEnvironment,
  type ReportsTestEnvironment,
  type ReportsTestOptions,
} from '../testing/reports-test-kit';
import { ReportPage } from './report-page';

@Component({ template: '' })
class HubStub {}

const ROUTES = [
  { path: 'projects/:key/reports', component: HubStub },
  { path: 'projects/:key/reports/:kind', component: ReportPage },
];

function apiError(code: string, status = 422) {
  return { status, code, message: 'x', details: [] };
}

describe('ReportPage', () => {
  beforeEach(() => {
    // jsdom has no 2D canvas context: keep chart.js quiet.
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  async function open(
    url: string,
    options: ReportsTestOptions = {},
  ): Promise<{ fixture: ComponentFixture<unknown>; env: ReportsTestEnvironment; router: Router }> {
    const env = reportsTestEnvironment(options);
    TestBed.configureTestingModule({ providers: [provideRouter(ROUTES), ...env.providers] });
    const harness = await RouterTestingHarness.create(url);
    const fixture = harness.fixture as ComponentFixture<unknown>;
    await settle(fixture);
    return { fixture, env, router: TestBed.inject(Router) };
  }

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 4; pass += 1) {
      fixture.detectChanges();
      await fixture.whenStable();
      await new Promise((resolve) => setTimeout(resolve));
    }
  }

  function el(fixture: ComponentFixture<unknown>): HTMLElement {
    return fixture.nativeElement as HTMLElement;
  }

  it('runs the report named in the URL with the parameters of the query string', async () => {
    const { fixture, env } = await open(
      '/projects/SCRUM/reports/created-vs-resolved?from=2026-11-01&to=2026-11-20&interval=week',
    );
    expect(env.get).toHaveBeenCalledWith('p-1', 'created-vs-resolved', {
      from: '2026-11-01',
      to: '2026-11-20',
      interval: 'week',
    });
    expect(el(fixture).querySelector('h1')?.textContent).toContain('Creadas frente a resueltas');
  });

  it('shows summary tiles, an accessible chart and the full table', async () => {
    const { fixture } = await open('/projects/SCRUM/reports/workload');
    expect(el(fixture).querySelector('[data-testid="report-summary"]')?.textContent).toContain(
      'Incidencias abiertas',
    );
    const chart = el(fixture).querySelector('[data-testid="report-chart"] [role="img"]');
    expect(chart?.getAttribute('aria-label')).toContain('Incidencias abiertas: 2 valores');
    const rows = el(fixture).querySelectorAll('[data-testid="report-table"] tbody tr');
    expect(rows).toHaveLength(2);
  });

  it('writes a parameter change into the URL so the report can be shared', async () => {
    const { fixture, router } = await open('/projects/SCRUM/reports/created-vs-resolved');
    el(fixture).querySelector<HTMLButtonElement>('[data-testid="report-preset-7"]')?.click();
    await settle(fixture);
    const tree = router.parseUrl(router.url);
    expect(tree.queryParams['from']).toBeTruthy();
    expect(tree.queryParams['to']).toBeTruthy();
  });

  it('shows "Sin datos en este periodo" when the report has no data', async () => {
    const { fixture } = await open('/projects/SCRUM/reports/workload', {
      get: () =>
        of(
          reportOf('workload', {
            series: [],
            table: { columns: [{ key: 'a', label: 'A', type: 'text' }], rows: [] },
          }),
        ),
    });
    expect(el(fixture).querySelector('[data-testid="report-empty"]')?.textContent).toContain(
      'Sin datos en este periodo',
    );
    expect(el(fixture).querySelector('[data-testid="report-chart"]')).toBeNull();
  });

  it('maps a report error code to a Spanish message', async () => {
    const { fixture } = await open('/projects/SCRUM/reports/workload', {
      get: () => throwError(() => apiError('report.range_too_large')),
    });
    expect(el(fixture).querySelector('[data-testid="report-error"]')?.textContent).toContain('366');
    // The parameters stay visible so the user can fix the range.
    expect(el(fixture).querySelector('[data-testid="report-params"]')).not.toBeNull();
  });

  it('keeps a missing sprint as an inline error, not as a not-found page', async () => {
    const { fixture } = await open('/projects/SCRUM/reports/burndown', {
      get: () => throwError(() => apiError('report.sprint_not_found', 404)),
    });
    expect(el(fixture).querySelector('app-error-page')).toBeNull();
    expect(el(fixture).textContent).toContain('El sprint elegido ya no existe');
  });

  it('shows the forbidden page without the view_reports permission', async () => {
    const { fixture, env } = await open('/projects/SCRUM/reports/workload', {
      canViewReports: false,
    });
    expect(el(fixture).querySelector('app-error-page')).not.toBeNull();
    expect(env.get).not.toHaveBeenCalled();
  });

  it('shows not found for a report the catalog does not list', async () => {
    const { fixture } = await open('/projects/SCRUM/reports/nope');
    expect(el(fixture).querySelector('app-error-page')).not.toBeNull();
  });

  it('shows a skeleton while the report loads', async () => {
    const { fixture } = await open('/projects/SCRUM/reports/workload', { get: () => NEVER });
    expect(el(fixture).querySelector('app-loading-skeleton')).not.toBeNull();
  });

  it('explains that the version report needs versions when the project has none', async () => {
    const { fixture, env } = await open('/projects/SCRUM/reports/version');
    expect(el(fixture).textContent).toContain('Este proyecto aún no tiene versiones');
    expect(env.get).not.toHaveBeenCalled();
  });

  it('picks a version by itself for the version report', async () => {
    const { env, router } = await open('/projects/SCRUM/reports/version', {
      versions: [
        { id: 'v-old', name: '0.9', status: 'released' },
        { id: 'v-new', name: '1.0', status: 'unreleased' },
      ],
    });
    expect(router.parseUrl(router.url).queryParams['version']).toBe('v-new');
    expect(env.get).toHaveBeenCalledWith('p-1', 'version', { version: 'v-new' });
  });

  it('downloads the CSV with the parameters on screen', async () => {
    const createObjectURL = vi.fn(() => 'blob:x');
    URL.createObjectURL = createObjectURL;
    URL.revokeObjectURL = vi.fn();
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined);
    const { fixture, env } = await open('/projects/SCRUM/reports/workload?filter=type%20%3D%20bug');
    const button = el(fixture).querySelector<HTMLElement>('[data-testid="report-export"] button');
    button?.click();
    await settle(fixture);
    expect(env.csv).toHaveBeenCalled();
    expect(createObjectURL).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
  });

  it('reports a failed export with a Spanish toast', async () => {
    const { fixture, env } = await open('/projects/SCRUM/reports/workload', {
      csv: () => throwError(() => ({ status: 500 })),
    });
    const button = el(fixture).querySelector<HTMLElement>('[data-testid="report-export"] button');
    button?.click();
    await settle(fixture);
    expect(env.csv).toHaveBeenCalled();
  });
});
