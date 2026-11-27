import { HttpErrorResponse } from '@angular/common/http';

import { ApiError } from '../models/api-error.model';
import { invitationErrorMessage, orgCreateErrorMessage, ROLE_LABELS } from './org-copy';

function apiError(code: string, message = 'Mensaje del servidor.'): ApiError {
  return { status: 409, code, message, details: [] };
}

describe('org copy', () => {
  it('labels every role in Spanish', () => {
    expect(ROLE_LABELS).toEqual({
      owner: 'Propietario',
      admin: 'Administrador',
      member: 'Miembro',
      guest: 'Invitado',
    });
  });

  it('maps invitation failures by code and names the invited email when known', () => {
    expect(invitationErrorMessage(apiError('org.email_mismatch'), 'ana@acme.test')).toBe(
      'Esta invitación es para ana@acme.test. Inicia sesión con esa cuenta para aceptarla.',
    );
    expect(invitationErrorMessage(apiError('org.email_mismatch'))).toMatch(/otra cuenta/);
    expect(invitationErrorMessage(apiError('org.invitation_used'))).toBe(
      'Esta invitación ya se ha utilizado.',
    );
    expect(invitationErrorMessage(apiError('org.invitation_expired'))).toMatch(/caducado/);
    expect(invitationErrorMessage(apiError('org.invitation_not_found'))).toBe(
      'Este enlace no es válido.',
    );
  });

  it('keeps the server message for unknown invitation codes and uses a generic one for non-API errors', () => {
    expect(invitationErrorMessage(apiError('org.other', 'Otro motivo.'))).toBe('Otro motivo.');
    expect(invitationErrorMessage(new Error('boom'))).toBe(
      'No se pudo completar la operación. Inténtalo de nuevo.',
    );
  });

  it('maps organization create failures by code', () => {
    expect(orgCreateErrorMessage(apiError('org.limit_reached'))).toMatch(/\(5\)/);
    expect(orgCreateErrorMessage(apiError('org.slug_taken'))).toBe(
      'Ese identificador ya está en uso. Elige otro.',
    );
    expect(orgCreateErrorMessage(apiError('org.slug_reserved'))).toMatch(/reservado/);
    expect(orgCreateErrorMessage(new HttpErrorResponse({ status: 0 }))).toBe(
      'No se pudo conectar con el servidor. Verifica tu conexión e inténtalo nuevamente.',
    );
  });
});
