import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { FavoritesStore } from './favorites.store';

const BASE = '/api/v1';

function favoriteDto(id: string, key: string): Record<string, unknown> {
  return {
    type: 'project',
    id,
    name: `Proyecto ${key}`,
    key,
    color: 'blue',
    created_at: '2026-11-01T10:00:00Z',
  };
}

describe('FavoritesStore', () => {
  let store: FavoritesStore;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideCorelogTesting()],
    });
    store = TestBed.inject(FavoritesStore);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('loads favorites and tells which projects are starred', () => {
    store.fetchFavorites();
    backend.expectOne(`${BASE}/favorites?type=project`).flush({ data: [favoriteDto('a', 'AAA')] });

    expect(store.favorites().map((item) => item.key)).toEqual(['AAA']);
    expect(store.isStarred('a')).toBe(true);
    expect(store.isStarred('b')).toBe(false);
  });

  it('keeps a favorites read failure', () => {
    store.fetchFavorites();
    backend
      .expectOne(`${BASE}/favorites?type=project`)
      .flush({ error: { code: 'boom' } }, { status: 500, statusText: 'Server Error' });

    expect(store.favoritesError()).toMatchObject({ status: 500 });
    expect(store.favorites()).toEqual([]);
  });

  it('stars at once and keeps the star when the request succeeds', async () => {
    const result = firstValueFrom(store.star({ id: 'a', name: 'Alfa', key: 'AAA' }));

    expect(store.isStarred('a')).toBe(true);
    backend.expectOne(`${BASE}/favorites/project/a`).flush(null);
    await result;
    expect(store.favorites()[0]).toMatchObject({ id: 'a', name: 'Alfa', key: 'AAA', color: '' });
  });

  it('undoes the star when the request fails', async () => {
    const result = firstValueFrom(store.star({ id: 'a', name: 'Alfa', key: 'AAA' })).catch(
      (e: unknown) => e,
    );
    backend
      .expectOne(`${BASE}/favorites/project/a`)
      .flush({ error: { code: 'project.not_found' } }, { status: 404, statusText: 'Not Found' });

    expect(await result).toMatchObject({ status: 404 });
    expect(store.isStarred('a')).toBe(false);
  });

  it('unstars at once and restores the star when the request fails', async () => {
    store.fetchFavorites();
    backend.expectOne(`${BASE}/favorites?type=project`).flush({ data: [favoriteDto('a', 'AAA')] });

    const ok = firstValueFrom(store.unstar('a'));
    expect(store.isStarred('a')).toBe(false);
    backend.expectOne(`${BASE}/favorites/project/a`).flush(null);
    await ok;
    expect(store.isStarred('a')).toBe(false);

    store.fetchFavorites();
    backend.expectOne(`${BASE}/favorites?type=project`).flush({ data: [favoriteDto('a', 'AAA')] });
    const failed = firstValueFrom(store.unstar('a')).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/favorites/project/a`)
      .flush({ error: { code: 'boom' } }, { status: 500, statusText: 'Server Error' });
    await failed;
    expect(store.isStarred('a')).toBe(true);
  });

  it('loads recent projects with the limit', () => {
    store.fetchRecent(5);
    backend
      .expectOne(`${BASE}/recent?type=project&limit=5`)
      .flush({ data: [{ ...favoriteDto('a', 'AAA'), viewed_at: '2026-11-02T08:00:00Z' }] });

    expect(store.recent()[0]).toMatchObject({ key: 'AAA', viewedAt: '2026-11-02T08:00:00Z' });
  });

  it('reset clears favorites and recents', () => {
    store.fetchFavorites();
    backend.expectOne(`${BASE}/favorites?type=project`).flush({ data: [favoriteDto('a', 'AAA')] });

    store.reset();

    expect(store.favorites()).toEqual([]);
    expect(store.recent()).toEqual([]);
  });
});
