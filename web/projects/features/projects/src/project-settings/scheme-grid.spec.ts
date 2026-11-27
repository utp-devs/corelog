import { describe, expect, it } from 'vitest';

import { groupPermissions, permissionCopy } from './permission-catalog';
import {
  defaultGrid,
  diffGrids,
  gridOf,
  grantsOf,
  hasHolder,
  schemeProblems,
  toggleHolder,
} from './scheme-grid';
import { SAMPLE_GRANTS, schemeOf } from './settings-test-kit';

const ORDER = Object.keys(SAMPLE_GRANTS);

describe('scheme grid', () => {
  it('round trips the grants in the given order with holders in column order', () => {
    const grid = gridOf(schemeOf({ create_issue: ['contributor', 'lead'] }).grants);
    expect(grantsOf(grid, ['create_issue', 'comment_issue'])).toEqual([
      { permission: 'create_issue', holders: ['lead', 'contributor'] },
      { permission: 'comment_issue', holders: [] },
    ]);
  });

  it('toggles a holder without touching the original grid', () => {
    const grid = gridOf(schemeOf(SAMPLE_GRANTS).grants);
    const next = toggleHolder(grid, 'create_issue', 'viewer', true);
    expect(hasHolder(next, 'create_issue', 'viewer')).toBe(true);
    expect(hasHolder(grid, 'create_issue', 'viewer')).toBe(false);
    const back = toggleHolder(next, 'create_issue', 'viewer', false);
    expect(hasHolder(back, 'create_issue', 'viewer')).toBe(false);
  });

  it('lists what was added and removed per permission', () => {
    const before = gridOf(schemeOf(SAMPLE_GRANTS).grants);
    let after = toggleHolder(before, 'create_issue', 'viewer', true);
    after = toggleHolder(after, 'create_issue', 'contributor', false);
    expect(diffGrids(before, after, ORDER)).toEqual([
      {
        permission: 'create_issue',
        label: 'Crear incidencias',
        added: ['viewer'],
        removed: ['contributor'],
      },
    ]);
    expect(diffGrids(before, before, ORDER)).toEqual([]);
  });

  it('knows the default scheme for the restore preview', () => {
    const defaults = defaultGrid(ORDER);
    expect(hasHolder(defaults, 'delete_project', 'org_admin')).toBe(true);
    expect(hasHolder(defaults, 'edit_own_issue', 'reporter')).toBe(true);
    expect(hasHolder(defaults, 'manage_members', 'contributor')).toBe(false);
  });

  it('reports the rules a scheme would break', () => {
    const grid = gridOf(schemeOf(SAMPLE_GRANTS).grants);
    expect(schemeProblems(grid)).toEqual([]);
    expect(schemeProblems(toggleHolder(grid, 'manage_members', 'lead', false))).toHaveLength(1);
    expect(schemeProblems(toggleHolder(grid, 'edit_project', 'lead', false))).toHaveLength(1);
    const noLead = toggleHolder(grid, 'delete_project', 'lead', false);
    const noDelete = toggleHolder(noLead, 'delete_project', 'org_admin', false);
    expect(schemeProblems(noDelete)).toHaveLength(1);
  });
});

describe('permission catalog', () => {
  it('groups known permissions in catalog order and keeps unknown ones in their own group', () => {
    const groups = groupPermissions(['create_issue', 'browse_project', 'brand_new_permission']);
    expect(groups.map((group) => group.key)).toEqual(['project', 'issues', 'other']);
    expect(groups[2].permissions).toEqual(['brand_new_permission']);
    expect(permissionCopy('brand_new_permission').label).toBe('brand_new_permission');
  });
});
