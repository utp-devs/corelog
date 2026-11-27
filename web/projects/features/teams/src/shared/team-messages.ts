import { isApiError } from '@corelog/core';

/** Spanish message of a failed call: the API message when there is one, otherwise the fallback. */
export function messageOf(error: unknown, fallback: string): string {
  return isApiError(error) && error.message ? error.message : fallback;
}

/** Spanish message for a failed membership change, mapped from the stable error code. */
export function memberErrorMessage(error: unknown, fallback: string): string {
  const code = isApiError(error) ? error.code : null;
  if (code === 'team.member_exists') return 'Esa persona ya es miembro del equipo.';
  if (code === 'team.user_not_in_org') return 'Esa persona no pertenece a la organización.';
  if (code === 'team.forbidden')
    return 'No tienes permiso para gestionar los miembros de este equipo.';
  return messageOf(error, fallback);
}

/** Long Spanish date of the team creation, for example "1 de noviembre de 2026". */
export function formatTeamDate(iso: string): string {
  return new Date(iso).toLocaleDateString('es-ES', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

/** Member count text in Spanish; a team without a count from the API shows zero members. */
export function memberCountText(count: number | undefined): string {
  const value = count ?? 0;
  return value === 1 ? '1 miembro' : `${value} miembros`;
}
