import type { CalendarRange } from '../models/calendar.model';

/** Formats a date in local time as AAAA-MM-DD, so the day does not shift with the time zone. */
export function isoDate(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

/** First day of the month containing `date`, at local midnight. */
export function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

/** Inclusive range of the month containing `date`, as AAAA-MM-DD strings. */
export function monthRange(date: Date): CalendarRange {
  const start = startOfMonth(date);
  const end = new Date(start.getFullYear(), start.getMonth() + 1, 0);
  return { from: isoDate(start), to: isoDate(end) };
}

/**
 * Cells of a month grid with weeks starting on Monday. `null` fills the gaps before the first day
 * and after the last one, so the result always has a multiple of seven cells.
 */
export function monthCells(monthStart: Date): (Date | null)[] {
  const first = startOfMonth(monthStart);
  const daysInMonth = new Date(first.getFullYear(), first.getMonth() + 1, 0).getDate();
  const offset = (first.getDay() + 6) % 7;
  const cells: (Date | null)[] = Array.from({ length: offset }, () => null);
  for (let day = 1; day <= daysInMonth; day += 1) {
    cells.push(new Date(first.getFullYear(), first.getMonth(), day));
  }
  while (cells.length % 7 !== 0) {
    cells.push(null);
  }
  return cells;
}

/** Date `months` months away from `date`, pinned to the first day of that month. */
export function shiftMonth(date: Date, months: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}
