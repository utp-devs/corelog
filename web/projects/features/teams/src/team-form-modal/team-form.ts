import { ValidatorFn } from '@angular/forms';

export const TEAM_NAME_MIN_LENGTH = 2;
export const TEAM_NAME_MAX_LENGTH = 120;
export const TEAM_DESCRIPTION_MAX_LENGTH = 500;

export const TEAM_NAME_LENGTH_MESSAGE = 'El nombre debe tener entre 2 y 120 caracteres.';
export const TEAM_NAME_HELP = 'Entre 2 y 120 caracteres.';
export const TEAM_DESCRIPTION_LENGTH_MESSAGE = 'La descripción no puede superar 500 caracteres.';
export const TEAM_NAME_TAKEN_MESSAGE = 'Ya existe un equipo con ese nombre en la organización.';
export const TEAM_SAVE_FALLBACK_MESSAGE = 'No se pudo guardar el equipo.';

/** Values the form sends to the page. The color is a hex value for the API, or null for the derived color. */
export interface TeamFormValues {
  name: string;
  description: string;
  avatarColor: string | null;
}

/** Starting values of the form. */
export interface TeamFormInitial {
  name: string;
  description: string;
  avatarColor: string | null;
}

/** Trimmed name length must be between 2 and 120 characters, as the API enforces. */
export const teamNameValidator: ValidatorFn = (control) => {
  const length = String(control.value ?? '').trim().length;
  return length >= TEAM_NAME_MIN_LENGTH && length <= TEAM_NAME_MAX_LENGTH ? null : { length: true };
};
