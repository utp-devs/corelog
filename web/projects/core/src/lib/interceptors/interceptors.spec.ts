import { HttpClient, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';

import { SKIP_ERROR_NOTIFICATION, SKIP_LOADING, publicRequest } from '../config/http-context';
import { LocationRecorder, provideCorelogTesting, recordLocation } from '../../testing';
import { LoadingService } from '../services/loading.service';
import { AUTH_TOKEN_KEY, SessionService } from '../services/session.service';
import { authInterceptor } from './auth.interceptor';
import { errorInterceptor } from './error.interceptor';
import { loadingInterceptor } from './loading.interceptor';

function setup(recorder: LocationRecorder = recordLocation()): {
  http: HttpClient;
  backend: HttpTestingController;
  session: SessionService;
  messages: MessageService;
  loading: LoadingService;
} {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(recorder),
      provideHttpClient(withInterceptors([loadingInterceptor, errorInterceptor, authInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return {
    http: TestBed.inject(HttpClient),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
    messages: TestBed.inject(MessageService),
    loading: TestBed.inject(LoadingService),
  };
}

describe('HTTP interceptors', () => {
  beforeEach(() => localStorage.clear());

  it('adds the bearer token to API requests', () => {
    const { http, backend, session } = setup();
    session.startSession('token-1');
    http.get('/api/v1/users/me').subscribe();
    const request = backend.expectOne('/api/v1/users/me');
    expect(request.request.headers.get('Authorization')).toBe('Bearer token-1');
    request.flush({});
    backend.verify();
  });

  it('does not add the token to public requests or to URLs outside the API', () => {
    const { http, backend, session } = setup();
    session.startSession('token-1');
    http.post('/api/v1/auth/login', {}, { context: publicRequest() }).subscribe();
    http.get('https://ejemplo.test/recurso').subscribe();
    const login = backend.expectOne('/api/v1/auth/login');
    const external = backend.expectOne('https://ejemplo.test/recurso');
    expect(login.request.headers.has('Authorization')).toBe(false);
    expect(external.request.headers.has('Authorization')).toBe(false);
    login.flush({});
    external.flush({});
  });

  it('sends no Authorization header when there is no session', () => {
    const { http, backend } = setup();
    http.get('/api/v1/users/me').subscribe();
    const request = backend.expectOne('/api/v1/users/me');
    expect(request.request.headers.has('Authorization')).toBe(false);
    request.flush({});
  });

  it('ends a signed-in session on 401 and sends the user to login', () => {
    const recorder = recordLocation();
    const { http, backend, session } = setup(recorder);
    session.startSession('token-caducado');
    let received: { status?: number } | undefined;
    http
      .get('/api/v1/users/me')
      .subscribe({ error: (error: { status: number }) => (received = error) });
    backend
      .expectOne('/api/v1/users/me')
      .flush(
        { error: { message: 'Tu sesión ha expirado.', code: 'auth.unauthorized' } },
        { status: 401, statusText: 'Unauthorized' },
      );
    expect(received?.status).toBe(401);
    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
    expect(recorder.urls[0]).toContain('/login?returnUrl=');
  });

  it('shows the normalized message as a toast on server errors', () => {
    const { http, backend, session, messages } = setup();
    session.startSession('token-1');
    const add = vi.spyOn(messages, 'add');
    http.get('/api/v1/fallo').subscribe({ error: () => undefined });
    backend
      .expectOne('/api/v1/fallo')
      .flush({ error: { message: 'Algo salió mal.' } }, { status: 500, statusText: 'Error' });
    expect(add).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'error', detail: 'Algo salió mal.' }),
    );
  });

  it('keeps silent when the request opts out of error notifications', () => {
    const { http, backend, messages } = setup();
    const add = vi.spyOn(messages, 'add');
    http
      .get('/api/v1/silencioso', { context: publicRequest().set(SKIP_ERROR_NOTIFICATION, true) })
      .subscribe({ error: () => undefined });
    backend.expectOne('/api/v1/silencioso').flush({}, { status: 500, statusText: 'Error' });
    expect(add).not.toHaveBeenCalled();
  });

  it('counts pending requests in the loading service and releases them on completion', () => {
    const { http, backend, loading } = setup();
    http.get('/api/v1/a').subscribe();
    http.get('/api/v1/b', { context: publicRequest().set(SKIP_LOADING, true) }).subscribe();
    expect(loading.isLoading()).toBe(true);
    expect(loading.pendingRequests()).toBe(1);
    backend.expectOne('/api/v1/a').flush({});
    expect(loading.isLoading()).toBe(false);
    backend.expectOne('/api/v1/b').flush({});
  });
});
