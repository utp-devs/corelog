import { projectPath, type RecentItem } from '@corelog/core';

import { VIEW_ALL_LABEL, type NavMenu } from './navigation';
import type { ShellIconName } from './shell-icon';

/** One row of a top menu. */
export interface MenuEntry {
  readonly key: string;
  readonly label: string;
  /** Router path, or an absolute path when `external` is set. */
  readonly path: string;
  readonly icon?: ShellIconName;
  /** Muted text after the label (project key, issue key, member count). */
  readonly hint?: string;
  readonly external?: boolean;
  readonly queryParams?: Readonly<Record<string, string>>;
  /** Project whose colored avatar leads the row (instead of an icon). */
  readonly avatar?: { readonly name: string; readonly key: string };
}

/** A titled group of rows. An empty group shows `emptyText`. */
export interface MenuSection {
  readonly key: string;
  readonly title: string;
  readonly entries: readonly MenuEntry[];
  readonly emptyText: string;
}

/** Where the data of a menu stands: its rows are read when the menu opens. */
export type MenuStatus = 'ready' | 'loading' | 'error';

export interface MenuModel {
  readonly status: MenuStatus;
  readonly sections: readonly MenuSection[];
  /** Links under the sections: "Ver todos ..." and the extras of the menu. */
  readonly footer: readonly MenuEntry[];
  /** Primary action ("Crear ..."), present only when the user may do it. */
  readonly create: MenuEntry | null;
}

/** Team the menus can list. */
export interface MenuSubject {
  readonly id: string;
  readonly name: string;
  readonly hint: string;
}

/** Project of the starred and recent lists. */
export interface ProjectMenuItem {
  readonly id: string;
  readonly name: string;
  readonly key: string;
}

/** What the menus need to know about the user and the loaded data. */
export interface MenuContext {
  /** Recently visited issues (local history). */
  readonly recent: readonly RecentItem[];
  readonly favoriteProjects: readonly ProjectMenuItem[];
  readonly recentProjects: readonly ProjectMenuItem[];
  /** State of the read of the starred and recent projects. */
  readonly projectsStatus: MenuStatus;
  readonly teams: readonly MenuSubject[];
  readonly canCreateProject: boolean;
  readonly canCreateTeam: boolean;
}

/** Most rows a section lists. */
export const MENU_SECTION_LIMIT = 5;

const VIEW_ALL_TARGETS: Readonly<Record<string, string>> = {
  projects: 'los proyectos',
  filters: 'las incidencias',
  dashboards: 'los paneles',
  teams: 'los equipos',
};

function hrefOf(menu: NavMenu, path: string): string {
  return `${menu.linkBase ?? ''}${path}`;
}

function recentEntries(menu: NavMenu, items: readonly RecentItem[], icon: ShellIconName) {
  return items.slice(0, MENU_SECTION_LIMIT).map<MenuEntry>((item) => ({
    key: `${item.kind}-${item.id}`,
    label: item.title,
    path: hrefOf(menu, item.link),
    icon,
    hint: item.subtitle,
    external: menu.external,
  }));
}

function projectEntry(menu: NavMenu, project: ProjectMenuItem, group: string): MenuEntry {
  return {
    key: `${group}-${project.id}`,
    label: project.name,
    path: hrefOf(menu, projectPath(project)),
    hint: project.key,
    external: menu.external,
    avatar: { name: project.name, key: project.key },
  };
}

/** Starred projects and recent ones, without repeating a project in both groups. */
function projectSections(menu: NavMenu, context: MenuContext): MenuSection[] {
  const starredIds = new Set(context.favoriteProjects.map((project) => project.id));
  const recent = context.recentProjects.filter((project) => !starredIds.has(project.id));
  const sections: MenuSection[] = [
    {
      key: 'favorites',
      title: 'Destacados',
      entries: context.favoriteProjects
        .slice(0, MENU_SECTION_LIMIT)
        .map((project) => projectEntry(menu, project, 'favorite')),
      emptyText: 'Marca un proyecto con la estrella para tenerlo aquí.',
    },
  ];
  // Recent projects that are all starred already are listed above: no empty group then.
  if (recent.length > 0 || context.recentProjects.length === 0) {
    sections.push({
      key: 'recent',
      title: 'Recientes',
      entries: recent
        .slice(0, MENU_SECTION_LIMIT)
        .map((project) => projectEntry(menu, project, 'recent')),
      emptyText: 'Aún no has visitado proyectos',
    });
  }
  return sections;
}

function sectionsOf(menu: NavMenu, context: MenuContext): MenuSection[] {
  switch (menu.key) {
    case 'projects':
      return projectSections(menu, context);
    case 'filters':
      return [
        {
          key: 'recent',
          title: 'Recientes',
          entries: recentEntries(
            menu,
            context.recent.filter((item) => item.kind === 'issue'),
            'issue',
          ),
          emptyText: 'Aún no has visitado incidencias.',
        },
      ];
    case 'dashboards':
      return [
        {
          key: 'dashboards',
          title: 'Tus paneles',
          entries: [
            {
              key: 'summary',
              label: 'Mi resumen',
              path: menu.path,
              icon: 'dashboard',
              external: menu.external,
            },
          ],
          emptyText: '',
        },
      ];
    case 'teams':
      return [
        {
          key: 'teams',
          title: 'Tus equipos',
          entries: context.teams.slice(0, MENU_SECTION_LIMIT).map<MenuEntry>((team) => ({
            key: `team-${team.id}`,
            label: team.name,
            path: hrefOf(menu, `/teams/${encodeURIComponent(team.id)}`),
            icon: 'team',
            hint: team.hint,
            external: menu.external,
          })),
          emptyText: 'Aún no perteneces a equipos.',
        },
      ];
    default:
      return [];
  }
}

function createOf(menu: NavMenu, context: MenuContext): MenuEntry | null {
  if (menu.key === 'projects' && context.canCreateProject) {
    return {
      key: 'create-project',
      label: 'Crear proyecto',
      path: hrefOf(menu, '/projects/new'),
      icon: 'plus',
      external: menu.external,
    };
  }
  if (menu.key === 'teams' && context.canCreateTeam) {
    return {
      key: 'create-team',
      label: 'Crear equipo',
      path: menu.path,
      icon: 'plus',
      external: menu.external,
      queryParams: { new: '1' },
    };
  }
  return null;
}

/** Rows of a top menu: sections with real items, the "view all" links and the create action. */
export function buildMenuModel(menu: NavMenu, context: MenuContext): MenuModel {
  const target = VIEW_ALL_TARGETS[menu.key];
  const footer: MenuEntry[] = [
    {
      key: 'view-all',
      label: target ? `${VIEW_ALL_LABEL} ${target}` : VIEW_ALL_LABEL,
      path: menu.path,
      external: menu.external,
    },
    ...(menu.extras ?? []).map<MenuEntry>((link) => ({
      key: link.key,
      label: link.label,
      path: link.path,
      external: link.external,
      icon: link.key === 'people' ? 'members' : undefined,
    })),
  ];
  const status = menu.key === 'projects' ? context.projectsStatus : 'ready';
  return { status, sections: sectionsOf(menu, context), footer, create: createOf(menu, context) };
}

/** Icon of each top menu in the drawer. */
export const MENU_ICONS: Readonly<Record<string, ShellIconName>> = {
  projects: 'folder',
  filters: 'issue',
  dashboards: 'dashboard',
  teams: 'team',
};
