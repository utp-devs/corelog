import { FormControl, FormGroup } from '@angular/forms';

import { ApiError } from '../models';
import { SERVER_ERROR_KEY, applyServerErrors } from './server-errors';
import { noBlankValidator } from './app.validators';

describe('validators', () => {
  it('applyServerErrors marks the matching controls with the server message', () => {
    const form = new FormGroup({
      email: new FormControl('x'),
      nombre: new FormControl('Ana'),
    });
    const error: ApiError = {
      status: 422,
      code: 'validation.failed',
      message: 'Datos inválidos',
      details: [
        { field: 'email', message: 'correo ya registrado' },
        { field: 'otro', message: 'n/a' },
      ],
    };
    expect(applyServerErrors(form, error)).toBe(true);
    expect(form.controls.email.errors).toEqual({ [SERVER_ERROR_KEY]: 'correo ya registrado' });
    expect(form.controls.email.touched).toBe(true);
    expect(form.controls.nombre.errors).toBeNull();
  });

  it('applyServerErrors returns false when no detail matches a control', () => {
    const form = new FormGroup({ email: new FormControl('x') });
    const error: ApiError = { status: 500, code: 'http.500', message: 'x', details: [] };
    expect(applyServerErrors(form, error)).toBe(false);
  });

  it('noBlankValidator rejects empty and whitespace-only strings', () => {
    expect(noBlankValidator(new FormControl('   '))).toEqual({ blank: true });
    expect(noBlankValidator(new FormControl(''))).toEqual({ blank: true });
    expect(noBlankValidator(new FormControl('Ana'))).toBeNull();
  });
});
