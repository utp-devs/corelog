import type { Country } from '../models/country.model';
import {
  browserCountry,
  flagOf,
  formatNational,
  isPlausibleNational,
  parseInternational,
  phoneDigits,
  phonePlaceholder,
  samePhone,
  toE164,
} from './phone-format';

const PE: Country = { code: 'PE', name: 'Perú', dialCode: '+51', example: '912345678', areas: [] };
const US: Country = {
  code: 'US',
  name: 'Estados Unidos',
  dialCode: '+1',
  example: '2025550123',
  areas: [],
};
const DO: Country = {
  code: 'DO',
  name: 'República Dominicana',
  dialCode: '+1',
  example: '8092345678',
  areas: ['809', '829', '849'],
};
const CO: Country = {
  code: 'CO',
  name: 'Colombia',
  dialCode: '+57',
  example: '3211234567',
  areas: [],
};
const NO_EXAMPLE: Country = {
  code: 'XX',
  name: 'Sin ejemplo',
  dialCode: '+999',
  example: null,
  areas: [],
};
const ALL = [PE, US, DO, CO, NO_EXAMPLE];

describe('phone formatting', () => {
  it('keeps only digits', () => {
    expect(phoneDigits('+51 (912) 345-678')).toBe('51912345678');
  });

  it('spaces the national number as it grows, following the length of the example', () => {
    expect(formatNational('9', PE)).toBe('9');
    expect(formatNational('91234', PE)).toBe('912 34');
    expect(formatNational('912345678', PE)).toBe('912 345 678');
    expect(formatNational('2025550123', US)).toBe('202 555 0123');
    expect(formatNational('', PE)).toBe('');
  });

  it('spaces a country without example by groups of three', () => {
    expect(formatNational('1234567', NO_EXAMPLE)).toBe('123 4567');
  });

  it('builds the placeholder from the example', () => {
    expect(phonePlaceholder(PE)).toBe('912 345 678');
    expect(phonePlaceholder(NO_EXAMPLE)).toBe('Número de teléfono');
    expect(phonePlaceholder(undefined)).toBe('Número de teléfono');
  });

  it('judges the length against the country', () => {
    expect(isPlausibleNational('912345678', PE)).toBe(true);
    expect(isPlausibleNational('12345', PE)).toBe(false);
    expect(isPlausibleNational('9123456789012', PE)).toBe(false);
    expect(isPlausibleNational('123456', NO_EXAMPLE)).toBe(true);
  });

  it('builds the E.164 text', () => {
    expect(toE164(PE, '912345678')).toBe('+51912345678');
    expect(toE164(PE, '')).toBe('');
    expect(toE164(undefined, '912')).toBe('');
  });
});

describe('parseInternational', () => {
  it('finds the country of a pasted international number', () => {
    expect(parseInternational('+51 912 345 678', ALL)).toEqual({
      country: PE,
      national: '912345678',
    });
    expect(parseInternational('0051912345678', ALL)?.country.code).toBe('PE');
    expect(parseInternational('+57 321 123 4567', ALL)?.country.code).toBe('CO');
  });

  it('tells countries that share +1 apart by area code', () => {
    expect(parseInternational('+1 809 234 5678', ALL)?.country.code).toBe('DO');
    expect(parseInternational('+1 202 555 0123', ALL)?.country.code).toBe('US');
  });

  it('ignores text that is not international or has an unknown prefix', () => {
    expect(parseInternational('912345678', ALL)).toBeNull();
    expect(parseInternational('+5', ALL)).toBeNull();
    expect(parseInternational('+000 12', ALL)).toBeNull();
  });
});

describe('phone helpers', () => {
  it('draws a flag from the country code', () => {
    expect(flagOf('pe')).toBe('\u{1F1F5}\u{1F1EA}');
    expect(flagOf('')).toBe('');
    expect(flagOf('PER')).toBe('');
  });

  it('picks the country of the browser language when the catalog has it', () => {
    expect(browserCountry(ALL, ['es-CO', 'en'])?.code).toBe('CO');
    expect(browserCountry(ALL, ['es', 'en-ZZ'])).toBeUndefined();
  });

  it('compares phones by country and digits', () => {
    expect(
      samePhone({ country: 'PE', number: '912 345 678' }, { country: 'PE', number: '912345678' }),
    ).toBe(true);
    expect(
      samePhone({ country: 'PE', number: '912345678' }, { country: 'CO', number: '912345678' }),
    ).toBe(false);
    expect(samePhone(null, undefined)).toBe(true);
    expect(samePhone(null, { country: 'PE', number: '9' })).toBe(false);
  });
});
