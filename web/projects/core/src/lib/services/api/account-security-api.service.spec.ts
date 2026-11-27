import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { AccountSecurityApiService } from './account-security-api.service';

function setup(): { service: AccountSecurityApiService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(AccountSecurityApiService),
    http: TestBed.inject(HttpTestingController),
  };
}

describe('AccountSecurityApiService', () => {
  it('maps sessions from snake_case and keeps the current flag', () => {
    const { service, http } = setup();
    let sessions: unknown;

    service.listSessions().subscribe((value) => (sessions = value));
    http.expectOne({ method: 'GET', url: '/api/v1/users/me/sessions' }).flush({
      data: [
        {
          id: 's-1',
          user_agent: 'Chrome',
          created_at: '2026-11-22T10:00:00Z',
          last_seen_at: '2026-11-22T11:00:00Z',
          current: true,
        },
      ],
    });

    expect(sessions).toEqual([
      {
        id: 's-1',
        userAgent: 'Chrome',
        createdAt: '2026-11-22T10:00:00Z',
        lastSeenAt: '2026-11-22T11:00:00Z',
        current: true,
      },
    ]);
    http.verify();
  });

  it('revokes one session, the others, and deactivates with the password', () => {
    const { service, http } = setup();

    service.revokeSession('s-2').subscribe();
    http.expectOne({ method: 'DELETE', url: '/api/v1/users/me/sessions/s-2' }).flush(null);

    service.revokeOtherSessions().subscribe();
    http.expectOne({ method: 'POST', url: '/api/v1/users/me/sessions/revoke-others' }).flush(null);

    service.deactivateAccount('clave-prueba').subscribe();
    const deactivate = http.expectOne({ method: 'POST', url: '/api/v1/users/me/deactivate' });
    expect(deactivate.request.body).toEqual({ password: 'clave-prueba' });
    deactivate.flush(null);
    http.verify();
  });

  it('sends the given token explicitly on logout', () => {
    const { service, http } = setup();

    service.logout('token-actual').subscribe();

    const request = http.expectOne({ method: 'POST', url: '/api/v1/auth/logout' });
    expect(request.request.headers.get('Authorization')).toBe('Bearer token-actual');
    request.flush(null);
  });
});
