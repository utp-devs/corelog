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
import { TeamStore } from './team.store';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';

let activeOrganization: string | null = ORG_ID;

function teamDto(
  id: string,
  name: string,
  extra: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id,
    organization_id: ORG_ID,
    name,
    description: '',
    avatar_color: null,
    created_by: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
    ...extra,
  };
}

function membershipDto(teamId: string, userId: string, role: string): Record<string, unknown> {
  return { team_id: teamId, user_id: userId, role, created_at: '2026-11-01T10:00:00Z' };
}

function flushApiError(request: TestRequest, status: number, code: string, message: string): void {
  request.flush({ error: { code, message } }, { status, statusText: 'Error' });
}

describe('TeamStore', () => {
  let store: TeamStore;
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
    store = TestBed.inject(TeamStore);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  function loadTeams(teams: Record<string, unknown>[]): void {
    store.fetchTeams();
    backend.expectOne(`${BASE}/teams?organization_id=${ORG_ID}`).flush({ data: teams });
  }

  function loadMembers(
    teamId: string,
    memberships: Record<string, unknown>[],
    people: Record<string, unknown>[] = [],
  ): void {
    store.fetchMembers(teamId);
    backend.expectOne(`${BASE}/teams/${teamId}/members`).flush({ data: memberships });
    backend.expectOne(`${BASE}/organizations/${ORG_ID}/members`).flush({ data: people });
  }

  it('loads teams and keeps an error with its code when the list fails', () => {
    loadTeams([teamDto('t1', 'Alfa')]);
    expect(store.teams().map((t) => t.id)).toEqual(['t1']);
    expect(store.status()).toBe('idle');

    store.fetchTeams();
    flushApiError(
      backend.expectOne(`${BASE}/teams?organization_id=${ORG_ID}`),
      404,
      'team.organization_not_found',
      'no existe',
    );
    expect(store.status()).toBe('error');
    expect(store.error()?.code).toBe('team.organization_not_found');
  });

  it('creates a team using the current organization id and keeps the list sorted', async () => {
    loadTeams([teamDto('t1', 'Alfa')]);

    const created = firstValueFrom(store.createTeam({ name: 'Beta' }));
    const request = backend.expectOne(`${BASE}/teams`);
    expect(request.request.body).toEqual({ organization_id: ORG_ID, name: 'Beta' });
    request.flush({ data: teamDto('t2', 'Beta') });
    await created;

    expect(store.teams().map((t) => t.name)).toEqual(['Alfa', 'Beta']);
  });

  it('refuses to create a team without an active organization', async () => {
    activeOrganization = null;

    const error = await firstValueFrom(store.createTeam({ name: 'Gamma' })).catch(
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ code: 'team.organization_required' });
    backend.expectNone(`${BASE}/teams`);
  });

  it('surfaces a duplicate name code from create', async () => {
    const result = firstValueFrom(store.createTeam({ name: 'Beta' })).catch((e: unknown) => e);
    flushApiError(backend.expectOne(`${BASE}/teams`), 409, 'team.name_taken', 'ya existe');

    const error = await result;

    expect(error).toMatchObject({ code: 'team.name_taken' });
    expect(store.error()?.code).toBe('team.name_taken');
  });

  it('keeps memberCount and myRole when the update response does not carry them', async () => {
    loadTeams([teamDto('t1', 'Alfa', { member_count: 3, my_role: 'maintainer' })]);

    const updated = firstValueFrom(store.updateTeam('t1', { name: 'Alfa renombrado' }));
    backend.expectOne(`${BASE}/teams/t1`).flush({ data: teamDto('t1', 'Alfa renombrado') });
    await updated;

    expect(store.teams()[0]).toMatchObject({
      name: 'Alfa renombrado',
      memberCount: 3,
      myRole: 'maintainer',
    });
  });

  it('replaces the updated team in the list and in the detail view', async () => {
    loadTeams([teamDto('t1', 'Alfa')]);
    store.fetchTeam('t1');
    backend.expectOne(`${BASE}/teams/t1`).flush({ data: teamDto('t1', 'Alfa') });

    const updated = firstValueFrom(store.updateTeam('t1', { name: 'Alfa renombrado' }));
    const request = backend.expectOne(`${BASE}/teams/t1`);
    expect(request.request.body).toEqual({ name: 'Alfa renombrado' });
    request.flush({ data: teamDto('t1', 'Alfa renombrado') });
    await updated;

    expect(store.teams()[0].name).toBe('Alfa renombrado');
    expect(store.currentTeam()?.name).toBe('Alfa renombrado');
  });

  it('removes a team from the list and clears the detail view when it is the open team', async () => {
    loadTeams([teamDto('t1', 'Alfa'), teamDto('t2', 'Beta')]);
    store.fetchTeam('t1');
    backend.expectOne(`${BASE}/teams/t1`).flush({ data: teamDto('t1', 'Alfa') });

    const removed = firstValueFrom(store.removeTeam('t1'));
    const request = backend.expectOne(`${BASE}/teams/t1`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null);
    await removed;

    expect(store.teams().map((t) => t.id)).toEqual(['t2']);
    expect(store.currentTeam()).toBeNull();
  });

  it('joins memberships with names from the organization directory', () => {
    loadMembers(
      't1',
      [membershipDto('t1', 'u1', 'maintainer'), membershipDto('t1', 'u2', 'member')],
      [{ user_id: 'u1', name: 'Ana', email: 'ana@example.com', role: 'member', joined_at: '' }],
    );

    const members = store.members();
    expect(members[0]).toMatchObject({
      userId: 'u1',
      role: 'maintainer',
      name: 'Ana',
      email: 'ana@example.com',
    });
    expect(members[1]).toMatchObject({
      userId: 'u2',
      name: 'Miembro de la organización',
      email: '',
    });
    expect(store.membersStatus()).toBe('idle');
  });

  it('still lists members when the organization directory is not readable (guests)', () => {
    store.fetchMembers('t1');
    backend
      .expectOne(`${BASE}/teams/t1/members`)
      .flush({ data: [membershipDto('t1', 'u1', 'member')] });
    flushApiError(
      backend.expectOne(`${BASE}/organizations/${ORG_ID}/members`),
      403,
      'org.forbidden',
      'no',
    );

    expect(store.members()).toHaveLength(1);
    expect(store.members()[0].name).toBe('Miembro de la organización');
    expect(store.membersStatus()).toBe('idle');
  });

  it('keeps the membership error apart from the general error', () => {
    store.fetchMembers('t1');
    flushApiError(backend.expectOne(`${BASE}/teams/t1/members`), 403, 'team.forbidden', 'no');

    expect(store.membersStatus()).toBe('error');
    expect(store.membersError()?.code).toBe('team.forbidden');
    expect(store.error()).toBeNull();
  });

  it('adds a member with a role and refreshes the list', async () => {
    const added = firstValueFrom(store.addMember('t1', 'u3', 'maintainer'));
    const request = backend.expectOne(`${BASE}/teams/t1/members`);
    expect(request.request.body).toEqual({ user_id: 'u3', role: 'maintainer' });
    request.flush({ data: membershipDto('t1', 'u3', 'maintainer') });

    backend.expectOne(`${BASE}/teams/t1/members`).flush({ data: [] });
    backend.expectOne(`${BASE}/organizations/${ORG_ID}/members`).flush({ data: [] });
    await added;
  });

  it('passes team.user_not_in_org from addMember through to the caller', async () => {
    const result = firstValueFrom(store.addMember('t1', 'u9')).catch((e: unknown) => e);
    flushApiError(
      backend.expectOne(`${BASE}/teams/t1/members`),
      422,
      'team.user_not_in_org',
      'no pertenece',
    );

    expect(await result).toMatchObject({ code: 'team.user_not_in_org' });
  });

  it('changes a member role in place', async () => {
    loadMembers(
      't1',
      [membershipDto('t1', 'u1', 'member')],
      [{ user_id: 'u1', name: 'Ana', email: '' }],
    );

    const changed = firstValueFrom(store.updateMemberRole('t1', 'u1', 'maintainer'));
    backend
      .expectOne(`${BASE}/teams/t1/members/u1`)
      .flush({ data: membershipDto('t1', 'u1', 'maintainer') });
    await changed;

    expect(store.members()[0].role).toBe('maintainer');
  });

  it('removes a member from the list', async () => {
    loadMembers(
      't1',
      [membershipDto('t1', 'u1', 'member')],
      [{ user_id: 'u1', name: 'Ana', email: '' }],
    );

    const removed = firstValueFrom(store.removeMember('t1', 'u1'));
    const request = backend.expectOne(`${BASE}/teams/t1/members/u1`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null);
    await removed;

    expect(store.members()).toEqual([]);
  });

  it('resets every team state on reset', () => {
    loadTeams([teamDto('t1', 'Alfa')]);
    store.reset();

    expect(store.teams()).toEqual([]);
    expect(store.status()).toBe('idle');
    expect(store.error()).toBeNull();
  });
});
