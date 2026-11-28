import {
  NO_RECURRENCE,
  buildRecurrence,
  parseRecurrence,
  summarizeRecurrence,
  weekdayCodeOf,
} from './recurrence';

describe('recurrence', () => {
  it('parses and rebuilds a weekly rule', () => {
    const spec = parseRecurrence('FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10');
    expect(spec).toEqual({
      freq: 'WEEKLY',
      interval: 1,
      weekdays: ['MO', 'WE'],
      end: { kind: 'count', count: 10 },
    });
    expect(buildRecurrence(spec)).toBe('FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10');
  });

  it('reads UNTIL and interval', () => {
    const spec = parseRecurrence('FREQ=DAILY;INTERVAL=3;UNTIL=20261231T000000Z');
    expect(spec.interval).toBe(3);
    expect(spec.end).toEqual({ kind: 'until', date: '2026-12-31' });
    expect(buildRecurrence(spec)).toBe('FREQ=DAILY;INTERVAL=3;UNTIL=20261231');
  });

  it('treats empty or unknown rules as no repetition', () => {
    expect(parseRecurrence(null)).toBe(NO_RECURRENCE);
    expect(parseRecurrence('FREQ=HOURLY')).toBe(NO_RECURRENCE);
    expect(buildRecurrence(NO_RECURRENCE)).toBeNull();
  });

  it('summarizes in Spanish', () => {
    expect(summarizeRecurrence(parseRecurrence('FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10'))).toBe(
      'Cada semana los lunes y miércoles, 10 veces',
    );
    expect(summarizeRecurrence(parseRecurrence('FREQ=WEEKLY;BYDAY=TU'))).toBe(
      'Cada semana el martes',
    );
    expect(summarizeRecurrence(parseRecurrence('FREQ=DAILY;INTERVAL=2'))).toBe('Cada 2 días');
    expect(summarizeRecurrence(parseRecurrence('FREQ=MONTHLY;COUNT=1'))).toBe('Cada mes, 1 vez');
    expect(summarizeRecurrence(parseRecurrence('FREQ=YEARLY;UNTIL=20271231'))).toContain(
      'hasta el 31 de diciembre de 2027',
    );
    expect(summarizeRecurrence(NO_RECURRENCE)).toBe('No se repite');
  });

  it('maps a date to its weekday code', () => {
    expect(weekdayCodeOf(new Date(2026, 10, 23))).toBe('MO');
    expect(weekdayCodeOf(new Date(2026, 10, 29))).toBe('SU');
  });
});
