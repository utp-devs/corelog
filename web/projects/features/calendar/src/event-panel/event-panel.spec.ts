import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { detail, item } from '../testing/calendar-fixtures';
import { EventPanel } from './event-panel';

function setup(patch: Record<string, unknown> = {}) {
  TestBed.configureTestingModule({ imports: [EventPanel] });
  const fixture = TestBed.createComponent(EventPanel);
  fixture.componentRef.setInput('item', item({ title: 'Retro' }));
  fixture.componentRef.setInput('detail', detail({ title: 'Retro', myResponse: 'needs_action' }));
  for (const [key, value] of Object.entries(patch)) fixture.componentRef.setInput(key, value);
  fixture.componentRef.setInput('open', true);
  fixture.detectChanges();
  return {
    fixture,
    el: fixture.nativeElement as HTMLElement,
    component: fixture.componentInstance,
  };
}

describe('EventPanel', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists when and the repetition in Spanish', () => {
    const { el } = setup({
      detail: detail({ recurrenceRule: 'FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10' }),
    });
    expect(el.querySelector('[data-testid="event-recurrence"]')?.textContent).toContain(
      'Cada semana los lunes y miércoles, 10 veces',
    );
    expect(el.querySelector('[data-testid="event-when"]')?.textContent).toContain('2026');
  });

  it('emits the answer of the invitation', () => {
    const { el, component } = setup();
    const respond = vi.fn();
    component.respond.subscribe(respond);
    el.querySelector<HTMLButtonElement>('[data-testid="rsvp-accepted"]')?.click();
    el.querySelector<HTMLButtonElement>('[data-testid="rsvp-declined"]')?.click();
    expect(respond.mock.calls).toEqual([['accepted'], ['declined']]);
  });

  it('hides the answer buttons for someone who is not invited', () => {
    const { el } = setup({ detail: detail({ myResponse: null }) });
    expect(el.querySelector('[data-testid="rsvp-accepted"]')).toBeNull();
  });

  it('offers edit and delete only to who can edit', () => {
    const editable = setup();
    expect(editable.el.querySelector('[data-testid="event-edit"]')).not.toBeNull();
    expect(editable.el.querySelector('[data-testid="event-delete"]')).not.toBeNull();
    TestBed.resetTestingModule();
    const readOnly = setup({ detail: detail({ canEdit: false }) });
    expect(readOnly.el.querySelector('[data-testid="event-edit"]')).toBeNull();
    expect(readOnly.el.querySelector('[data-testid="event-delete"]')).toBeNull();
  });
});
