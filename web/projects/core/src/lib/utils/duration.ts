/** Working time of an organization: how many hours make a day and how many days make a week. */
export interface WorkingTime {
  readonly hoursPerDay: number;
  readonly daysPerWeek: number;
}

export const DEFAULT_WORKING_TIME: WorkingTime = { hoursPerDay: 8, daysPerWeek: 5 };

/** The server accepts at most three 24-hour days in one worklog. */
export const MAX_DURATION_SECONDS = 3 * 24 * 3600;

const UNIT_PATTERN = /(\d+(?:[.,]\d+)?)\s*([wdhm])/gi;

/**
 * Parses a duration such as `1w 2d 3h 30m` into seconds, using the working time of the
 * organization (a day is `hoursPerDay` hours, a week `daysPerWeek` days). Units may appear once each,
 * in any order. Returns null when the text is empty, has unknown parts or repeats a unit.
 * A bare number is read as minutes only when `bareAsMinutes` is set.
 */
export function parseDuration(
  text: string,
  working: WorkingTime = DEFAULT_WORKING_TIME,
  bareAsMinutes = false,
): number | null {
  const source = text.trim().toLowerCase();
  if (source === '') return null;
  if (bareAsMinutes && /^\d+$/.test(source)) return Number(source) * 60;

  const perUnit: Record<string, number> = {
    m: 60,
    h: 3600,
    d: working.hoursPerDay * 3600,
    w: working.daysPerWeek * working.hoursPerDay * 3600,
  };
  const seen = new Set<string>();
  let total = 0;
  let consumed = 0;
  for (const match of source.matchAll(UNIT_PATTERN)) {
    const unit = match[2];
    if (seen.has(unit)) return null;
    seen.add(unit);
    total += Number(match[1].replace(',', '.')) * perUnit[unit];
    consumed += match[0].length;
  }
  if (seen.size === 0) return null;
  // Everything that is not a unit group must be whitespace.
  if (source.replace(UNIT_PATTERN, '').trim() !== '' || consumed === 0) return null;
  return Math.round(total);
}

/** Formats seconds as `1w 2d 3h 30m` (zero parts omitted); 0 becomes `0m`. */
export function formatDuration(
  seconds: number,
  working: WorkingTime = DEFAULT_WORKING_TIME,
): string {
  let rest = Math.max(0, Math.round(seconds));
  if (rest === 0) return '0m';
  const week = working.daysPerWeek * working.hoursPerDay * 3600;
  const day = working.hoursPerDay * 3600;
  const parts: string[] = [];
  for (const [unit, size] of [
    ['w', week],
    ['d', day],
    ['h', 3600],
    ['m', 60],
  ] as const) {
    const count = Math.floor(rest / size);
    if (count > 0) parts.push(`${count}${unit}`);
    rest -= count * size;
  }
  return parts.length > 0 ? parts.join(' ') : '1m';
}
