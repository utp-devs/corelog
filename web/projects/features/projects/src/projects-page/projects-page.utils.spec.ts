import { describe, expect, it } from 'vitest';
import type { Project } from '@corelog/core';

import { createProjectRights, filterProjects } from './projects-page.utils';

function project(overrides: Partial<Project>): Project {
  return {
    id: 'p',
    organizationId: 'o1',
    key: 'CORE',
    name: 'CoreLog',
    description: '',
    type: 'kanban',
    leadId: '',
    visibility: 'open',
    archivedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

const list: Project[] = [
  project({ id: 'p-1', key: 'CORE', name: 'CoreLog' }),
  project({ id: 'p-2', key: 'SEC', name: 'Seguridad', visibility: 'private' }),
  project({ id: 'p-3', key: 'OLD', name: 'Heredado', archivedAt: '2026-02-01T00:00:00Z' }),
];

describe('filterProjects', () => {
  it('hides archived projects by default and matches name or key', () => {
    expect(filterProjects(list, '', 'all', 'active').map((p) => p.id)).toEqual(['p-1', 'p-2']);
    expect(filterProjects(list, 'sec', 'all', 'active').map((p) => p.id)).toEqual(['p-2']);
    expect(filterProjects(list, '  core ', 'all', 'all').map((p) => p.id)).toEqual(['p-1']);
  });

  it('filters by visibility and by archived state', () => {
    expect(filterProjects(list, '', 'private', 'all').map((p) => p.id)).toEqual(['p-2']);
    expect(filterProjects(list, '', 'all', 'archived').map((p) => p.id)).toEqual(['p-3']);
  });
});

describe('createProjectRights', () => {
  it('allows global admins and organization owners and admins', () => {
    expect(createProjectRights('admin', null, 'admins').allowed).toBe(true);
    expect(createProjectRights('user', 'owner', 'admins').allowed).toBe(true);
    expect(createProjectRights('user', 'admin', 'admins').allowed).toBe(true);
  });

  it('lets members create only when the policy says so', () => {
    expect(createProjectRights('user', 'member', 'members').allowed).toBe(true);
    const closed = createProjectRights('user', 'member', 'admins');
    expect(closed.allowed).toBe(false);
    expect(closed.reason).toContain('administradores');
  });

  it('never lets guests create', () => {
    const guest = createProjectRights('user', 'guest', 'members');
    expect(guest.allowed).toBe(false);
    expect(guest.reason).toContain('invitados');
  });
});
