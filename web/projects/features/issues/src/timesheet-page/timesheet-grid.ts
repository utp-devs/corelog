import { Timesheet } from '@corelog/core';

export interface TimesheetCell {
  readonly seconds: number;
}

export interface TimesheetRow {
  readonly issueId: string;
  readonly issueKey: string;
  readonly issueTitle: string;
  /** Seconds per day, in the order of the week days. */
  readonly cells: readonly number[];
  readonly totalSeconds: number;
}

export interface TimesheetColumn {
  readonly date: string;
  readonly totalSeconds: number;
  readonly capacitySeconds: number;
}

export interface TimesheetGrid {
  readonly columns: readonly TimesheetColumn[];
  readonly rows: readonly TimesheetRow[];
  readonly totalSeconds: number;
}

/** ISO date (yyyy-MM-dd) of a local date. */
export function toIsoDate(date: Date): string {
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** The seven days of the week (Monday first) that contains `reference`. */
export function weekDays(reference: Date): string[] {
  const start = new Date(reference.getFullYear(), reference.getMonth(), reference.getDate());
  const offset = (start.getDay() + 6) % 7;
  start.setDate(start.getDate() - offset);
  return Array.from({ length: 7 }, (_, index) => {
    const day = new Date(start);
    day.setDate(start.getDate() + index);
    return toIsoDate(day);
  });
}

/** Moves a date by whole weeks. */
export function shiftWeeks(reference: Date, weeks: number): Date {
  const next = new Date(reference);
  next.setDate(next.getDate() + weeks * 7);
  return next;
}

/** Pivots the day-grouped timesheet into one row per issue and one column per day of the week. */
export function buildTimesheetGrid(
  sheet: Timesheet | null,
  days: readonly string[],
): TimesheetGrid {
  const byDate = new Map((sheet?.days ?? []).map((day) => [day.date, day]));
  const rows = new Map<string, { key: string; title: string; cells: number[] }>();
  const columns: TimesheetColumn[] = days.map((date, index) => {
    const day = byDate.get(date);
    for (const entry of day?.entries ?? []) {
      const row = rows.get(entry.issueId) ?? {
        key: entry.issueKey,
        title: entry.issueTitle,
        cells: days.map(() => 0),
      };
      row.cells[index] += entry.timeSpentSeconds;
      rows.set(entry.issueId, row);
    }
    return {
      date,
      totalSeconds: day?.totalSeconds ?? 0,
      capacitySeconds: day?.capacitySeconds ?? 0,
    };
  });
  const list = Array.from(rows, ([issueId, row]) => ({
    issueId,
    issueKey: row.key,
    issueTitle: row.title,
    cells: row.cells,
    totalSeconds: row.cells.reduce((sum, value) => sum + value, 0),
  })).sort((a, b) => a.issueKey.localeCompare(b.issueKey, 'es', { numeric: true }));
  return {
    columns,
    rows: list,
    totalSeconds: columns.reduce((sum, column) => sum + column.totalSeconds, 0),
  };
}
