import { provideHttpClient } from '@angular/common/http';
import {
  HttpTestingController,
  TestRequest,
  provideHttpClientTesting,
} from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { ProjectStore } from './project.store';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';

let activeOrganization: string | null = ORG_ID;

function projectDto(id: string, name: string): Record<string, unknown> {
  return {
    id,
    organization_id: ORG_ID,
    key: id.toUpperCase(),
    name,
    description: '',
    type: 'kanban',
    lead_id: 'u1',
    visibility: 'open',
    archived_at: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
  };
}

function flushApiError(request: TestRequest, status: number, code: string, message: string): void {
  request.flush({ error: { code, message } }, { status, statusText: 'Error' });
}

describe('ProjectStore', () => {
  let store: ProjectStore;
  let backend: HttpTestingController;

  beforeEach(() => {
    activeOrganization = ORG_ID;
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideCorelogTesting(),
        { provide: ORGANIZATION_ID_PROVIDER, useValue: () => activeOrganization },
      ],
    });
    store = TestBed.inject(ProjectStore);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('reads one project and exposes it only for the requested id', () => {
    store.loadOne('project-1');
    backend
      .expectOne(`${BASE}/projects/project-1`)
      .flush({ data: projectDto('project-1', 'Core') });

    expect(store.project()?.name).toBe('Core');
    expect(store.projectError()).toBeNull();

    store.loadOne('project-2');
    expect(store.project()).toBeNull();
    backend
      .expectOne(`${BASE}/projects/project-2`)
      .flush({ data: projectDto('project-2', 'Otro') });
    expect(store.project()?.name).toBe('Otro');
  });

  it('keeps the access failure of a single project read', () => {
    store.loadOne('project-3');
    flushApiError(
      backend.expectOne(`${BASE}/projects/project-3`),
      403,
      'project.forbidden',
      'Sin acceso',
    );

    expect(store.project()).toBeNull();
    expect(store.projectError()?.status).toBe(403);
    expect(store.projectLoading()).toBe(false);
  });

  it('creates a project in the current organization without sending a lead', async () => {
    const created = firstValueFrom(
      store.createProject({
        key: 'CORE',
        name: 'Core',
        description: '',
        type: 'kanban',
        visibility: 'open',
      }),
    );
    const request = backend.expectOne(`${BASE}/projects`);
    const body = request.request.body as Record<string, unknown>;
    expect(body).toEqual({
      organization_id: ORG_ID,
      key: 'CORE',
      name: 'Core',
      description: '',
      type: 'kanban',
      visibility: 'open',
    });
    expect(body).not.toHaveProperty('lead_id');
    request.flush({ data: projectDto('core', 'Core') });
    await created;

    expect(store.projects()).toHaveLength(1);
  });

  it('refuses to create a project without an active organization', async () => {
    activeOrganization = null;

    const error = await firstValueFrom(
      store.createProject({ key: 'CORE', name: 'Core', description: '', type: 'kanban' }),
    ).catch((e: unknown) => e);

    expect(error).toMatchObject({ code: 'project.organization_required' });
    backend.expectNone(`${BASE}/projects`);
  });

  it('keeps the error code when listing fails', () => {
    store.fetchProjects();
    backend
      .expectOne(`${BASE}/projects?organization_id=${ORG_ID}`)
      .error(new ProgressEvent('error'), { status: 0, statusText: 'Unknown Error' });

    expect(store.status()).toBe('error');
    expect(store.error()).toMatchObject({ status: 0, code: 'network.unreachable' });
  });

  it('surfaces project.user_not_in_org when adding a member', async () => {
    const result = firstValueFrom(store.addMember('p1', 'u9', 'viewer')).catch((e: unknown) => e);
    flushApiError(
      backend.expectOne(`${BASE}/projects/p1/members`),
      422,
      'project.user_not_in_org',
      'fuera',
    );

    expect(await result).toMatchObject({ code: 'project.user_not_in_org' });
    expect(store.error()?.code).toBe('project.user_not_in_org');
  });

  it('surfaces project.user_not_found when adding a member', async () => {
    const result = firstValueFrom(store.addMember('p1', 'u9', 'viewer')).catch((e: unknown) => e);
    flushApiError(
      backend.expectOne(`${BASE}/projects/p1/members`),
      404,
      'project.user_not_found',
      'no existe',
    );

    expect(await result).toMatchObject({ code: 'project.user_not_found' });
  });

  it('adds a member to the list on success', async () => {
    const added = firstValueFrom(store.addMember('p1', 'u2', 'contributor'));
    backend.expectOne(`${BASE}/projects/p1/members`).flush({
      data: { project_id: 'p1', user_id: 'u2', role: 'contributor', created_at: '' },
    });
    await added;

    expect(store.members().map((m) => m.userId)).toEqual(['u2']);
    expect(store.error()).toBeNull();
  });

  it('updates a project in the list', async () => {
    store.fetchProjects();
    backend
      .expectOne(`${BASE}/projects?organization_id=${ORG_ID}`)
      .flush({ data: [projectDto('p1', 'Alfa')] });
    expect(store.status()).toBe('idle');

    const updated = firstValueFrom(
      store.updateProject('p1', {
        name: 'Alfa 2',
        description: '',
        type: 'scrum',
        leadId: 'u1',
        visibility: 'private',
      }),
    );
    backend.expectOne(`${BASE}/projects/p1`).flush({ data: projectDto('p1', 'Alfa 2') });
    await updated;

    expect(store.projects()[0].name).toBe('Alfa 2');
  });

  it('loads and mutates the members, labels and components of a project', async () => {
    store.fetchMembers('p1');
    backend.expectOne(`${BASE}/projects/p1/members`).flush({
      data: [{ project_id: 'p1', user_id: 'u1', role: 'lead', created_at: '' }],
    });
    expect(store.members()[0].role).toBe('lead');

    const changed = firstValueFrom(store.updateMember('p1', 'u1', 'contributor'));
    backend.expectOne(`${BASE}/projects/p1/members/u1`).flush({
      data: { project_id: 'p1', user_id: 'u1', role: 'contributor', created_at: '' },
    });
    await changed;
    expect(store.members()[0].role).toBe('contributor');

    const removed = firstValueFrom(store.removeMember('p1', 'u1'));
    backend.expectOne(`${BASE}/projects/p1/members/u1`).flush(null);
    await removed;
    expect(store.members()).toEqual([]);

    store.fetchLabels('p1');
    backend.expectOne(`${BASE}/projects/p1/labels`).flush({
      data: [
        {
          id: 'l1',
          project_id: 'p1',
          name: 'bug',
          color: '#FF0000',
          created_at: '',
          updated_at: '',
        },
      ],
    });
    expect(store.labels().map((l) => l.id)).toEqual(['l1']);

    const labelCreated = firstValueFrom(store.createLabel('p1', 'deuda', '#00FF00'));
    backend.expectOne(`${BASE}/projects/p1/labels`).flush({
      data: {
        id: 'l2',
        project_id: 'p1',
        name: 'deuda',
        color: '#00FF00',
        created_at: '',
        updated_at: '',
      },
    });
    await labelCreated;
    expect(store.labels().map((l) => l.id)).toEqual(['l1', 'l2']);

    const labelDeleted = firstValueFrom(store.deleteLabel('p1', 'l1'));
    backend.expectOne(`${BASE}/projects/p1/labels/l1`).flush(null);
    await labelDeleted;
    expect(store.labels().map((l) => l.id)).toEqual(['l2']);

    store.fetchComponents('p1');
    backend.expectOne(`${BASE}/projects/p1/components`).flush({
      data: [
        {
          id: 'c1',
          project_id: 'p1',
          name: 'API',
          description: '',
          created_at: '',
          updated_at: '',
        },
      ],
    });
    expect(store.components()[0].name).toBe('API');

    const componentCreated = firstValueFrom(store.createComponent('p1', 'UI', 'Front'));
    backend.expectOne(`${BASE}/projects/p1/components`).flush({
      data: {
        id: 'c2',
        project_id: 'p1',
        name: 'UI',
        description: 'Front',
        created_at: '',
        updated_at: '',
      },
    });
    await componentCreated;
    expect(store.components().map((c) => c.id)).toEqual(['c1', 'c2']);

    const componentDeleted = firstValueFrom(store.deleteComponent('p1', 'c1'));
    backend.expectOne(`${BASE}/projects/p1/components/c1`).flush(null);
    await componentDeleted;
    expect(store.components().map((c) => c.id)).toEqual(['c2']);
  });

  it('resets every project state on reset', () => {
    store.fetchProjects();
    backend
      .expectOne(`${BASE}/projects?organization_id=${ORG_ID}`)
      .flush({ data: [projectDto('p1', 'Alfa')] });
    store.reset();

    expect(store.projects()).toEqual([]);
    expect(store.status()).toBe('idle');
  });
});
