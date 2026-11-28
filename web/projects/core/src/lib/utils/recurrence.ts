/** Repetition of a calendar event, in the subset of RFC 5545 the API accepts. */
export type RecurrenceFreq = 'NONE' | 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';
export type WeekdayCode = 'MO' | 'TU' | 'WE' | 'TH' | 'FR' | 'SA' | 'SU';

export type RecurrenceEnd =
  | { readonly kind: 'never' }
  | { readonly kind: 'count'; readonly count: number }
  | { readonly kind: 'until'; readonly date: string };

export interface RecurrenceSpec {
  readonly freq: RecurrenceFreq;
  readonly interval: number;
  /** Only used by weekly rules. */
  readonly weekdays: readonly WeekdayCode[];
  readonly end: RecurrenceEnd;
}

export const WEEKDAY_CODES: readonly WeekdayCode[] = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU'];

/** Plural names, for "los lunes y miércoles". */
const WEEKDAY_PLURAL: Readonly<Record<WeekdayCode, string>> = {
  MO: 'lunes',
  TU: 'martes',
  WE: 'miércoles',
  TH: 'jueves',
  FR: 'viernes',
  SA: 'sábados',
  SU: 'domingos',
};

/** Singular names, for "el lunes". */
const WEEKDAY_SINGULAR: Readonly<Record<WeekdayCode, string>> = {
  MO: 'lunes',
  TU: 'martes',
  WE: 'miércoles',
  TH: 'jueves',
  FR: 'viernes',
  SA: 'sábado',
  SU: 'domingo',
};

export const WEEKDAY_SHORT: Readonly<Record<WeekdayCode, string>> = {
  MO: 'L',
  TU: 'M',
  WE: 'X',
  TH: 'J',
  FR: 'V',
  SA: 'S',
  SU: 'D',
};

export const WEEKDAY_LABEL: Readonly<Record<WeekdayCode, string>> = {
  MO: 'Lunes',
  TU: 'Martes',
  WE: 'Miércoles',
  TH: 'Jueves',
  FR: 'Viernes',
  SA: 'Sábado',
  SU: 'Domingo',
};

export const NO_RECURRENCE: RecurrenceSpec = {
  freq: 'NONE',
  interval: 1,
  weekdays: [],
  end: { kind: 'never' },
};

export const MAX_RECURRENCE_COUNT = 999;
export const MAX_RECURRENCE_INTERVAL = 99;

/** Weekday code of a local date (getDay 0 is Sunday). */
export function weekdayCodeOf(date: Date): WeekdayCode {
  return WEEKDAY_CODES[(date.getDay() + 6) % 7];
}

function isWeekday(value: string): value is WeekdayCode {
  return (WEEKDAY_CODES as readonly string[]).includes(value);
}

function untilToIso(value: string): string | null {
  const match = /^(\d{4})(\d{2})(\d{2})/.exec(value);
  return match ? `${match[1]}-${match[2]}-${match[3]}` : null;
}

/** Reads a rule such as `FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10`. Anything unreadable is "no repetition". */
export function parseRecurrence(rule: string | null | undefined): RecurrenceSpec {
  if (!rule) return NO_RECURRENCE;
  const parts = new Map<string, string>();
  for (const piece of rule.replace(/^RRULE:/i, '').split(';')) {
    const [key, value] = piece.split('=');
    if (key && value !== undefined) parts.set(key.trim().toUpperCase(), value.trim());
  }
  const freq = parts.get('FREQ');
  if (freq !== 'DAILY' && freq !== 'WEEKLY' && freq !== 'MONTHLY' && freq !== 'YEARLY') {
    return NO_RECURRENCE;
  }
  const interval = Math.min(
    Math.max(Number.parseInt(parts.get('INTERVAL') ?? '1', 10) || 1, 1),
    MAX_RECURRENCE_INTERVAL,
  );
  const weekdays = (parts.get('BYDAY') ?? '')
    .split(',')
    .map((code) => code.trim().toUpperCase())
    .filter(isWeekday);
  const count = Number.parseInt(parts.get('COUNT') ?? '', 10);
  const until = untilToIso(parts.get('UNTIL') ?? '');
  const end: RecurrenceEnd =
    Number.isFinite(count) && count > 0
      ? { kind: 'count', count }
      : until
        ? { kind: 'until', date: until }
        : { kind: 'never' };
  return { freq, interval, weekdays: freq === 'WEEKLY' ? weekdays : [], end };
}

/** Rule text for the API, or null when the event does not repeat. */
export function buildRecurrence(spec: RecurrenceSpec): string | null {
  if (spec.freq === 'NONE') return null;
  const parts = [`FREQ=${spec.freq}`];
  if (spec.interval > 1) parts.push(`INTERVAL=${spec.interval}`);
  if (spec.freq === 'WEEKLY' && spec.weekdays.length > 0) {
    const ordered = WEEKDAY_CODES.filter((code) => spec.weekdays.includes(code));
    parts.push(`BYDAY=${ordered.join(',')}`);
  }
  if (spec.end.kind === 'count') parts.push(`COUNT=${spec.end.count}`);
  if (spec.end.kind === 'until') parts.push(`UNTIL=${spec.end.date.replaceAll('-', '')}`);
  return parts.join(';');
}

function joinList(items: readonly string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} y ${items[items.length - 1]}`;
}

function longDate(iso: string): string {
  const [year, month, day] = iso.split('-').map(Number);
  return new Date(year, month - 1, day).toLocaleDateString('es-ES', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

/** Human summary in Spanish, for example "Cada semana los lunes y miércoles, 10 veces". */
export function summarizeRecurrence(spec: RecurrenceSpec): string {
  if (spec.freq === 'NONE') return 'No se repite';
  const n = spec.interval;
  let text: string;
  switch (spec.freq) {
    case 'DAILY':
      text = n === 1 ? 'Cada día' : `Cada ${n} días`;
      break;
    case 'WEEKLY': {
      text = n === 1 ? 'Cada semana' : `Cada ${n} semanas`;
      const days = WEEKDAY_CODES.filter((code) => spec.weekdays.includes(code));
      if (days.length === 1) text += ` el ${WEEKDAY_SINGULAR[days[0]]}`;
      else if (days.length > 1)
        text += ` los ${joinList(days.map((code) => WEEKDAY_PLURAL[code]))}`;
      break;
    }
    case 'MONTHLY':
      text = n === 1 ? 'Cada mes' : `Cada ${n} meses`;
      break;
    default:
      text = n === 1 ? 'Cada año' : `Cada ${n} años`;
  }
  if (spec.end.kind === 'count') {
    text += spec.end.count === 1 ? ', 1 vez' : `, ${spec.end.count} veces`;
  } else if (spec.end.kind === 'until') {
    text += `, hasta el ${longDate(spec.end.date)}`;
  }
  return text;
}
