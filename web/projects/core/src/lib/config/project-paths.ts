/**
 * Router path of the project area, relative to the application base href (the workspace app serves
 * it at /workspace/projects).
 */
export const PROJECTS_PATH = '/projects';

/** Sections of a project that have their own route. */
export const PROJECT_SECTIONS = ['board', 'backlog', 'sprints', 'calendar', 'settings'] as const;
export type ProjectSection = (typeof PROJECT_SECTIONS)[number];

/** What a project path needs: the key when known, the id otherwise. */
export interface ProjectRef {
  readonly id: string;
  readonly key?: string | null;
}

/**
 * Router path of a project or one of its sections. The key is preferred; the UUID is used when the
 * key is not known (the project area resolves both). A plain string is taken as the identifier as is.
 * Use with routerLink or navigateByUrl.
 */
export function projectPath(project: ProjectRef | string, section?: string): string {
  const identifier = typeof project === 'string' ? project : project.key || project.id;
  const base = `${PROJECTS_PATH}/${encodeURIComponent(identifier)}`;
  return section ? `${base}/${section}` : base;
}
