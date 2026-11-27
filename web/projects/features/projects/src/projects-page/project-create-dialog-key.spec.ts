import { describe, expect, it } from 'vitest';

import { suggestProjectKey } from './project-create-dialog-key';

describe('suggestProjectKey', () => {
  it('takes the first letters of a single word', () => {
    expect(suggestProjectKey('Seguridad')).toBe('SEGU');
    expect(suggestProjectKey('Web')).toBe('WEB');
  });

  it('takes the initials of several words, without accents or symbols', () => {
    expect(suggestProjectKey('Plataforma de Pagos')).toBe('PDP');
    expect(suggestProjectKey('  Gestión & Análisis: Móvil ')).toBe('GAM');
  });

  it('limits long names to five initials and returns nothing for an empty name', () => {
    expect(suggestProjectKey('a b c d e f g')).toBe('ABCDE');
    expect(suggestProjectKey('   ')).toBe('');
  });
});
