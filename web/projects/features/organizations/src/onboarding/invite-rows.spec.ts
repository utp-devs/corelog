import { describe, expect, it } from 'vitest';

import { validateInviteRows } from './invite-rows';

describe('validateInviteRows', () => {
  it('ignores empty rows and keeps the order and roles of the valid ones', () => {
    const result = validateInviteRows([
      { id: 1, email: ' ana@example.com ', role: 'admin' },
      { id: 2, email: '', role: 'guest' },
      { id: 3, email: 'luis@example.com', role: 'member' },
    ]);
    expect(result.errors).toEqual({});
    expect(result.items).toEqual([
      { email: 'ana@example.com', role: 'admin' },
      { email: 'luis@example.com', role: 'member' },
    ]);
  });

  it('reports invalid emails on their row', () => {
    const result = validateInviteRows([{ id: 7, email: 'sin-arroba', role: 'member' }]);
    expect(result.items).toEqual([]);
    expect(result.errors).toEqual({ 7: 'Introduce un correo electrónico válido' });
  });

  it('reports a repeated email on the second row, ignoring case', () => {
    const result = validateInviteRows([
      { id: 1, email: 'ana@example.com', role: 'member' },
      { id: 2, email: 'ANA@example.com', role: 'guest' },
    ]);
    expect(result.items).toEqual([{ email: 'ana@example.com', role: 'member' }]);
    expect(result.errors).toEqual({ 2: 'Este correo ya está en la lista' });
  });

  it('returns no items for an all-empty list', () => {
    expect(validateInviteRows([{ id: 1, email: '  ', role: 'member' }])).toEqual({
      items: [],
      errors: {},
    });
  });
});
