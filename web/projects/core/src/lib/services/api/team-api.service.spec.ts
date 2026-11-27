import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { ORGANIZATION_ID_PROVIDER } from '../../utils/org-id-provider';
import { TeamApiService } from './team-api.service';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';

const teamDto = {
  id: 'team-1',
  organization_id: ORG_ID,
  name: 'Plataforma',
  description: 'Infra y datos',
  avatar_color: '#1F6FEB',
  created_by: 'user-1',
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-02T10:00:00Z',
};

const memberDto = {
  team_id: 'team-1',
  user_id: 'user-2',
  role: 'maintainer' as const,
  created_at: '2026-11-03T10:00:00Z',
};

describe('TeamApiService', () => {
  let api: TeamApiService;
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
    api = TestBed.inject(TeamApiService);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('lists teams of the active organization and maps the new fields', async () => {
    const result = firstValueFrom(api.list());
    const request = backend.expectOne(`${BASE}/teams?organization_id=${ORG_ID}`);
    expect(request.request.method).toBe('GET');
    request.flush({ data: [teamDto] });

    const [team] = await result;
    expect(team).toEqual({
      id: 'team-1',
      organizationId: ORG_ID,
      name: 'Plataforma',
      description: 'Infra y datos',
      avatarColor: '#1F6FEB',
      createdBy: 'user-1',
      createdAt: '2026-11-01T10:00:00Z',
      updatedAt: '2026-11-02T10:00:00Z',
    });
  });

  it('maps member_count and my_role when the API sends them', async () => {
    const result = firstValueFrom(api.list());
    backend
      .expectOne(`${BASE}/teams?organization_id=${ORG_ID}`)
      .flush({ data: [{ ...teamDto, member_count: 4, my_role: 'maintainer' }] });

    const [team] = await result;
    expect(team.memberCount).toBe(4);
    expect(team.myRole).toBe('maintainer');
  });

  it('leaves member_count and my_role unset when the API omits them', async () => {
    const result = firstValueFrom(api.list());
    backend.expectOne(`${BASE}/teams?organization_id=${ORG_ID}`).flush({ data: [teamDto] });

    const [team] = await result;
    expect('memberCount' in team).toBe(false);
    expect('myRole' in team).toBe(false);
  });

  it('fails with org.no_current and sends no request when no organization is active', async () => {
    activeOrganization = null;

    const error = await firstValueFrom(api.list()).catch((e: unknown) => e);

    expect(error).toMatchObject({ status: 0, code: 'org.no_current' });
  });

  it('lists the teams where a person is a member, with explicit ids', async () => {
    const result = firstValueFrom(api.listForMember(ORG_ID, 'user-9'));
    backend
      .expectOne(`${BASE}/teams?organization_id=${ORG_ID}&member_id=user-9`)
      .flush({ data: [teamDto] });

    const teams = await result;
    expect(teams[0].id).toBe('team-1');
  });

  it('gets one team by id', async () => {
    const result = firstValueFrom(api.get('team-1'));
    backend.expectOne(`${BASE}/teams/team-1`).flush({ data: teamDto });

    expect((await result).name).toBe('Plataforma');
  });

  it('sends organization_id in the body when creating a team', async () => {
    const result = firstValueFrom(api.create({ organizationId: ORG_ID, name: 'Plataforma' }));
    const request = backend.expectOne(`${BASE}/teams`);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({ organization_id: ORG_ID, name: 'Plataforma' });
    request.flush({ data: teamDto });

    expect((await result).id).toBe('team-1');
  });

  it('sends only the provided fields on partial update', async () => {
    const result = firstValueFrom(
      api.update('team-1', { name: 'Plataforma core', avatarColor: null }),
    );
    const request = backend.expectOne(`${BASE}/teams/team-1`);
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({ name: 'Plataforma core', avatar_color: null });
    request.flush({ data: teamDto });

    await result;
  });

  it('deletes a team', async () => {
    const result = firstValueFrom(api.remove('team-1'));
    const request = backend.expectOne(`${BASE}/teams/team-1`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null);

    expect(await result).toBeUndefined();
  });

  it('lists members and maps roles', async () => {
    const result = firstValueFrom(api.listMembers('team-1'));
    backend.expectOne(`${BASE}/teams/team-1/members`).flush({ data: [memberDto] });

    const [member] = await result;
    expect(member).toEqual({
      teamId: 'team-1',
      userId: 'user-2',
      role: 'maintainer',
      createdAt: '2026-11-03T10:00:00Z',
    });
  });

  it('reads the organization people with names for the members view', async () => {
    const result = firstValueFrom(api.listOrganizationPeople(ORG_ID));
    backend.expectOne(`${BASE}/organizations/${ORG_ID}/members`).flush({
      data: [
        {
          user_id: 'user-2',
          name: 'Luis Soto',
          email: 'luis@example.com',
          role: 'member',
          joined_at: '',
        },
      ],
    });

    expect(await result).toEqual([
      { userId: 'user-2', name: 'Luis Soto', email: 'luis@example.com' },
    ]);
  });

  it('omits role when adding a member so the server default applies', async () => {
    const result = firstValueFrom(api.addMember('team-1', 'user-2'));
    const request = backend.expectOne(`${BASE}/teams/team-1/members`);
    expect(request.request.body).toEqual({ user_id: 'user-2' });
    request.flush({ data: { ...memberDto, role: 'member' } });

    expect((await result).role).toBe('member');
  });

  it('sends the role when adding a member with one', async () => {
    const result = firstValueFrom(api.addMember('team-1', 'user-2', 'maintainer'));
    const request = backend.expectOne(`${BASE}/teams/team-1/members`);
    expect(request.request.body).toEqual({ user_id: 'user-2', role: 'maintainer' });
    request.flush({ data: memberDto });

    await result;
  });

  it('changes a member role with the maintainer or member value', async () => {
    const result = firstValueFrom(api.updateMemberRole('team-1', 'user-2', 'maintainer'));
    const request = backend.expectOne(`${BASE}/teams/team-1/members/user-2`);
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({ role: 'maintainer' });
    request.flush({ data: memberDto });

    await result;
  });

  it('removes a member', async () => {
    const result = firstValueFrom(api.removeMember('team-1', 'user-2'));
    const request = backend.expectOne(`${BASE}/teams/team-1/members/user-2`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null);

    expect(await result).toBeUndefined();
  });

  it('lets API error codes pass through unchanged', async () => {
    const result = firstValueFrom(api.addMember('team-1', 'user-2')).catch((e: unknown) => e);
    backend
      .expectOne(`${BASE}/teams/team-1/members`)
      .flush(
        { error: { code: 'team.member_exists', message: 'ya es miembro' } },
        { status: 409, statusText: 'Conflict' },
      );

    const error = await result;
    expect(error).toBeInstanceOf(HttpErrorResponse);
    expect((error as HttpErrorResponse).status).toBe(409);
    expect((error as HttpErrorResponse).error.error.code).toBe('team.member_exists');
  });
});
