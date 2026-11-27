import type { Project, ProjectMemberRole, ProjectType, ProjectVisibility } from '@corelog/core';

export type VisibilityFilter = 'all' | 'open' | 'private';
export type ArchivedFilter = 'active' | 'archived' | 'all';

export const VISIBILITY_LABELS: Record<ProjectVisibility, string> = {
  open: 'Abierto',
  private: 'Privado',
};

export const TYPE_LABELS: Record<ProjectType, string> = {
  kanban: 'Kanban',
  scrum: 'Scrum',
};

export const MEMBER_ROLE_OPTIONS: { value: ProjectMemberRole; label: string }[] = [
  { value: 'lead', label: 'Líder' },
  { value: 'contributor', label: 'Colaborador' },
  { value: 'viewer', label: 'Lector' },
];

/** Filters by text (name or key), visibility and archived state. Works on the loaded list, no requests. */
export function filterProjects(
  projects: readonly Project[],
  query: string,
  visibility: VisibilityFilter,
  archived: ArchivedFilter,
): Project[] {
  const term = query.trim().toLowerCase();
  return projects.filter((project) => {
    if (visibility !== 'all' && project.visibility !== visibility) return false;
    const isArchived = project.archivedAt !== null;
    if (archived === 'active' && isArchived) return false;
    if (archived === 'archived' && !isArchived) return false;
    if (term && !`${project.name} ${project.key}`.toLowerCase().includes(term)) return false;
    return true;
  });
}
