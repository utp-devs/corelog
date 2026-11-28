import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { Router, provideRouter } from '@angular/router';
import {
  LOCATION_ASSIGN,
  OrganizationStore,
  SessionService,
  type Organization,
  type OrgRole,
} from '@corelog/core';
import { of } from 'rxjs';

import { ADMIN_NAV } from './nav.config';
import { routes } from './app.routes';

const ORG = { id: 'o1', name: 'Acme', slug: 'acme' } as Organization;

interface SetupOptions {
  readonly authenticated?: boolean;
  readonly orgs?: readonly Organization[];
  readonly role?: OrgRole | null;
}

function setup({ authenticated = true, orgs = [ORG], role = 'owner' }: SetupOptions = {}) {
  const assign = vi.fn();
  const session = {
    isAuthenticated: () => authenticated,
    redirectToLogin: vi.fn(),
  } as unknown as SessionService;
  const organizations = {
    ensureOrgs: () => of(orgs),
    currentRole: signal(role),
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

describe('admin routes', () => {
  it('mounts the shell with the session and organization guards', () => {
    const shell = routes.find((route) => route.path === '');
    expect(shell?.canActivate).toHaveLength(2);
    expect(shell?.canActivateChild).toHaveLength(2);
  });

  it('guards the organization area with the org-admin check and the 403 page sits beside it', () => {
    const shell = routes.find((route) => route.path === '');
    const organization = shell?.children?.find((route) => route.path === 'organization');
    expect(organization?.canActivate).toHaveLength(1);
    expect(organization?.canActivateChild).toHaveLength(1);
    expect(shell?.children?.some((route) => route.path === '403')).toBe(true);
    expect(organization?.canActivate).not.toContain(shell?.canActivate?.[0]);
  });

  it('mounts settings, members, catalogs, statuses and invitations under /organization', () => {
    const shell = routes.find((route) => route.path === '');
    const organization = shell?.children?.find((route) => route.path === 'organization');
    expect(organization?.children?.map((route) => route.path)).toEqual([
      '',
      'overview',
      'settings',
      'members',
      'catalogs',
      'custom-fields',
      'statuses',
      'invitations',
      'legal',
      'notifications',
      'email-outbox',
      'jobs',
      'audit',
      'webhooks',
      'platform-audit',
    ]);
  });

  it('redirects the admin root to the organization overview for owners', async () => {
    const { router } = setup({ role: 'owner' });
    await router.navigateByUrl('/');
    expect(router.url).toBe('/organization/overview');
  });

  it('redirects the admin root to the 403 page for members', async () => {
    const { router } = setup({ role: 'member' });
    await router.navigateByUrl('/');
    expect(router.url).toBe('/403');
  });

  it('keeps the organization pages for owners and admins', async () => {
    for (const role of ['owner', 'admin'] as const) {
      for (const url of [
        '/organization/settings',
        '/organization/members',
        '/organization/catalogs',
        '/organization/custom-fields',
        '/organization/statuses',
        '/organization/invitations',
        '/organization/overview',
        '/organization/notifications',
      ]) {
        TestBed.resetTestingModule();
        const { router } = setup({ role });
        await router.navigateByUrl(url);
        expect(router.url).toBe(url);
      }
    }
  });

  it('answers members with the 403 page instead of the organization pages', async () => {
    for (const role of ['member', 'guest', null] as const) {
      TestBed.resetTestingModule();
      const { router } = setup({ role });
      await router.navigateByUrl('/organization/members');
      expect(router.url).toBe('/403');
    }
  });

  it('shows the 403 page for members when they open it directly', async () => {
    const { router } = setup({ role: 'member' });
    await router.navigateByUrl('/403');
    expect(router.url).toBe('/403');
  });

  it('sends signed-out visitors to the login page', async () => {
    const { router, session } = setup({ authenticated: false });
    const result = await router.navigateByUrl('/organization/settings');
    expect(result).toBe(false);
    expect(session.redirectToLogin).toHaveBeenCalled();
  });

  it('sends users without an organization to onboarding with a full-page navigation', async () => {
    const { router, assign } = setup({ orgs: [] });
    await router.navigateByUrl('/organization/settings');
    expect(assign).toHaveBeenCalledWith('/onboarding');
  });

  it('hands the login page over to the portal', async () => {
    const { router, assign } = setup({ authenticated: false });
    await router.navigateByUrl('/login');
    expect(assign).toHaveBeenCalledWith('/login');
  });

  it('hands the profile over to the workspace, keeping the section', async () => {
    const { router, assign } = setup();
    await router.navigateByUrl('/profile');
    expect(assign).toHaveBeenLastCalledWith('/workspace/profile');
    await router.navigateByUrl('/profile/security');
    expect(assign).toHaveBeenLastCalledWith('/workspace/profile/security');
  });

  it('hands every cross-app menu and sidebar link to the workspace with the same address', async () => {
    const crossApp = [
      ...ADMIN_NAV.menus.flatMap((menu) => [
        menu.path,
        ...(menu.extras ?? []).map((link) => link.path),
      ]),
      ...ADMIN_NAV.sidebars.profile.map((item) => item.path),
      ...ADMIN_NAV.sidebars.project.map((item) => item.path.replace(':projectId', 'p1')),
    ];
    for (const url of crossApp) {
      TestBed.resetTestingModule();
      const { router, assign } = setup();
      await router.navigateByUrl(url);
      expect(assign).toHaveBeenCalledWith(url);
    }
  });

  it('shows the not-found page inside the shell for unknown paths', async () => {
    const { router, assign } = setup();
    await router.navigateByUrl('/no-existe');
    expect(router.url).toBe('/no-existe');
    expect(assign).not.toHaveBeenCalled();
  });
});
