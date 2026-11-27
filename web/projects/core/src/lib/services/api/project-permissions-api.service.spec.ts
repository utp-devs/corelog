import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import { ProjectPermissionsApiService } from './project-permissions-api.service';

const BASE = '/api/v1/projects/CORE';

const schemeDto = {
  project_id: 'p1',
  is_default: false,
  grants: [{ permission: 'create_issue', holders: ['lead', 'contributor'] }],
};

describe('ProjectPermissionsApiService', () => {
  let api: ProjectPermissionsApiService;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(ProjectPermissionsApiService);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('reads the scheme (no data envelope) and maps it to camelCase', async () => {
    const result = firstValueFrom(api.getScheme('CORE'));
    backend.expectOne(`${BASE}/permissions`).flush(schemeDto);

    expect(await result).toEqual({
      projectId: 'p1',
      isDefault: false,
      grants: [{ permission: 'create_issue', holders: ['lead', 'contributor'] }],
    });
  });

  it('replaces the whole scheme with the given grants', async () => {
    const result = firstValueFrom(
      api.putScheme('CORE', [{ permission: 'create_issue', holders: ['lead'] }]),
    );
    const request = backend.expectOne(`${BASE}/permissions`);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({
      grants: [{ permission: 'create_issue', holders: ['lead'] }],
    });
    request.flush(schemeDto);

    expect((await result).isDefault).toBe(false);
  });

  it('resets the scheme to the default one', async () => {
    const result = firstValueFrom(api.resetScheme('CORE'));
    const request = backend.expectOne(`${BASE}/permissions/reset`);
    expect(request.request.method).toBe('POST');
    request.flush({ ...schemeDto, is_default: true });

    expect((await result).isDefault).toBe(true);
  });

  it('reads the effective permissions of the caller', async () => {
    const result = firstValueFrom(api.me('CORE'));
    const request = backend.expectOne(`${BASE}/permissions/me`);
    expect(request.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(false);
    request.flush({ role: 'contributor', permissions: ['browse_project', 'create_issue'] });

    expect(await result).toEqual({
      role: 'contributor',
      permissions: ['browse_project', 'create_issue'],
    });
  });

  it('can read the effective permissions without a toast on failure', () => {
    void firstValueFrom(api.me('CORE', { silent: true })).catch(() => undefined);
    const request = backend.expectOne(`${BASE}/permissions/me`);
    expect(request.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    request.flush({ role: 'none', permissions: [] });
  });

  it('lists, sets and deletes team roles', async () => {
    const roleDto = {
      project_id: 'p1',
      team_id: 't1',
      team_name: 'Plataforma',
      role: 'contributor',
      created_at: '2026-11-01T10:00:00Z',
    };

    const listed = firstValueFrom(api.listTeamRoles('CORE'));
    backend.expectOne(`${BASE}/team-roles`).flush({ data: [roleDto] });
    expect(await listed).toEqual([
      {
        projectId: 'p1',
        teamId: 't1',
        teamName: 'Plataforma',
        role: 'contributor',
        createdAt: '2026-11-01T10:00:00Z',
      },
    ]);

    const saved = firstValueFrom(api.putTeamRole('CORE', 't1', 'viewer'));
    const put = backend.expectOne(`${BASE}/team-roles`);
    expect(put.request.method).toBe('PUT');
    expect(put.request.body).toEqual({ team_id: 't1', role: 'viewer' });
    put.flush({ data: { ...roleDto, role: 'viewer' } });
    expect((await saved).role).toBe('viewer');

    const removed = firstValueFrom(api.deleteTeamRole('CORE', 't1'));
    const del = backend.expectOne(`${BASE}/team-roles/t1`);
    expect(del.request.method).toBe('DELETE');
    del.flush(null);
    expect(await removed).toBeUndefined();
  });

  it('lets an invalid scheme error through', async () => {
    const result = firstValueFrom(api.putScheme('CORE', [])).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/permissions`)
      .flush(
        { error: { code: 'authorization.invalid_scheme', message: 'Esquema no valido' } },
        { status: 422, statusText: 'Unprocessable Entity' },
      );

    expect(await result).toMatchObject({ status: 422 });
  });
});
