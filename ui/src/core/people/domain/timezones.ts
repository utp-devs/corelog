/** Used when the runtime cannot list IANA zones (old browsers). */
export const FALLBACK_TIMEZONES = [
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
]

type IntlWithZones = { supportedValuesOf?: (key: 'timeZone') => string[] }

/** Sorted IANA zones; keeps the current value when it is not in the list (so the select shows it). */
export function timezoneOptions(current?: string | null): string[] {
  const supported = (Intl as unknown as IntlWithZones).supportedValuesOf
  let zones: string[] = FALLBACK_TIMEZONES
  if (typeof supported === 'function') {
    try {
      const listed = supported('timeZone')
      if (listed.length > 0) zones = listed
    } catch {
      zones = FALLBACK_TIMEZONES
    }
  }
  const unique = Array.from(new Set(zones))
  if (current && !unique.includes(current)) unique.unshift(current)
  return unique
}
