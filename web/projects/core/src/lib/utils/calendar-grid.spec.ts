import type { CalendarItem } from '../models/calendar-v2.model';
import {
  addDays,
  groupItemsByDay,
  isoToZoned,
  itemDays,
  layoutTimedItems,
  monthGridDays,
  shiftAnchor,
  startOfWeek,
  viewRange,
  viewTitle,
  weekDays,
  zonedToIso,
} from './calendar-grid';

function item(over: Partial<CalendarItem>): CalendarItem {
  return {
    kind: 'event',
    title: 'Evento',
    startsAt: '2026-11-25T10:00:00',
    endsAt: '2026-11-25T11:00:00',
    allDay: false,
    date: '2026-11-25',
    sourceId: 'e1',
    projectId: null,
    projectKey: null,
    link: '',
    color: '',
    location: '',
    timezone: '',
    visibility: 'private',
    recurring: false,
    occurrenceStart: null,
    canEdit: true,
    myResponse: null,
    ...over,
  };
}

describe('calendar grid math', () => {
  it('starts weeks on Monday', () => {
    expect(startOfWeek(new Date(2026, 10, 29)).getDate()).toBe(23);
    expect(weekDays(new Date(2026, 10, 25)).map((d) => d.getDate())).toEqual([
      23, 24, 25, 26, 27, 28, 29,
    ]);
  });

  it('builds whole weeks around a month', () => {
    const days = monthGridDays(new Date(2026, 10, 15));
    expect(days.length % 7).toBe(0);
    expect(days[0].getDay()).toBe(1);
    expect(days[0].getMonth()).toBe(9);
    expect(days[days.length - 1].getDay()).toBe(0);
  });

  it('computes the requested range per view', () => {
    expect(viewRange('day', new Date(2026, 10, 25))).toEqual({
      from: '2026-11-25',
      to: '2026-11-25',
    });
    expect(viewRange('week', new Date(2026, 10, 25))).toEqual({
      from: '2026-11-23',
      to: '2026-11-29',
    });
    expect(viewRange('month', new Date(2026, 10, 25)).from).toBe('2026-10-26');
  });

  it('shifts by the unit of the view', () => {
    const anchor = new Date(2026, 10, 25);
    expect(shiftAnchor('day', anchor, 1).getDate()).toBe(26);
    expect(shiftAnchor('week', anchor, -1).getDate()).toBe(18);
    expect(shiftAnchor('month', anchor, 1)).toEqual(new Date(2026, 11, 1));
    expect(addDays(anchor, 10).getMonth()).toBe(11);
  });

  it('titles the views in Spanish', () => {
    expect(viewTitle('month', new Date(2026, 10, 25))).toBe('Noviembre de 2026');
    expect(viewTitle('week', new Date(2026, 10, 25))).toContain('23');
    expect(viewTitle('day', new Date(2026, 10, 25))).toContain('25 de noviembre');
  });

  it('lists the days of all-day items with an exclusive end', () => {
    const multi = item({
      allDay: true,
      date: '2026-11-25',
      startsAt: '2026-11-25T00:00:00Z',
      endsAt: '2026-11-28T00:00:00Z',
    });
    expect(itemDays(multi)).toEqual(['2026-11-25', '2026-11-26', '2026-11-27']);
    const single = item({
      allDay: true,
      date: '2026-11-19',
      startsAt: '2026-11-19T00:00:00Z',
      endsAt: '2026-11-20T00:00:00Z',
    });
    expect(itemDays(single)).toEqual(['2026-11-19']);
  });

  it('does not spill a timed item ending at midnight into the next day', () => {
    const late = item({ startsAt: '2026-11-25T22:00:00', endsAt: '2026-11-26T00:00:00' });
    expect(itemDays(late)).toEqual(['2026-11-25']);
  });

  it('groups by day with all-day items first', () => {
    const timed = item({ title: 'B' });
    const allDay = item({
      allDay: true,
      title: 'A',
      startsAt: '2026-11-25T00:00:00Z',
      endsAt: '2026-11-26T00:00:00Z',
    });
    const grouped = groupItemsByDay([timed, allDay]);
    expect(grouped['2026-11-25'].map((entry) => entry.title)).toEqual(['A', 'B']);
  });

  it('puts overlapping items side by side', () => {
    const a = item({ title: 'a', startsAt: '2026-11-25T10:00:00', endsAt: '2026-11-25T11:00:00' });
    const b = item({ title: 'b', startsAt: '2026-11-25T10:30:00', endsAt: '2026-11-25T11:30:00' });
    const c = item({ title: 'c', startsAt: '2026-11-25T13:00:00', endsAt: '2026-11-25T14:00:00' });
    const placed = layoutTimedItems([c, b, a]);
    const byTitle = Object.fromEntries(placed.map((p) => [p.item.title, p]));
    expect(byTitle['a'].lanes).toBe(2);
    expect(byTitle['b'].lane).toBe(1);
    expect(byTitle['c'].lanes).toBe(1);
    expect(byTitle['c'].startMinute).toBe(13 * 60);
  });

  it('converts wall time in a zone to UTC and back', () => {
    expect(zonedToIso('2026-11-25', '10:00', 'UTC')).toBe('2026-11-25T10:00:00Z');
    expect(zonedToIso('2026-11-25', '10:00', 'America/Bogota')).toBe('2026-11-25T15:00:00Z');
    expect(zonedToIso('2026-07-01', '10:00', 'Europe/Madrid')).toBe('2026-07-01T08:00:00Z');
    expect(isoToZoned('2026-11-25T15:00:00Z', 'America/Bogota')).toEqual({
      date: '2026-11-25',
      time: '10:00',
    });
  });
});
