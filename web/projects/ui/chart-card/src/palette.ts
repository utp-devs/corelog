/** Series colors for charts, all taken from tokens (never hex). */
export const CHART_SERIES = [
  'var(--cl-brand)',
  'var(--cl-success)',
  'var(--cl-warning)',
  'var(--cl-danger)',
  'var(--cl-type-epic)',
  'var(--cl-type-subtask)',
  'var(--cl-info)',
  'var(--cl-cat-todo)',
] as const;

/** Color for series `index`; wraps around when there are more series than colors. */
export function seriesColor(index: number): string {
  const count = CHART_SERIES.length;
  return CHART_SERIES[((index % count) + count) % count];
}

const TOKEN_REFERENCE = /^var\((--[a-z0-9-]+)\)$/;

/**
 * Canvas drawing needs concrete colors, not `var()` references. A token reference is replaced
 * by the value the scope currently computes for it; any other value is returned unchanged.
 */
export function resolveTokenColor(value: string, scope: Element): string {
  const match = TOKEN_REFERENCE.exec(value);
  if (!match) return value;
  const computed = getComputedStyle(scope).getPropertyValue(match[1]).trim();
  return computed || value;
}
