import type { ProjectMemberRole } from '../models';

// Mirror of the backend authorization policy. When the backend rule changes, this table must change in the
// same commit; the specs pin the expected matrix.

export type ProjectPermission =
  | 'view_project'
  | 'edit_project'
  | 'delete_project'
  | 'manage_members'
  | 'create_issue'
  | 'edit_issue'
  | 'comment'
  | 'watch_issue'
  | 'move_cards'
  | 'manage_board'
  | 'manage_workflow'
  | 'manage_sprint';

/** Effective role of a person inside a project: global administrator, member with a role, or none. */
export type EffectiveProjectRole = 'admin' | ProjectMemberRole | null;

export interface PermissionDefinition {
  key: ProjectPermission;
  label: string;
  description: string;
  /** Member roles that hold the permission. The global administrator always holds it. */
  memberRoles: ProjectMemberRole[];
}

const ALL: ProjectMemberRole[] = ['lead', 'contributor', 'viewer'];
const EDITORS: ProjectMemberRole[] = ['lead', 'contributor'];
const LEAD: ProjectMemberRole[] = ['lead'];

export const projectPermissions: PermissionDefinition[] = [
  {
    key: 'view_project',
    label: 'Ver el proyecto',
    description: 'Consultar datos, tablero, backlog y sprints.',
    memberRoles: ALL,
  },
  {
    key: 'watch_issue',
    label: 'Seguir incidencias',
    description: 'Recibir avisos de las incidencias del proyecto.',
    memberRoles: ALL,
  },
  {
    key: 'comment',
    label: 'Comentar',
    description: 'Escribir comentarios en las incidencias.',
    memberRoles: EDITORS,
  },
  {
    key: 'create_issue',
    label: 'Crear incidencias',
    description: 'Dar de alta incidencias nuevas en el proyecto.',
    memberRoles: EDITORS,
  },
  {
    key: 'edit_issue',
    label: 'Editar incidencias',
    description: 'Cambiar campos, estado y responsable.',
    memberRoles: EDITORS,
  },
  {
    key: 'move_cards',
    label: 'Mover tarjetas y backlog',
    description: 'Reordenar el tablero y el backlog.',
    memberRoles: EDITORS,
  },
  {
    key: 'manage_sprint',
    label: 'Planificar sprints',
    description: 'Crear, iniciar y cerrar sprints.',
    memberRoles: LEAD,
  },
  {
    key: 'manage_board',
    label: 'Gestionar tableros',
    description: 'Editar columnas y límites WIP.',
    memberRoles: LEAD,
  },
  {
    key: 'manage_workflow',
    label: 'Gestionar flujo de estados',
    description: 'Definir las transiciones permitidas.',
    memberRoles: LEAD,
  },
  {
    key: 'edit_project',
    label: 'Editar el proyecto',
    description: 'Cambiar nombre, descripción y visibilidad.',
    memberRoles: LEAD,
  },
  {
    key: 'manage_members',
    label: 'Gestionar miembros',
    description: 'Agregar, quitar y cambiar roles de miembros.',
    memberRoles: LEAD,
  },
  {
    key: 'delete_project',
    label: 'Eliminar el proyecto',
    description: 'Solo el administrador global puede eliminarlo.',
    memberRoles: [],
  },
];

/** Whether an effective role holds a permission. The global administrator holds all of them. */
export function hasProjectPermission(
  role: EffectiveProjectRole,
  permission: ProjectPermission,
): boolean {
  if (role === 'admin') return true;
  if (role === null) return false;
  const definition = projectPermissions.find((item) => item.key === permission);
  return definition ? definition.memberRoles.includes(role) : false;
}

/** Effective role from the global role and the project membership. */
export function effectiveProjectRole(
  globalRole: string | undefined,
  userId: string | undefined,
  members: { userId: string; role: ProjectMemberRole }[],
): EffectiveProjectRole {
  if (globalRole === 'admin') return 'admin';
  if (!userId) return null;
  return members.find((member) => member.userId === userId)?.role ?? null;
}
