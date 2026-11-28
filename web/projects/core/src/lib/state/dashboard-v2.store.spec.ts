import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { DashboardV2Store } from './dashboard-v2.store';

const BASE = '/api/v1';

function dto(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    name: `Panel ${id}`,
    description: '',
    layout: '2',
    is_default: false,
    owner: { id: 'u-1', name: 'Ana' },
    organization_id: 'o-1',
    access: 'owner',
    can_edit: true,
    can_manage: true,
    is_favorite: false,
    shared: false,
    gadget_count: 0,
    gadgets: [],
    updated_at: '2026-11-27T10:00:00Z',
    ...overrides,
  };
}

function setup() {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(DashboardV2Store),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('DashboardV2Store', () => {
  it('loads the personal dashboard', () => {
    const { store, backend } = setup();
    store.loadMine();
    backend.expectOne(`${BASE}/me/dashboard`).flush({ data: dto('d-1', { is_default: true }) });
    expect(store.current.value()?.isDefault).toBe(true);
  });

  it('tells a hidden dashboard (404) from a failed load', () => {
    const { store, backend } = setup();
    store.loadOne('d-9');
    backend
      .expectOne(`${BASE}/dashboards/d-9`)
      .flush({ error: { message: 'no' } }, { status: 404, statusText: 'Not Found' });
    expect(store.notFound()).toBe(true);
    expect(store.loadError()).toContain('panel');
  });

  it('shows the stored result of a save and clears the saving flag', () => {
    const { store, backend } = setup();
    store.loadOne('d-1');
    backend.expectOne(`${BASE}/dashboards/d-1`).flush({ data: dto('d-1') });

    store.saveGadgets('d-1', '3', []).subscribe();
    expect(store.saving()).toBe(true);
    backend
      .expectOne(`${BASE}/dashboards/d-1/gadgets`)
      .flush({ data: dto('d-1', { layout: '3' }) });
    expect(store.saving()).toBe(false);
    expect(store.current.value()?.layout).toBe('3');
  });

  it('clears the saving flag when a save fails', () => {
    const { store, backend } = setup();
    store.saveGadgets('d-1', '2', []).subscribe({ error: () => undefined });
    backend
      .expectOne(`${BASE}/dashboards/d-1/gadgets`)
      .flush({ error: { message: 'x' } }, { status: 422, statusText: 'x' });
    expect(store.saving()).toBe(false);
  });

  it('follows favorites in the list and in the dashboard in view', () => {
    const { store, backend } = setup();
    store.loadList();
    backend
      .expectOne((r) => r.url === `${BASE}/dashboards`)
      .flush({ data: [dto('d-1'), dto('d-2')], meta: { page: 1, page_size: 100, total: 2 } });
    const first = store.items()[0];

    store.toggleFavorite(first).subscribe();
    const star = backend.expectOne(`${BASE}/favorites/dashboard/d-1`);
    expect(star.request.method).toBe('PUT');
    star.flush(null);
    expect(store.items().map((d) => d.isFavorite)).toEqual([true, false]);
  });

  it('removes a deleted dashboard from the list', () => {
    const { store, backend } = setup();
    store.loadList();
    backend
      .expectOne((r) => r.url === `${BASE}/dashboards`)
      .flush({ data: [dto('d-1'), dto('d-2')], meta: { page: 1, page_size: 100, total: 2 } });
    store.remove('d-1').subscribe();
    backend
      .expectOne(`${BASE}/dashboards/d-1`)
      .flush(null, { status: 204, statusText: 'No Content' });
    expect(store.items().map((d) => d.id)).toEqual(['d-2']);
  });
});
