import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { FilterStore } from './filter.store';

function dto(id: string, name: string, access = 'owner'): Record<string, unknown> {
  return {
    id,
    name,
    description: '',
    query: 'project = KAN',
    columns: ['key'],
    sort: 'updated DESC',
    owner: { id: 'u1', name: 'Ana' },
    organization_id: 'o1',
    access,
    is_favorite: false,
    is_subscribed: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };
}

describe('FilterStore', () => {
  let store: FilterStore;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    store = TestBed.inject(FilterStore);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('lists filters and splits own from shared ones', () => {
    store.load('all');
    http
      .expectOne((r) => r.url === '/api/v1/filters' && r.params.get('scope') === 'all')
      .flush({ data: [dto('1', 'Mío'), dto('2', 'Del equipo', 'view')] });
    expect(store.mine().map((f) => f.name)).toEqual(['Mío']);
    expect(store.shared().map((f) => f.name)).toEqual(['Del equipo']);
    expect(store.find('2')?.access).toBe('view');
  });

  it('saves a filter into the sorted list and reports a taken name', async () => {
    const saved = store.save({ name: 'Bugs', query: 'type = Error', columns: ['key'], sort: '' });
    http.expectOne('/api/v1/filters').flush({ data: dto('3', 'Bugs') });
    expect((await saved).id).toBe('3');
    expect(store.filters().map((f) => f.name)).toEqual(['Bugs']);

    const duplicate = store.save({ name: 'bugs', query: 'x' });
    http
      .expectOne('/api/v1/filters')
      .flush(
        { error: { code: 'filter.name_taken', message: 'nombre en uso' } },
        { status: 409, statusText: 'Conflict' },
      );
    await expect(duplicate).rejects.toMatchObject({ code: 'filter.name_taken' });
  });

  it('shares and subscribes, updating the list in place', async () => {
    store.load('all');
    http.expectOne((r) => r.url === '/api/v1/filters').flush({ data: [dto('1', 'Mío')] });

    const share = store.share('1', [{ type: 'organization', targetId: 'o1', permission: 'view' }]);
    const request = http.expectOne('/api/v1/filters/1/shares');
    expect(request.request.body).toEqual({
      shares: [{ type: 'organization', target_id: 'o1', permission: 'view' }],
    });
    request.flush({
      data: [{ type: 'organization', target_id: 'o1', target_name: 'Acme', permission: 'view' }],
    });
    await share;
    expect(store.find('1')?.shares[0].targetName).toBe('Acme');

    const schedule = {
      frequency: 'weekly',
      hour: 8,
      weekday: 1,
      timezone: 'America/Lima',
    } as const;
    const subscribe = store.subscribe('1', schedule);
    http.expectOne('/api/v1/filters/1/subscription').flush({ data: {} });
    await subscribe;
    expect(store.find('1')?.isSubscribed).toBe(true);

    const cancel = store.unsubscribe('1');
    http
      .expectOne('/api/v1/filters/1/subscription')
      .flush(null, { status: 204, statusText: 'No Content' });
    await cancel;
    expect(store.find('1')?.subscription).toBeNull();
  });

  it('removes a deleted filter', async () => {
    store.load('all');
    http.expectOne((r) => r.url === '/api/v1/filters').flush({ data: [dto('1', 'Mío')] });
    const gone = store.remove('1');
    http.expectOne('/api/v1/filters/1').flush(null, { status: 204, statusText: 'No Content' });
    await gone;
    expect(store.filters()).toEqual([]);
  });
});
