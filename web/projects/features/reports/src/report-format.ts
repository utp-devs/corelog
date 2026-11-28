import type { ReportCell, ReportColumn, ReportColumnType } from '@corelog/core';

export const EMPTY_CELL = '–';

const NUMBER = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 2 });
const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})/;
const ISO_DAY = /^\d{4}-\d{2}-\d{2}$/;

/** A date (`AAAA-MM-DD`, with or without time) as `dd/mm/aaaa`; other text is kept. */
export function formatDate(value: string): string {
  const match = ISO_DATE.exec(value);
  return match ? `${match[3]}/${match[2]}/${match[1]}` : value;
}

export function formatNumber(value: number): string {
  return NUMBER.format(value);
}

/** Text of a table cell by column type: dates as dd/mm/aaaa, numbers with Spanish separators, percents with a sign. */
export function formatCell(value: ReportCell | undefined, type: ReportColumnType): string {
  if (value === null || value === undefined || value === '') return EMPTY_CELL;
  if (typeof value === 'boolean') return value ? 'Sí' : 'No';
  if (type === 'date') return formatDate(String(value));
  if (type === 'number' || type === 'percent') {
    const numeric = typeof value === 'number' ? value : Number(value);
    if (Number.isNaN(numeric)) return String(value);
    return type === 'percent' ? `${NUMBER.format(numeric)} %` : NUMBER.format(numeric);
  }
  // Period labels of a text column (a day) read like the date columns.
  return ISO_DAY.test(String(value)) ? formatDate(String(value)) : String(value);
}

export type SortDirection = 'asc' | 'desc';

function isEmpty(value: ReportCell | undefined): boolean {
  return value === null || value === undefined || value === '';
}

function compare(a: ReportCell, b: ReportCell, type: ReportColumnType): number {
  if (type === 'number' || type === 'percent') return Number(a) - Number(b);
  return String(a).localeCompare(String(b), 'es', { numeric: true, sensitivity: 'base' });
}

/** Rows ordered by one column; empty cells always go last. The input is not modified. */
export function sortRows(
  rows: readonly (readonly ReportCell[])[],
  columnIndex: number,
  column: ReportColumn,
  direction: SortDirection,
): (readonly ReportCell[])[] {
  const sign = direction === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    const left = a[columnIndex];
    const right = b[columnIndex];
    if (isEmpty(left) || isEmpty(right)) {
      return isEmpty(left) === isEmpty(right) ? 0 : isEmpty(left) ? 1 : -1;
    }
    return sign * compare(left, right, column.type);
  });
}
