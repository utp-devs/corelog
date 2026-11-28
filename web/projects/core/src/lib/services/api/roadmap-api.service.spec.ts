import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { Roadmap } from '../../models/roadmap.model';
import { RoadmapApiService } from './roadmap-api.service';

const BASE = '/api/v1/projects/p-1/roadmap';

function bar(overrides: Record<string, unknown> = {}) {
  return {
    id: 'e-1',
    kind: 'epic',
    key: 'CL-1',
    title: 'Portal',
    status: 'todo',
    status_category: 'in_progress',
    start: '2026-11-01',
    end: '2026-12-01',
    start_source: 'own',
    end_source: 'derived',
    dated: true,
    complete: false,
    progress: {
      total: 4,
      todo: 1,
      in_progress: 2,
      done: 1,
      percent: 25,
      points_total: 11,
      points_done: 1,
    },
    ...overrides,
  };
}

function setup() {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(RoadmapApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('RoadmapApiService', () => {
  it('sends the grouping, filter and search and maps the bars', () => {
    const { service, backend } = setup();
    let roadmap: Roadmap | undefined;
    service
      .get('p-1', { groupBy: 'version', filter: 'open', q: 'portal' })
      .subscribe((value) => (roadmap = value));
    const request = backend.expectOne((r) => r.url === BASE);
    expect(request.request.params.get('group_by')).toBe('version');
    expect(request.request.params.get('filter')).toBe('open');
    expect(request.request.params.get('q')).toBe('portal');
    request.flush({
      data: {
        from: null,
        to: null,
        group_by: 'version',
        bars: [bar(), bar({ id: 'e-2', dated: false, start: null, end: null, key: undefined })],
        versions: null,
        sprints: [bar({ id: 's-1', kind: 'sprint' })],
        dependencies: [{ from: 'e-1', to: 'e-2', kind: 'blocks' }],
      },
    });

    expect(roadmap?.groupBy).toBe('version');
    expect(roadmap?.bars[0]).toMatchObject({
      key: 'CL-1',
      statusCategory: 'in_progress',
      progress: { percent: 25, inProgress: 2, pointsTotal: 11 },
    });
    expect(roadmap?.bars[1]).toMatchObject({ dated: false, key: '', start: null });
    expect(roadmap?.versions).toEqual([]);
    expect(roadmap?.sprints[0].kind).toBe('sprint');
    expect(roadmap?.dependencies).toEqual([{ from: 'e-1', to: 'e-2', kind: 'blocks' }]);
  });

  it('does not send the all filter', () => {
    const { service, backend } = setup();
    service.get('p-1', { filter: 'all' }).subscribe();
    const request = backend.expectOne((r) => r.url === BASE);
    expect(request.request.params.has('filter')).toBe(false);
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
  });

  it('saves the manual order of the epics', () => {
    const { service, backend } = setup();
    service.rank('p-1', ['e-2', 'e-1']).subscribe();
    const request = backend.expectOne(`${BASE}/rank`);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({ epic_ids: ['e-2', 'e-1'] });
    request.flush(null, { status: 204, statusText: 'No Content' });
  });
});
