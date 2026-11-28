import { TestBed } from '@angular/core/testing';
import { groupItemsByDay } from '@corelog/core';
import { vi } from 'vitest';

import { item } from '../testing/calendar-fixtures';
import { CalendarTimeGrid } from './calendar-time-grid';

function setup(mode: 'week' | 'day') {
  TestBed.configureTestingModule({ imports: [CalendarTimeGrid] });
  const fixture = TestBed.createComponent(CalendarTimeGrid);
  const items = [
    item({ title: 'Con hora' }),
    item({
      title: 'Todo el día',
      sourceId: 'ad',
      allDay: true,
      startsAt: '2026-11-25T00:00:00Z',
      endsAt: '2026-11-26T00:00:00Z',
    }),
    item({
      title: 'Solapado',
      sourceId: 'ov',
      startsAt: '2026-11-25T10:30:00',
      endsAt: '2026-11-25T11:30:00',
    }),
  ];
  fixture.componentRef.setInput('anchor', new Date(2026, 10, 25));
  fixture.componentRef.setInput('mode', mode);
  fixture.componentRef.setInput('itemsByDay', groupItemsByDay(items));
  fixture.detectChanges();
  return {
    fixture,
    el: fixture.nativeElement as HTMLElement,
    component: fixture.componentInstance,
  };
}

describe('CalendarTimeGrid', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows seven columns in week mode and one in day mode', () => {
    expect(setup('week').el.querySelectorAll('.calendar-time-grid__column').length).toBe(7);
    TestBed.resetTestingModule();
    expect(setup('day').el.querySelectorAll('.calendar-time-grid__column').length).toBe(1);
  });

  it('puts all-day items in their own row and timed items in the column', () => {
    const { el } = setup('week');
    expect(el.querySelector('[data-testid="calendar-allday"]')?.textContent).toContain(
      'Todo el día',
    );
    const events = el.querySelectorAll('.calendar-time-grid__event');
    expect(events.length).toBe(2);
    const lefts = Array.from(events).map((event) =>
      (event as HTMLElement).style.getPropertyValue('--left'),
    );
    expect(new Set(lefts).size).toBe(2);
  });

  it('reports a click on an item and on an empty slot', () => {
    const { fixture, el, component } = setup('day');
    const selected = vi.fn();
    const slot = vi.fn();
    component.itemSelected.subscribe(selected);
    component.slotActivated.subscribe(slot);
    el.querySelector<HTMLButtonElement>('.calendar-time-grid__event')?.click();
    expect(selected).toHaveBeenCalled();
    el.querySelector<HTMLElement>('.calendar-time-grid__column')?.dispatchEvent(
      new MouseEvent('click', { bubbles: true, clientY: 10 }),
    );
    fixture.detectChanges();
    expect(slot).toHaveBeenCalledWith(expect.objectContaining({ allDay: false }));
  });
});
