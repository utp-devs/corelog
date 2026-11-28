import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  ProjectApiService,
  SessionService,
  SprintReport,
  SprintV2ApiService,
  projectRouteResolver,
} from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { sprintOf } from '../testing/agile-fixtures';
import {
  PROJECT_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  signedInSession,
} from '../testing/planning-test-kit';
import { SprintReportPage } from './sprint-report-page';

ensureMatchMedia();

function reportOf(status: 'active' | 'closed' | 'planned' = 'closed'): SprintReport {
  return {
    sprint: sprintOf('7', {
      name: 'Sprint 7',
      status,
      goal: 'Cerrar pagos',
      startDate: '2026-11-02',
      endDate: '2026-11-13',
    }),
    committedPoints: 10,
    completedPoints: 5,
    incompletePoints: 15,
    addedPoints: 8,
    removedPoints: 2,
    completed: [
      {
        issueId: 'a',
        key: 'CORE-1',
        summary: 'Pagar',
        points: 5,
        statusCategory: 'done',
        addedAfterStart: false,
      },
    ],
    incomplete: [
      {
        issueId: 'b',
        key: 'CORE-4',
        summary: 'Reembolsos',
        points: 8,
        statusCategory: 'todo',
        addedAfterStart: true,
      },
    ],
    removed: [],
    addedAfterStart: [
      {
        issueId: 'b',
        key: 'CORE-4',
        summary: 'Reembolsos',
        points: 8,
        statusCategory: 'todo',
        addedAfterStart: true,
      },
    ],
    burndown: [
      { date: '2026-11-02', ideal: 10, remaining: 10, scope: 10, completed: 0 },
      { date: '2026-11-03', ideal: 9, remaining: 8, scope: 10, completed: 2 },
    ],
    velocity: [{ sprintId: '6', name: 'Sprint 6', points: 12 }],
  };
}

async function renderPage(report: Observable<SprintReport> = of(reportOf())) {
  TestBed.resetTestingModule();
  const api = { report: vi.fn(() => report) };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter([
        {
          path: 'projects/:key',
          resolve: { project: projectRouteResolver },
          children: [{ path: 'sprints/:id/report', component: SprintReportPage }],
        },
      ]),
      { provide: SessionService, useValue: signedInSession() },
      { provide: SprintV2ApiService, useValue: api },
      {
        provide: ProjectApiService,
        useValue: { get: vi.fn(() => of(projectOf())), listMembers: vi.fn(() => of([])) },
      },
    ],
  });
  const harness = await RouterTestingHarness.create(`/projects/${PROJECT_ID}/sprints/7/report`);
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
  return { api, harness };
}

function byTestId(id: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-testid="${id}"]`);
}

describe('SprintReportPage', () => {
  beforeEach(() => {
    // jsdom has no 2D canvas context: chart.js cannot draw, the data table still renders.
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('shows committed against completed points and the scope changes', async () => {
    const { api, harness } = await renderPage();
    expect(api.report).toHaveBeenCalledWith('7');
    const tiles = byTestId('report-tiles')?.textContent ?? '';
    expect(tiles).toContain('Comprometidos');
    expect(tiles).toContain('10');
    expect(tiles).toContain('Completados');
    expect(tiles).toContain('Añadidos');
    expect(document.querySelector('h1')?.textContent).toContain('Informe de Sprint 7');
    harness.fixture.destroy();
  });

  it('lists completed, unfinished, added and removed issues with their points', async () => {
    const { harness } = await renderPage();
    expect(byTestId('report-list-completed')?.textContent).toContain('CORE-1');
    expect(byTestId('report-list-incomplete')?.textContent).toContain('Reembolsos');
    expect(byTestId('report-list-added')?.textContent).toContain('CORE-4');
    expect(byTestId('report-list-removed')?.textContent).toContain('Ninguna');
    harness.fixture.destroy();
  });

  it('draws the burndown and the velocity with an accessible data table', async () => {
    const { harness } = await renderPage();
    expect(byTestId('report-burndown')).not.toBeNull();
    expect(byTestId('report-velocity')).not.toBeNull();
    expect(byTestId('report-burndown')?.querySelector('table')).not.toBeNull();
    harness.fixture.destroy();
  });

  it('explains that a planned sprint has no report yet', async () => {
    const { harness } = await renderPage(of(reportOf('planned')));
    expect(document.body.textContent).toContain('Este sprint todavía no empezó');
    expect(byTestId('report-tiles')).toBeNull();
    harness.fixture.destroy();
  });

  it('shows a skeleton while loading and an error with retry when it fails', async () => {
    const loading = await renderPage(NEVER);
    expect(document.querySelector('[aria-label="Cargando informe"]')).not.toBeNull();
    loading.harness.fixture.destroy();
    const failed = await renderPage(failWith(500));
    expect(document.querySelector('app-error-state')).not.toBeNull();
    failed.harness.fixture.destroy();
  });
});
