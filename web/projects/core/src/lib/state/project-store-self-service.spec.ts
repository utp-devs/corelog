import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { ProjectStore } from './project.store';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';
const ID_A = '0f0e0d0c-0b0a-4908-8706-050403020100';
const ID_B = '1f0e0d0c-0b0a-4908-8706-050403020100';

function projectDto(id: string, key: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    organization_id: ORG_ID,
    key,
    name: `Proyecto ${key}`,
    description: '',
    type: 'kanban',
    lead_id: 'u1',
    visibility: 'open',
    archived_at: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
    ...overrides,
  };
}

describe('ProjectStore (self-service additions)', () => {
  let store: ProjectStore;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideCorelogTesting(),
        { provide: ORGANIZATION_ID_PROVIDER, useValue: () => ORG_ID },
      ],
    });
    store = TestBed.inject(ProjectStore);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  function loadPage(): void {
    store.fetchPage({ page: 1, pageSize: 2 });
    backend
      .expectOne((req) => req.url === `${BASE}/projects`)
      .flush({
        data: [projectDto(ID_A, 'AAA'), projectDto(ID_B, 'BBB')],
        meta: { total: 5, page: 1, page_size: 2 },
      });
  }

  it('loads a page with its meta', () => {
    expect(store.pageMeta()).toBeNull();
    expect(store.pageLoaded()).toBe(false);

    loadPage();

    expect(store.pageItems().map((item) => item.key)).toEqual(['AAA', 'BBB']);
    expect(store.pageMeta()).toEqual({ total: 5, page: 1, pageSize: 2 });
    expect(store.pageLoaded()).toBe(true);
    expect(store.pageError()).toBeNull();
  });

  it('keeps a page read failure', () => {
    store.fetchPage();
    backend
      .expectOne((req) => req.url === `${BASE}/projects`)
      .flush(
        { error: { code: 'organization.not_found' } },
        { status: 404, statusText: 'Not Found' },
      );

    expect(store.pageError()).toMatchObject({ status: 404 });
    expect(store.pageItems()).toEqual([]);
  });

  it('checks a key through the API', async () => {
    const result = firstValueFrom(store.keyAvailable('core'));
    backend
      .expectOne(`${BASE}/projects/key-available?organization_id=${ORG_ID}&key=core`)
      .flush({ data: { available: true, key: 'CORE' } });

    expect(await result).toEqual({ available: true, key: 'CORE' });
  });

  it('archives a project and refreshes it in the page and the one in view', async () => {
    loadPage();
    store.loadOne(ID_A);
    backend.expectOne(`${BASE}/projects/${ID_A}`).flush({ data: projectDto(ID_A, 'AAA') });

    const result = firstValueFrom(store.archiveProject(ID_A));
    backend
      .expectOne(`${BASE}/projects/${ID_A}/archive`)
      .flush({ data: projectDto(ID_A, 'AAA', { archived_at: '2026-11-02T10:00:00Z' }) });
    await result;

    expect(store.pageItems()[0].archivedAt).toBe('2026-11-02T10:00:00Z');
    expect(store.project()?.archivedAt).toBe('2026-11-02T10:00:00Z');
  });

  it('restores a project', async () => {
    loadPage();

    const result = firstValueFrom(store.restoreProject(ID_B));
    backend.expectOne(`${BASE}/projects/${ID_B}/restore`).flush({
      data: projectDto(ID_B, 'BBB', { name: 'Restaurado' }),
    });
    await result;

    expect(store.pageItems()[1].name).toBe('Restaurado');
  });

  it('fails an archive with the code kept (409 already archived)', async () => {
    const result = firstValueFrom(store.archiveProject(ID_A)).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/projects/${ID_A}/archive`)
      .flush(
        { error: { code: 'project.already_archived', message: 'Ya está archivado' } },
        { status: 409, statusText: 'Conflict' },
      );

    expect(await result).toMatchObject({ status: 409, code: 'project.already_archived' });
    expect(store.error()?.code).toBe('project.already_archived');
  });

  it('deletes with the confirmation and drops the project from the page and the total', async () => {
    loadPage();

    const result = firstValueFrom(store.deleteProject(ID_A, 'Proyecto AAA'));
    const request = backend.expectOne(`${BASE}/projects/${ID_A}`);
    expect(request.request.method).toBe('DELETE');
    expect(request.request.body).toEqual({ confirmation: 'Proyecto AAA' });
    request.flush(null);
    await result;

    expect(store.pageItems().map((item) => item.key)).toEqual(['BBB']);
    expect(store.pageMeta()?.total).toBe(4);
  });

  it('fails a delete with project.confirmation_mismatch and keeps the page', async () => {
    loadPage();

    const result = firstValueFrom(store.deleteProject(ID_A, 'otro')).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/projects/${ID_A}`)
      .flush(
        { error: { code: 'project.confirmation_mismatch', message: 'No coincide' } },
        { status: 422, statusText: 'Unprocessable Entity' },
      );

    expect(await result).toMatchObject({ code: 'project.confirmation_mismatch' });
    expect(store.pageItems()).toHaveLength(2);
  });

  it('reset clears the page', () => {
    loadPage();

    store.reset();

    expect(store.pageItems()).toEqual([]);
    expect(store.pageMeta()).toBeNull();
  });
});
