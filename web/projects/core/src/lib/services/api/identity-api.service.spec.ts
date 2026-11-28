import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../../testing';
import { authInterceptor } from '../../interceptors/auth.interceptor';
import { errorInterceptor } from '../../interceptors/error.interceptor';
import { AuthApiService, mapUser } from './auth-api.service';
import { LegalApiService } from './legal-api.service';
import { PhoneApiService } from './phone-api.service';

function setup(): HttpTestingController {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(withInterceptors([authInterceptor, errorInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return TestBed.inject(HttpTestingController);
}

describe('mapUser identity fields', () => {
  it('maps names, locale, phone and pending consents', () => {
    const user = mapUser({
      id: 'u',
      name: 'Ada Lovelace',
      first_name: 'Ada',
      last_name: 'Lovelace',
      locale: 'es-PE',
      email: 'a@b.c',
      role: 'user',
      created_at: 'x',
      phone: { country: 'PE', number: '912345678', e164: '+51912345678', verified: true },
      pending_consents: [{ slug: 'terms_of_use', title: 'Términos', version: 2 }],
    });
    expect(user.firstName).toBe('Ada');
    expect(user.lastName).toBe('Lovelace');
    expect(user.locale).toBe('es-PE');
    expect(user.phone).toEqual({
      country: 'PE',
      number: '912345678',
      e164: '+51912345678',
      verified: true,
    });
    expect(user.pendingConsents).toEqual([{ slug: 'terms_of_use', title: 'Términos', version: 2 }]);
  });

  it('defaults the identity fields on an older API', () => {
    const user = mapUser({ id: 'u', name: 'Ada', email: 'a@b.c', role: 'user', created_at: 'x' });
    expect(user.firstName).toBe('');
    expect(user.phone).toBeNull();
    expect(user.pendingConsents).toEqual([]);
  });
});

describe('AuthApiService identity calls', () => {
  it('sends the confirmation with the password change and the phone with the profile', () => {
    const http = setup();
    const api = TestBed.inject(AuthApiService);
    api
      .changePassword({ currentPassword: 'a', newPassword: 'b', newPasswordConfirmation: 'b' })
      .subscribe();
    expect(http.expectOne('/api/v1/users/me/password').request.body).toEqual({
      current_password: 'a',
      new_password: 'b',
      new_password_confirmation: 'b',
    });

    api.updateProfile({ firstName: 'Ada', phone: null }).subscribe();
    const patch = http.expectOne('/api/v1/users/me');
    expect(patch.request.body).toEqual({ first_name: 'Ada', phone: null });
  });
});

describe('PhoneApiService', () => {
  it('requests a code and verifies it', async () => {
    const http = setup();
    const api = TestBed.inject(PhoneApiService);

    const sent = firstValueFrom(api.requestVerification());
    http
      .expectOne('/api/v1/users/me/phone/verification')
      .flush(
        { data: { destination: '+51******678', expires_at: '2026-01-01T00:10:00Z' } },
        { status: 202, statusText: 'Accepted' },
      );
    expect(await sent).toEqual({ destination: '+51******678', expiresAt: '2026-01-01T00:10:00Z' });

    const verified = firstValueFrom(api.verify('123456'));
    const verify = http.expectOne('/api/v1/users/me/phone/verify');
    expect(verify.request.body).toEqual({ code: '123456' });
    verify.flush({
      data: {
        id: 'u',
        name: 'Ada',
        email: 'a@b.c',
        role: 'user',
        created_at: 'x',
        phone: { country: 'PE', number: '912345678', e164: '+51912345678', verified: true },
      },
    });
    expect((await verified).phone?.verified).toBe(true);
  });
});

describe('LegalApiService administration', () => {
  it('publishes a version with the snake case body', async () => {
    const http = setup();
    const api = TestBed.inject(LegalApiService);
    const publishing = firstValueFrom(
      api.adminPublish({
        slug: 'terms_of_use',
        title: 'T',
        summary: 's',
        bodyMarkdown: '# x',
        required: true,
      }),
    );
    const post = http.expectOne('/api/v1/admin/legal/documents');
    expect(post.request.body).toEqual({
      slug: 'terms_of_use',
      title: 'T',
      summary: 's',
      body_markdown: '# x',
      required: true,
    });
    post.flush({ data: { slug: 'terms_of_use', title: 'T', version: 2, body_markdown: '# x' } });
    expect((await publishing).version).toBe(2);
  });

  it('filters the audit list by user', async () => {
    const http = setup();
    const api = TestBed.inject(LegalApiService);
    const listing = firstValueFrom(api.adminConsents('user-1', 50));
    const get = http.expectOne((r) => r.url === '/api/v1/admin/legal/consents');
    expect(get.request.params.get('user')).toBe('user-1');
    expect(get.request.params.get('limit')).toBe('50');
    get.flush({
      data: [
        {
          id: 'c1',
          user_id: 'user-1',
          slug: 'privacy_policy',
          version: 1,
          accepted_at: '2026-01-01T00:00:00Z',
          source: 'register',
          revoked_at: null,
        },
      ],
    });
    expect((await listing)[0]).toMatchObject({
      userId: 'user-1',
      source: 'register',
      revokedAt: null,
    });
  });
});
