import type {
  OrgRole,
  OrgWhoCan,
  Project,
  ProjectMemberRole,
  ProjectType,
  ProjectVisibility,
} from '@corelog/core';

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

export interface CreateProjectRights {
  readonly allowed: boolean;
  /** Spanish explanation shown as a tooltip when creation is not allowed. */
  readonly reason: string | null;
}

/**
 * Who may create projects: global admins, organization owners and admins, and organization members
 * when the organization policy opens creation to members.
 */
export function createProjectRights(
  globalRole: string | null | undefined,
  orgRole: OrgRole | null | undefined,
  policy: OrgWhoCan | null | undefined,
): CreateProjectRights {
  if (globalRole === 'admin' || orgRole === 'owner' || orgRole === 'admin') {
    return { allowed: true, reason: null };
  }
  if (orgRole === 'member' && policy === 'members') return { allowed: true, reason: null };
  if (orgRole === 'guest') {
    return { allowed: false, reason: 'Los invitados no pueden crear proyectos.' };
  }
  return {
    allowed: false,
    reason: 'Solo los administradores de la organización pueden crear proyectos.',
  };
}
