import { ApiError } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { memberCountText, memberErrorMessage, messageOf } from './team-messages';

function apiError(code: string, message = 'server text'): ApiError {
  return { status: 400, code, message, details: [] };
}

describe('team messages', () => {
  it('uses the API message when present and the fallback otherwise', () => {
    expect(messageOf(apiError('x', 'Detalle'), 'Respaldo')).toBe('Detalle');
    expect(messageOf(new Error('boom'), 'Respaldo')).toBe('Respaldo');
    expect(messageOf(apiError('x', ''), 'Respaldo')).toBe('Respaldo');
  });

  it('maps membership error codes to Spanish copy', () => {
    expect(memberErrorMessage(apiError('team.member_exists'), 'f')).toBe(
      'Esa persona ya es miembro del equipo.',
    );
    expect(memberErrorMessage(apiError('team.user_not_in_org'), 'f')).toBe(
      'Esa persona no pertenece a la organización.',
    );
    expect(memberErrorMessage(apiError('team.forbidden'), 'f')).toBe(
      'No tienes permiso para gestionar los miembros de este equipo.',
    );
    expect(memberErrorMessage(apiError('team.other', 'Otro'), 'f')).toBe('Otro');
  });

  it('pluralizes the member count', () => {
    expect(memberCountText(1)).toBe('1 miembro');
    expect(memberCountText(4)).toBe('4 miembros');
    expect(memberCountText(undefined)).toBe('0 miembros');
  });
});
