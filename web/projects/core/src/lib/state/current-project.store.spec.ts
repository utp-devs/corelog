import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { CurrentProjectStore } from './current-project.store';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';
const PROJECT_UUID = '0f0e0d0c-0b0a-4908-8706-050403020100';

function projectDto(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: PROJECT_UUID,
    organization_id: ORG_ID,
    key: 'CORE',
    name: 'CoreLog',
    description: '',
    type: 'scrum',
    template: 'scrum',
    lead_id: 'u1',
    visibility: 'open',
    archived_at: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
    ...overrides,
  };
}

describe('CurrentProjectStore', () => {
  let store: CurrentProjectStore;
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
    store = TestBed.inject(CurrentProjectStore);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  function openByKey(
    key = 'CORE',
    permissions = { role: 'contributor', permissions: ['browse_project'] },
  ): void {
    store.open(key);
    backend
      .expectOne(`${BASE}/projects/${key}?organization_id=${ORG_ID}`)
      .flush({ data: projectDto() });
    backend.expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`).flush(permissions);
  }

  it('starts empty', () => {
    expect(store.project()).toBeUndefined();
    expect(store.projectId()).toBeNull();
    expect(store.role()).toBeNull();
    expect(store.can('browse_project')).toBe(false);
    expect(store.navItems()).toEqual([]);
  });

  it('resolves a key into the project and then reads its permissions by UUID', () => {
    openByKey('CORE', { role: 'lead', permissions: ['browse_project', 'edit_project'] });

    expect(store.identifier()).toBe('CORE');
    expect(store.project()?.id).toBe(PROJECT_UUID);
    expect(store.projectId()).toBe(PROJECT_UUID);
    expect(store.key()).toBe('CORE');
    expect(store.role()).toBe('lead');
    expect(store.can('edit_project')).toBe(true);
    expect(store.can('delete_project')).toBe(false);
    expect(store.error()).toBeNull();
    expect(store.isArchived()).toBe(false);
  });

  it('resolves a UUID without sending the organization', () => {
    store.open(PROJECT_UUID);
    backend.expectOne(`${BASE}/projects/${PROJECT_UUID}`).flush({ data: projectDto() });
    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
      .flush({ role: 'viewer', permissions: ['browse_project'] });

    expect(store.project()?.key).toBe('CORE');
    expect(store.role()).toBe('viewer');
  });

  it('keeps the 404 failure, reads no permissions and exposes no project', () => {
    store.open('NOPE');
    backend
      .expectOne(`${BASE}/projects/NOPE?organization_id=${ORG_ID}`)
      .flush(
        { error: { code: 'project.not_found', message: 'No existe' } },
        { status: 404, statusText: 'Not Found' },
      );

    expect(store.project()).toBeUndefined();
    expect(store.error()).toMatchObject({ status: 404, code: 'project.not_found' });
    expect(store.loading()).toBe(false);
    expect(store.navItems()).toEqual([]);
  });

  it('keeps the 403 failure', () => {
    store.open('CORE');
    backend
      .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
      .flush({ error: { code: 'forbidden' } }, { status: 403, statusText: 'Forbidden' });

    expect(store.error()).toMatchObject({ status: 403 });
  });

  it('does not fail the project when only the permissions read fails', () => {
    store.open('CORE');
    backend
      .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
      .flush({ data: projectDto() });
    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
      .flush({ error: { code: 'boom' } }, { status: 500, statusText: 'Server Error' });

    expect(store.project()?.key).toBe('CORE');
    expect(store.error()).toBeNull();
    expect(store.role()).toBeNull();
  });

  it('clears the previous project when another identifier is opened', () => {
    openByKey('CORE');
    store.open('OTHER');

    expect(store.project()).toBeUndefined();
    expect(store.role()).toBeNull();
    backend.expectOne(`${BASE}/projects/OTHER?organization_id=${ORG_ID}`).flush({
      data: projectDto({ id: 'other-id', key: 'OTHER' }),
    });
    backend
      .expectOne(`${BASE}/projects/other-id/permissions/me`)
      .flush({ role: 'viewer', permissions: [] });
    expect(store.key()).toBe('OTHER');
  });

  it('reload reads the project again by the same identifier and keeps the value meanwhile', () => {
    openByKey('CORE');

    store.reload();

    expect(store.project()?.key).toBe('CORE');
    backend
      .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
      .flush({ data: projectDto({ name: 'Renombrado' }) });
    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
      .flush({ role: 'lead', permissions: ['browse_project'] });
    expect(store.project()?.name).toBe('Renombrado');
  });

  it('reload without an opened project sends nothing', () => {
    store.reload();
  });

  it('setProject replaces the project only when it is the one in view', () => {
    openByKey('CORE');
    const archived = { ...store.project()!, archivedAt: '2026-11-02T10:00:00Z' };

    store.setProject(archived);
    expect(store.isArchived()).toBe(true);

    store.setProject({ ...archived, id: 'another', name: 'Otro' });
    expect(store.project()?.name).toBe('CoreLog');
  });

  it('reloadPermissions reads them again', () => {
    openByKey('CORE');

    store.reloadPermissions();

    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
      .flush({ role: 'lead', permissions: ['browse_project', 'manage_members'] });
    expect(store.can('manage_members')).toBe(true);
  });

  it('reset clears everything', () => {
    openByKey('CORE');

    store.reset();

    expect(store.project()).toBeUndefined();
    expect(store.permissions()).toBeUndefined();
    expect(store.identifier()).toBeNull();
  });

  describe('navItems', () => {
    it('lists every entry of a scrum project while the permissions are loading', () => {
      store.open('CORE');
      backend
        .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
        .flush({ data: projectDto() });
      backend.expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`);

      expect(store.navItems().map((item) => item.key)).toEqual([
        'board',
        'backlog',
        'sprints',
        'calendar',
        'settings',
      ]);
    });

    it('drops backlog and sprints for kanban projects', () => {
      store.open('CORE');
      backend
        .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
        .flush({ data: projectDto({ type: 'kanban', template: 'task_tracking' }) });
      backend
        .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
        .flush({ role: 'viewer', permissions: ['browse_project'] });

      expect(store.navItems().map((item) => item.key)).toEqual(['board', 'calendar']);
      expect(store.navItems()[0].path).toBe('/projects/CORE/board');
    });

    it('lists nothing when the caller cannot browse the project', () => {
      openByKey('CORE', { role: 'none', permissions: [] });

      expect(store.navItems()).toEqual([]);
    });
  });
});
