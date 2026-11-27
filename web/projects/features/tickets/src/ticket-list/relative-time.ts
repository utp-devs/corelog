const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
const WEEK_MS = 7 * DAY_MS;

const FORMATTER = new Intl.RelativeTimeFormat('es', { numeric: 'auto' });

/**
 * Short relative time in Spanish ("hace 3 horas", "ayer") for dates up to four weeks old. Older
 * dates, future dates and invalid input return null, so the caller shows the absolute date.
 */
export function relativeTime(iso: string, now: Date = new Date()): string | null {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) {
    return null;
  }
  const elapsed = now.getTime() - then;
  if (elapsed < 0 || elapsed >= 4 * WEEK_MS) {
    return null;
  }
  if (elapsed < MINUTE_MS) {
    return 'hace un momento';
  }
  if (elapsed < HOUR_MS) {
    return FORMATTER.format(-Math.floor(elapsed / MINUTE_MS), 'minute');
  }
  if (elapsed < DAY_MS) {
    return FORMATTER.format(-Math.floor(elapsed / HOUR_MS), 'hour');
  }
  if (elapsed < WEEK_MS) {
    return FORMATTER.format(-Math.floor(elapsed / DAY_MS), 'day');
  }
  return FORMATTER.format(-Math.floor(elapsed / WEEK_MS), 'week');
}

/** Readable short key of an issue: the prefix and the first six characters of its identifier. */
export function shortIssueKey(fullKey: string): string {
  const dash = fullKey.indexOf('-');
  if (dash < 0) {
    return fullKey;
  }
  return `${fullKey.slice(0, dash + 1)}${fullKey.slice(dash + 1, dash + 7)}`;
}
