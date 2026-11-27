import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { FavoritesApiService } from './favorites-api.service';

const BASE = '/api/v1';
const ID = '0f0e0d0c-0b0a-4908-8706-050403020100';

const favoriteDto = {
  type: 'project',
  id: ID,
  name: 'Plataforma web',
  key: 'WEB',
  color: 'blue',
  created_at: '2026-11-01T10:00:00Z',
};

describe('FavoritesApiService', () => {
  let api: FavoritesApiService;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(FavoritesApiService);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('lists the starred projects', async () => {
    const result = firstValueFrom(api.list());
    backend.expectOne(`${BASE}/favorites?type=project`).flush({ data: [favoriteDto] });

    expect(await result).toEqual([
      {
        type: 'project',
        id: ID,
        name: 'Plataforma web',
        key: 'WEB',
        color: 'blue',
        createdAt: '2026-11-01T10:00:00Z',
      },
    ]);
  });

  it('stars and unstars by kind and id', async () => {
    const starred = firstValueFrom(api.star('project', ID));
    const put = backend.expectOne(`${BASE}/favorites/project/${ID}`);
    expect(put.request.method).toBe('PUT');
    put.flush(null);
    expect(await starred).toBeUndefined();

    const unstarred = firstValueFrom(api.unstar('project', ID));
    const del = backend.expectOne(`${BASE}/favorites/project/${ID}`);
    expect(del.request.method).toBe('DELETE');
    del.flush(null);
    expect(await unstarred).toBeUndefined();
  });

  it('lists recent projects with the optional limit and maps viewedAt', async () => {
    const result = firstValueFrom(api.recent('project', 5));
    backend
      .expectOne(`${BASE}/recent?type=project&limit=5`)
      .flush({ data: [{ ...favoriteDto, viewed_at: '2026-11-02T08:00:00Z' }] });

    expect((await result)[0]).toMatchObject({
      key: 'WEB',
      createdAt: '2026-11-01T10:00:00Z',
      viewedAt: '2026-11-02T08:00:00Z',
    });
  });

  it('omits the limit when it is not given', async () => {
    const result = firstValueFrom(api.recent());
    backend.expectOne(`${BASE}/recent?type=project`).flush({ data: [] });

    expect(await result).toEqual([]);
  });

  it('lets a 404 on star through (hidden or missing project)', async () => {
    const result = firstValueFrom(api.star('project', ID)).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/favorites/project/${ID}`)
      .flush({ error: { code: 'project.not_found' } }, { status: 404, statusText: 'Not Found' });

    expect(await result).toMatchObject({ status: 404 });
  });
});
