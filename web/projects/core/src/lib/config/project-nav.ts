import type { Project } from '../models';
import { ProjectRef, ProjectSection, projectPath } from './project-paths';

/** One entry of the navigation of a project (sidebar or tabs). `path` is a router path. */
export interface ProjectNavItem {
  readonly key: ProjectSection;
  readonly label: string;
  readonly path: string;
}

interface ProjectNavDefinition {
  readonly key: ProjectSection;
  readonly label: string;
  /** Permission the caller needs to see the entry. */
  readonly permission: string;
  /** Only projects with sprints (scrum template) list the entry. */
  readonly scrumOnly?: boolean;
}

/** Order of the entries. Sections added to the project area are listed here. */
const PROJECT_NAV: readonly ProjectNavDefinition[] = [
  { key: 'board', label: 'Tablero', permission: 'browse_project' },
  { key: 'backlog', label: 'Backlog', permission: 'browse_project', scrumOnly: true },
  { key: 'sprints', label: 'Sprints', permission: 'browse_project', scrumOnly: true },
  { key: 'calendar', label: 'Calendario', permission: 'browse_project' },
  { key: 'settings', label: 'Configuración', permission: 'edit_project' },
];

/** Whether the project has sprints: the scrum template, or the legacy scrum type when no template is set. */
export function hasSprints(project: Pick<Project, 'template' | 'type'>): boolean {
  return (project.template ?? project.type) === 'scrum';
}

/**
 * Navigation entries of a project: Backlog and Sprints only for scrum projects, and only the entries
 * whose permission `can` grants. Pure: the project shell passes `CurrentProjectStore.can`.
 */
export function buildProjectNav(
  project: ProjectRef & Pick<Project, 'template' | 'type'>,
  can: (permission: string) => boolean,
): ProjectNavItem[] {
  const scrum = hasSprints(project);
  return PROJECT_NAV.filter((item) => (!item.scrumOnly || scrum) && can(item.permission)).map(
    (item) => ({ key: item.key, label: item.label, path: projectPath(project, item.key) }),
  );
}
