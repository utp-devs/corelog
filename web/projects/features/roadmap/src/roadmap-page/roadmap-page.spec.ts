import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CurrentProjectStore, SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { RoadmapPage } from './roadmap-page';

const URL = '/api/v1/projects/p-1/roadmap';

function bar(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    kind: 'epic',
    key: id.toUpperCase(),
    title: `Epica ${id}`,
    status: 'todo',
    status_category: 'in_progress',
    start: '2026-11-03',
    end: '2026-12-20',
    start_source: 'own',
    end_source: 'own',
    dated: true,
    complete: false,
    progress: {
      total: 4,
      todo: 1,
      in_progress: 1,
      done: 2,
      percent: 50,
      points_total: 0,
      points_done: 0,
    },
    ...overrides,
  };
}

const UNDATED = bar('c', {
  dated: false,
  start: null,
  end: null,
  start_source: 'none',
  end_source: 'none',
});

function setup(canEdit = true) {
  const matchMedia = window.matchMedia;
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
      {
        provide: CurrentProjectStore,
        useValue: {
          projectId: signal('p-1'),
          key: signal('CL'),
          isArchived: signal(false),
          can: () => canEdit,
          reload: () => undefined,
        },
      },
    ],
  });
  window.matchMedia = matchMedia;
  const fixture = TestBed.createComponent(RoadmapPage);
  fixture.detectChanges();
  return { fixture, backend: TestBed.inject(HttpTestingController) };
}

function respond(
  fixture: ComponentFixture<RoadmapPage>,
  backend: HttpTestingController,
  bars: unknown[],
  extra: Record<string, unknown> = {},
) {
  backend
    .expectOne((r) => r.url === URL)
    .flush({
      data: {
        from: null,
        to: null,
        group_by: 'epic',
        bars,
        versions: [],
        sprints: [],
        dependencies: [],
        ...extra,
      },
    });
  fixture.detectChanges();
}

const root = (fixture: ComponentFixture<RoadmapPage>) => fixture.nativeElement as HTMLElement;

describe('RoadmapPage', () => {
  it('loads the roadmap of the project by its id and draws a bar per dated epic', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, [bar('a'), bar('b'), UNDATED]);
    const el = root(fixture);
    expect(el.querySelector('[data-testid="timeline-bar-a"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="timeline-bar-b"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="timeline-bar-c"]')).toBeNull();
  });

  it('lists the items without dates in their own section instead of hiding them', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, [bar('a'), UNDATED]);
    const section = root(fixture).querySelector('[data-testid="roadmap-undated"]');
    expect(section?.textContent).toContain('Sin fechas (1)');
    expect(section?.textContent).toContain('Epica c');
  });

  it('shows a note when no item has dates but still lists them', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, [UNDATED]);
    expect(root(fixture).querySelector('[data-testid="roadmap-no-dated"]')).not.toBeNull();
    expect(root(fixture).querySelector('[data-testid="roadmap-undated"]')).not.toBeNull();
  });

  it('shows the module illustration and a message without epics', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, []);
    const el = root(fixture);
    expect(el.textContent).toContain('Aún no hay épicas');
    expect(el.querySelector('img')?.getAttribute('src')).toBe(
      '/images/modules/module-roadmap.webp',
    );
  });

  it('reloads grouped by version, with a filter and with a search', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, [bar('a')]);
    const el = root(fixture);

    (el.querySelector('[data-testid="roadmap-group-version"]') as HTMLElement).click();
    const byVersion = backend.expectOne((r) => r.url === URL);
    expect(byVersion.request.params.get('group_by')).toBe('version');
    byVersion.flush({
      data: {
        from: null,
        to: null,
        group_by: 'version',
        bars: [],
        versions: [],
        sprints: [],
        dependencies: [],
      },
    });
    fixture.detectChanges();

    (el.querySelector('[data-testid="roadmap-filter-done"]') as HTMLElement).click();
    const done = backend.expectOne((r) => r.url === URL);
    expect(done.request.params.get('filter')).toBe('done');
    done.flush({
      data: {
        from: null,
        to: null,
        group_by: 'version',
        bars: [],
        versions: [],
        sprints: [],
        dependencies: [],
      },
    });
    fixture.detectChanges();
    expect(el.textContent).toContain('Nada coincide con los filtros');
  });

  it('switches to the accessible table view with every item', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, [bar('a'), UNDATED]);
    const el = root(fixture);
    (el.querySelector('[data-testid="roadmap-view-table"]') as HTMLElement).click();
    fixture.detectChanges();

    const table = el.querySelector('[data-testid="roadmap-table"]') as HTMLTableElement;
    expect(table.querySelectorAll('tbody tr')).toHaveLength(2);
    expect(table.querySelectorAll('th[scope="col"]')).toHaveLength(5);
    expect(table.textContent).toContain('Sin fecha');
    expect(el.querySelector('app-timeline')).toBeNull();
  });

  it('saves the new order of the epics and keeps it', () => {
    const { fixture, backend } = setup();
    respond(fixture, backend, [bar('a'), bar('b')]);
    const el = root(fixture);
    (el.querySelector('[aria-label="Bajar Epica a"]') as HTMLElement).click();
    const rank = backend.expectOne(`${URL}/rank`);
    expect(rank.request.body).toEqual({ epic_ids: ['b', 'a'] });
    rank.flush(null, { status: 204, statusText: 'No Content' });
    fixture.detectChanges();
    expect(
      [...el.querySelectorAll('.timeline__title')].map((t) => t.getAttribute('data-testid')),
    ).toEqual(['timeline-row-b', 'timeline-row-a']);
  });

  it('does not offer reordering without permission to edit issues', () => {
    const { fixture, backend } = setup(false);
    respond(fixture, backend, [bar('a'), bar('b')]);
    expect(root(fixture).querySelector('.timeline__move')).toBeNull();
  });

  it('shows an error with retry when the roadmap cannot be loaded', () => {
    const { fixture, backend } = setup();
    backend
      .expectOne((r) => r.url === URL)
      .flush({ error: { message: 'x' } }, { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(root(fixture).querySelector('app-error-state')).not.toBeNull();
  });
});
