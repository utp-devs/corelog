import { ComponentFixture, TestBed } from '@angular/core/testing';
import { groupItemsByDay, type CalendarItem } from '@corelog/core';
import { vi } from 'vitest';

import { item } from '../testing/calendar-fixtures';
import { CalendarMonth } from './calendar-month';

function setup(items: CalendarItem[]): {
  fixture: ComponentFixture<CalendarMonth>;
  el: HTMLElement;
  component: CalendarMonth;
} {
  TestBed.configureTestingModule({ imports: [CalendarMonth] });
  const fixture = TestBed.createComponent(CalendarMonth);
  fixture.componentRef.setInput('anchor', new Date(2026, 10, 15));
  fixture.componentRef.setInput('itemsByDay', groupItemsByDay(items));
  fixture.detectChanges();
  return {
    fixture,
    el: fixture.nativeElement as HTMLElement,
    component: fixture.componentInstance,
  };
}

describe('CalendarMonth', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('draws whole weeks starting on Monday', () => {
    const { el } = setup([]);
    const headers = Array.from(el.querySelectorAll('[role="columnheader"]')).map((h) =>
      h.textContent?.trim(),
    );
    expect(headers[0]).toBe('Lun');
    expect(headers[6]).toBe('Dom');
    expect(el.querySelectorAll('[role="gridcell"]').length % 7).toBe(0);
    expect(el.querySelector('[data-day="2026-11-25"]')?.getAttribute('aria-description')).toContain(
      'miércoles, 25 de noviembre de 2026',
    );
  });

  it('shows the items of a day and reports a click', () => {
    const { fixture, el, component } = setup([item({ title: 'Demo' })]);
    const selected = vi.fn();
    component.itemSelected.subscribe(selected);
    const chip = el.querySelector<HTMLButtonElement>(
      '[data-day="2026-11-25"] button[aria-label^="Evento"]',
    );
    expect(chip?.textContent).toContain('Demo');
    chip?.click();
    fixture.detectChanges();
    expect(selected).toHaveBeenCalledWith(expect.objectContaining({ title: 'Demo' }));
  });

  it('collapses extra items into a "+N más" link', () => {
    const many = ['A', 'B', 'C', 'D', 'E'].map((title) => item({ title, sourceId: title }));
    const { el } = setup(many);
    const more = el.querySelector('[data-day="2026-11-25"] .calendar-month__more');
    expect(more?.textContent).toContain('+2 más');
  });

  it('moves focus with the arrow keys and asks the host when leaving the window', () => {
    const { fixture, el, component } = setup([]);
    const left = vi.fn();
    component.outOfRange.subscribe(left);
    const cell = el.querySelector<HTMLElement>('[data-day="2026-11-25"]');
    cell?.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    fixture.detectChanges();
    expect(left).not.toHaveBeenCalled();
    const last = el.querySelector<HTMLElement>('[data-day="2026-12-06"]');
    last?.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(left).toHaveBeenCalledTimes(1);
  });

  it('starts a new event with Enter on a day', () => {
    const { el, component } = setup([]);
    const activated = vi.fn();
    component.dayActivated.subscribe(activated);
    el.querySelector<HTMLElement>('[data-day="2026-11-10"]')?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
    );
    expect(activated).toHaveBeenCalledTimes(1);
    expect((activated.mock.calls[0][0] as Date).getDate()).toBe(10);
  });
});
