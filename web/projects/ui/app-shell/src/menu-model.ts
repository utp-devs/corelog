import type { RecentItem } from '@corelog/core';

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
}

/** A titled group of rows. An empty group shows `emptyText`. */
export interface MenuSection {
  readonly key: string;
  readonly title: string;
  readonly entries: readonly MenuEntry[];
  readonly emptyText: string;
}

export interface MenuModel {
  readonly sections: readonly MenuSection[];
  /** Links under the sections: "Ver todos ..." and the extras of the menu. */
  readonly footer: readonly MenuEntry[];
  /** Primary action ("Crear ..."), present only when the user may do it. */
  readonly create: MenuEntry | null;
}

/** Project or team the menus can list. */
export interface MenuSubject {
  readonly id: string;
  readonly name: string;
  readonly hint: string;
}

/** What the menus need to know about the user and the loaded data. */
export interface MenuContext {
  readonly recent: readonly RecentItem[];
  readonly projects: readonly MenuSubject[];
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

function sectionsOf(menu: NavMenu, context: MenuContext): MenuSection[] {
  switch (menu.key) {
    case 'projects': {
      const recent = recentEntries(
        menu,
        context.recent.filter((item) => item.kind === 'project'),
        'folder',
      );
      if (recent.length > 0) {
        return [{ key: 'recent', title: 'Recientes', entries: recent, emptyText: '' }];
      }
      const own = context.projects.slice(0, MENU_SECTION_LIMIT).map<MenuEntry>((project) => ({
        key: `project-${project.id}`,
        label: project.name,
        path: hrefOf(menu, `/projects/${encodeURIComponent(project.id)}/board`),
        icon: 'folder',
        hint: project.hint,
        external: menu.external,
      }));
      return [
        { key: 'own', title: 'Tus proyectos', entries: own, emptyText: 'Aún no hay proyectos.' },
      ];
    }
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
      path: menu.path,
      icon: 'plus',
      external: menu.external,
      queryParams: { new: '1' },
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
  return { sections: sectionsOf(menu, context), footer, create: createOf(menu, context) };
}

/** Icon of each top menu in the drawer. */
export const MENU_ICONS: Readonly<Record<string, ShellIconName>> = {
  projects: 'folder',
  filters: 'issue',
  dashboards: 'dashboard',
  teams: 'team',
};
