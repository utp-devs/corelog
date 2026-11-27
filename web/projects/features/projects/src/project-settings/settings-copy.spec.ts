import { HttpErrorResponse } from '@angular/common/http';
import { describe, expect, it } from 'vitest';

import { settingsErrorMessage } from './settings-copy';

const error = (status: number, code: string, message = 'texto del servidor') => ({
  status,
  code,
  message,
  details: [],
});

describe('settingsErrorMessage', () => {
  it('explains the last lead rule in Spanish', () => {
    expect(settingsErrorMessage(error(409, 'project.last_lead'), 'x')).toContain(
      'al menos un líder',
    );
  });

  it('uses the server text for unknown codes and the fallback when there is none', () => {
    expect(settingsErrorMessage(error(422, 'other.code'), 'x')).toBe('texto del servidor');
    expect(settingsErrorMessage(error(500, 'other.code', ' '), 'Falló')).toBe('Falló');
  });

  it('explains a missing permission', () => {
    expect(settingsErrorMessage(error(403, 'http.403'), 'x')).toContain('No tienes permiso');
    expect(settingsErrorMessage(new HttpErrorResponse({ status: 403 }), 'x')).toContain(
      'No tienes permiso',
    );
  });
});
