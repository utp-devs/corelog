import {
  DAY_WIDTH,
  MIN_BAR_WIDTH,
  barGeometry,
  buildScale,
  dayNumber,
  defaultRange,
  isoOfDay,
  packLanes,
} from './roadmap-timeline';

const day = (iso: string) => dayNumber(iso) as number;

describe('roadmap timeline', () => {
  it('converts ISO days to numbers and back', () => {
    expect(dayNumber('1970-01-02')).toBe(1);
    expect(isoOfDay(day('2026-11-27'))).toBe('2026-11-27');
    expect(dayNumber(null)).toBeNull();
    expect(dayNumber('mañana')).toBeNull();
  });

  it('builds a month scale from whole months with labels and positions', () => {
    const scale = buildScale(day('2026-11-15'), day('2027-01-10'), 'month', null);
    expect(scale.ticks.map((t) => t.label)).toEqual(['nov 2026', 'dic 2026', 'ene 2027']);
    expect(scale.startDay).toBe(day('2026-11-01'));
    expect(scale.endDay).toBe(day('2027-02-01'));
    expect(scale.ticks[1].x).toBe(30 * DAY_WIDTH.month);
    expect(scale.width).toBe((day('2027-02-01') - day('2026-11-01')) * DAY_WIDTH.month);
    expect(scale.todayX).toBeNull();
  });

  it('starts week scales on Monday and quarter scales on the quarter', () => {
    const week = buildScale(day('2026-11-25'), day('2026-12-08'), 'week', null);
    expect(isoOfDay(week.startDay)).toBe('2026-11-23');
    expect(week.ticks[0].label).toBe('23 nov');
    const quarter = buildScale(day('2026-11-25'), day('2027-02-08'), 'quarter', null);
    expect(quarter.ticks.map((t) => t.label)).toEqual(['T4 2026', 'T1 2027']);
  });

  it('places the today line only inside the range', () => {
    const inside = buildScale(day('2026-11-01'), day('2026-12-31'), 'month', day('2026-11-11'));
    expect(inside.todayX).toBe(10 * DAY_WIDTH.month);
    const outside = buildScale(day('2026-11-01'), day('2026-12-31'), 'month', day('2027-05-01'));
    expect(outside.todayX).toBeNull();
  });

  it('positions bars with an inclusive end day', () => {
    const scale = buildScale(day('2026-11-01'), day('2026-12-31'), 'month', null);
    const geometry = barGeometry('2026-11-03', '2026-11-12', scale);
    expect(geometry).toMatchObject({ x: 2 * DAY_WIDTH.month, clippedStart: false });
    expect(geometry?.width).toBe(10 * DAY_WIDTH.month);
  });

  it('keeps a one day bar visible and treats a single date as a one day bar', () => {
    const scale = buildScale(day('2026-11-01'), day('2026-12-31'), 'quarter', null);
    expect(barGeometry('2026-11-03', '2026-11-03', scale)?.width).toBe(MIN_BAR_WIDTH);
    expect(barGeometry(null, '2026-11-03', scale)).not.toBeNull();
  });

  it('clips bars to the range and skips what is outside or undated', () => {
    const scale = buildScale(day('2026-11-01'), day('2026-11-30'), 'month', null);
    const clipped = barGeometry('2026-10-01', '2026-12-31', scale);
    expect(clipped).toMatchObject({ x: 0, clippedStart: true, clippedEnd: true });
    expect(barGeometry('2025-01-01', '2025-02-01', scale)).toBeNull();
    expect(barGeometry(null, null, scale)).toBeNull();
  });

  it('shows the dated items with air and always includes today', () => {
    const today = day('2026-11-27');
    const range = defaultRange([{ start: '2026-12-01', end: '2026-12-31' }], today, 10);
    expect(range).toEqual({ from: today - 10, to: day('2026-12-31') + 10 });
    const empty = defaultRange([{ start: null, end: null }], today);
    expect(empty.from).toBeLessThan(today);
    expect(empty.to).toBeGreaterThan(today);
  });

  it('packs overlapping items into separate lanes and reuses free ones', () => {
    const { packed, lanes } = packLanes(
      [
        { start: '2026-11-01', end: '2026-11-15' },
        { start: '2026-11-10', end: '2026-11-20' },
        { start: '2026-11-18', end: '2026-11-30' },
        { start: null, end: null },
      ],
      1,
    );
    expect(lanes).toBe(2);
    expect(packed.map((p) => p.lane)).toEqual([0, 1, 0]);
  });

  it('never overlaps two items of the same lane', () => {
    const items = Array.from({ length: 30 }, (_, i) => ({
      start: isoOfDay(day('2026-01-01') + ((i * 7) % 50)),
      end: isoOfDay(day('2026-01-01') + ((i * 7) % 50) + (i % 9) + 1),
    }));
    const { packed } = packLanes(items, 1);
    for (const a of packed) {
      for (const b of packed) {
        if (a === b || a.lane !== b.lane) continue;
        const apart = day(a.item.end) < day(b.item.start) || day(b.item.end) < day(a.item.start);
        expect(apart).toBe(true);
      }
    }
  });
});
