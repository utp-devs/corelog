import { permissionsFor, type OrgRole } from '@corelog/core';
import {
  buildSidebar,
  buildTopNav,
  isSidebarItemActive,
  resolveSidebarContext,
  sidebarTitleOf,
} from '@corelog/ui/app-shell';

import { WORKSPACE_NAV } from './nav.config';

const member = { role: 'user' as const };
const platformAdmin = { role: 'admin' as const };
const org = (role: OrgRole) => ({ id: 'o1', name: 'Acme', slug: 'acme', role });

describe('workspace navigation', () => {
  describe('top menus', () => {
    it('lists Proyectos, Filtros, Paneles and Equipos in that order', () => {
      const { menus } = buildTopNav(org('member'), WORKSPACE_NAV);
      expect(menus.map((menu) => menu.label)).toEqual([
        'Proyectos',
        'Filtros',
        'Paneles',
        'Equipos',
      ]);
      expect(menus.map((menu) => menu.path)).toEqual([
        '/projects',
        '/issues',
        '/dashboard',
        '/teams',
      ]);
    });

    it('offers Equipos to every member, with Ver personas pointing at /people', () => {
      const teams = buildTopNav(null, WORKSPACE_NAV).menus.find((menu) => menu.key === 'teams');
      expect(teams?.extras).toEqual([{ key: 'people', label: 'Ver personas', path: '/people' }]);
    });

    it('shows the organization name only when there is an organization', () => {
      expect(buildTopNav(null, WORKSPACE_NAV).orgName).toBeNull();
      expect(buildTopNav(org('member'), WORKSPACE_NAV).orgName).toBe('Acme');
    });

    it('keeps every menu target inside the workspace', () => {
      const targets = buildTopNav(null, WORKSPACE_NAV).menus.flatMap((menu) => [
        menu.path,
        ...(menu.extras ?? []).map((link) => link.path),
      ]);
      expect(targets.some((target) => target.startsWith('/admin/'))).toBe(false);
    });
  });

  describe('permissions', () => {
    it('grants org administration to platform admins and to owners and admins of the org', () => {
      expect([...permissionsFor(platformAdmin, 'member')]).toEqual([
        'org.admin',
        'project.permissions',
        'platform.admin',
      ]);
      expect(permissionsFor(member, 'owner').has('org.admin')).toBe(true);
      expect(permissionsFor(member, 'admin').has('org.admin')).toBe(true);
    });

    it('withholds org administration from members and guests', () => {
      expect(permissionsFor(member, 'member').size).toBe(0);
      expect(permissionsFor(member, 'guest').size).toBe(0);
    });
  });

  describe('sidebar contexts', () => {
    it('maps project paths to the project context with the decoded id', () => {
      expect(resolveSidebarContext('/projects/ab%20cd/board')).toEqual({
        kind: 'project',
        projectId: 'ab cd',
      });
    });

    it('keeps /projects without an id in the global context', () => {
      expect(resolveSidebarContext('/projects').kind).toBe('global');
    });

    it('maps organization and profile paths to their contexts', () => {
      expect(resolveSidebarContext('/organization/members').kind).toBe('org-admin');
      expect(resolveSidebarContext('/profile').kind).toBe('profile');
      expect(resolveSidebarContext('/profile/security').kind).toBe('profile');
    });

    it('keeps people, issues and dashboards in the global context (no sidebar)', () => {
      for (const path of ['/people', '/people/u1', '/issues', '/issues/new', '/dashboard']) {
        expect(resolveSidebarContext(path).kind).toBe('global');
      }
      expect(sidebarTitleOf(resolveSidebarContext('/issues'), WORKSPACE_NAV)).toBe('');
    });
  });

  describe('sidebar items', () => {
    it('has no items in the global context', () => {
      expect(buildSidebar({ kind: 'global' }, new Set(), WORKSPACE_NAV)).toEqual([]);
    });

    it('lists no sidebar items on project pages, which have their own tabs', () => {
      const context = { kind: 'project' as const, projectId: 'p1' };
      expect(buildSidebar(context, new Set(), WORKSPACE_NAV)).toEqual([]);
      expect(buildSidebar(context, permissionsFor(platformAdmin), WORKSPACE_NAV)).toEqual([]);
    });

    it('hides organization administration from users without org.admin', () => {
      const context = { kind: 'org-admin' as const };
      expect(buildSidebar(context, permissionsFor(member, 'member'), WORKSPACE_NAV)).toEqual([]);
      expect(
        buildSidebar(context, permissionsFor(member, 'owner'), WORKSPACE_NAV).map(
          (item) => item.path,
        ),
      ).toEqual([
        '/admin/organization/settings',
        '/admin/organization/members',
        '/admin/organization/invitations',
        '/admin/organization/audit',
      ]);
    });

    it('lists every profile section for everyone and titles each context', () => {
      const items = buildSidebar({ kind: 'profile' }, new Set(), WORKSPACE_NAV);
      expect(items.map((item) => [item.label, item.path])).toEqual([
        ['General', '/profile'],
        ['Seguridad', '/profile/security'],
        ['Notificaciones', '/profile/notifications'],
        ['Apariencia', '/profile/appearance'],
        ['Privacidad', '/profile/privacy'],
      ]);
      expect(items[0].exact).toBe(true);
      expect(sidebarTitleOf({ kind: 'profile' }, WORKSPACE_NAV)).toBe('Mi cuenta');
      expect(sidebarTitleOf({ kind: 'org-admin' }, WORKSPACE_NAV)).toBe('Organización');
    });
  });

  describe('active item', () => {
    it('matches exact and nested paths but not a sibling prefix', () => {
      expect(isSidebarItemActive({ path: '/projects/p1/board' }, '/projects/p1/board')).toBe(true);
      expect(isSidebarItemActive({ path: '/projects/p1/board' }, '/projects/p1/board/x')).toBe(
        true,
      );
      expect(isSidebarItemActive({ path: '/projects/p1/board' }, '/projects/p1/boards')).toBe(
        false,
      );
    });
  });
});
