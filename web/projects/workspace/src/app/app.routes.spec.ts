import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import {
  LOCATION_ASSIGN,
  OrganizationStore,
  SessionService,
  type Organization,
} from '@corelog/core';
import { of } from 'rxjs';

import { WORKSPACE_NAV } from './nav.config';
import { routes } from './app.routes';

const ACME = { id: 'o1', name: 'Acme', slug: 'acme', role: 'member' } as Organization;

interface SetupOptions {
  readonly authenticated?: boolean;
  readonly orgs?: readonly Organization[];
}

function setup({ authenticated = true, orgs = [ACME] }: SetupOptions = {}) {
  const assign = vi.fn();
  const session = {
    isAuthenticated: () => authenticated,
    redirectToLogin: vi.fn(),
  } as unknown as SessionService;
  const organizations = {
    ensureOrgs: () => of(orgs),
  } as unknown as OrganizationStore;
  TestBed.configureTestingModule({
    providers: [
      provideRouter(routes),
      { provide: LOCATION_ASSIGN, useValue: assign },
      { provide: SessionService, useValue: session },
      { provide: OrganizationStore, useValue: organizations },
    ],
  });
  return { router: TestBed.inject(Router), assign, session };
}

describe('workspace routes', () => {
  it('mounts the shell at the root with the session and organization guards', () => {
    const shell = routes.find((route) => route.path === '');
    expect(shell?.canActivate).toHaveLength(2);
    expect(shell?.canActivateChild).toHaveLength(2);
  });

  it('mounts every workspace area under the shell', () => {
    const shell = routes.find((route) => route.path === '');
    const paths = shell?.children?.map((route) => route.path);
    expect(paths).toEqual([
      '',
      'issues',
      'browse',
      'timesheet',
      'filters',
      'profile',
      'people',
      'projects',
      'dashboard',
      'teams',
      'notes',
      '**',
    ]);
  });

  it('hands the portal and admin paths over with a full-page navigation', () => {
    expect(routes.slice(0, 3).map((route) => route.path)).toEqual([
      'login',
      'onboarding',
      'admin/**',
    ]);
    expect(routes.slice(0, 3).every((route) => route.children?.length === 0)).toBe(true);
  });

  it('redirects the shell root to the issues list', async () => {
    const { router } = setup();
    await router.navigateByUrl('/');
    expect(router.url).toBe('/issues');
  });

  it('sends the old issue detail address to the browse path', async () => {
    const { router } = setup();
    await router.navigateByUrl('/issues/ABC-1');
    expect(router.url).toBe('/browse/ABC-1');
  });

  it('keeps the issue, profile, people, dashboard and team pages at their paths', async () => {
    // A fresh router per address: the unsaved-changes guard needs a rendered page to leave.
    for (const url of [
      '/issues',
      '/issues/new',
      '/browse/CORE-1',
      '/profile',
      '/profile/security',
      '/people',
      '/people/u1',
      '/dashboard',
      '/teams',
      '/teams/t1',
    ]) {
      TestBed.resetTestingModule();
      const { router } = setup();
      await router.navigateByUrl(url);
      expect(router.url).toBe(url);
    }
  });

  it('takes the project key (or id) in the key parameter and resolves it', () => {
    const shell = routes.find((route) => route.path === '');
    const projects = shell?.children?.find((route) => route.path === 'projects');
    const detail = projects?.children?.find((route) => route.path === ':key');
    expect(detail).toBeDefined();
    expect(detail?.resolve?.['project']).toBeDefined();
  });

  it('opens the project board when the project has no section', async () => {
    const { router } = setup();
    await router.navigateByUrl('/projects/p1');
    expect(router.url).toBe('/projects/p1/board');
  });

  it('keeps every project section at its own path', async () => {
    const { router } = setup();
    for (const section of ['board', 'backlog', 'sprints', 'calendar', 'settings/details']) {
      await router.navigateByUrl(`/projects/p1/${section}`);
      expect(router.url).toBe(`/projects/p1/${section}`);
    }
  });

  it('falls back to the board for an unknown project section', async () => {
    const { router } = setup();
    await router.navigateByUrl('/projects/p1/desconocida');
    expect(router.url).toBe('/projects/p1/board');
  });

  it('sends signed-out visitors to the login page and keeps the current url', async () => {
    const { router, session } = setup({ authenticated: false });
    const result = await router.navigateByUrl('/issues');
    expect(result).toBe(false);
    expect(session.redirectToLogin).toHaveBeenCalled();
    expect(router.url).not.toBe('/issues');
  });

  it('sends users without an organization to onboarding with a full-page navigation', async () => {
    const { router, assign } = setup({ orgs: [] });
    const result = await router.navigateByUrl('/issues');
    expect(result).toBe(false);
    expect(assign).toHaveBeenCalledWith('/onboarding');
  });

  it('hands the login page over to the portal', async () => {
    const { router, assign } = setup({ authenticated: false });
    const result = await router.navigateByUrl('/login');
    expect(result).toBe(false);
    expect(assign).toHaveBeenCalledWith('/login');
  });

  it('hands organization administration over to the admin app with the same address', async () => {
    const { router, assign } = setup();
    await router.navigateByUrl('/admin/organization/members');
    expect(assign).toHaveBeenCalledWith('/admin/organization/members');
  });

  it('reaches every cross-app sidebar link through a handover route', async () => {
    const { router, assign } = setup();
    for (const item of WORKSPACE_NAV.sidebars['org-admin']) {
      assign.mockClear();
      await router.navigateByUrl(item.path);
      expect(assign).toHaveBeenCalledWith(item.path);
    }
  });

  it('shows the not-found page inside the shell for unknown paths', async () => {
    const { router, assign } = setup();
    await router.navigateByUrl('/no-existe');
    expect(router.url).toBe('/no-existe');
    expect(assign).not.toHaveBeenCalled();
  });
});
