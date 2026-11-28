import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { detail } from '../testing/calendar-fixtures';
import { EventDialog, type EventSaveRequest } from './event-dialog';

function setup(patch: Record<string, unknown> = {}) {
  TestBed.configureTestingModule({ imports: [EventDialog] });
  const fixture = TestBed.createComponent(EventDialog);
  fixture.componentRef.setInput('timezone', 'UTC');
  fixture.componentRef.setInput('start', { date: '2026-11-25', time: '09:00', allDay: false });
  for (const [key, value] of Object.entries(patch)) fixture.componentRef.setInput(key, value);
  fixture.componentRef.setInput('open', true);
  fixture.detectChanges();
  const saved = vi.fn<(request: EventSaveRequest) => void>();
  fixture.componentInstance.save.subscribe(saved);
  return { fixture, el: fixture.nativeElement as HTMLElement, saved };
}

function type(el: HTMLElement, selector: string, value: string): void {
  const input = el.querySelector<HTMLInputElement>(selector);
  if (!input) throw new Error(`missing ${selector}`);
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

describe('EventDialog', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('requires a title', () => {
    const { fixture, el, saved } = setup();
    el.querySelector<HTMLButtonElement>('[data-testid="event-save"] button')?.click();
    fixture.detectChanges();
    expect(saved).not.toHaveBeenCalled();
    expect(el.textContent).toContain('Escribe un título.');
  });

  it('saves a new timed event in the chosen time zone', () => {
    const { fixture, el, saved } = setup();
    type(el, '[data-testid="event-title"]', '  Planificación  ');
    fixture.detectChanges();
    el.querySelector<HTMLButtonElement>('[data-testid="event-save"] button')?.click();
    expect(saved).toHaveBeenCalledTimes(1);
    const { input, scope } = saved.mock.calls[0][0];
    expect(scope).toBe('series');
    expect(input.title).toBe('Planificación');
    expect(input.startsAt).toBe('2026-11-25T09:00:00Z');
    expect(input.endsAt).toBe('2026-11-25T10:00:00Z');
    expect(input.recurrenceRule).toBeNull();
    expect(input.allDay).toBe(false);
  });

  it('sends a weekly rule from the repetition editor', () => {
    const { fixture, el, saved } = setup();
    type(el, '[data-testid="event-title"]', 'Daily');
    const select = el.querySelector<HTMLElement>('app-recurrence-editor');
    expect(select).not.toBeNull();
    const instance = fixture.componentInstance as unknown as {
      recurrence: { set(spec: unknown): void };
    };
    instance.recurrence.set({
      freq: 'WEEKLY',
      interval: 1,
      weekdays: ['MO', 'WE'],
      end: { kind: 'count', count: 10 },
    });
    fixture.detectChanges();
    el.querySelector<HTMLButtonElement>('[data-testid="event-save"] button')?.click();
    expect(saved.mock.calls[0][0].input.recurrenceRule).toBe('FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10');
  });

  it('uses midnight UTC and an exclusive end for all-day events', () => {
    const { fixture, el, saved } = setup({
      start: { date: '2026-11-25', time: '09:00', allDay: true },
    });
    type(el, '[data-testid="event-title"]', 'Feriado');
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="event-start-time"]')).toBeNull();
    el.querySelector<HTMLButtonElement>('[data-testid="event-save"] button')?.click();
    const { input } = saved.mock.calls[0][0];
    expect(input.startsAt).toBe('2026-11-25T00:00:00Z');
    expect(input.endsAt).toBe('2026-11-26T00:00:00Z');
  });

  it('offers occurrence or series when editing a recurring event', () => {
    const { fixture, el, saved } = setup({
      event: detail({ recurrenceRule: 'FREQ=DAILY;COUNT=3' }),
      occurrence: '2026-11-26T10:00:00Z',
      occurrenceTimes: { startsAt: '2026-11-26T10:00:00Z', endsAt: '2026-11-26T11:00:00Z' },
      start: null,
    });
    expect(el.querySelector('[data-testid="event-scope"]')).not.toBeNull();
    // Only the occurrence is edited: no repetition editor.
    expect(el.querySelector('app-recurrence-editor')).toBeNull();
    el.querySelector<HTMLButtonElement>('[data-testid="event-save"] button')?.click();
    fixture.detectChanges();
    const request = saved.mock.calls[0][0];
    expect(request.scope).toBe('occurrence');
    expect(request.input.startsAt).toBe('2026-11-26T10:00:00Z');
    expect(request.input.recurrenceRule).toBeUndefined();
  });
});
