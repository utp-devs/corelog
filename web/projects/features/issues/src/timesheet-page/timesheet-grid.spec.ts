import { Timesheet } from '@corelog/core';

import { buildTimesheetGrid, shiftWeeks, toIsoDate, weekDays } from './timesheet-grid';

describe('weekDays', () => {
  it('starts on Monday and spans seven days', () => {
    const days = weekDays(new Date(2026, 10, 27));
    expect(days[0]).toBe('2026-11-23');
    expect(days[6]).toBe('2026-11-29');
    expect(days).toHaveLength(7);
  });

  it('treats Sunday as the last day of its week', () => {
    expect(weekDays(new Date(2026, 10, 29))[0]).toBe('2026-11-23');
  });

  it('shifts by whole weeks', () => {
    expect(toIsoDate(shiftWeeks(new Date(2026, 10, 27), -1))).toBe('2026-11-20');
  });
});

describe('buildTimesheetGrid', () => {
  const entry = (id: string, issueId: string, key: string, seconds: number) => ({
    id,
    issueId,
    issueKey: key,
    issueTitle: `Titulo ${key}`,
    startedAt: '2026-11-24T10:00:00Z',
    timeSpentSeconds: seconds,
    timeSpent: '',
    description: '',
  });
  const sheet: Timesheet = {
    from: '2026-11-23',
    to: '2026-11-29',
    totalSeconds: 5 * 3600,
    total: '5h',
    days: [
      {
        date: '2026-11-24',
        totalSeconds: 3 * 3600,
        total: '3h',
        capacitySeconds: 8 * 3600,
        entries: [entry('a', 'i1', 'COR-2', 7200), entry('b', 'i2', 'COR-1', 3600)],
      },
      {
        date: '2026-11-26',
        totalSeconds: 2 * 3600,
        total: '2h',
        capacitySeconds: 8 * 3600,
        entries: [entry('c', 'i1', 'COR-2', 7200)],
      },
    ],
  };

  it('pivots entries into one row per issue with the seconds of each day', () => {
    const grid = buildTimesheetGrid(sheet, weekDays(new Date(2026, 10, 24)));
    expect(grid.rows.map((row) => row.issueKey)).toEqual(['COR-1', 'COR-2']);
    expect(grid.rows[1].cells).toEqual([0, 7200, 0, 7200, 0, 0, 0]);
    expect(grid.rows[1].totalSeconds).toBe(14400);
    expect(grid.totalSeconds).toBe(5 * 3600);
  });

  it('keeps empty days and the capacity of the logged days', () => {
    const grid = buildTimesheetGrid(sheet, weekDays(new Date(2026, 10, 24)));
    expect(grid.columns[0]).toEqual({ date: '2026-11-23', totalSeconds: 0, capacitySeconds: 0 });
    expect(grid.columns[1].capacitySeconds).toBe(8 * 3600);
  });

  it('handles a missing timesheet', () => {
    const grid = buildTimesheetGrid(null, weekDays(new Date(2026, 10, 24)));
    expect(grid.rows).toEqual([]);
    expect(grid.totalSeconds).toBe(0);
  });
});
