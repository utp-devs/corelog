import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import { ORGANIZATION_ID_PROVIDER } from '../../utils/org-id-provider';
import { ProjectApiService, isProjectUuid } from './project-api.service';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';
const PROJECT_UUID = '0f0e0d0c-0b0a-4908-8706-050403020100';

const projectDto = {
  id: PROJECT_UUID,
  organization_id: ORG_ID,
  key: 'CORE',
  name: 'CoreLog',
  description: '',
  type: 'kanban' as const,
  lead_id: 'user-1',
  visibility: 'open' as const,
  template: 'bug_tracking' as const,
  color: 'blue',
  icon: 'rocket',
  default_assignee_policy: 'lead' as const,
  archived_at: null,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T10:00:00Z',
};

describe('ProjectApiService (self-service contract)', () => {
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

  it('tells UUIDs from keys', () => {
    expect(isProjectUuid(PROJECT_UUID)).toBe(true);
    expect(isProjectUuid('CORE')).toBe(false);
  });

  it('maps template, color, icon and the assignee policy', async () => {
    const result = firstValueFrom(api.get(PROJECT_UUID));
    backend.expectOne(`${BASE}/projects/${PROJECT_UUID}`).flush({ data: projectDto });

    expect(await result).toMatchObject({
      template: 'bug_tracking',
      color: 'blue',
      icon: 'rocket',
      defaultAssigneePolicy: 'lead',
    });
  });

  it('gets a project by key scoped to the active organization', async () => {
    const result = firstValueFrom(api.get('CORE'));
    backend
      .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
      .flush({ data: projectDto });

    expect((await result).key).toBe('CORE');
  });

  it('does not send the organization when the identifier is a UUID', async () => {
    const result = firstValueFrom(api.get(PROJECT_UUID));
    const request = backend.expectOne(`${BASE}/projects/${PROJECT_UUID}`);
    expect(request.request.params.keys()).toEqual([]);
    request.flush({ data: projectDto });
    await result;
  });

  it('keeps toasts on a project read unless the caller asks for a silent one', () => {
    void firstValueFrom(api.get('CORE', { silent: true })).catch(() => undefined);
    const silent = backend.expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`);
    expect(silent.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    silent.flush({ data: projectDto });

    void firstValueFrom(api.get('CORE')).catch(() => undefined);
    const loud = backend.expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`);
    expect(loud.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(false);
    loud.flush({ data: projectDto });
  });

  it('lists a page with every filter, sort and pagination parameter and maps the meta', async () => {
    const result = firstValueFrom(
      api.listPage({
        q: ' core ',
        template: 'scrum',
        type: 'scrum',
        leadId: 'user-1',
        starred: true,
        archived: 'only',
        sort: 'updated_at',
        desc: true,
        page: 2,
        pageSize: 10,
      }),
    );
    const request = backend.expectOne((req) => req.url === `${BASE}/projects`);
    const params = request.request.params;
    expect(params.get('organization_id')).toBe(ORG_ID);
    expect(params.get('q')).toBe('core');
    expect(params.get('template')).toBe('scrum');
    expect(params.get('type')).toBe('scrum');
    expect(params.get('lead_id')).toBe('user-1');
    expect(params.get('starred')).toBe('true');
    expect(params.get('archived')).toBe('only');
    expect(params.get('sort')).toBe('updated_at');
    expect(params.get('desc')).toBe('true');
    expect(params.get('page')).toBe('2');
    expect(params.get('page_size')).toBe('10');
    request.flush({ data: [projectDto], meta: { total: 31, page: 2, page_size: 10 } });

    const page = await result;
    expect(page.meta).toEqual({ total: 31, page: 2, pageSize: 10 });
    expect(page.items[0].key).toBe('CORE');
  });

  it('omits the filters that are not set', async () => {
    const result = firstValueFrom(api.listPage());
    const request = backend.expectOne((req) => req.url === `${BASE}/projects`);
    expect(request.request.params.keys()).toEqual(['organization_id']);
    request.flush({ data: [], meta: { total: 0, page: 1, page_size: 25 } });

    expect((await result).meta).toEqual({ total: 0, page: 1, pageSize: 25 });
  });

  it('fails with org.no_current when listing a page or checking a key without organization', async () => {
    activeOrganization = null;

    await expect(firstValueFrom(api.listPage())).rejects.toMatchObject({ code: 'org.no_current' });
    await expect(firstValueFrom(api.keyAvailable('CORE'))).rejects.toMatchObject({
      code: 'org.no_current',
    });
  });

  it('checks whether a key is available in the active organization', async () => {
    const result = firstValueFrom(api.keyAvailable('core'));
    backend
      .expectOne(`${BASE}/projects/key-available?organization_id=${ORG_ID}&key=core`)
      .flush({ data: { available: false, key: 'CORE' } });

    expect(await result).toEqual({ available: false, key: 'CORE' });
  });

  it('archives and restores a project', async () => {
    const archived = firstValueFrom(api.archive(PROJECT_UUID));
    const archive = backend.expectOne(`${BASE}/projects/${PROJECT_UUID}/archive`);
    expect(archive.request.method).toBe('POST');
    archive.flush({ data: { ...projectDto, archived_at: '2026-11-02T10:00:00Z' } });
    expect((await archived).archivedAt).toBe('2026-11-02T10:00:00Z');

    const restored = firstValueFrom(api.restore(PROJECT_UUID));
    const restore = backend.expectOne(`${BASE}/projects/${PROJECT_UUID}/restore`);
    expect(restore.request.method).toBe('POST');
    restore.flush({ data: projectDto });
    expect((await restored).archivedAt).toBeNull();
  });

  it('sends the new fields on create', async () => {
    const result = firstValueFrom(
      api.create({
        organizationId: ORG_ID,
        key: 'CORE',
        name: 'CoreLog',
        description: '',
        type: 'kanban',
        template: 'bug_tracking',
        color: 'blue',
        icon: 'rocket',
        defaultAssigneePolicy: 'lead',
        memberIds: ['user-2'],
        teamIds: ['team-1'],
      }),
    );
    const request = backend.expectOne(`${BASE}/projects`);
    expect(request.request.body).toMatchObject({
      template: 'bug_tracking',
      color: 'blue',
      icon: 'rocket',
      default_assignee_policy: 'lead',
      member_ids: ['user-2'],
      team_ids: ['team-1'],
    });
    request.flush({ data: projectDto });
    await result;
  });

  it('sends color, icon and policy on update only when given', async () => {
    const result = firstValueFrom(
      api.update(PROJECT_UUID, {
        name: 'CoreLog',
        description: '',
        type: 'kanban',
        leadId: 'user-1',
        visibility: 'open',
        color: 'red',
        defaultAssigneePolicy: 'unassigned',
      }),
    );
    const request = backend.expectOne(`${BASE}/projects/${PROJECT_UUID}`);
    expect(request.request.body).toMatchObject({
      color: 'red',
      default_assignee_policy: 'unassigned',
    });
    expect(request.request.body).not.toHaveProperty('icon');
    request.flush({ data: projectDto });
    await result;
  });

  it('lets API errors through (409 key taken, 422 confirmation mismatch)', async () => {
    const taken = firstValueFrom(
      api.create({
        organizationId: ORG_ID,
        key: 'CORE',
        name: 'CoreLog',
        description: '',
        type: 'kanban',
      }),
    ).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/projects`)
      .flush(
        { error: { code: 'project.key_taken', message: 'Clave en uso' } },
        { status: 409, statusText: 'Conflict' },
      );
    expect(await taken).toBeInstanceOf(HttpErrorResponse);

    const mismatch = firstValueFrom(api.remove(PROJECT_UUID, 'otro')).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}`)
      .flush(
        { error: { code: 'project.confirmation_mismatch' } },
        { status: 422, statusText: 'Unprocessable Entity' },
      );
    expect(await mismatch).toMatchObject({ status: 422 });
  });

  it('maps team grants on members', async () => {
    const listed = firstValueFrom(api.listMembers('project-1'));
    backend.expectOne(`${BASE}/projects/project-1/members`).flush({
      data: [
        {
          project_id: 'project-1',
          user_id: 'u1',
          role: 'lead',
          effective_role: 'lead',
          via_team: false,
          created_at: '2026-11-01T10:00:00Z',
        },
        {
          project_id: 'project-1',
          user_id: 'u2',
          role: 'contributor',
          effective_role: 'contributor',
          via_team: true,
          team_id: 't1',
          team_name: 'Plataforma',
        },
      ],
    });

    const [direct, viaTeam] = await listed;
    expect(direct).toMatchObject({ effectiveRole: 'lead', viaTeam: false });
    expect(viaTeam).toMatchObject({
      viaTeam: true,
      teamId: 't1',
      teamName: 'Plataforma',
      effectiveRole: 'contributor',
      createdAt: '',
    });
  });
});
