/** Used when the runtime cannot list IANA zones (old browsers). */
export const FALLBACK_TIMEZONES: readonly string[] = [
  'UTC',
  'America/Bogota',
  'America/Mexico_City',
  'America/Lima',
  'America/Santiago',
  'America/Argentina/Buenos_Aires',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'Europe/Madrid',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Australia/Sydney',
];

interface IntlWithZones {
  readonly supportedValuesOf?: (key: 'timeZone') => string[];
}

/** Runtime IANA zones, or the fallback list. Keeps the current value when it is not in the list. */
function runtimeZones(): readonly string[] {
  const supported = (Intl as unknown as IntlWithZones).supportedValuesOf;
  if (typeof supported !== 'function') return FALLBACK_TIMEZONES;
  try {
    const listed = supported('timeZone');
    return listed.length > 0 ? listed : FALLBACK_TIMEZONES;
  } catch {
    return FALLBACK_TIMEZONES;
  }
}

/** Unique zones for a select, with the current value first when it is missing from the list. */
export function timezoneOptions(current?: string | null): string[] {
  const unique = Array.from(new Set(runtimeZones()));
  if (current && !unique.includes(current)) unique.unshift(current);
  return unique;
}
