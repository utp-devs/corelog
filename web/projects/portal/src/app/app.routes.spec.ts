import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { LOCATION_ASSIGN, SessionService } from '@corelog/core';

import { routes } from './app.routes';

function setup(authenticated: boolean) {
  const assign = vi.fn();
  const session = {
    isAuthenticated: () => authenticated,
    redirectToLogin: vi.fn(),
  } as unknown as SessionService;
  TestBed.configureTestingModule({
    providers: [
      provideRouter(routes),
      { provide: LOCATION_ASSIGN, useValue: assign },
      { provide: SessionService, useValue: session },
    ],
  });
  return { router: TestBed.inject(Router), assign, session };
}

describe('portal routes', () => {
  it('mounts sign-in, registration and invitation at the root, not under login', () => {
    const paths = routes.map((route) => route.path);
    expect(paths).toEqual(['', 'login', 'register', 'invite/:token', 'onboarding', '**']);
  });

  it('guards sign-in and registration for signed-out users only', () => {
    const guarded = routes.filter((route) => ['login', 'register'].includes(route.path ?? ''));
    expect(guarded.map((route) => route.canActivate?.length)).toEqual([1, 1]);
    expect(routes.find((route) => route.path === 'invite/:token')?.canActivate).toBeUndefined();
  });

  it('keeps the onboarding wizard behind the auth guard', () => {
    const onboarding = routes.find((route) => route.path === 'onboarding');
    expect(onboarding?.canActivate).toHaveLength(1);
    expect(onboarding?.canActivateChild).toHaveLength(1);
  });

  it('sends a signed-out visitor of the root to the login page', async () => {
    const { router, assign } = setup(false);
    await router.navigateByUrl('/');
    expect(router.url).toBe('/login');
    expect(assign).not.toHaveBeenCalled();
  });

  it('sends a signed-in visitor of the root to the workspace home with a full-page navigation', async () => {
    const { router, assign } = setup(true);
    const result = await router.navigateByUrl('/');
    expect(result).toBe(false);
    expect(assign).toHaveBeenCalledWith('/workspace/');
  });

  it('sends a signed-in visitor of the login page to the workspace home', async () => {
    const { router, assign } = setup(true);
    await router.navigateByUrl('/login');
    expect(assign).toHaveBeenCalledWith('/workspace/');
  });

  it('keeps the login page for signed-out visitors', async () => {
    const { router, assign } = setup(false);
    await router.navigateByUrl('/login');
    expect(router.url).toBe('/login');
    expect(assign).not.toHaveBeenCalled();
  });

  it('keeps the invitation preview public', async () => {
    const { router, session } = setup(false);
    await router.navigateByUrl('/invite/abc123');
    expect(router.url).toBe('/invite/abc123');
    expect(session.redirectToLogin).not.toHaveBeenCalled();
  });

  it('asks signed-out visitors of the onboarding wizard to sign in', async () => {
    const { router, session } = setup(false);
    const result = await router.navigateByUrl('/onboarding');
    expect(result).toBe(false);
    expect(session.redirectToLogin).toHaveBeenCalled();
  });

  it('opens the onboarding wizard for signed-in users', async () => {
    const { router } = setup(true);
    await router.navigateByUrl('/onboarding?new=1');
    expect(router.url).toBe('/onboarding?new=1');
  });

  it('shows the not-found page for unknown paths', async () => {
    const { router } = setup(false);
    await router.navigateByUrl('/no-existe');
    expect(router.url).toBe('/no-existe');
  });
});
