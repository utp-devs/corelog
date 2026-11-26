import { HttpErrorResponse } from '@angular/common/http';
import { TimeoutError } from 'rxjs';

import { asApiError, fieldErrorsOf } from './api-error';

describe('asApiError', () => {
  it('maps the CoreLog error body with code and field details', () => {
    const error = new HttpErrorResponse({
      status: 422,
      error: {
        error: {
          message: 'Los datos no son válidos.',
          code: 'validation.failed',
          details: [{ field: 'email', message: 'un correo válido es obligatorio' }],
        },
      },
    });
    const apiError = asApiError(error);
    expect(apiError.status).toBe(422);
    expect(apiError.code).toBe('validation.failed');
    expect(apiError.message).toBe('Los datos no son válidos.');
    expect(fieldErrorsOf(apiError)).toEqual({ email: 'un correo válido es obligatorio' });
  });

  it('reports a network failure (status 0) as unreachable', () => {
    const apiError = asApiError(new HttpErrorResponse({ status: 0, error: null }));
    expect(apiError.status).toBe(0);
    expect(apiError.code).toBe('network.unreachable');
    expect(apiError.message).toContain('No se pudo conectar');
  });

  it('reports a timeout as network.timeout', () => {
    const apiError = asApiError(new TimeoutError());
    expect(apiError.code).toBe('network.timeout');
    expect(apiError.status).toBe(0);
  });

  it('uses the status message when the body is not a CoreLog error', () => {
    const apiError = asApiError(new HttpErrorResponse({ status: 503, error: '<html>' }));
    expect(apiError.code).toBe('http.503');
    expect(apiError.message).toBe('El servidor no está disponible en este momento.');
    expect(apiError.details).toEqual([]);
  });

  it('returns an already normalized error unchanged and wraps unknown errors', () => {
    const normalized = { status: 409, code: 'x', message: 'y', details: [] };
    expect(asApiError(normalized)).toBe(normalized);
    expect(asApiError(new Error('fallo local')).code).toBe('unknown');
    expect(asApiError('texto').message).toBe('Ocurrió un error inesperado.');
  });
});
