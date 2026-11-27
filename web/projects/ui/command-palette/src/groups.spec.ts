import { describe, expect, it } from 'vitest';

import { highlightSegments } from './groups';

describe('highlightSegments', () => {
  it('marks the words of the query, ignoring case and accents', () => {
    expect(highlightSegments('Exportar datos de Navegación', 'expor navegacion')).toEqual([
      { text: 'Expor', match: true },
      { text: 'tar datos de ', match: false },
      { text: 'Navegación', match: true },
    ]);
  });

  it('returns the whole text unmarked when nothing matches or the query is empty', () => {
    expect(highlightSegments('Tablero', 'zzz')).toEqual([{ text: 'Tablero', match: false }]);
    expect(highlightSegments('Tablero', '  ')).toEqual([{ text: 'Tablero', match: false }]);
  });

  it('merges overlapping matches', () => {
    expect(highlightSegments('abcdef', 'abc cde')).toEqual([
      { text: 'abcde', match: true },
      { text: 'f', match: false },
    ]);
  });
});
