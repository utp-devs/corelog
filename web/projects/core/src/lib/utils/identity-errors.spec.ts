import { identityFieldErrors, missingConsentSlugs } from './identity-errors';

function apiError(code: string, details: { field: string; message: string }[] = []) {
  return { status: 422, code, message: 'x', details };
}

describe('identityFieldErrors', () => {
  it('maps the details of the error to form fields', () => {
    const errors = identityFieldErrors(
      apiError('validation.failed', [
        { field: 'first_name', message: 'es obligatorio' },
        { field: 'password_confirmation', message: 'No coinciden' },
      ]),
    );
    expect(errors).toEqual({ firstName: 'es obligatorio', passwordConfirmation: 'No coinciden' });
  });

  it('falls back to the stable code when there are no details', () => {
    expect(identityFieldErrors(apiError('user.invalid_phone')).phone).toContain('teléfono');
    expect(identityFieldErrors(apiError('user.phone_taken')).phone).toContain('ya está en uso');
    expect(identityFieldErrors(apiError('user.invalid_name')).firstName).toBeTruthy();
    expect(identityFieldErrors(apiError('user.password_mismatch')).passwordConfirmation).toBe(
      'Las contraseñas no coinciden',
    );
  });

  it('returns nothing for an error that belongs to no field', () => {
    expect(identityFieldErrors(apiError('http.500'))).toEqual({});
  });
});

describe('missingConsentSlugs', () => {
  it('lists the slugs the server says are missing', () => {
    const error = apiError('legal.consent_required', [
      { field: 'privacy_policy', message: 'Falta' },
      { field: 'terms_of_use', message: 'Falta' },
    ]);
    expect(missingConsentSlugs(error)).toEqual(['privacy_policy', 'terms_of_use']);
    expect(missingConsentSlugs(apiError('other'))).toEqual([]);
  });
});
