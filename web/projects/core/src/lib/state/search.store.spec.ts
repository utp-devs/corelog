import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { RECENT_STORAGE_KEY } from '../utils/recent-items';
import { SearchStore } from './search.store';

const hit = {
  kind: 'issue',
  id: 'i-1',
  title: 'Exportar',
  subtitle: 'CORE',
  link: '/browse/i-1',
} as const;

describe('SearchStore', () => {
  let store: SearchStore;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    store = TestBed.inject(SearchStore);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('sends a query of at least two characters and stores the hits', () => {
    store.search('exportar');

    expect(store.query()).toBe('exportar');
    httpMock
      .expectOne(
        (request) => request.url === '/api/v1/search' && request.params.get('q') === 'exportar',
      )
      .flush({ data: [hit] });

    expect(store.hits()).toEqual([hit]);
    expect(store.results.loading()).toBe(false);
  });

  it('clears the results and sends nothing for a short query', () => {
    store.search('exportar');
    httpMock.expectOne('/api/v1/search?q=exportar&limit=10').flush({ data: [hit] });

    store.search('e');

    expect(store.searchable()).toBe(false);
    expect(store.hits()).toEqual([]);
    httpMock.expectNone(() => true);
  });

  it('exposes the failure of the search as an error', () => {
    store.search('exportar');
    httpMock
      .expectOne((request) => request.url === '/api/v1/search')
      .flush({ error: { message: 'Fallo' } }, { status: 500, statusText: 'Server Error' });

    expect(store.results.error()?.message).toBe('Fallo');
    expect(store.hits()).toEqual([]);
  });

  it('keeps recent items newest first and persists them', () => {
    store.rememberRecent(hit);
    store.rememberRecent({ ...hit, id: 'i-2', link: '/browse/i-2' });

    expect(store.recent().map((item) => item.id)).toEqual(['i-2', 'i-1']);
    expect(localStorage.getItem(RECENT_STORAGE_KEY)).toContain('i-2');
  });

  it('clears the query and results on reset without dropping recent items', () => {
    store.rememberRecent(hit);
    store.search('exportar');
    httpMock.expectOne((request) => request.url === '/api/v1/search').flush({ data: [hit] });

    store.reset();

    expect(store.query()).toBe('');
    expect(store.hits()).toEqual([]);
    expect(store.recent()).toHaveLength(1);
  });
});
