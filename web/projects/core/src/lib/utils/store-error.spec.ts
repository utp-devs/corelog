import { HttpErrorResponse } from '@angular/common/http';

import { storeError } from './store-error';

describe('storeError', () => {
  it('keeps the code of an API error', () => {
    const apiError = {
      status: 409,
      code: 'team.name_taken',
      message: 'ya existe',
      details: [],
    };
    expect(storeError(apiError, 'fallback')).toBe(apiError);
  });

  it('normalizes an HTTP failure with its API code', () => {
    const failure = new HttpErrorResponse({
      status: 403,
      error: { error: { code: 'team.forbidden', message: 'no' } },
    });
    expect(storeError(failure, 'fallback')).toMatchObject({ status: 403, code: 'team.forbidden' });
  });

  it('uses the fallback message for unexpected failures', () => {
    expect(storeError(new Error('boom'), 'No se pudo guardar')).toEqual({
      status: 0,
      code: 'unknown',
      message: 'No se pudo guardar',
      details: [],
    });
  });
});
