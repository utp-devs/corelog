import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { authInterceptor } from '../interceptors/auth.interceptor';
import { errorInterceptor } from '../interceptors/error.interceptor';
import { AuthStore } from './auth.store';
import { ConsentStore } from './consent.store';
import { CountryStore } from './country.store';
import { LegalStore } from './legal.store';

const USER = {
  id: 'u-1',
  name: 'Ada Lovelace',
  first_name: 'Ada',
  last_name: 'Lovelace',
  email: 'ada@example.com',
  role: 'user',
  created_at: '2026-01-01T00:00:00Z',
};

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

async function signIn(http: HttpTestingController, pending: unknown[] = []): Promise<void> {
  const auth = TestBed.inject(AuthStore);
  const done = auth.login({ email: 'ada@example.com', password: 'clave-prueba' });
  http
    .expectOne('/api/v1/auth/login')
    .flush({ data: { token: 'tok', user: USER, pending_consents: pending } });
  await done;
}

describe('ConsentStore', () => {
  beforeEach(() => localStorage.clear());

  it('takes the pending documents from the session user', async () => {
    const http = setup();
    const store = TestBed.inject(ConsentStore);
    expect(store.hasPending()).toBe(false);

    await signIn(http, [{ slug: 'privacy_policy', title: 'Política de privacidad', version: 2 }]);
    expect(store.pending()).toEqual([
      { slug: 'privacy_policy', title: 'Política de privacidad', version: 2 },
    ]);
    expect(store.hasPending()).toBe(true);
  });

  it('records the acceptance and refreshes the user so the pending list empties', async () => {
    const http = setup();
    const store = TestBed.inject(ConsentStore);
    await signIn(http, [{ slug: 'terms_of_use', title: 'Términos', version: 3 }]);

    const accepting = store.accept([{ slug: 'terms_of_use', version: 3 }], 'login_reprompt');
    const post = http.expectOne('/api/v1/me/consents');
    expect(post.request.method).toBe('POST');
    expect(post.request.body).toEqual({
      accepted: [{ slug: 'terms_of_use', version: 3 }],
      source: 'login_reprompt',
    });
    post.flush({
      data: { accepted: [{ slug: 'terms_of_use', version: 3, accepted_at: 'x' }], pending: [] },
    });
    // The refresh of the session user is awaited after the status arrives.
    await Promise.resolve();
    await Promise.resolve();
    http.expectOne('/api/v1/users/me').flush({ data: { ...USER, pending_consents: [] } });
    await accepting;

    expect(store.hasPending()).toBe(false);
    expect(store.status()?.accepted[0].slug).toBe('terms_of_use');
    expect(store.loading()).toBe(false);
  });

  it('keeps the pending list and exposes the error when the acceptance fails', async () => {
    const http = setup();
    const store = TestBed.inject(ConsentStore);
    await signIn(http, [{ slug: 'terms_of_use', title: 'Términos', version: 3 }]);

    const accepting = store.accept([{ slug: 'terms_of_use', version: 2 }], 'login_reprompt');
    http
      .expectOne('/api/v1/me/consents')
      .flush(
        { error: { message: 'versión no vigente', code: 'legal.version_invalid' } },
        { status: 422, statusText: 'Unprocessable' },
      );
    await expect(accepting).rejects.toMatchObject({ code: 'legal.version_invalid' });
    expect(store.hasPending()).toBe(true);
    expect(store.error()).toBe('versión no vigente');
  });

  it('revokes an optional consent and reloads the status', async () => {
    const http = setup();
    const store = TestBed.inject(ConsentStore);
    await signIn(http);

    const revoking = store.revoke('communications');
    const del = http.expectOne('/api/v1/me/consents/communications');
    expect(del.request.method).toBe('DELETE');
    del.flush(null, { status: 204, statusText: 'No Content' });
    await Promise.resolve();
    await Promise.resolve();
    http.expectOne('/api/v1/me/consents').flush({ data: { accepted: [], pending: [] } });
    await revoking;
    expect(store.status()?.accepted).toEqual([]);
  });
});

describe('CountryStore', () => {
  it('loads the catalog once and finds countries by code', async () => {
    const http = setup();
    const store = TestBed.inject(CountryStore);
    const first = store.load();
    const second = store.load();
    http.expectOne('/api/v1/meta/countries').flush({
      data: [
        { code: 'PE', name: 'Perú', dial_code: '+51', example: '912345678' },
        { code: 'CO', name: 'Colombia', dial_code: '+57', example: '3211234567' },
      ],
    });
    await Promise.all([first, second]);

    expect(store.status()).toBe('ready');
    expect(store.find('pe')?.dialCode).toBe('+51');
    expect(store.find('ZZ')).toBeUndefined();
    await store.load();
    http.verify();
  });

  it('preselects the browser region, then Peru, then the first country', async () => {
    const http = setup();
    const store = TestBed.inject(CountryStore);
    const loading = store.load();
    http.expectOne('/api/v1/meta/countries').flush({
      data: [
        { code: 'CO', name: 'Colombia', dial_code: '+57' },
        { code: 'PE', name: 'Perú', dial_code: '+51' },
      ],
    });
    await loading;
    expect(store.defaultCountry(['es-CO'])?.code).toBe('CO');
    expect(store.defaultCountry(['en'])?.code).toBe('PE');
  });

  it('reports an error status when the catalog cannot be loaded', async () => {
    const http = setup();
    const store = TestBed.inject(CountryStore);
    const loading = store.load();
    http.expectOne('/api/v1/meta/countries').flush({}, { status: 500, statusText: 'Error' });
    await loading;
    expect(store.status()).toBe('error');
  });
});

describe('LegalStore', () => {
  it('lists the documents and caches the bodies already read', async () => {
    const http = setup();
    const store = TestBed.inject(LegalStore);
    const loading = store.load();
    http.expectOne('/api/v1/legal/documents').flush({
      data: [
        { slug: 'privacy_policy', title: 'Privacidad', version: 1, required: true },
        { slug: 'communications', title: 'Comunicaciones', version: 1, required: false },
      ],
    });
    await loading;
    expect(store.find('privacy_policy')?.required).toBe(true);
    expect(store.communications()?.slug).toBe('communications');

    const first = store.read('privacy_policy');
    http.expectOne('/api/v1/legal/documents/privacy_policy').flush({
      data: { slug: 'privacy_policy', title: 'Privacidad', version: 1, body_markdown: '# Hola' },
    });
    expect((await first).bodyMarkdown).toBe('# Hola');
    expect((await store.read('privacy_policy')).title).toBe('Privacidad');
    http.verify();
  });

  it('asks for a specific version', async () => {
    const http = setup();
    const store = TestBed.inject(LegalStore);
    const reading = store.read('terms_of_use', 2);
    const request = http.expectOne((r) => r.url === '/api/v1/legal/documents/terms_of_use');
    expect(request.request.params.get('version')).toBe('2');
    request.flush({
      data: { slug: 'terms_of_use', title: 'Términos', version: 2, body_markdown: 'x' },
    });
    expect((await reading).version).toBe(2);
  });
});
