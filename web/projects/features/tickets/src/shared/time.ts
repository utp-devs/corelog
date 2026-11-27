import { relativeTime } from '../ticket-list/relative-time';

const EXACT = new Intl.DateTimeFormat('es', { dateStyle: 'long', timeStyle: 'short' });
const SHORT = new Intl.DateTimeFormat('es', { dateStyle: 'short', timeStyle: 'short' });

/** Relative text ("hace 3 horas") for recent dates, the short date for older ones. */
export function timeAgo(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return relativeTime(iso) ?? SHORT.format(date);
}

/** Full date and time, for tooltips next to a relative time. */
export function exactDate(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : EXACT.format(date);
}
