import { ApiError, ProjectMemberRole, asApiError } from '@corelog/core';

/** Spanish name of each project role. */
export const ROLE_LABELS: Record<ProjectMemberRole, string> = {
  lead: 'Líder',
  contributor: 'Colaborador',
  viewer: 'Observador',
};

/** Short explanation of each project role, for the pickers and the roles page. */
export const ROLE_SUMMARIES: Record<ProjectMemberRole, string> = {
  lead: 'Dirige el proyecto: lo configura y gestiona a las personas.',
  contributor: 'Trabaja en las incidencias del proyecto.',
  viewer: 'Consulta el proyecto y sigue las incidencias, sin editar.',
};

export const ROLE_ORDER: readonly ProjectMemberRole[] = ['lead', 'contributor', 'viewer'];

export const ROLE_OPTIONS = ROLE_ORDER.map((role) => ({ value: role, label: ROLE_LABELS[role] }));

/** Spanish copy of the errors that the project settings endpoints answer with a stable code. */
const MESSAGES_BY_CODE: Readonly<Record<string, string>> = {
  'project.last_lead':
    'El proyecto debe conservar al menos un líder. Nombra a otro líder antes de cambiar o quitar a esta persona.',
  'project.archived': 'El proyecto está archivado. Restáuralo para poder hacer cambios.',
  'project.user_not_in_org': 'Esa persona no pertenece a la organización del proyecto.',
  'project.user_not_found': 'No se encontró a esa persona.',
  'project.team_not_in_org': 'Ese equipo no pertenece a la organización del proyecto.',
  'project.team_grant_not_found': 'Ese equipo ya no tiene acceso al proyecto.',
  'project.confirmation_mismatch': 'El nombre escrito no coincide con el del proyecto.',
  'project.already_archived': 'El proyecto ya estaba archivado.',
  'project.not_archived': 'El proyecto no estaba archivado.',
  'project.not_found': 'El proyecto no existe o no tienes acceso a él.',
};

/** Message for the person: the copy of the code when known, otherwise the server text, otherwise the fallback. */
export function settingsErrorMessage(error: unknown, fallback: string): string {
  const apiError: ApiError = asApiError(error);
  if (apiError.status === 403) return 'No tienes permiso para hacer este cambio en el proyecto.';
  return MESSAGES_BY_CODE[apiError.code] ?? (apiError.message.trim() || fallback);
}
