import { asApiError } from './api-error';

export type IdentityField =
  'firstName' | 'lastName' | 'phone' | 'passwordConfirmation' | 'password' | 'email';

const FIELD_BY_API_NAME: Readonly<Record<string, IdentityField>> = {
  first_name: 'firstName',
  last_name: 'lastName',
  phone: 'phone',
  'phone.number': 'phone',
  password_confirmation: 'passwordConfirmation',
  password: 'password',
  email: 'email',
};

const FIELD_BY_CODE: Readonly<Record<string, { field: IdentityField; message: string }>> = {
  'user.invalid_name': {
    field: 'firstName',
    message: 'El nombre solo admite letras, espacios, guiones y apóstrofes.',
  },
  'user.invalid_phone': {
    field: 'phone',
    message: 'El teléfono no es válido para el país elegido.',
  },
  'user.phone_taken': { field: 'phone', message: 'Este teléfono ya está en uso en otra cuenta.' },
  'user.password_mismatch': {
    field: 'passwordConfirmation',
    message: 'Las contraseñas no coinciden',
  },
  'auth.email_taken': { field: 'email', message: 'Ya existe una cuenta con este correo.' },
};

/**
 * Maps a failed identity request (register, profile save, password change) to form fields, by the
 * details of the error first and by stable code after. Empty when no error belongs to a field.
 */
export function identityFieldErrors(error: unknown): Partial<Record<IdentityField, string>> {
  const api = asApiError(error);
  const result: Partial<Record<IdentityField, string>> = {};
  for (const detail of api.details) {
    const field = FIELD_BY_API_NAME[detail.field];
    if (field && !result[field]) result[field] = detail.message;
  }
  const byCode = FIELD_BY_CODE[api.code];
  if (byCode && !result[byCode.field]) result[byCode.field] = byCode.message;
  return result;
}

/** Slugs the server reports as missing when the registration lacks a required document. */
export function missingConsentSlugs(error: unknown): string[] {
  const api = asApiError(error);
  return api.code === 'legal.consent_required' ? api.details.map((d) => d.field) : [];
}
