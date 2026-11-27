import { permissionsFor, type OrgRole } from '@corelog/core';
import {
  buildSidebar,
  buildTopNav,
  isSidebarItemActive,
  resolveSidebarContext,
  sidebarTitleOf,
} from '@corelog/ui/app-shell';

import { ADMIN_BRAND, ADMIN_NAV } from './nav.config';

const member = { role: 'user' as const };
const platformAdmin = { role: 'admin' as const };
const org = (role: OrgRole) => ({ id: 'o1', name: 'Acme', slug: 'acme', role });

describe('admin navigation', () => {
  describe('top menus', () => {
    it('lists the same four menus as the workspace, pointing at the workspace', () => {
      const { menus } = buildTopNav(org('owner'), ADMIN_NAV);
      expect(menus.map((menu) => menu.label)).toEqual([
        'Proyectos',
        'Filtros',
        'Paneles',
        'Equipos',
      ]);
      expect(menus.map((menu) => menu.path)).toEqual([
        '/workspace/projects',
        '/workspace/issues',
        '/workspace/dashboard',
        '/workspace/teams',
      ]);
    });

    it('lists Ver personas under Equipos, pointing at the workspace people page', () => {
      const teams = buildTopNav(null, ADMIN_NAV).menus.find((menu) => menu.key === 'teams');
      expect(teams?.extras).toEqual([
        { key: 'people', label: 'Ver personas', path: '/workspace/people', external: true },
      ]);
    });

    it('links the brand to the workspace home', () => {
      expect(ADMIN_BRAND.homePath).toBe('/workspace/issues');
      expect(ADMIN_BRAND.ariaLabel).toBe('CoreLog, ir a incidencias');
    });
  });

  describe('permissions', () => {
    it('grants organization administration to owners and admins only', () => {
      expect(permissionsFor(member, 'owner').has('org.admin')).toBe(true);
      expect(permissionsFor(member, 'admin').has('org.admin')).toBe(true);
      expect(permissionsFor(member, 'member').has('org.admin')).toBe(false);
    });

    it('grants project permissions to platform admins only', () => {
      expect(permissionsFor(platformAdmin, 'member').has('project.permissions')).toBe(true);
      expect(permissionsFor(member, 'owner').has('project.permissions')).toBe(false);
    });
  });

  describe('sidebar contexts', () => {
    it('maps organization paths to the org-admin context', () => {
      expect(resolveSidebarContext('/organization/settings').kind).toBe('org-admin');
      expect(resolveSidebarContext('/organization/invitations').kind).toBe('org-admin');
    });

    it('maps profile and project paths to their contexts', () => {
      expect(resolveSidebarContext('/profile').kind).toBe('profile');
      expect(resolveSidebarContext('/projects/p1/board')).toEqual({
        kind: 'project',
        projectId: 'p1',
      });
    });

    it('keeps the workspace paths in the global context (no sidebar)', () => {
      expect(resolveSidebarContext('/workspace/projects').kind).toBe('global');
      expect(resolveSidebarContext('/workspace/profile').kind).toBe('global');
      expect(sidebarTitleOf(resolveSidebarContext('/workspace/people'), ADMIN_NAV)).toBe('');
    });
  });

  describe('sidebar items', () => {
    it('lists the organization administration items for owners, with local paths', () => {
      const items = buildSidebar({ kind: 'org-admin' }, permissionsFor(member, 'owner'), ADMIN_NAV);
      expect(items.map((item) => [item.label, item.path])).toEqual([
        ['Configuración', '/organization/settings'],
        ['Miembros', '/organization/members'],
        ['Invitaciones', '/organization/invitations'],
      ]);
    });

    it('hides organization administration from members', () => {
      expect(
        buildSidebar({ kind: 'org-admin' }, permissionsFor(member, 'member'), ADMIN_NAV),
      ).toEqual([]);
    });

    it('lists the project items under the workspace project path', () => {
      const items = buildSidebar({ kind: 'project', projectId: 'p1' }, new Set(), ADMIN_NAV);
      expect(items.map((item) => item.path)).toEqual([
        '/workspace/projects/p1/board',
        '/workspace/projects/p1/backlog',
        '/workspace/projects/p1/sprints',
        '/workspace/projects/p1/calendar',
      ]);
    });

    it('shows project permissions only with the permission', () => {
      const context = { kind: 'project' as const, projectId: 'p1' };
      expect(
        buildSidebar(context, permissionsFor(member), ADMIN_NAV).map((item) => item.key),
      ).not.toContain('permissions');
      expect(buildSidebar(context, permissionsFor(platformAdmin), ADMIN_NAV).at(-1)?.path).toBe(
        '/workspace/projects/p1/permisos',
      );
    });

    it('lists the profile item under the workspace for everyone', () => {
      expect(buildSidebar({ kind: 'profile' }, new Set(), ADMIN_NAV)).toEqual([
        expect.objectContaining({ key: 'profile', path: '/workspace/profile' }),
      ]);
    });
  });

  describe('active item', () => {
    it('matches exact and nested paths but not a sibling prefix', () => {
      expect(
        isSidebarItemActive({ path: '/organization/members' }, '/organization/members/x'),
      ).toBe(true);
      expect(isSidebarItemActive({ path: '/organization/members' }, '/organization/membersx')).toBe(
        false,
      );
    });
  });
});
