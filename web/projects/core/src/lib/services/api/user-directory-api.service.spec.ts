import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { UserDirectoryApiService } from './user-directory-api.service';

const USER_RESPONSE = {
  id: 'u-1',
  name: 'Ada',
  email: 'ada@example.com',
  role: 'user',
  team_id: 't-1',
  created_at: '2026-01-01T00:00:00Z',
};

function setup(): { service: UserDirectoryApiService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(UserDirectoryApiService),
    http: TestBed.inject(HttpTestingController),
  };
}

describe('UserDirectoryApiService', () => {
  it('searches teammates of the given organization', () => {
    const { service, http } = setup();
    let users: { teamId: string | null }[] = [];

    service.searchTeammates('org-1', 'ana').subscribe((value) => (users = value));

    const request = http.expectOne((req) => req.url === '/api/v1/users/search');
    expect(request.request.method).toBe('GET');
    expect(request.request.params.get('q')).toBe('ana');
    expect(request.request.params.get('organization_id')).toBe('org-1');
    request.flush({ data: [USER_RESPONSE] });

    expect(users[0].teamId).toBe('t-1');
    http.verify();
  });

  it('sends only the filters that are set in the administrative search', () => {
    const { service, http } = setup();

    service.search({ query: '' }).subscribe();
    const plain = http.expectOne((req) => req.url === '/api/v1/users');
    expect(plain.request.params.keys()).toEqual(['q']);
    plain.flush({ data: [] });

    service.search({ query: 'ana', teamId: 't-9' }).subscribe();
    const filtered = http.expectOne((req) => req.url === '/api/v1/users');
    expect(filtered.request.params.get('team_id')).toBe('t-9');
    filtered.flush({ data: [] });
    http.verify();
  });

  it('moves a user to a team, or removes the team with null', () => {
    const { service, http } = setup();

    service.setTeam('u-1', 't-2').subscribe();
    const assign = http.expectOne({ method: 'PATCH', url: '/api/v1/users/u-1/team' });
    expect(assign.request.body).toEqual({ team_id: 't-2' });
    assign.flush({ data: USER_RESPONSE });

    service.setTeam('u-1', null).subscribe();
    const remove = http.expectOne({ method: 'PATCH', url: '/api/v1/users/u-1/team' });
    expect(remove.request.body).toEqual({ team_id: null });
    remove.flush({ data: USER_RESPONSE });
    http.verify();
  });
});
