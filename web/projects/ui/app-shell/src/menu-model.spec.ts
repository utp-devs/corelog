import type { RecentItem } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { MENU_SECTION_LIMIT, buildMenuModel, type MenuContext } from './menu-model';
import type { NavMenu } from './navigation';

const projects: NavMenu = { key: 'projects', label: 'Proyectos', path: '/projects' };
const adminProjects: NavMenu = {
  key: 'projects',
  label: 'Proyectos',
  path: '/workspace/projects',
  external: true,
  linkBase: '/workspace',
};
const teams: NavMenu = {
  key: 'teams',
  label: 'Equipos',
  path: '/teams',
  extras: [{ key: 'people', label: 'Ver personas', path: '/people' }],
};

function recent(kind: 'project' | 'issue', index: number): RecentItem {
  return {
    kind,
    id: `${kind}-${index}`,
    title: `${kind} ${index}`,
    subtitle: `K-${index}`,
    link: kind === 'project' ? `/projects/${kind}-${index}/board` : `/issues/${kind}-${index}`,
  };
}

const empty: MenuContext = {
  recent: [],
  projects: [],
  teams: [],
  canCreateProject: false,
  canCreateTeam: false,
};

describe('buildMenuModel', () => {
  it('shows only recent projects in the projects menu and caps the list', () => {
    const items = [
      ...Array.from({ length: 8 }, (_, index) => recent('project', index)),
      recent('issue', 1),
    ];
    const model = buildMenuModel(projects, { ...empty, recent: items });
    expect(model.sections).toHaveLength(1);
    expect(model.sections[0].title).toBe('Recientes');
    expect(model.sections[0].entries).toHaveLength(MENU_SECTION_LIMIT);
    expect(model.sections[0].entries[0]).toMatchObject({
      label: 'project 0',
      hint: 'K-0',
      path: '/projects/project-0/board',
    });
  });

  it('falls back to the own projects when nothing was visited', () => {
    const model = buildMenuModel(projects, {
      ...empty,
      projects: [{ id: 'p1', name: 'Demo', hint: 'DEM' }],
    });
    expect(model.sections[0].title).toBe('Tus proyectos');
    expect(model.sections[0].entries[0].path).toBe('/projects/p1/board');
  });

  it('keeps the empty text when there is nothing to list', () => {
    const model = buildMenuModel(projects, empty);
    expect(model.sections[0].entries).toEqual([]);
    expect(model.sections[0].emptyText).not.toBe('');
  });

  it('prefixes recent links and flags them external for a menu of another app', () => {
    const model = buildMenuModel(adminProjects, { ...empty, recent: [recent('project', 1)] });
    expect(model.sections[0].entries[0]).toMatchObject({
      path: '/workspace/projects/project-1/board',
      external: true,
    });
    expect(model.footer[0]).toMatchObject({ path: '/workspace/projects', external: true });
  });

  it('adds the view-all link, the extras and the create action', () => {
    const model = buildMenuModel(teams, { ...empty, canCreateTeam: true });
    expect(model.footer.map((entry) => entry.label)).toEqual([
      'Ver todos los equipos',
      'Ver personas',
    ]);
    expect(model.create).toMatchObject({ label: 'Crear equipo', queryParams: { new: '1' } });
  });

  it('omits the create action without permission', () => {
    expect(buildMenuModel(projects, empty).create).toBeNull();
    expect(buildMenuModel(projects, { ...empty, canCreateProject: true }).create?.label).toBe(
      'Crear proyecto',
    );
  });

  it('lists only recent issues in the filters menu', () => {
    const model = buildMenuModel(
      { key: 'filters', label: 'Filtros', path: '/issues' },
      { ...empty, recent: [recent('project', 1), recent('issue', 2)] },
    );
    expect(model.sections[0].entries.map((entry) => entry.label)).toEqual(['issue 2']);
  });
});
