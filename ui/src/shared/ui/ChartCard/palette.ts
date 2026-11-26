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
] as const

/** Color for series `index`; wraps around when there are more series than colors. */
export function seriesColor(index: number): string {
  const count = CHART_SERIES.length
  return CHART_SERIES[((index % count) + count) % count]
}
