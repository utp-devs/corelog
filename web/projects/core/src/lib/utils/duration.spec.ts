import { DEFAULT_WORKING_TIME, formatDuration, parseDuration } from './duration';

describe('parseDuration', () => {
  it('reads every unit with the working time of the organization', () => {
    expect(parseDuration('1w 2d 3h 30m')).toBe((5 * 8 + 2 * 8 + 3) * 3600 + 30 * 60);
    expect(parseDuration('1d', { hoursPerDay: 6, daysPerWeek: 4 })).toBe(6 * 3600);
    expect(parseDuration('1w', { hoursPerDay: 6, daysPerWeek: 4 })).toBe(24 * 3600);
  });

  it('accepts any order, no spaces, decimals and upper case', () => {
    expect(parseDuration('30m2h')).toBe(2.5 * 3600);
    expect(parseDuration('1.5H')).toBe(5400);
    expect(parseDuration('1,5h')).toBe(5400);
  });

  it('rejects empty text, unknown parts and repeated units', () => {
    expect(parseDuration('')).toBeNull();
    expect(parseDuration('abc')).toBeNull();
    expect(parseDuration('2x')).toBeNull();
    expect(parseDuration('1h 2h')).toBeNull();
    expect(parseDuration('1h foo')).toBeNull();
    expect(parseDuration('90')).toBeNull();
  });

  it('reads a bare number as minutes only when asked', () => {
    expect(parseDuration('90', DEFAULT_WORKING_TIME, true)).toBe(5400);
  });

  it('keeps zero durations', () => {
    expect(parseDuration('0m')).toBe(0);
  });
});

describe('formatDuration', () => {
  it('writes the largest units first and omits zero parts', () => {
    expect(formatDuration(5400)).toBe('1h 30m');
    expect(formatDuration(8 * 3600)).toBe('1d');
    expect(formatDuration((5 * 8 + 8 + 1) * 3600)).toBe('1w 1d 1h');
    expect(formatDuration(0)).toBe('0m');
  });

  it('round-trips with parseDuration', () => {
    const text = '2d 4h 15m';
    expect(formatDuration(parseDuration(text) ?? 0)).toBe(text);
  });
});
