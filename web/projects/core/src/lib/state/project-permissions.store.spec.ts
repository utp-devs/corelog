import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { ProjectPermissionsStore } from './project-permissions.store';

const BASE = '/api/v1/projects/p1';

const schemeDto = {
  project_id: 'p1',
  is_default: true,
  grants: [{ permission: 'create_issue', holders: ['lead', 'contributor'] }],
};

const teamRoleDto = {
  project_id: 'p1',
  team_id: 't1',
  team_name: 'Plataforma',
  role: 'contributor',
  created_at: '2026-11-01T10:00:00Z',
};

describe('ProjectPermissionsStore', () => {
  let store: ProjectPermissionsStore;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideCorelogTesting()],
    });
    store = TestBed.inject(ProjectPermissionsStore);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('loads the scheme', () => {
    store.fetchScheme('p1');
    backend.expectOne(`${BASE}/permissions`).flush(schemeDto);

    expect(store.isDefault()).toBe(true);
    expect(store.grants()).toEqual([
      { permission: 'create_issue', holders: ['lead', 'contributor'] },
    ]);
  });

  it('keeps a scheme read failure', () => {
    store.fetchScheme('p1');
    backend
      .expectOne(`${BASE}/permissions`)
      .flush({ error: { code: 'project.not_found' } }, { status: 404, statusText: 'Not Found' });

    expect(store.schemeError()).toMatchObject({ status: 404 });
    expect(store.grants()).toEqual([]);
  });

  it('saves the scheme and keeps it as a custom one', async () => {
    const result = firstValueFrom(
      store.saveScheme('p1', [{ permission: 'create_issue', holders: ['lead'] }]),
    );
    backend.expectOne(`${BASE}/permissions`).flush({
      ...schemeDto,
      is_default: false,
      grants: [{ permission: 'create_issue', holders: ['lead'] }],
    });

    expect((await result).isDefault).toBe(false);
    expect(store.isDefault()).toBe(false);
    expect(store.grants()[0].holders).toEqual(['lead']);
  });

  it('fails a save with the normalized error and keeps its code', async () => {
    const result = firstValueFrom(store.saveScheme('p1', [])).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/permissions`)
      .flush(
        { error: { code: 'authorization.invalid_scheme', message: 'Esquema no valido' } },
        { status: 422, statusText: 'Unprocessable Entity' },
      );

    expect(await result).toMatchObject({ status: 422, code: 'authorization.invalid_scheme' });
  });

  it('resets the scheme to the default', async () => {
    const result = firstValueFrom(store.resetScheme('p1'));
    backend.expectOne(`${BASE}/permissions/reset`).flush(schemeDto);

    expect((await result).isDefault).toBe(true);
    expect(store.isDefault()).toBe(true);
  });

  it('loads, sets, replaces and removes team roles', async () => {
    store.fetchTeamRoles('p1');
    backend.expectOne(`${BASE}/team-roles`).flush({ data: [teamRoleDto] });
    expect(store.teamRoles().map((role) => role.teamName)).toEqual(['Plataforma']);

    const replaced = firstValueFrom(store.setTeamRole('p1', 't1', 'viewer'));
    backend.expectOne(`${BASE}/team-roles`).flush({ data: { ...teamRoleDto, role: 'viewer' } });
    await replaced;
    expect(store.teamRoles()).toHaveLength(1);
    expect(store.teamRoles()[0].role).toBe('viewer');

    const added = firstValueFrom(store.setTeamRole('p1', 't2', 'lead'));
    backend
      .expectOne(`${BASE}/team-roles`)
      .flush({ data: { ...teamRoleDto, team_id: 't2', team_name: 'Datos', role: 'lead' } });
    await added;
    expect(store.teamRoles()).toHaveLength(2);

    const removed = firstValueFrom(store.removeTeamRole('p1', 't1'));
    backend.expectOne(`${BASE}/team-roles/t1`).flush(null);
    await removed;
    expect(store.teamRoles().map((role) => role.teamId)).toEqual(['t2']);
  });

  it('fails a team role change with project.team_not_in_org', async () => {
    const result = firstValueFrom(store.setTeamRole('p1', 'tx', 'viewer')).catch((e: unknown) => e);
    backend.expectOne(`${BASE}/team-roles`).flush(
      {
        error: { code: 'project.team_not_in_org', message: 'El equipo no es de la organización' },
      },
      { status: 422, statusText: 'Unprocessable Entity' },
    );

    expect(await result).toMatchObject({ code: 'project.team_not_in_org' });
  });

  it('reset clears the scheme and the team roles', () => {
    store.fetchScheme('p1');
    backend.expectOne(`${BASE}/permissions`).flush(schemeDto);

    store.reset();

    expect(store.scheme()).toBeUndefined();
    expect(store.teamRoles()).toEqual([]);
  });
});
