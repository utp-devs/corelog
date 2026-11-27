import { ApiError } from '../models/api-error.model';
import { profileFieldErrors } from './profile-errors';

describe('profileFieldErrors', () => {
  it('maps API detail fields (snake_case) to form fields (camelCase)', () => {
    const error: ApiError = {
      status: 422,
      code: 'validation.failed',
      message: 'x',
      details: [
        { field: 'job_title', message: 'Demasiado largo' },
        { field: 'display_name', message: 'Demasiado largo' },
        { field: 'unknown_field', message: 'ignorado' },
      ],
    };

    expect(profileFieldErrors(error)).toEqual({
      jobTitle: 'Demasiado largo',
      displayName: 'Demasiado largo',
    });
  });

  it('maps the timezone error code even without details', () => {
    const error: ApiError = {
      status: 422,
      code: 'user.timezone_invalid',
      message: 'x',
      details: [],
    };
    expect(profileFieldErrors(error)).toEqual({ timezone: 'Zona horaria no válida.' });
  });

  it('returns an empty object for errors that belong to no field', () => {
    expect(
      profileFieldErrors({ status: 500, code: 'http.500', message: 'x', details: [] }),
    ).toEqual({});
    expect(profileFieldErrors(new Error('x'))).toEqual({});
  });

  it('keeps the first message of a field', () => {
    const error: ApiError = {
      status: 422,
      code: 'validation.failed',
      message: 'x',
      details: [
        { field: 'name', message: 'primero' },
        { field: 'name', message: 'segundo' },
      ],
    };
    expect(profileFieldErrors(error)).toEqual({ name: 'primero' });
  });
});
