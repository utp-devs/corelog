import { describe, expect, it } from 'vitest'

import {
  buildSidebar,
  buildTopNav,
  isSidebarItemActive,
  permissionsFor,
  resolveSidebarContext,
  type OrgInfo,
} from './navigation'

const org: OrgInfo = { id: 'o1', name: 'Acme', slug: 'acme' }
const adminPerms = permissionsFor({ role: 'admin' })
const BASE_MENUS = ['projects', 'filters', 'dashboards', 'teams']

describe('buildTopNav', () => {
  it('shows the base menus and Equipos to a regular user', () => {
    const nav = buildTopNav({ role: 'user' }, null)
    expect(nav.menus.map((m) => m.key)).toEqual(BASE_MENUS)
    expect(nav.canCreate).toBe(true)
    expect(nav.orgName).toBeNull()
  })

  it('offers Equipos to every role, not only admins', () => {
    const nav = buildTopNav({ role: 'admin' }, org)
    expect(nav.menus.find((m) => m.key === 'teams')?.path).toBe('/teams')
    expect(nav.orgName).toBe('Acme')
    expect(buildTopNav({ role: 'user' }, org).menus.map((m) => m.key)).toContain('teams')
  })

  it('keeps every existing route reachable from the menus', () => {
    const paths = buildTopNav({ role: 'admin' }, null).menus.map((m) => m.path)
    expect(paths).toEqual(expect.arrayContaining(['/projects', '/issues', '/dashboard', '/teams']))
  })

  it('has no create action for an anonymous user but keeps the menus', () => {
    const nav = buildTopNav(null, null)
    expect(nav.canCreate).toBe(false)
    expect(nav.menus.map((m) => m.key)).toEqual(BASE_MENUS)
  })

  it('lists Ver personas under Equipos, pointing at /people', () => {
    const teams = buildTopNav({ role: 'user' }, org).menus.find((m) => m.key === 'teams')
    expect(teams?.extras).toEqual([{ key: 'people', label: 'Ver personas', path: '/people' }])
    expect(buildTopNav({ role: 'user' }, org).menus.find((m) => m.key === 'projects')?.extras).toBeUndefined()
  })

  it('starts recent lists empty', () => {
    expect(buildTopNav({ role: 'user' }, null).menus.every((m) => m.recent.length === 0)).toBe(true)
  })
})

describe('permissionsFor', () => {
  it('grants org and project administration to platform admins', () => {
    expect(adminPerms.has('org.admin')).toBe(true)
    expect(adminPerms.has('project.permissions')).toBe(true)
    expect(permissionsFor({ role: 'user' }).size).toBe(0)
    expect(permissionsFor(null).size).toBe(0)
  })

  it('grants org administration to owners and admins of the active organization', () => {
    expect(permissionsFor({ role: 'user' }, 'owner').has('org.admin')).toBe(true)
    expect(permissionsFor({ role: 'user' }, 'admin').has('org.admin')).toBe(true)
  })

  it('withholds org administration from members and guests and never grants project permissions from the org role', () => {
    expect(permissionsFor({ role: 'user' }, 'member').size).toBe(0)
    expect(permissionsFor({ role: 'user' }, 'guest').size).toBe(0)
    expect(permissionsFor({ role: 'user' }, 'owner').has('project.permissions')).toBe(false)
  })
})

describe('resolveSidebarContext', () => {
  it('maps project paths to the project context', () => {
    expect(resolveSidebarContext('/projects/p1/board')).toEqual({ kind: 'project', projectId: 'p1' })
    expect(resolveSidebarContext('/projects/p1')).toEqual({ kind: 'project', projectId: 'p1' })
  })

  it('keeps /projects without id in the global context', () => {
    expect(resolveSidebarContext('/projects')).toEqual({ kind: 'global' })
  })

  it('maps organization and profile paths', () => {
    expect(resolveSidebarContext('/organization/members')).toEqual({ kind: 'org-admin' })
    expect(resolveSidebarContext('/profile')).toEqual({ kind: 'profile' })
  })

  it('keeps every /profile/:section in the profile context', () => {
    expect(resolveSidebarContext('/profile/security')).toEqual({ kind: 'profile' })
    expect(resolveSidebarContext('/profile/appearance')).toEqual({ kind: 'profile' })
  })

  it('keeps the people pages in the global context (no sidebar)', () => {
    expect(resolveSidebarContext('/people')).toEqual({ kind: 'global' })
    expect(resolveSidebarContext('/people/u1')).toEqual({ kind: 'global' })
  })

  it('uses global for work pages and onboarding', () => {
    expect(resolveSidebarContext('/issues/new')).toEqual({ kind: 'global' })
    expect(resolveSidebarContext('/dashboard')).toEqual({ kind: 'global' })
    expect(resolveSidebarContext('/')).toEqual({ kind: 'global' })
    expect(resolveSidebarContext('/teams')).toEqual({ kind: 'global' })
  })
})

describe('buildSidebar', () => {
  it('has no items in the global context', () => {
    expect(buildSidebar({ kind: 'global' }, adminPerms)).toEqual([])
  })

  it('lists project items under the project base path', () => {
    const items = buildSidebar({ kind: 'project', projectId: 'p1' }, new Set())
    expect(items.map((i) => i.key)).toEqual(['board', 'backlog', 'sprints', 'calendar'])
    expect(items[0].path).toBe('/projects/p1/board')
  })

  it('hides project permissions without the permission and shows it with it', () => {
    const context = { kind: 'project', projectId: 'p1' } as const
    expect(buildSidebar(context, new Set())).not.toContainEqual(expect.objectContaining({ key: 'permissions' }))
    expect(buildSidebar(context, adminPerms)).toContainEqual(expect.objectContaining({ key: 'permissions', path: '/projects/p1/permisos' }))
  })

  it('hides org admin items from users without org.admin', () => {
    expect(buildSidebar({ kind: 'org-admin' }, new Set())).toEqual([])
    expect(buildSidebar({ kind: 'org-admin' }, adminPerms).map((i) => i.path)).toEqual([
      '/organization/settings',
      '/organization/members',
      '/organization/invitations',
    ])
  })

  it('lists the profile item for everyone', () => {
    expect(buildSidebar({ kind: 'profile' }, new Set()).map((i) => i.path)).toEqual(['/profile'])
  })
})

describe('isSidebarItemActive', () => {
  it('matches exact and nested paths but not a sibling prefix', () => {
    expect(isSidebarItemActive({ path: '/projects/p1/board' }, '/projects/p1/board')).toBe(true)
    expect(isSidebarItemActive({ path: '/projects/p1' }, '/projects/p1/board')).toBe(true)
    expect(isSidebarItemActive({ path: '/projects/p1' }, '/projects/p10')).toBe(false)
  })
})
