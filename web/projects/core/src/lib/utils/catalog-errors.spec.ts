import { catalogErrorMessage } from './catalog-errors';

const error = (code: string, message = 'servidor') => ({
  status: 409,
  code,
  message,
  details: [],
});

describe('catalogErrorMessage', () => {
  it('translates the codes of the protections', () => {
    expect(catalogErrorMessage(error('catalog.in_use'), 'x')).toContain('Hay incidencias');
    expect(catalogErrorMessage(error('catalog.last_required_item'), 'x')).toContain('obligatorio');
    expect(catalogErrorMessage(error('project.issue_types_required'), 'x')).toContain('subtarea');
  });

  it('uses the server text for unknown codes and the fallback for other failures', () => {
    expect(catalogErrorMessage(error('other.code', 'texto del servidor'), 'x')).toBe(
      'texto del servidor',
    );
    expect(catalogErrorMessage(new Error('boom'), 'respaldo')).toBe('respaldo');
  });
});
