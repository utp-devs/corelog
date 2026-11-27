import { describe, expect, it } from 'vitest';

import {
  INITIAL_LIST_STATE,
  clearedFilters,
  createProjectRights,
  hasActiveFilters,
  toListFilters,
} from './projects-page.utils';

describe('toListFilters', () => {
  it('sends only the filters that are set', () => {
    expect(toListFilters(INITIAL_LIST_STATE)).toEqual({
      archived: 'exclude',
      sort: 'name',
      desc: false,
      page: 1,
      pageSize: 20,
    });
    expect(
      toListFilters({
        ...INITIAL_LIST_STATE,
        q: '  core ',
        template: 'scrum',
        leadId: 'u1',
        starred: true,
        sort: 'updated_at:desc',
      }),
    ).toEqual({
      q: 'core',
      template: 'scrum',
      leadId: 'u1',
      starred: true,
      archived: 'exclude',
      sort: 'updated_at',
      desc: true,
      page: 1,
      pageSize: 20,
    });
  });
});

describe('hasActiveFilters and clearedFilters', () => {
  it('ignores sort and paging', () => {
    expect(hasActiveFilters({ ...INITIAL_LIST_STATE, sort: 'key:asc', page: 3 })).toBe(false);
    expect(hasActiveFilters({ ...INITIAL_LIST_STATE, archived: 'only' })).toBe(true);
    expect(hasActiveFilters({ ...INITIAL_LIST_STATE, q: 'x' })).toBe(true);
  });

  it('clears filters but keeps sort and page size', () => {
    const cleared = clearedFilters({
      ...INITIAL_LIST_STATE,
      q: 'x',
      starred: true,
      sort: 'key:asc',
      pageSize: 50,
      page: 4,
    });
    expect(cleared).toEqual({ ...INITIAL_LIST_STATE, sort: 'key:asc', pageSize: 50 });
  });
});

describe('createProjectRights', () => {
  it('lets global admins and organization owners and admins create', () => {
    expect(createProjectRights('admin', null, 'admins').allowed).toBe(true);
    expect(createProjectRights('user', 'owner', 'admins').allowed).toBe(true);
    expect(createProjectRights('user', 'admin', 'admins').allowed).toBe(true);
  });

  it('lets members create only when the policy allows it', () => {
    expect(createProjectRights('user', 'member', 'members').allowed).toBe(true);
    const closed = createProjectRights('user', 'member', 'admins');
    expect(closed.allowed).toBe(false);
    expect(closed.reason).toMatch(/administradores/);
  });

  it('never lets guests create', () => {
    const guest = createProjectRights('user', 'guest', 'members');
    expect(guest.allowed).toBe(false);
    expect(guest.reason).toMatch(/invitados/);
  });
});
