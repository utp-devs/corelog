/** Zoom levels of the roadmap timeline. */
export type TimelineZoom = 'week' | 'month' | 'quarter';

export const TIMELINE_ZOOMS: readonly TimelineZoom[] = ['week', 'month', 'quarter'];

/** Width of one day in pixels at each zoom level. */
export const DAY_WIDTH: Readonly<Record<TimelineZoom, number>> = {
  week: 20,
  month: 7,
  quarter: 2.5,
};

/** Narrowest a bar is drawn, so a one-day item stays visible and clickable. */
export const MIN_BAR_WIDTH = 8;

const MS_PER_DAY = 86_400_000;
const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})/;
const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];

/** Days since 1970-01-01 (UTC) of an ISO day; null when the text is not a date. */
export function dayNumber(iso: string | null | undefined): number | null {
  if (!iso) return null;
  const match = ISO_DAY.exec(iso);
  if (!match) return null;
  return Math.floor(
    Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])) / MS_PER_DAY,
  );
}

/** ISO day (YYYY-MM-DD) of a day number. */
export function isoOfDay(day: number): string {
  return new Date(day * MS_PER_DAY).toISOString().slice(0, 10);
}

/** Day number of today in the local calendar. */
export function todayDayNumber(now: Date = new Date()): number {
  return Math.floor(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate()) / MS_PER_DAY);
}

function ymd(day: number): { year: number; month: number; date: number; weekday: number } {
  const d = new Date(day * MS_PER_DAY);
  return {
    year: d.getUTCFullYear(),
    month: d.getUTCMonth(),
    date: d.getUTCDate(),
    weekday: d.getUTCDay(),
  };
}

function startOfMonth(day: number): number {
  const { year, month } = ymd(day);
  return Math.floor(Date.UTC(year, month, 1) / MS_PER_DAY);
}

function startOfQuarter(day: number): number {
  const { year, month } = ymd(day);
  return Math.floor(Date.UTC(year, month - (month % 3), 1) / MS_PER_DAY);
}

/** Monday of the week of a day. */
function startOfWeek(day: number): number {
  const { weekday } = ymd(day);
  return day - ((weekday + 6) % 7);
}

function nextTick(day: number, zoom: TimelineZoom): number {
  if (zoom === 'week') return day + 7;
  const { year, month } = ymd(day);
  const step = zoom === 'month' ? 1 : 3;
  return Math.floor(Date.UTC(year, month + step, 1) / MS_PER_DAY);
}

function firstTick(day: number, zoom: TimelineZoom): number {
  if (zoom === 'week') return startOfWeek(day);
  return zoom === 'month' ? startOfMonth(day) : startOfQuarter(day);
}

function tickLabel(day: number, zoom: TimelineZoom): string {
  const { year, month, date } = ymd(day);
  if (zoom === 'week') return `${date} ${MONTHS[month]}`;
  if (zoom === 'month') return `${MONTHS[month]} ${year}`;
  return `T${Math.floor(month / 3) + 1} ${year}`;
}

export interface TimelineTick {
  readonly label: string;
  /** Left edge in pixels. */
  readonly x: number;
  readonly width: number;
  readonly day: number;
}

export interface TimelineScale {
  readonly zoom: TimelineZoom;
  readonly startDay: number;
  /** First day after the range. */
  readonly endDay: number;
  readonly dayWidth: number;
  readonly width: number;
  readonly ticks: readonly TimelineTick[];
  /** Pixel of the start of today, or null when today is outside the range. */
  readonly todayX: number | null;
}

/**
 * Time axis for a range of days. The range is widened to whole ticks (weeks, months or quarters), so
 * every tick has a label and the axis never ends in a partial column.
 */
