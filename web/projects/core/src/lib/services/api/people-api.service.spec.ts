import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { PeopleApiService } from './people-api.service';

function setup(): { service: PeopleApiService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(PeopleApiService),
    http: TestBed.inject(HttpTestingController),
  };
}

describe('PeopleApiService', () => {
  it('lists members with job title, department and avatar flag', () => {
    const { service, http } = setup();
    let members: unknown;

    service.listMembers('org-1').subscribe((value) => (members = value));

    http.expectOne({ method: 'GET', url: '/api/v1/organizations/org-1/members' }).flush({
      data: [
        {
          user_id: 'u-1',
          name: 'Ana',
          display_name: 'Ana T.',
          email: 'ana@corelog.test',
          job_title: 'Analista',
          department: 'Ops',
          has_avatar: true,
          role: 'admin',
          joined_at: '2026-01-10T00:00:00Z',
        },
      ],
    });

    expect(members).toEqual([
      {
        userId: 'u-1',
        name: 'Ana',
        displayName: 'Ana T.',
        email: 'ana@corelog.test',
        jobTitle: 'Analista',
        department: 'Ops',
        hasAvatar: true,
        role: 'admin',
        joinedAt: '2026-01-10T00:00:00Z',
      },
    ]);
    http.verify();
  });

  it('defaults missing member fields instead of leaving them undefined', () => {
    const { service, http } = setup();
    let members: {
      displayName: string;
      jobTitle: string;
      department: string;
      hasAvatar: boolean;
    }[] = [];

    service.listMembers('org-1').subscribe((value) => (members = value));
    http.expectOne('/api/v1/organizations/org-1/members').flush({
      data: [
        {
          user_id: 'u-2',
          name: 'Luis',
          email: 'l@x.test',
          role: 'member',
          joined_at: '2026-01-11T00:00:00Z',
        },
      ],
    });

    expect(members[0]).toMatchObject({
      displayName: '',
      jobTitle: '',
      department: '',
      hasAvatar: false,
    });
  });

  it('gets one person profile from the people endpoint', () => {
    const { service, http } = setup();
    let person: unknown;

    service.getPerson('org-1', 'u-1').subscribe((value) => (person = value));

    http.expectOne({ method: 'GET', url: '/api/v1/organizations/org-1/people/u-1' }).flush({
      data: {
        user_id: 'u-1',
        name: 'Ana',
        display_name: '',
        email: 'ana@corelog.test',
        job_title: 'Analista',
        department: 'Ops',
        timezone: 'America/Bogota',
        has_avatar: false,
        org_role: 'member',
        joined_at: '2026-01-10T00:00:00Z',
      },
    });

    expect(person).toEqual({
      userId: 'u-1',
      name: 'Ana',
      displayName: '',
      email: 'ana@corelog.test',
      jobTitle: 'Analista',
      department: 'Ops',
      timezone: 'America/Bogota',
      hasAvatar: false,
      orgRole: 'member',
      joinedAt: '2026-01-10T00:00:00Z',
    });
    http.verify();
  });

  it('encodes the organization and user identifiers in the path', () => {
    const { service, http } = setup();

    service.getPerson('org/1', 'u 1').subscribe();

    http.expectOne('/api/v1/organizations/org%2F1/people/u%201').flush({
      data: {
        user_id: 'u 1',
        name: 'Ana',
        email: 'a@x.test',
        org_role: 'guest',
        joined_at: '2026-01-10T00:00:00Z',
      },
    });
    http.verify();
  });

  it('sends only the provided fields in snake_case on update', () => {
    const { service, http } = setup();
    let user: { displayName: string; timezone: string } | undefined;

    service
      .updateMe({ displayName: 'Ana T.', timezone: 'America/Bogota' })
      .subscribe((value) => (user = value));

    const request = http.expectOne({ method: 'PATCH', url: '/api/v1/users/me' });
    expect(request.request.body).toEqual({ display_name: 'Ana T.', timezone: 'America/Bogota' });
    request.flush({
      data: {
        id: 'u-1',
        name: 'Ana',
        email: 'a@x.test',
        role: 'user',
        display_name: 'Ana T.',
        timezone: 'America/Bogota',
        created_at: '2026-01-10T00:00:00Z',
      },
    });

    expect(user?.displayName).toBe('Ana T.');
    expect(user?.timezone).toBe('America/Bogota');
    http.verify();
  });

  it('loads the current user', () => {
    const { service, http } = setup();
    let role: string | undefined;

    service.getMe().subscribe((user) => (role = user.role));

    http.expectOne({ method: 'GET', url: '/api/v1/users/me' }).flush({
      data: {
        id: 'u-1',
        name: 'Ana',
        email: 'a@x.test',
        role: 'admin',
        timezone: 'UTC',
        created_at: '2026-01-10T00:00:00Z',
      },
    });

    expect(role).toBe('admin');
    http.verify();
  });
});
