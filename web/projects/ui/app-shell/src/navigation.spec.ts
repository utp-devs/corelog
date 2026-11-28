import { permissionsFor, type Permission } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import {
  DEFAULT_SHELL_NAV_CONFIG,
  buildSidebar,
  buildTopNav,
  isSidebarItemActive,
  resolveSidebarContext,
  sidebarTitleOf,
  type ShellOrg,
} from './navigation';

const org: ShellOrg = { id: 'o1', name: 'Acme', slug: 'acme' };
const adminPerms: ReadonlySet<Permission> = permissionsFor({ role: 'admin' });
const BASE_MENUS = ['projects', 'filters', 'dashboards', 'teams'];

describe('buildTopNav', () => {
  it('shows the base menus without an organization name', () => {
    const nav = buildTopNav(null);
    expect(nav.menus.map((m) => m.key)).toEqual(BASE_MENUS);
    expect(nav.orgName).toBeNull();
  });

  it('offers Equipos to every role and shows the organization name', () => {
    const nav = buildTopNav(org);
    expect(nav.menus.find((m) => m.key === 'teams')?.path).toBe('/teams');
    expect(nav.orgName).toBe('Acme');
  });

  it('keeps every existing route reachable from the menus', () => {
    const paths = buildTopNav(null).menus.map((m) => m.path);
    expect(paths).toEqual(expect.arrayContaining(['/projects', '/issues', '/dashboard', '/teams']));
  });

  it('builds the same menus with or without an organization', () => {
    expect(buildTopNav(org).menus).toEqual(buildTopNav(null).menus);
  });

  it('lists Ver personas under Equipos, pointing at /people', () => {
    const teams = buildTopNav(org).menus.find((m) => m.key === 'teams');
    expect(teams?.extras).toEqual([{ key: 'people', label: 'Ver personas', path: '/people' }]);
    expect(buildTopNav(org).menus.find((m) => m.key === 'projects')?.extras).toBeUndefined();
  });

  it('carries no recent entries in the menus (the dropdowns show the empty placeholder)', () => {
    expect(buildTopNav(null).menus.every((m) => !('recent' in m))).toBe(true);
  });
});

describe('resolveSidebarContext', () => {
  it('maps project paths to the project context', () => {
    expect(resolveSidebarContext('/projects/p1/board')).toEqual({
      kind: 'project',
      projectId: 'p1',
    });
    expect(resolveSidebarContext('/projects/p1')).toEqual({ kind: 'project', projectId: 'p1' });
  });

  it('keeps /projects without id in the global context', () => {
    expect(resolveSidebarContext('/projects')).toEqual({ kind: 'global' });
  });

  it('maps organization and profile paths', () => {
    expect(resolveSidebarContext('/organization/members')).toEqual({ kind: 'org-admin' });
    expect(resolveSidebarContext('/profile')).toEqual({ kind: 'profile' });
  });

  it('keeps every /profile/:section in the profile context', () => {
    expect(resolveSidebarContext('/profile/security')).toEqual({ kind: 'profile' });
    expect(resolveSidebarContext('/profile/appearance')).toEqual({ kind: 'profile' });
  });

  it('keeps the people pages in the global context (no sidebar)', () => {
    expect(resolveSidebarContext('/people')).toEqual({ kind: 'global' });
    expect(resolveSidebarContext('/people/u1')).toEqual({ kind: 'global' });
  });

  it('uses global for work pages and onboarding', () => {
    expect(resolveSidebarContext('/issues/new')).toEqual({ kind: 'global' });
    expect(resolveSidebarContext('/dashboard')).toEqual({ kind: 'global' });
    expect(resolveSidebarContext('/')).toEqual({ kind: 'global' });
    expect(resolveSidebarContext('/teams')).toEqual({ kind: 'global' });
  });
});

describe('buildSidebar', () => {
  it('has no items in the global context', () => {
    expect(buildSidebar({ kind: 'global' }, adminPerms)).toEqual([]);
  });

  it('lists no sidebar items on project pages, which have their own tabs', () => {
    const context = { kind: 'project', projectId: 'p1' } as const;
    expect(buildSidebar(context, new Set())).toEqual([]);
    expect(buildSidebar(context, adminPerms)).toEqual([]);
  });

  it('hides org admin items from users without org.admin', () => {
    expect(buildSidebar({ kind: 'org-admin' }, new Set())).toEqual([]);
    expect(buildSidebar({ kind: 'org-admin' }, adminPerms).map((i) => i.path)).toEqual([
      '/organization/settings',
      '/organization/members',
      '/organization/catalogs',
      '/organization/statuses',
      '/organization/invitations',
      '/organization/legal',
    ]);
  });

  it('lists every profile section for everyone and titles each context', () => {
    expect(buildSidebar({ kind: 'profile' }, new Set()).map((i) => i.path)).toEqual([
      '/profile',
      '/profile/security',
      '/profile/notifications',
      '/profile/appearance',
      '/profile/tokens',
      '/profile/privacy',
    ]);
    expect(sidebarTitleOf({ kind: 'project', projectId: 'p1' }, DEFAULT_SHELL_NAV_CONFIG)).toBe(
      'Proyecto',
    );
    expect(sidebarTitleOf({ kind: 'global' }, DEFAULT_SHELL_NAV_CONFIG)).toBe('');
  });
});

describe('isSidebarItemActive', () => {
  it('matches exact and nested paths but not a sibling prefix', () => {
    expect(isSidebarItemActive({ path: '/projects/p1/board' }, '/projects/p1/board')).toBe(true);
    expect(isSidebarItemActive({ path: '/projects/p1' }, '/projects/p1/board')).toBe(true);
    expect(isSidebarItemActive({ path: '/projects/p1' }, '/projects/p10')).toBe(false);
  });

  it('keeps an exact item inactive on the sections nested under its path', () => {
    const general = { path: '/profile', exact: true };
    expect(isSidebarItemActive(general, '/profile')).toBe(true);
    expect(isSidebarItemActive(general, '/profile/appearance')).toBe(false);
    expect(isSidebarItemActive({ path: '/profile/appearance' }, '/profile/appearance')).toBe(true);
  });
});
