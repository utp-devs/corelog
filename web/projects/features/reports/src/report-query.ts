import { ParamMap, Params } from '@angular/router';
import { ReportQuery, isoDate } from '@corelog/core';

export const RANGE_PRESETS = [7, 30, 90] as const;
export type RangePreset = (typeof RANGE_PRESETS)[number];

/** Days of the default range of the server: the last 30 days, today included. */
export const DEFAULT_RANGE_DAYS = 30;

const DAY_MS = 86_400_000;

function parseIso(value: string): Date {
  const [year, month, day] = value.split('-').map(Number);
  return new Date(year, month - 1, day);
}

/** The ISO date `delta` days after `value`. */
export function addDays(value: string, delta: number): string {
  const date = parseIso(value);
  date.setDate(date.getDate() + delta);
  return isoDate(date);
}

/** Days between two ISO dates, inclusive of both ends. */
export function inclusiveDays(from: string, to: string): number {
  return Math.round((parseIso(to).getTime() - parseIso(from).getTime()) / DAY_MS) + 1;
}

/** Range of the last `days` days, today included. */
export function rangeForDays(days: number, today: string): { from: string; to: string } {
  return { from: addDays(today, -(days - 1)), to: today };
}

/** The range a query shows: its own dates, or the default of the server (last 30 days). */
export function effectiveRange(query: ReportQuery, today: string): { from: string; to: string } {
  const to = query.to || today;
  return { from: query.from || addDays(to, -(DEFAULT_RANGE_DAYS - 1)), to };
}

/** The preset that matches the range of the query, or null. */
export function activePreset(query: ReportQuery, today: string): RangePreset | null {
  const { from, to } = effectiveRange(query, today);
  if (to !== today) return null;
  return RANGE_PRESETS.find((days) => addDays(today, -(days - 1)) === from) ?? null;
}

const TEXT_KEYS = [
  ['from', 'from'],
  ['to', 'to'],
  ['groupBy', 'group_by'],
  ['filter', 'filter'],
  ['sprint', 'sprint'],
  ['version', 'version'],
  ['interval', 'interval'],
] as const;

/** Reads the report query from the URL query parameters. */
export function queryFromParams(params: ParamMap): ReportQuery {
  const query: Record<string, string | number> = {};
  for (const [field, key] of TEXT_KEYS) {
    const value = params.get(key);
    if (value) query[field] = value;
  }
  const limit = Number(params.get('limit'));
  if (Number.isInteger(limit) && limit > 0) query['limit'] = limit;
  return query as ReportQuery;
}

/** URL query parameters of a report query; empty values are null so the router drops them. */
export function queryToParams(query: ReportQuery): Params {
  const params: Params = {};
  for (const [field, key] of TEXT_KEYS) {
    params[key] = query[field] || null;
  }
  params['limit'] = query.limit ?? null;
  return params;
}
