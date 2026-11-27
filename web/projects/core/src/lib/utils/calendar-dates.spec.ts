import { isoDate, monthCells, monthRange, shiftMonth } from './calendar-dates';

describe('calendar dates', () => {
  it('formats local dates as AAAA-MM-DD', () => {
    expect(isoDate(new Date(2026, 10, 5, 23, 59))).toBe('2026-11-05');
  });

  it('returns the inclusive range of a month, including leap years', () => {
    expect(monthRange(new Date(2026, 10, 15))).toEqual({ from: '2026-11-01', to: '2026-11-30' });
    expect(monthRange(new Date(2028, 1, 10))).toEqual({ from: '2028-02-01', to: '2028-02-29' });
  });

  it('builds Monday-first grids padded to whole weeks', () => {
    // November 2026 starts on a Sunday, so six empty cells come before day 1.
    const cells = monthCells(new Date(2026, 10, 1));
    expect(cells.length % 7).toBe(0);
    expect(cells.slice(0, 6).every((cell) => cell === null)).toBe(true);
    expect(cells[6]?.getDate()).toBe(1);
    expect(cells.filter((cell) => cell !== null)).toHaveLength(30);
  });

  it('adds no leading gap when the month starts on Monday', () => {
    // June 2026 starts on Monday and ends on Tuesday, so only trailing gaps are added.
    const cells = monthCells(new Date(2026, 5, 1));
    expect(cells[0]?.getDate()).toBe(1);
    expect(cells).toHaveLength(35);
  });

  it('moves across year boundaries and pins the day to the first', () => {
    expect(isoDate(shiftMonth(new Date(2026, 0, 20), -1))).toBe('2025-12-01');
    expect(isoDate(shiftMonth(new Date(2026, 11, 3), 1))).toBe('2027-01-01');
  });
});
