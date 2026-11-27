const HEX_PATTERN = /^#[0-9a-fA-F]{6}$/;
const PALETTE_SIZE = 12;

/** Value stored for a palette swatch: 'palette-N' maps to the --cl-palette-N-bg token. */
export function paletteValue(index: number): string {
  return `palette-${index}`;
}

/** Palette index (1..12) of a stored value, or null for custom or empty values. */
export function paletteIndexOf(value: string | null): number | null {
  const match = /^palette-(\d+)$/.exec(value ?? '');
  if (!match) return null;
  const index = Number(match[1]);
  return index >= 1 && index <= PALETTE_SIZE ? index : null;
}

export function isHexColor(value: string): boolean {
  return HEX_PATTERN.test(value);
}

/**
 * Converts the color picker value into the hex color the API stores. Custom hex values pass through; a palette
 * swatch ('palette-N') is resolved to the hex of its --cl-palette-N-bg token. Returns null when there is no
 * color or the token cannot be read, so the team keeps its derived palette color.
 */
export function resolveAvatarColor(
  value: string | null,
  root: Element | null = documentRoot(),
): string | null {
  if (value === null) return null;
  if (isHexColor(value)) return value;
  const index = paletteIndexOf(value);
  if (index === null || root === null) return null;
  const view = root.ownerDocument.defaultView;
  if (!view) return null;
  const resolved = view.getComputedStyle(root).getPropertyValue(`--cl-palette-${index}-bg`).trim();
  return isHexColor(resolved) ? resolved : null;
}

function documentRoot(): Element | null {
  return typeof document === 'undefined' ? null : document.documentElement;
}
