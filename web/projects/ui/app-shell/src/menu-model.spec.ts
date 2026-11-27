import type { RecentItem } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import {
  MENU_SECTION_LIMIT,
  buildMenuModel,
  type MenuContext,
  type ProjectMenuItem,
} from './menu-model';
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

function project(index: number): ProjectMenuItem {
  return { id: `id-${index}`, name: `Proyecto ${index}`, key: `K${index}` };
}

const empty: MenuContext = {
  recent: [],
  favoriteProjects: [],
  recentProjects: [],
  projectsStatus: 'ready',
  teams: [],
  canCreateProject: false,
  canCreateTeam: false,
};

describe('buildMenuModel', () => {
  it('groups the projects menu in starred and recent, capped and with a colored avatar', () => {
    const model = buildMenuModel(projects, {
      ...empty,
      favoriteProjects: Array.from({ length: 7 }, (_, index) => project(index)),
      recentProjects: Array.from({ length: 8 }, (_, index) => project(index + 20)),
    });
    expect(model.sections.map((section) => section.title)).toEqual(['Destacados', 'Recientes']);
    expect(model.sections[0].entries).toHaveLength(MENU_SECTION_LIMIT);
    expect(model.sections[1].entries).toHaveLength(MENU_SECTION_LIMIT);
    expect(model.sections[0].entries[0]).toMatchObject({
      label: 'Proyecto 0',
      hint: 'K0',
      path: '/projects/K0',
      avatar: { name: 'Proyecto 0', key: 'K0' },
    });
  });

  it('does not repeat a starred project in the recent group', () => {
    const model = buildMenuModel(projects, {
      ...empty,
      favoriteProjects: [project(1)],
      recentProjects: [project(1), project(2)],
    });
    expect(model.sections[1].entries.map((entry) => entry.label)).toEqual(['Proyecto 2']);
  });

  it('drops the recent group when every recent project is already starred', () => {
    const model = buildMenuModel(projects, {
      ...empty,
      favoriteProjects: [project(1)],
      recentProjects: [project(1)],
    });
    expect(model.sections.map((section) => section.key)).toEqual(['favorites']);
  });

  it('says so when no project was visited yet', () => {
    const model = buildMenuModel(projects, empty);
    expect(model.sections[1].entries).toEqual([]);
    expect(model.sections[1].emptyText).toBe('Aún no has visitado proyectos');
    expect(model.sections[0].emptyText).not.toBe('');
  });

  it('carries the read status of the projects menu', () => {
    expect(buildMenuModel(projects, { ...empty, projectsStatus: 'loading' }).status).toBe(
      'loading',
    );
    expect(buildMenuModel(teams, { ...empty, projectsStatus: 'error' }).status).toBe('ready');
  });

  it('prefixes project links and flags them external for a menu of another app', () => {
    const model = buildMenuModel(adminProjects, { ...empty, recentProjects: [project(1)] });
    expect(model.sections[1].entries[0]).toMatchObject({
      path: '/workspace/projects/K1',
      external: true,
    });
    expect(model.footer[0]).toMatchObject({ path: '/workspace/projects', external: true });
  });

  it('links the create action to the new project page', () => {
    expect(buildMenuModel(projects, { ...empty, canCreateProject: true }).create).toMatchObject({
      label: 'Crear proyecto',
      path: '/projects/new',
    });
    expect(buildMenuModel(adminProjects, { ...empty, canCreateProject: true }).create?.path).toBe(
      '/workspace/projects/new',
    );
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
