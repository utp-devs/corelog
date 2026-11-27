import { ApiError } from '../models/api-error.model';
import { describeDirectoryError, FORBIDDEN_DIRECTORY_MESSAGE } from './directory-errors';

describe('describeDirectoryError', () => {
  it('explains the missing permission to guests (403)', () => {
    const error: ApiError = { status: 403, code: 'http.403', message: 'x', details: [] };
    expect(describeDirectoryError(error)).toBe(FORBIDDEN_DIRECTORY_MESSAGE);
  });

  it('shows the API message for other failures', () => {
    const error: ApiError = {
      status: 500,
      code: 'http.500',
      message: 'Fallo interno',
      details: [],
    };
    expect(describeDirectoryError(error)).toBe('Fallo interno');
  });
});
