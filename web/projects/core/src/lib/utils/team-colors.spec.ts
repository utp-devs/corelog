import { isHexColor, paletteIndexOf, paletteValue, resolveAvatarColor } from './team-colors';

/** Minimal stand-in for an element whose computed style exposes the palette tokens. */
function rootWithTokens(tokens: Record<string, string>): Element {
  const view = {
    getComputedStyle: () => ({
      getPropertyValue: (name: string) => tokens[name] ?? '',
    }),
  };
  return { ownerDocument: { defaultView: view } } as unknown as Element;
}

describe('team colors', () => {
  it('accepts only six digit hex colors', () => {
    expect(isHexColor('#1F6FEB')).toBe(true);
    expect(isHexColor('#1F6')).toBe(false);
    expect(isHexColor('blue')).toBe(false);
  });

  it('round-trips palette values and rejects indexes outside 1..12', () => {
    expect(paletteValue(4)).toBe('palette-4');
    expect(paletteIndexOf('palette-4')).toBe(4);
    expect(paletteIndexOf('palette-0')).toBeNull();
    expect(paletteIndexOf('palette-13')).toBeNull();
    expect(paletteIndexOf(null)).toBeNull();
  });

  it('keeps custom hex values and clears the empty color', () => {
    expect(resolveAvatarColor('#ABCDEF', null)).toBe('#ABCDEF');
    expect(resolveAvatarColor(null, null)).toBeNull();
  });

  it('resolves a palette swatch to the hex of its background token', () => {
    const root = rootWithTokens({ '--cl-palette-3-bg': '#112233' });
    expect(resolveAvatarColor('palette-3', root)).toBe('#112233');
  });

  it('returns null when the palette token is not a hex color or no root is available', () => {
    expect(resolveAvatarColor('palette-3', rootWithTokens({}))).toBeNull();
    expect(resolveAvatarColor('palette-3', null)).toBeNull();
  });
});
