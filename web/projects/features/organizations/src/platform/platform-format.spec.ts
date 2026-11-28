import {
  OUTBOX_STATUS_TONES,
  durationLabel,
  relativeTimeLabel,
  scheduleLabel,
} from './platform-format';

const NOW = new Date('2026-11-27T12:00:00Z');

describe('platform format', () => {
  it('words past and future times in Spanish', () => {
    expect(relativeTimeLabel('2026-11-27T11:55:00Z', NOW)).toBe('hace 5 minutos');
    expect(relativeTimeLabel('2026-11-27T10:00:00Z', NOW)).toBe('hace 2 horas');
    expect(relativeTimeLabel('2026-11-27T12:00:30Z', NOW)).toBe('dentro de 30 segundos');
  });

  it('shows a dash for a missing or invalid date', () => {
    expect(relativeTimeLabel(null, NOW)).toBe('-');
    expect(relativeTimeLabel('nope', NOW)).toBe('-');
  });

  it('reads durations and schedules', () => {
    expect(durationLabel(120)).toBe('120 ms');
    expect(durationLabel(3200)).toBe('3,2 s');
    expect(durationLabel(null)).toBe('-');
    expect(scheduleLabel('every 10s')).toBe('Cada 10 segundos');
    expect(scheduleLabel('every 1h')).toBe('Cada hora');
    expect(scheduleLabel('daily at 08:00 UTC')).toBe('Cada día a las 08:00 UTC');
    expect(scheduleLabel('0 8 * * *')).toBe('0 8 * * *');
  });

  it('colors sent green, pending neutral and dead red', () => {
    expect(OUTBOX_STATUS_TONES).toEqual({ pending: 'neutral', sent: 'green', dead: 'red' });
  });
});
