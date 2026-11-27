import { describe, expect, it } from 'vitest';

import { relativeTime } from './relative-time';

const NOW = new Date('2026-11-26T12:00:00Z');

describe('relativeTime', () => {
  it('describes recent dates in Spanish', () => {
    expect(relativeTime('2026-11-26T11:59:40Z', NOW)).toBe('hace un momento');
    expect(relativeTime('2026-11-26T11:30:00Z', NOW)).toBe('hace 30 minutos');
    expect(relativeTime('2026-11-26T09:00:00Z', NOW)).toBe('hace 3 horas');
    expect(relativeTime('2026-11-25T11:00:00Z', NOW)).toBe('ayer');
    expect(relativeTime('2026-11-12T12:00:00Z', NOW)).toBe('hace 2 semanas');
  });

  it('returns null for old, future or invalid dates', () => {
    expect(relativeTime('2026-09-01T00:00:00Z', NOW)).toBeNull();
    expect(relativeTime('2026-12-01T00:00:00Z', NOW)).toBeNull();
    expect(relativeTime('not a date', NOW)).toBeNull();
  });
});
