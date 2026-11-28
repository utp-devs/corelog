import type { CalendarItem, CalendarViewMode } from '../models/calendar-v2.model';
import type { CalendarRange } from '../models/calendar.model';
import { isoDate } from './calendar-dates';

const DAY_MS = 86_400_000;

/** Date from AAAA-MM-DD at local midnight. */
export function parseIsoDate(iso: string): Date {
  const [year, month, day] = iso.split('-').map(Number);
  return new Date(year, month - 1, day);
}

export function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

export function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
}

/** Monday of the week containing `date`. */
export function startOfWeek(date: Date): Date {
  return addDays(startOfDay(date), -((date.getDay() + 6) % 7));
}

/** The seven days of the week containing `date`, Monday first. */
export function weekDays(date: Date): Date[] {
  const monday = startOfWeek(date);
  return Array.from({ length: 7 }, (_, index) => addDays(monday, index));
}

/** Whole weeks that cover the month of `date` (five or six rows), Monday first. */
export function monthGridDays(date: Date): Date[] {
  const first = new Date(date.getFullYear(), date.getMonth(), 1);
  const last = new Date(date.getFullYear(), date.getMonth() + 1, 0);
  const start = startOfWeek(first);
  const end = addDays(startOfWeek(last), 6);
  const count = Math.round((end.getTime() - start.getTime()) / DAY_MS) + 1;
  return Array.from({ length: count }, (_, index) => addDays(start, index));
}

export function isSameDay(a: Date, b: Date): boolean {
  return isoDate(a) === isoDate(b);
}

/** Days shown by a view, as local dates. The agenda lists the month like the month view. */
export function viewDays(view: CalendarViewMode, anchor: Date): Date[] {
  switch (view) {
    case 'day':
      return [startOfDay(anchor)];
    case 'week':
      return weekDays(anchor);
    default:
      return monthGridDays(anchor);
  }
}

/** Inclusive range to request for a view. */
export function viewRange(view: CalendarViewMode, anchor: Date): CalendarRange {
  const days = viewDays(view, anchor);
  return { from: isoDate(days[0]), to: isoDate(days[days.length - 1]) };
}

/** Moves the anchor one step of the view: a day, a week or a month. */
export function shiftAnchor(view: CalendarViewMode, anchor: Date, steps: number): Date {
  switch (view) {
    case 'day':
      return addDays(anchor, steps);
    case 'week':
      return addDays(anchor, steps * 7);
    default:
      return new Date(anchor.getFullYear(), anchor.getMonth() + steps, 1);
  }
}

