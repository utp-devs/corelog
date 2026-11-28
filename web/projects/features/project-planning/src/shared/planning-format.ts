import { AgileSprint, BacklogTotals, SprintStatus } from '@corelog/core';
import { LozengeTone } from '@corelog/ui/lozenge';

const MS_PER_DAY = 86_400_000;
const SECONDS_PER_HOUR = 3600;
const DEFAULT_SPRINT_DAYS = 14;

/** Spanish name of each sprint state. */
export const SPRINT_STATUS_LABELS: Readonly<Record<SprintStatus, string>> = {
  planned: 'Planificado',
  active: 'Activo',
  closed: 'Cerrado',
};

export const SPRINT_STATUS_TONES: Readonly<Record<SprintStatus, LozengeTone>> = {
  planned: 'info',
  active: 'success',
  closed: 'neutral',
};

/** Local date as AAAA-MM-DD. */
export function isoDay(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

/** Day of an ISO date (AAAA-MM-DD) at local midnight. Null when it does not parse. */
export function parseDay(value: string | null | undefined): Date | null {
  const match = value ? /^(\d{4})-(\d{2})-(\d{2})/.exec(value) : null;
  return match ? new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3])) : null;
}

/** ISO day that is `days` after the given ISO day (or after today). */
export function addDays(value: string | null | undefined, days: number): string {
  const base = parseDay(value) ?? new Date();
  const result = new Date(base.getFullYear(), base.getMonth(), base.getDate() + days);
  return isoDay(result);
}

/** Default start and end of a new sprint: today and two weeks later. */
export function defaultSprintDates(): { start: string; end: string } {
  const start = isoDay(new Date());
  return { start, end: addDays(start, DEFAULT_SPRINT_DAYS) };
}

/** ISO day (2026-11-19) as dd/MM/yyyy. Other text is returned unchanged. */
export function formatDay(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  return match ? `${match[3]}/${match[2]}/${match[1]}` : value;
}

/** Dates of a sprint as dd/MM/yyyy - dd/MM/yyyy, or a note when it has none. */
export function sprintDatesOf(sprint: Pick<AgileSprint, 'startDate' | 'endDate'>): string {
  return sprint.startDate && sprint.endDate
    ? `${formatDay(sprint.startDate)} - ${formatDay(sprint.endDate)}`
    : 'Sin fechas';
}

/** Whole days from today to the end of the sprint; negative once it passed. Null without an end date. */
export function daysLeftOf(
  endDate: string | null | undefined,
  today: Date = new Date(),
): number | null {
  const end = parseDay(endDate);
  if (!end) return null;
  const start = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  return Math.round((end.getTime() - start.getTime()) / MS_PER_DAY);
}

/** Text for the sprint chip: days left, last day, or overdue. */
export function daysLeftText(days: number | null): string {
  if (days === null) return 'Sin fecha de fin';
  if (days < 0) return days === -1 ? 'Venció ayer' : `Venció hace ${-days} días`;
  if (days === 0) return 'Termina hoy';
  return days === 1 ? 'Queda 1 día' : `Quedan ${days} días`;
}

/** Points as the planning screens write them: "8", "2,5"; empty when zero-less is wanted, use `pointsLabel`. */
export function formatPoints(points: number): string {
  return new Intl.NumberFormat('es', { maximumFractionDigits: 1 }).format(points);
}

/** "8 pts" / "1 pt". */
export function pointsLabel(points: number): string {
  return `${formatPoints(points)} ${points === 1 ? 'pt' : 'pts'}`;
}

/** "3 incidencias" / "1 incidencia". */
export function issuesLabel(count: number): string {
  return `${count} ${count === 1 ? 'incidencia' : 'incidencias'}`;
}

/** Hours of an estimate in seconds, as "2 h" or "1,5 h". Empty for no estimate. */
export function hoursLabel(seconds: number): string {
  return seconds > 0 ? `${formatPoints(seconds / SECONDS_PER_HOUR)} h` : '';
}

/** Shares of a section by status category as percentages of its points (or of its issues without points). */
export function categoryShares(totals: BacklogTotals): {
  todo: number;
  in_progress: number;
  done: number;
} {
  const usePoints = totals.points > 0;
  const total = usePoints ? totals.points : totals.issues;
  const part = (category: 'todo' | 'in_progress' | 'done'): number => {
    const value = totals.byCategory[category];
    if (!value || total === 0) return 0;
    return ((usePoints ? value.points : value.issues) / total) * 100;
  };
  return { todo: part('todo'), in_progress: part('in_progress'), done: part('done') };
}
