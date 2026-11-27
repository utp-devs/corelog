import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { SessionService } from '../services/session.service';
import { DashboardStore } from './dashboard.store';

const ME_URL = '/api/v1/dashboard/me';

function summary(overrides: Record<string, unknown> = {}) {
  return {
    assigned_open: 3,
    assigned_by_status: { open: 1, in_progress: 2, custom: 1 },
    overdue: 1,
    project_count: 2,
    active_sprints: [],
    recent_issues: [],
    ...overrides,
  };
}

function setup(): {
  store: DashboardStore;
  backend: HttpTestingController;
  session: SessionService;
} {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(DashboardStore),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
  };
}

describe('DashboardStore', () => {
  beforeEach(() => localStorage.clear());

  it('builds the status rows with Spanish labels and shares of the total', () => {
    const { store, backend } = setup();
    store.load();
    backend.expectOne(ME_URL).flush({ data: summary() });

    expect(store.statusRows()).toEqual([
      { status: 'open', label: 'Abiertas', count: 1, percent: 25 },
      { status: 'in_progress', label: 'En curso', count: 2, percent: 50 },
      { status: 'custom', label: 'custom', count: 1, percent: 25 },
    ]);
    expect(store.hasWork()).toBe(true);
    expect(store.errorMessage()).toBeNull();
    backend.verify();
  });

  it('gives a zero share to every row when nothing is assigned', () => {
    const { store, backend } = setup();
    store.load();
    backend.expectOne(ME_URL).flush({
      data: summary({ assigned_open: 0, assigned_by_status: { closed: 0 } }),
    });

    expect(store.statusRows()).toEqual([
      { status: 'closed', label: 'Cerradas', count: 0, percent: 0 },
    ]);
    expect(store.hasWork()).toBe(false);
    backend.verify();
  });

  it('counts recent issues as work even without open assignments', () => {
    const { store, backend } = setup();
    store.load();
    backend.expectOne(ME_URL).flush({
      data: summary({
        assigned_open: 0,
        recent_issues: [
          { id: 'i-1', title: 'Revisar', status: 'closed', priority: 'low', project_key: 'CORE' },
        ],
      }),
    });

    expect(store.hasWork()).toBe(true);
    backend.verify();
  });

  it('reports a failed load in Spanish and retries', () => {
    const { store, backend } = setup();
    store.load();
    backend.expectOne(ME_URL).flush({}, { status: 500, statusText: 'Server Error' });

    expect(store.errorMessage()).toBe('No se pudo cargar tu resumen. Intenta de nuevo.');

    store.retry();
    backend.expectOne(ME_URL).flush({ data: summary() });
    expect(store.errorMessage()).toBeNull();
    backend.verify();
  });

  it('clears the summary when the session ends', () => {
    const { store, backend, session } = setup();
    store.load();
    backend.expectOne(ME_URL).flush({ data: summary() });

    session.startSession('token');
    session.clear();
    TestBed.flushEffects();

    expect(store.summary.hasValue()).toBe(false);
    expect(store.statusRows()).toEqual([]);
  });
});
