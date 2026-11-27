export const PALETTE_SIZE = 12;

const HEX_PATTERN = /^#[0-9a-fA-F]{6}$/;

export const HEX_COLOR_ERROR = 'Usa un color hexadecimal de 6 dígitos, por ejemplo: #RRGGBB.';

/** Value stored for a palette swatch: 'palette-N' maps to --cl-palette-N. */
export function paletteValue(index: number): string {
  return `palette-${index}`;
}

/** Palette index (1..12) of a stored value, or null for custom or empty values. */
export function paletteIndexOf(value: string | null): number | null {
  const match = /^palette-(\d+)$/.exec(value ?? '');
  if (!match) {
    return null;
  }
  const index = Number(match[1]);
  return index >= 1 && index <= PALETTE_SIZE ? index : null;
}

export function isHexColor(value: string): boolean {
  return HEX_PATTERN.test(value);
}

/** Next palette index for a navigation key, or null when the key is not a navigation key. Wraps around. */
export function nextPaletteIndex(key: string, current: number): number | null {
  switch (key) {
    case 'ArrowRight':
    case 'ArrowDown':
      return current === PALETTE_SIZE ? 1 : current + 1;
    case 'ArrowLeft':
    case 'ArrowUp':
      return current === 1 ? PALETTE_SIZE : current - 1;
    case 'Home':
      return 1;
    case 'End':
      return PALETTE_SIZE;
    default:
      return null;
  }
}
