import { bulkFailureText } from './issue-bulk-dialog';

describe('bulkFailureText', () => {
  it('explains the usual refusals in Spanish', () => {
    expect(bulkFailureText({ issue: 'COR-1', code: 'issue.forbidden', message: 'x' })).toContain(
      'No tienes permiso',
    );
    expect(bulkFailureText({ issue: 'COR-1', code: 'issue.not_found', message: 'x' })).toContain(
      'ya no existe',
    );
    expect(
      bulkFailureText({ issue: 'COR-1', code: 'issue.move_mapping_required', message: 'x' }),
    ).toContain('no admite');
  });

  it('falls back to the server message and then to a generic one', () => {
    expect(bulkFailureText({ issue: 'COR-1', code: 'otro', message: 'Mensaje del servidor' })).toBe(
      'Mensaje del servidor',
    );
    expect(bulkFailureText({ issue: 'COR-1', code: 'otro', message: '' })).toBe(
      'No se pudo cambiar.',
    );
  });
});