function capitalized(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** Heading of a view: "noviembre de 2026", "23 - 29 nov 2026" or "viernes, 27 de noviembre de 2026". */
export function viewTitle(view: CalendarViewMode, anchor: Date): string {
  if (view === 'day') {
    return capitalized(
      anchor.toLocaleDateString('es-ES', {
        weekday: 'long',
        day: 'numeric',
        month: 'long',
        year: 'numeric',
      }),
    );
  }
  if (view === 'week') {
    const days = weekDays(anchor);
    const first = days[0];
    const last = days[6];
    const sameMonth = first.getMonth() === last.getMonth();
    const start = first.toLocaleDateString('es-ES', {
      day: 'numeric',
      ...(sameMonth ? {} : { month: 'short' }),
    });
    const end = last.toLocaleDateString('es-ES', {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
    });
    return `${start} - ${end}`;
  }
  return capitalized(anchor.toLocaleDateString('es-ES', { month: 'long', year: 'numeric' }));
}

/** Local AAAA-MM-DD of an instant. */
export function localDateOf(iso: string): string {
  return isoDate(new Date(iso));
}

/** Local HH:mm of an instant. */
export function localTimeOf(iso: string): string {
  const date = new Date(iso);
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

/**
 * Days an item covers, as AAAA-MM-DD. All-day items are dates (midnight UTC, exclusive end); timed items
 * use local time and an end exactly at midnight does not spill into the next day.
 */
export function itemDays(item: CalendarItem): string[] {
  if (item.allDay) {
    const first = item.date;
    const endDay = item.endsAt.slice(0, 10);
    const last = endDay > first ? isoDate(addDays(parseIsoDate(endDay), -1)) : first;
    return daysBetween(first, last);
  }
  const start = new Date(item.startsAt);
  const end = new Date(item.endsAt);
  const first = isoDate(start);
  let last = isoDate(end);
  if (end.getTime() > start.getTime() && end.getHours() === 0 && end.getMinutes() === 0) {
    last = isoDate(addDays(startOfDay(end), -1));
  }
  if (last < first) last = first;
  return daysBetween(first, last);
}

function daysBetween(first: string, last: string): string[] {
  const days: string[] = [];
  let cursor = parseIsoDate(first);
  const stop = parseIsoDate(last);
  // Bounded so a malformed range cannot spin.
  for (let guard = 0; cursor <= stop && guard < 400; guard += 1) {
    days.push(isoDate(cursor));
    cursor = addDays(cursor, 1);
  }
  return days;
}

/** True when the item occupies a time slot of the day grid (timed and inside one day). */
export function isTimedItem(item: CalendarItem): boolean {
  return !item.allDay && itemDays(item).length === 1;
}

/** Items per day. Items spanning several days appear on each of them. */
export function groupItemsByDay(
  items: readonly CalendarItem[],
): Readonly<Record<string, readonly CalendarItem[]>> {
  const byDay: Record<string, CalendarItem[]> = {};
  for (const item of items) {
    for (const day of itemDays(item)) {
      (byDay[day] ??= []).push(item);
    }
  }
  for (const day of Object.keys(byDay)) {
    byDay[day].sort(
      (a, b) =>
        Number(b.allDay) - Number(a.allDay) ||
        a.startsAt.localeCompare(b.startsAt) ||
        a.title.localeCompare(b.title),
    );
  }
  return byDay;
}

/** Position of a timed item in the day column: minutes from midnight and side-by-side lane. */
export interface TimedPlacement {
  readonly item: CalendarItem;
  readonly startMinute: number;
  readonly durationMinutes: number;
  readonly lane: number;
  readonly lanes: number;
}

export const MIN_EVENT_MINUTES = 30;

function minutesOf(iso: string): number {
  const date = new Date(iso);
  return date.getHours() * 60 + date.getMinutes();
}

/** Packs overlapping timed items into lanes so they sit side by side instead of on top of each other. */
export function layoutTimedItems(items: readonly CalendarItem[]): TimedPlacement[] {
  const sorted = items
    .map((item) => {
      const start = minutesOf(item.startsAt);
      const rawEnd = minutesOf(item.endsAt);
      const end = Math.max(rawEnd <= start ? start : rawEnd, start + MIN_EVENT_MINUTES);
      return { item, start, end: Math.min(end, 24 * 60) };
    })
    .sort((a, b) => a.start - b.start || b.end - a.end);

  const placements: TimedPlacement[] = [];
  let cluster: { item: CalendarItem; start: number; end: number; lane: number }[] = [];
  let clusterEnd = -1;

  const flush = (): void => {
    const lanes = cluster.reduce((max, entry) => Math.max(max, entry.lane + 1), 1);
    for (const entry of cluster) {
      placements.push({
        item: entry.item,
        startMinute: entry.start,
        durationMinutes: entry.end - entry.start,
        lane: entry.lane,
        lanes,
      });
    }
    cluster = [];
  };

  for (const entry of sorted) {
    if (cluster.length > 0 && entry.start >= clusterEnd) flush();
    const taken = new Set(
      cluster.filter((other) => other.end > entry.start).map((other) => other.lane),
    );
    let lane = 0;
    while (taken.has(lane)) lane += 1;
    cluster.push({ ...entry, lane });
    clusterEnd = Math.max(clusterEnd, entry.end);
  }
  flush();
  return placements;
}

/** UTC offset in minutes of a time zone at a given instant. */
function zoneOffsetMinutes(timeZone: string, at: Date): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    hourCycle: 'h23',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).formatToParts(at);
  const read = (type: string): number => Number(parts.find((part) => part.type === type)?.value);
  const asUtc = Date.UTC(
    read('year'),
    read('month') - 1,
    read('day'),
    read('hour'),
    read('minute'),
    read('second'),
  );
  return Math.round((asUtc - at.getTime()) / 60_000);
}

/** The instant (RFC 3339, UTC) of a wall-clock date and time in a time zone. */
export function zonedToIso(date: string, time: string, timeZone: string): string {
  const [year, month, day] = date.split('-').map(Number);
  const [hour, minute] = time.split(':').map(Number);
  const wall = Date.UTC(year, month - 1, day, hour, minute);
  let instant = wall;
  try {
    // Two passes settle the offset around a daylight saving change.
    instant = wall - zoneOffsetMinutes(timeZone, new Date(wall)) * 60_000;
    instant = wall - zoneOffsetMinutes(timeZone, new Date(instant)) * 60_000;
  } catch {
    instant = wall;
  }
  return new Date(instant).toISOString().replace('.000Z', 'Z');
}

/** Wall-clock date and time of an instant in a time zone. */
export function isoToZoned(iso: string, timeZone: string): { date: string; time: string } {
  const at = new Date(iso);
  const offset = zoneOffsetMinutes(timeZone, at);
  const shifted = new Date(at.getTime() + offset * 60_000).toISOString();
  return { date: shifted.slice(0, 10), time: shifted.slice(11, 16) };
}

/** Midnight UTC of a date, the form the API uses for all-day events. */
export function allDayIso(date: string): string {
  return `${date}T00:00:00Z`;
}

/** Time zone of the browser, or UTC when it cannot be read. */
export function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

/** Full Spanish date for an accessible name: "viernes, 27 de noviembre de 2026". */
export function fullDateLabel(date: Date): string {
  return date.toLocaleDateString('es-ES', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}
