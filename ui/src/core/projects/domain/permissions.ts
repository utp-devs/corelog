import type { ProjectMemberRole } from './project'

// Espejo de internal/authorization/domain/policy.go. Si el backend cambia una regla,
// esta tabla debe cambiar en el mismo commit (lo verifica permissions.test.ts).

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
  | 'manage_sprint'

// Rol efectivo de una persona dentro de un proyecto: administrador global, miembro con rol o ninguno.
export type EffectiveProjectRole = 'admin' | ProjectMemberRole | null

export interface PermissionDefinition {
  key: ProjectPermission
  label: string
  description: string
  // Roles de miembro que lo tienen. El administrador global siempre lo tiene salvo que se indique lo contrario.
  memberRoles: ProjectMemberRole[]
}

const ALL: ProjectMemberRole[] = ['lead', 'contributor', 'viewer']
const EDITORS: ProjectMemberRole[] = ['lead', 'contributor']
const LEAD: ProjectMemberRole[] = ['lead']

export const projectPermissions: PermissionDefinition[] = [
  { key: 'view_project', label: 'Ver el proyecto', description: 'Consultar datos, tablero, backlog y sprints.', memberRoles: ALL },
  { key: 'watch_issue', label: 'Seguir incidencias', description: 'Recibir avisos de las incidencias del proyecto.', memberRoles: ALL },
  { key: 'comment', label: 'Comentar', description: 'Escribir comentarios en las incidencias.', memberRoles: EDITORS },
  { key: 'create_issue', label: 'Crear incidencias', description: 'Dar de alta incidencias nuevas en el proyecto.', memberRoles: EDITORS },
  { key: 'edit_issue', label: 'Editar incidencias', description: 'Cambiar campos, estado y responsable.', memberRoles: EDITORS },
  { key: 'move_cards', label: 'Mover tarjetas y backlog', description: 'Reordenar el tablero y el backlog.', memberRoles: EDITORS },
  { key: 'manage_sprint', label: 'Planificar sprints', description: 'Crear, iniciar y cerrar sprints.', memberRoles: LEAD },
  { key: 'manage_board', label: 'Gestionar tableros', description: 'Editar columnas y límites WIP.', memberRoles: LEAD },
  { key: 'manage_workflow', label: 'Gestionar flujo de estados', description: 'Definir las transiciones permitidas.', memberRoles: LEAD },
  { key: 'edit_project', label: 'Editar el proyecto', description: 'Cambiar nombre, descripción y visibilidad.', memberRoles: LEAD },
  { key: 'manage_members', label: 'Gestionar miembros', description: 'Agregar, quitar y cambiar roles de miembros.', memberRoles: LEAD },
  { key: 'delete_project', label: 'Eliminar el proyecto', description: 'Solo el administrador global puede eliminarlo.', memberRoles: [] },
]

// Permiso concedido a un rol efectivo. El administrador global tiene todos los permisos.
export function hasProjectPermission(role: EffectiveProjectRole, permission: ProjectPermission): boolean {
  if (role === 'admin') return true
  if (role === null) return false
  const definition = projectPermissions.find((item) => item.key === permission)
  return definition ? definition.memberRoles.includes(role) : false
}

// Calcula el rol efectivo a partir del rol global y la membresía del proyecto.
export function effectiveProjectRole(
  globalRole: string | undefined,
  userId: string | undefined,
  members: { userId: string; role: ProjectMemberRole }[],
): EffectiveProjectRole {
  if (globalRole === 'admin') return 'admin'
  if (!userId) return null
  return members.find((member) => member.userId === userId)?.role ?? null
}
