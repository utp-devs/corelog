import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { ORGANIZATION_ID_PROVIDER } from '../../utils/org-id-provider';
import { ProjectApiService } from './project-api.service';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';

const projectDto = {
  id: 'project-1',
  organization_id: ORG_ID,
  key: 'CORE',
  name: 'CoreLog',
  description: '',
  type: 'kanban' as const,
  lead_id: 'user-1',
  visibility: 'open' as const,
  archived_at: null,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T10:00:00Z',
};

describe('ProjectApiService', () => {
  let api: ProjectApiService;
  let backend: HttpTestingController;
  let activeOrganization: string | null;

  beforeEach(() => {
    activeOrganization = ORG_ID;
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: ORGANIZATION_ID_PROVIDER, useValue: () => activeOrganization },
      ],
    });
    api = TestBed.inject(ProjectApiService);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('lists projects with organization_id and maps organizationId, lead and open visibility', async () => {
    const result = firstValueFrom(api.list());
    backend.expectOne(`${BASE}/projects?organization_id=${ORG_ID}`).flush({ data: [projectDto] });

    const [project] = await result;
    expect(project).toEqual({
      id: 'project-1',
      organizationId: ORG_ID,
      key: 'CORE',
      name: 'CoreLog',
      description: '',
      type: 'kanban',
      leadId: 'user-1',
      visibility: 'open',
      archivedAt: null,
      createdAt: '2026-11-01T10:00:00Z',
      updatedAt: '2026-11-01T10:00:00Z',
    });
  });

  it('fails with org.no_current and sends no request when no organization is active', async () => {
    activeOrganization = null;

    const error = await firstValueFrom(api.list()).catch((e: unknown) => e);

    expect(error).toMatchObject({ status: 0, code: 'org.no_current' });
  });

  it('gets one project by id', async () => {
    const result = firstValueFrom(api.get('project-1'));
    backend.expectOne(`${BASE}/projects/project-1`).flush({ data: projectDto });

    expect((await result).key).toBe('CORE');
  });

  it('creates projects with organization_id and without lead_id', async () => {
    const result = firstValueFrom(
      api.create({
        organizationId: ORG_ID,
        key: 'CORE',
        name: 'CoreLog',
        description: '',
        type: 'kanban',
        visibility: 'open',
      }),
    );
    const request = backend.expectOne(`${BASE}/projects`);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({
      organization_id: ORG_ID,
      key: 'CORE',
      name: 'CoreLog',
      description: '',
      type: 'kanban',
      visibility: 'open',
    });
    expect(request.request.body).not.toHaveProperty('lead_id');
    request.flush({ data: projectDto });

    await result;
  });

  it('omits visibility on create when the organization default applies', async () => {
    const result = firstValueFrom(
      api.create({
        organizationId: ORG_ID,
        key: 'CORE',
        name: 'CoreLog',
        description: '',
        type: 'scrum',
      }),
    );
    const request = backend.expectOne(`${BASE}/projects`);
    expect(request.request.body).not.toHaveProperty('visibility');
    request.flush({ data: projectDto });

    await result;
  });

  it('sends the lead and visibility on update', async () => {
    const result = firstValueFrom(
      api.update('project-1', {
        name: 'CoreLog',
        description: '',
        type: 'scrum',
        leadId: 'user-2',
        visibility: 'private',
      }),
    );
    const request = backend.expectOne(`${BASE}/projects/project-1`);
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({
      name: 'CoreLog',
      description: '',
      type: 'scrum',
      lead_id: 'user-2',
      visibility: 'private',
    });
    request.flush({ data: projectDto });

    await result;
  });

  it('deletes a project', async () => {
    const result = firstValueFrom(api.remove('project-1'));
    const request = backend.expectOne(`${BASE}/projects/project-1`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null);

    expect(await result).toBeUndefined();
  });

  it('lists members, adds one with its role, changes it and removes it', async () => {
    const memberDto = {
      project_id: 'project-1',
      user_id: 'user-2',
      role: 'viewer',
      created_at: '',
    };

    const listed = firstValueFrom(api.listMembers('project-1'));
    backend.expectOne(`${BASE}/projects/project-1/members`).flush({ data: [memberDto] });
    expect((await listed)[0]).toEqual({
      projectId: 'project-1',
      userId: 'user-2',
      role: 'viewer',
      createdAt: '',
    });

    const added = firstValueFrom(api.addMember('project-1', 'user-2', 'viewer'));
    const addRequest = backend.expectOne(`${BASE}/projects/project-1/members`);
    expect(addRequest.request.body).toEqual({ user_id: 'user-2', role: 'viewer' });
    addRequest.flush({ data: memberDto });
    expect((await added).role).toBe('viewer');

    const updated = firstValueFrom(api.updateMember('project-1', 'user-2', 'contributor'));
    const patchRequest = backend.expectOne(`${BASE}/projects/project-1/members/user-2`);
    expect(patchRequest.request.method).toBe('PATCH');
    expect(patchRequest.request.body).toEqual({ role: 'contributor' });
    patchRequest.flush({ data: { ...memberDto, role: 'contributor' } });
    expect((await updated).role).toBe('contributor');

    const removed = firstValueFrom(api.removeMember('project-1', 'user-2'));
    const deleteRequest = backend.expectOne(`${BASE}/projects/project-1/members/user-2`);
    expect(deleteRequest.request.method).toBe('DELETE');
    deleteRequest.flush(null);
    expect(await removed).toBeUndefined();
  });

  it('lets project.user_not_in_org pass through from addMember', async () => {
    const result = firstValueFrom(api.addMember('project-1', 'user-9', 'viewer')).catch(
      (e: unknown) => e,
    );
    backend
      .expectOne(`${BASE}/projects/project-1/members`)
      .flush(
        { error: { code: 'project.user_not_in_org', message: 'fuera de la organización' } },
        { status: 422, statusText: 'Unprocessable Entity' },
      );

    const error = await result;
    expect(error).toBeInstanceOf(HttpErrorResponse);
    expect((error as HttpErrorResponse).error.error.code).toBe('project.user_not_in_org');
  });

  it('lists, creates and deletes labels', async () => {
    const labelDto = {
      id: 'label-1',
      project_id: 'project-1',
      name: 'bug',
      color: '#FF0000',
      created_at: '',
      updated_at: '',
    };

    const listed = firstValueFrom(api.listLabels('project-1'));
    backend.expectOne(`${BASE}/projects/project-1/labels`).flush({ data: [labelDto] });
    expect((await listed)[0]).toMatchObject({
      id: 'label-1',
      projectId: 'project-1',
      color: '#FF0000',
    });

    const created = firstValueFrom(api.createLabel('project-1', 'bug', '#FF0000'));
    const createRequest = backend.expectOne(`${BASE}/projects/project-1/labels`);
    expect(createRequest.request.body).toEqual({ name: 'bug', color: '#FF0000' });
    createRequest.flush({ data: labelDto });
    expect((await created).name).toBe('bug');

    const deleted = firstValueFrom(api.deleteLabel('project-1', 'label-1'));
    const deleteRequest = backend.expectOne(`${BASE}/projects/project-1/labels/label-1`);
    expect(deleteRequest.request.method).toBe('DELETE');
    deleteRequest.flush(null);
    expect(await deleted).toBeUndefined();
  });

  it('lists, creates and deletes components', async () => {
    const componentDto = {
      id: 'comp-1',
      project_id: 'project-1',
      name: 'API',
      description: 'Backend',
      created_at: '',
      updated_at: '',
    };

    const listed = firstValueFrom(api.listComponents('project-1'));
    backend.expectOne(`${BASE}/projects/project-1/components`).flush({ data: [componentDto] });
    expect((await listed)[0]).toMatchObject({ id: 'comp-1', description: 'Backend' });

    const created = firstValueFrom(api.createComponent('project-1', 'API', 'Backend'));
    const createRequest = backend.expectOne(`${BASE}/projects/project-1/components`);
    expect(createRequest.request.body).toEqual({ name: 'API', description: 'Backend' });
    createRequest.flush({ data: componentDto });
    expect((await created).projectId).toBe('project-1');

    const deleted = firstValueFrom(api.deleteComponent('project-1', 'comp-1'));
    const deleteRequest = backend.expectOne(`${BASE}/projects/project-1/components/comp-1`);
    expect(deleteRequest.request.method).toBe('DELETE');
    deleteRequest.flush(null);
    expect(await deleted).toBeUndefined();
  });
});
