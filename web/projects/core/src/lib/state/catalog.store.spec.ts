import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { provideCorelogDomain } from '../config/corelog-providers';
import { SessionService } from '../services/session.service';
import { CatalogStore } from './catalog.store';
import { OrganizationStore } from './organization.store';

const ORG = '00000000-0000-4000-8000-0000000000aa';
const BASE = `/api/v1/organizations/${ORG}`;

function dto(
  id: string,
  name: string,
  extra: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id,
    name,
    description: '',
    icon: 'task',
    color: 'blue',
    hierarchy_level: 0,
    position: 0,
    is_default: false,
    usage_count: 0,
    created_at: '2026-11-27T10:00:00Z',
    updated_at: '2026-11-27T10:00:00Z',
    ...extra,
  };
}

describe('CatalogStore', () => {
  let store: CatalogStore;
  let backend: HttpTestingController;
  let session: SessionService;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideCorelogDomain(),
      ],
    });
    session = TestBed.inject(SessionService);
    session.startSession('token-1');
    TestBed.inject(OrganizationStore).setCurrentOrg(ORG);
    store = TestBed.inject(CatalogStore);
    backend = TestBed.inject(HttpTestingController);
    TestBed.tick();
  });

  afterEach(() => backend.verify());

  function load(): void {
    store.ensureLoaded();
    backend.expectOne(`${BASE}/issue-types`).flush({
      data: [dto('t-1', 'Tarea', { is_default: true }), dto('t-2', 'Error', { icon: 'bug' })],
    });
    backend.expectOne(`${BASE}/priorities`).flush({
      data: [dto('p-1', 'Alta'), dto('p-2', 'Media', { is_default: true })],
    });
    backend.expectOne(`${BASE}/resolutions`).flush({ data: [dto('r-1', 'Hecho')] });
  }

  it('loads the three catalogs of the active organization once', () => {
    load();
    expect(store.loaded()).toBe(true);
    expect(store.issueTypes().map((item) => item.name)).toEqual(['Tarea', 'Error']);
    expect(store.priorities()).toHaveLength(2);
    expect(store.resolutions()[0].name).toBe('Hecho');

    store.ensureLoaded();
    backend.expectNone(`${BASE}/issue-types`);
  });

  it('exposes the default items and lookups by id', () => {
    load();
    expect(store.defaultIssueType()?.id).toBe('t-1');
    expect(store.defaultPriority()?.id).toBe('p-2');
    expect(store.issueTypeById('t-2')?.icon).toBe('bug');
    expect(store.priorityById('missing')).toBeUndefined();
  });

  it('keeps the error of a failed load and does not report loaded', () => {
    store.ensureLoaded();
    backend
      .expectOne(`${BASE}/issue-types`)
      .flush({ error: { code: 'x', message: 'fallo' } }, { status: 500, statusText: 'Error' });
    backend.expectOne(`${BASE}/priorities`).flush({ data: [] });
    backend.expectOne(`${BASE}/resolutions`).flush({ data: [] });
    expect(store.loaded()).toBe(false);
    expect(store.error()?.message).toBe('fallo');
  });

  it('refreshes the catalog it changed after a create', async () => {
    load();
    const created = firstValueFrom(
      store.create('priorities', { name: 'Bloqueante', color: 'pink' }),
    );
    const post = backend.expectOne(`${BASE}/priorities`);
    expect(post.request.method).toBe('POST');
    expect(post.request.body).toMatchObject({ name: 'Bloqueante', color: 'pink' });
    post.flush({ data: dto('p-3', 'Bloqueante') });
    backend
      .expectOne(`${BASE}/priorities`)
      .flush({ data: [dto('p-1', 'Alta'), dto('p-3', 'Bloqueante')] });
    expect((await created).id).toBe('p-3');
    expect(store.priorities().map((item) => item.name)).toEqual(['Alta', 'Bloqueante']);
  });

  it('sends reassign_to when deleting and surfaces the API error', async () => {
    load();
    const failed = firstValueFrom(store.remove('issue-types', 't-2')).catch(
      (error: unknown) => error,
    );
    backend
      .expectOne(`${BASE}/issue-types/t-2`)
      .flush(
        { error: { code: 'catalog.in_use', message: 'en uso' } },
        { status: 409, statusText: 'Conflict' },
      );
    expect(await failed).toMatchObject({ status: 409 });

    const done = firstValueFrom(store.remove('issue-types', 't-2', 't-1'));
    const request = backend.expectOne(`${BASE}/issue-types/t-2?reassign_to=t-1`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null, { status: 204, statusText: 'No Content' });
    backend.expectOne(`${BASE}/issue-types`).flush({ data: [dto('t-1', 'Tarea')] });
    await done;
    expect(store.issueTypes()).toHaveLength(1);
  });

  it('reorders at once and restores the order when the API refuses', async () => {
    load();
    const failed = firstValueFrom(store.reorder('priorities', ['p-2', 'p-1'])).catch(
      (error: unknown) => error,
    );
    expect(store.priorities().map((item) => item.id)).toEqual(['p-2', 'p-1']);
    backend
      .expectOne(`${BASE}/priorities/reorder`)
      .flush(
        { error: { code: 'catalog.order_mismatch', message: 'no' } },
        { status: 422, statusText: 'Unprocessable' },
      );
    await failed;
    expect(store.priorities().map((item) => item.id)).toEqual(['p-1', 'p-2']);
  });

  it('puts the ids of a reorder in the request body', async () => {
    load();
    const done = firstValueFrom(store.reorder('resolutions', ['r-1']));
    const request = backend.expectOne(`${BASE}/resolutions/reorder`);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({ ids: ['r-1'] });
    request.flush({ data: [dto('r-1', 'Hecho')] });
    expect(await done).toHaveLength(1);
  });

  it('reads and replaces the issue types of a project', async () => {
    const read = firstValueFrom(store.projectIssueTypes('proj-1'));
    backend
      .expectOne('/api/v1/projects/proj-1/issue-types')
      .flush({ data: { items: [dto('t-1', 'Tarea')], configured: false } });
    expect(await read).toMatchObject({ configured: false });

    const saved = firstValueFrom(store.saveProjectIssueTypes('proj-1', ['t-1', 't-2']));
    const put = backend.expectOne('/api/v1/projects/proj-1/issue-types');
    expect(put.request.body).toEqual({ issue_type_ids: ['t-1', 't-2'] });
    put.flush({ data: { items: [dto('t-1', 'Tarea'), dto('t-2', 'Error')], configured: true } });
    expect((await saved).items).toHaveLength(2);
  });

  it('drops the cache when the session ends', () => {
    load();
    session.clear();
    TestBed.tick();
    expect(store.loaded()).toBe(false);
    expect(store.issueTypes()).toEqual([]);
  });
});
