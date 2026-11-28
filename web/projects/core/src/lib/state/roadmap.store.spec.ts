import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { RoadmapStore } from './roadmap.store';

const URL = '/api/v1/projects/p-1/roadmap';

function bar(id: string, dated = true) {
  return {
    id,
    kind: 'epic',
    key: id.toUpperCase(),
    title: `Epica ${id}`,
    status: 'todo',
    status_category: 'todo',
    start: dated ? '2026-11-01' : null,
    end: dated ? '2026-12-01' : null,
    start_source: dated ? 'own' : 'none',
    end_source: dated ? 'own' : 'none',
    dated,
    complete: false,
    progress: {
      total: 0,
      todo: 0,
      in_progress: 0,
      done: 0,
      percent: 0,
      points_total: 0,
      points_done: 0,
    },
  };
}

function setup() {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return { store: TestBed.inject(RoadmapStore), backend: TestBed.inject(HttpTestingController) };
}

function flushRoadmap(
  backend: HttpTestingController,
  bars = [bar('a'), bar('b'), bar('c', false)],
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
      },
    });
}

describe('RoadmapStore', () => {
  it('splits dated and undated bars, never hiding the undated ones', () => {
    const { store, backend } = setup();
    store.load('p-1');
    flushRoadmap(backend);
    expect(store.dated().map((b) => b.id)).toEqual(['a', 'b']);
    expect(store.undated().map((b) => b.id)).toEqual(['c']);
  });

  it('reloads with the new grouping, filter and search', () => {
    const { store, backend } = setup();
    store.load('p-1');
    flushRoadmap(backend);

    const params = (): URLSearchParams => {
      const request = backend.expectOne((r) => r.url === URL);
      request.flush({
        data: {
          from: null,
          to: null,
          group_by: 'epic',
          bars: [],
          versions: [],
          sprints: [],
          dependencies: [],
        },
      });
      return new URLSearchParams(request.request.urlWithParams.split('?')[1] ?? '');
    };

    store.setGroupBy('sprint');
    expect(params().get('group_by')).toBe('sprint');
    store.setFilter('done');
    expect(params().get('filter')).toBe('done');
    store.setQuery(' portal ');
    expect(params().get('q')).toBe('portal');
  });

  it('shows the new order at once and keeps it when the request succeeds', () => {
    const { store, backend } = setup();
    store.load('p-1');
    flushRoadmap(backend);

    store.rank(['b', 'a']).subscribe();
    expect(store.bars().map((b) => b.id)).toEqual(['b', 'a', 'c']);
    backend.expectOne(`${URL}/rank`).flush(null, { status: 204, statusText: 'No Content' });
    expect(store.bars().map((b) => b.id)).toEqual(['b', 'a', 'c']);
  });

  it('goes back to the previous order when saving fails', () => {
    const { store, backend } = setup();
    store.load('p-1');
    flushRoadmap(backend);

    store.rank(['b', 'a']).subscribe({ error: () => undefined });
    backend
      .expectOne(`${URL}/rank`)
      .flush({ error: { message: 'x' } }, { status: 422, statusText: 'x' });
    expect(store.bars().map((b) => b.id)).toEqual(['a', 'b', 'c']);
  });

  it('reports an inaccessible project', () => {
    const { store, backend } = setup();
    store.load('p-1');
    backend
      .expectOne((r) => r.url === URL)
      .flush({ error: { message: 'no' } }, { status: 404, statusText: 'Not Found' });
    expect(store.forbidden()).toBe(true);
    expect(store.errorMessage()).toContain('hoja de ruta');
  });
});
