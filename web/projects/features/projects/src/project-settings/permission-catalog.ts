/** Copy of one permission of the catalog. */
export interface PermissionCopy {
  readonly label: string;
  readonly description: string;
}

export interface PermissionGroup {
  readonly key: string;
  readonly label: string;
  readonly permissions: readonly string[];
}

/** Permission groups in the order the matrix lists them. The keys are the permission names of the API. */
export const PERMISSION_GROUPS: readonly PermissionGroup[] = [
  {
    key: 'project',
    label: 'Proyecto',
    permissions: [
      'browse_project',
      'view_reports',
      'export_issues',
      'edit_project',
      'manage_members',
      'delete_project',
    ],
  },
  {
    key: 'issues',
    label: 'Incidencias',
    permissions: [
      'create_issue',
      'edit_issue',
      'edit_own_issue',
      'delete_issue',
      'assign_issue',
      'transition_issue',
      'link_issue',
      'bulk_change',
    ],
  },
  {
    key: 'comments',
    label: 'Comentarios y adjuntos',
    permissions: [
      'comment_issue',
      'edit_own_comment',
      'delete_own_comment',
      'manage_comments',
      'add_attachment',
      'delete_own_attachment',
      'manage_attachments',
    ],
  },
  {
    key: 'follow',
    label: 'Seguimiento y tiempo',
    permissions: ['watch_issue', 'vote_issue', 'log_work', 'manage_worklogs'],
  },
  {
    key: 'planning',
    label: 'Tablero y planificación',
    permissions: [
      'move_cards',
      'manage_board',
      'manage_sprints',
      'manage_versions',
      'manage_components',
      'manage_workflow',
      'manage_automation',
    ],
  },
];

export const OTHER_GROUP_LABEL = 'Otros permisos';

const COPY: Readonly<Record<string, PermissionCopy>> = {
  browse_project: {
    label: 'Ver el proyecto',
    description: 'Abrir el proyecto y ver sus incidencias.',
  },
  view_reports: {
    label: 'Ver reportes',
    description: 'Consultar los reportes y métricas del proyecto.',
  },
  export_issues: {
    label: 'Exportar incidencias',
    description: 'Descargar las incidencias en un archivo.',
  },
  edit_project: {
    label: 'Editar el proyecto',
    description: 'Cambiar nombre, descripción, apariencia y permisos, y archivar el proyecto.',
  },
  manage_members: {
    label: 'Gestionar personas',
    description: 'Agregar y quitar personas y equipos, y cambiar sus roles.',
  },
  delete_project: {
    label: 'Eliminar el proyecto',
    description: 'Eliminar el proyecto de forma definitiva.',
  },
  create_issue: { label: 'Crear incidencias', description: 'Dar de alta incidencias nuevas.' },
  edit_issue: {
    label: 'Editar incidencias',
    description: 'Cambiar los campos de cualquier incidencia.',
  },
  edit_own_issue: {
    label: 'Editar las propias',
    description: 'Editar las incidencias que la persona creó.',
  },
  delete_issue: { label: 'Eliminar incidencias', description: 'Borrar incidencias del proyecto.' },
  assign_issue: {
    label: 'Asignar incidencias',
    description: 'Elegir quién se encarga de una incidencia.',
  },
  transition_issue: {
    label: 'Cambiar de estado',
    description: 'Mover una incidencia por el flujo de trabajo.',
  },
  link_issue: {
    label: 'Vincular incidencias',
    description: 'Relacionar una incidencia con otra.',
  },
  bulk_change: { label: 'Cambios en lote', description: 'Editar varias incidencias a la vez.' },
  comment_issue: { label: 'Comentar', description: 'Escribir comentarios en las incidencias.' },
  edit_own_comment: {
    label: 'Editar sus comentarios',
    description: 'Corregir los comentarios propios.',
  },
  delete_own_comment: {
    label: 'Borrar sus comentarios',
    description: 'Eliminar los comentarios propios.',
  },
  manage_comments: {
    label: 'Moderar comentarios',
    description: 'Editar o borrar los comentarios de otras personas.',
  },
  add_attachment: { label: 'Adjuntar archivos', description: 'Subir archivos a las incidencias.' },
  delete_own_attachment: {
    label: 'Borrar sus adjuntos',
    description: 'Quitar los archivos que subió la persona.',
  },
  manage_attachments: {
    label: 'Gestionar adjuntos',
    description: 'Quitar los archivos que subieron otras personas.',
  },
  watch_issue: { label: 'Seguir incidencias', description: 'Recibir avisos de una incidencia.' },
  vote_issue: { label: 'Votar incidencias', description: 'Marcar las incidencias que importan.' },
  log_work: {
    label: 'Registrar tiempo',
    description: 'Anotar el tiempo trabajado en una incidencia.',
  },
  manage_worklogs: {
    label: 'Gestionar registros de tiempo',
    description: 'Editar o borrar el tiempo registrado por otras personas.',
  },
  move_cards: { label: 'Mover tarjetas', description: 'Reordenar el tablero y el backlog.' },
  manage_board: {
    label: 'Configurar el tablero',
    description: 'Editar columnas y límites de trabajo en curso.',
  },
  manage_sprints: { label: 'Planificar sprints', description: 'Crear, iniciar y cerrar sprints.' },
  manage_versions: { label: 'Gestionar versiones', description: 'Crear y publicar versiones.' },
  manage_components: {
    label: 'Gestionar componentes',
    description: 'Crear y editar los componentes del proyecto.',
  },
  manage_workflow: {
    label: 'Gestionar el flujo',
    description: 'Definir los estados y transiciones permitidas.',
  },
  manage_automation: {
    label: 'Gestionar automatizaciones',
    description: 'Crear y editar reglas automáticas.',
  },
};

/** Copy of a permission; an unknown one shows its API name so a newer server never hides a row. */
export function permissionCopy(permission: string): PermissionCopy {
  return COPY[permission] ?? { label: permission, description: '' };
}

/** A group of the matrix with the permissions the scheme actually lists. */
export interface PermissionRowGroup {
  readonly key: string;
  readonly label: string;
  readonly permissions: readonly string[];
}

/**
 * Groups the permissions of a scheme for display. Permissions of the catalog keep the catalog order;
 * those the catalog does not know go last, in their own group.
 */
export function groupPermissions(available: readonly string[]): PermissionRowGroup[] {
  const present = new Set(available);
  const known = new Set(PERMISSION_GROUPS.flatMap((group) => group.permissions));
  const groups: PermissionRowGroup[] = PERMISSION_GROUPS.map((group) => ({
    key: group.key,
    label: group.label,
    permissions: group.permissions.filter((permission) => present.has(permission)),
  })).filter((group) => group.permissions.length > 0);
  const others = available.filter((permission) => !known.has(permission));
  if (others.length > 0) {
    groups.push({ key: 'other', label: OTHER_GROUP_LABEL, permissions: others });
  }
  return groups;
}
