import type { Country, PhoneValue } from '../models/country.model';

/** Longest national number the E.164 plan allows. */
export const PHONE_MAX_DIGITS = 15;
const PHONE_MIN_DIGITS = 6;

/** Digits of a text, dropping spaces, dashes, dots and parentheses. */
export function phoneDigits(text: string): string {
  return text.replace(/\D/g, '');
}

/** Most digits a national number of the country may have. */
export function maxNationalDigits(country: Country | undefined): number {
  return country?.example ? country.example.length + 2 : PHONE_MAX_DIGITS;
}

/** Fewest digits a national number of the country may have. */
export function minNationalDigits(country: Country | undefined): number {
  return country?.example
    ? Math.max(PHONE_MIN_DIGITS, country.example.length - 2)
    : PHONE_MIN_DIGITS;
}

/** Group sizes used to space a national number of the given length. */
function groupsFor(length: number): readonly number[] {
  if (length <= 4) return [length];
  if (length === 7) return [3, 4];
  if (length === 8) return [4, 4];
  if (length === 9) return [3, 3, 3];
  if (length === 10) return [3, 3, 4];
  if (length === 11) return [3, 4, 4];
  const groups: number[] = [];
  for (let left = length; left > 0; left -= 3) groups.push(Math.min(3, left));
  return groups;
}

/**
 * Spaces national digits while the user types. The grouping follows the length of the country's example
 * (or the digits typed when there is none), so the text grows steadily and never reflows backwards.
 */
export function formatNational(digits: string, country: Country | undefined): string {
  const limit = country?.example ? country.example.length : Math.max(digits.length, 1);
  const pattern = groupsFor(Math.max(limit, digits.length));
  const parts: string[] = [];
  let at = 0;
  for (const size of pattern) {
    if (at >= digits.length) break;
    parts.push(digits.slice(at, at + size));
    at += size;
  }
  return parts.join(' ');
}

/** Placeholder of the national field: the example, spaced the same way the typed number is. */
export function phonePlaceholder(country: Country | undefined): string {
  return country?.example ? formatNational(country.example, country) : 'Número de teléfono';
}

/** True when the digits fit the length range of the country. */
export function isPlausibleNational(digits: string, country: Country | undefined): boolean {
  return digits.length >= minNationalDigits(country) && digits.length <= maxNationalDigits(country);
}

/** The text the API stores and shows: plus sign, prefix and national digits. */
export function toE164(country: Country | undefined, digits: string): string {
  return country && digits ? `${country.dialCode}${digits}` : '';
}

export interface InternationalMatch {
  readonly country: Country;
  /** Digits after the prefix. */
  readonly national: string;
}

/** Country that owns a +1 style number: the one whose area code matches, else the one without areas. */
function pickSharedPrefix(candidates: readonly Country[], national: string): Country {
  const byArea = candidates.find((c) => c.areas.some((area) => national.startsWith(area)));
  if (byArea) return byArea;
  return candidates.find((c) => c.areas.length === 0) ?? candidates[0];
}

/**
 * Reads a pasted international number (+51 912 345 678, 0051...). Returns the country that owns the
 * longest matching prefix and the digits left, or null when the text is not international.
 */
export function parseInternational(
  text: string,
  countries: readonly Country[],
): InternationalMatch | null {
  const trimmed = text.trim();
  const international = trimmed.startsWith('+') || trimmed.startsWith('00');
  if (!international) return null;
  const digits = phoneDigits(trimmed.startsWith('+') ? trimmed : trimmed.slice(2));
  for (let length = Math.min(4, digits.length); length >= 1; length -= 1) {
    const prefix = `+${digits.slice(0, length)}`;
    const candidates = countries.filter((c) => c.dialCode === prefix);
    if (candidates.length > 0) {
      const national = digits.slice(length);
      return { country: pickSharedPrefix(candidates, national), national };
    }
  }
  return null;
}

/** Flag emoji of an ISO country code (regional indicator symbols). */
export function flagOf(code: string): string {
  if (!/^[A-Za-z]{2}$/.test(code)) return '';
  return String.fromCodePoint(...[...code.toUpperCase()].map((c) => 0x1f1a5 + c.charCodeAt(0)));
}

/** Country matching the browser language region (es-CO gives CO), when the catalog has it. */
export function browserCountry(
  countries: readonly Country[],
  languages: readonly string[],
): Country | undefined {
  for (const language of languages) {
    const region = language.split('-')[1]?.toUpperCase();
    const found = region ? countries.find((c) => c.code === region) : undefined;
    if (found) return found;
  }
  return undefined;
}

export function samePhone(
  a: PhoneValue | null | undefined,
  b: PhoneValue | null | undefined,
): boolean {
  if (!a || !b) return !a && !b;
  return a.country === b.country && phoneDigits(a.number) === phoneDigits(b.number);
}