export function buildScale(
  fromDay: number,
  toDay: number,
  zoom: TimelineZoom,
  today: number | null = todayDayNumber(),
): TimelineScale {
  const dayWidth = DAY_WIDTH[zoom];
  const startDay = firstTick(Math.min(fromDay, toDay), zoom);
  let endDay = firstTick(Math.max(fromDay, toDay), zoom);
  endDay = nextTick(endDay, zoom);
  const ticks: TimelineTick[] = [];
  for (let day = startDay; day < endDay; day = nextTick(day, zoom)) {
    const following = nextTick(day, zoom);
    ticks.push({
      label: tickLabel(day, zoom),
      day,
      x: (day - startDay) * dayWidth,
      width: (following - day) * dayWidth,
    });
  }
  const todayX =
    today !== null && today >= startDay && today < endDay ? (today - startDay) * dayWidth : null;
  return { zoom, startDay, endDay, dayWidth, width: (endDay - startDay) * dayWidth, ticks, todayX };
}

export interface DatedRange {
  readonly start: string | null;
  readonly end: string | null;
}

/**
 * Range the timeline should show: the dated items plus some air on both sides, always including today.
 * Without dated items it shows one month back and three months ahead.
 */
export function defaultRange(
  items: readonly DatedRange[],
  today: number = todayDayNumber(),
  padDays = 14,
): { from: number; to: number } {
  let from = today - 30;
  let to = today + 90;
  let any = false;
  for (const item of items) {
    const start = dayNumber(item.start);
    const end = dayNumber(item.end);
    if (start === null && end === null) continue;
    const s = start ?? (end as number);
    const e = end ?? s;
    if (!any) {
      from = Math.min(s, e);
      to = Math.max(s, e);
      any = true;
    } else {
      from = Math.min(from, s, e);
      to = Math.max(to, s, e);
    }
  }
  if (!any) return { from, to };
  return { from: Math.min(from, today) - padDays, to: Math.max(to, today) + padDays };
}

export interface BarGeometry {
  readonly x: number;
  readonly width: number;
  /** The bar starts before the visible range. */
  readonly clippedStart: boolean;
  /** The bar ends after the visible range. */
  readonly clippedEnd: boolean;
}

/**
 * Horizontal position of a bar. The end day is inclusive. Bars are clipped to the scale; null when the
 * item has no dates or lies entirely outside the range.
 */
export function barGeometry(
  start: string | null,
  end: string | null,
  scale: TimelineScale,
): BarGeometry | null {
  const s0 = dayNumber(start);
  const e0 = dayNumber(end);
  if (s0 === null && e0 === null) return null;
  const s = s0 ?? (e0 as number);
  const e = Math.max(e0 ?? s, s);
  const lastDay = scale.endDay - 1;
  if (e < scale.startDay || s > lastDay) return null;
  const from = Math.max(s, scale.startDay);
  const to = Math.min(e, lastDay);
  const x = (from - scale.startDay) * scale.dayWidth;
  const width = Math.max(MIN_BAR_WIDTH, (to - from + 1) * scale.dayWidth);
  return { x, width, clippedStart: s < scale.startDay, clippedEnd: e > lastDay };
}

export interface PackedItem<T> {
  readonly item: T;
  readonly lane: number;
}

/**
 * Assigns items to lanes so that none of the items of a lane overlap. Items are placed greedily by
 * start; `gap` days of air are kept between neighbours of a lane. Items without dates are skipped.
 */
export function packLanes<T extends DatedRange>(
  items: readonly T[],
  gap = 1,
): { packed: PackedItem<T>[]; lanes: number } {
  const dated = items
    .map((item) => {
      const s0 = dayNumber(item.start);
      const e0 = dayNumber(item.end);
      if (s0 === null && e0 === null) return null;
      const s = s0 ?? (e0 as number);
      return { item, start: s, end: Math.max(e0 ?? s, s) };
    })
    .filter((entry): entry is { item: T; start: number; end: number } => entry !== null)
    .sort((a, b) => a.start - b.start || a.end - b.end);
  const laneEnds: number[] = [];
  const packed: PackedItem<T>[] = [];
  for (const entry of dated) {
    let lane = laneEnds.findIndex((end) => end + gap <= entry.start);
    if (lane < 0) {
      lane = laneEnds.length;
      laneEnds.push(entry.end);
    } else {
      laneEnds[lane] = entry.end;
    }
    packed.push({ item: entry.item, lane });
  }
  return { packed, lanes: laneEnds.length };
}
