import { FALLBACK_TIMEZONES, timezoneOptions } from './timezones';

describe('timezoneOptions', () => {
  it('returns a unique list that always includes the current zone', () => {
    const zones = timezoneOptions('Mars/Olympus');
    expect(zones[0]).toBe('Mars/Olympus');
    expect(new Set(zones).size).toBe(zones.length);
  });

  it('lists the runtime zones, which include the common fallbacks', () => {
    const zones = timezoneOptions('UTC');
    expect(zones.length).toBeGreaterThanOrEqual(FALLBACK_TIMEZONES.length);
    expect(zones).toContain('America/Bogota');
  });

  it('falls back to the fixed list when the runtime cannot list zones', () => {
    const intl = Intl as unknown as { supportedValuesOf?: unknown };
    const original = intl.supportedValuesOf;
    intl.supportedValuesOf = () => {
      throw new RangeError('unsupported');
    };
    try {
      expect(timezoneOptions()).toEqual([...FALLBACK_TIMEZONES]);
    } finally {
      intl.supportedValuesOf = original;
    }
  });
});
