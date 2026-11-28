import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  CalendarEventDetail,
  CalendarV2ApiService,
  CalendarWindow,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  ProjectStore,
} from '@corelog/core';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { detail, item } from '../testing/calendar-fixtures';
import { CalendarView } from './calendar-view';

interface Fakes {
  window: ReturnType<typeof vi.fn>;
  get: ReturnType<typeof vi.fn>;
  remove: ReturnType<typeof vi.fn>;
  respond: ReturnType<typeof vi.fn>;
  feed: ReturnType<typeof vi.fn>;
  createFeed: ReturnType<typeof vi.fn>;
}

function setup(options: { event?: CalendarEventDetail; recurring?: boolean } = {}): {
  fixture: ComponentFixture<CalendarView>;
  el: HTMLElement;
  fakes: Fakes;
} {
  const event = options.event ?? detail({ myResponse: 'needs_action' });
  const win: CalendarWindow = {
    truncated: false,
    items: [
      item({
        title: 'Reunión',
        recurring: options.recurring ?? false,
        occurrenceStart: options.recurring ? '2026-11-25T10:00:00Z' : null,
      }),
      item({
        kind: 'issue_due',
        title: 'Entregar',
        sourceId: 'i1',
        link: '/browse/CORE-1',
        canEdit: false,
      }),
    ],
  };
  const fakes: Fakes = {
    window: vi.fn(() => of(win)),
    get: vi.fn(() => of(event)),
    remove: vi.fn(() => of(undefined)),
    respond: vi.fn(() => of(undefined)),
    feed: vi.fn(() => of({ enabled: false, createdAt: null })),
    createFeed: vi.fn(() =>
      of({
        enabled: true,
        createdAt: 'x',
        token: 't',
        url: 'http://api/calendar/export.ics?token=t',
      }),
    ),
  };
  TestBed.configureTestingModule({
    imports: [CalendarView],
    providers: [
      provideRouter([]),
      { provide: CalendarV2ApiService, useValue: fakes },
      { provide: ProjectStore, useValue: { fetchProjects: vi.fn(), projects: signal([]) } },
      { provide: OrganizationApiService, useValue: { listMembers: () => of([]) } },
      { provide: ORGANIZATION_ID_PROVIDER, useValue: () => 'org-1' },
    ],
  });
  const fixture = TestBed.createComponent(CalendarView);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, fakes };
}

async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
  for (let pass = 0; pass < 4; pass += 1) {
    fixture.detectChanges();
    await Promise.resolve();
  }
}

describe('CalendarView', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date(2026, 10, 25, 12));
  });

  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  it('loads the visible window and draws the month with its items', async () => {
    const { fixture, el, fakes } = setup();
    await settle(fixture);
    expect(fakes.window).toHaveBeenCalled();
    expect(el.querySelector('[data-testid="calendar-month"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="calendar-title"]')?.textContent).toContain(
      'Noviembre de 2026',
    );
    expect(el.textContent).toContain('Reunión');
    expect(el.textContent).toContain('Entregar');
  });

  it('hides a kind when its legend button is turned off', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    el.querySelector<HTMLButtonElement>('[data-testid="calendar-group-issues"]')?.click();
    await settle(fixture);
    expect(el.textContent).not.toContain('Entregar');
    expect(el.textContent).toContain('Reunión');
  });

  it('moves with the arrows and returns to today', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    el.querySelector<HTMLButtonElement>('[data-testid="calendar-next"]')?.click();
    await settle(fixture);
    expect(el.querySelector('[data-testid="calendar-title"]')?.textContent).toContain('Diciembre');
    el.querySelector<HTMLElement>('[data-testid="calendar-today"] button')?.click();
    await settle(fixture);
    expect(el.querySelector('[data-testid="calendar-title"]')?.textContent).toContain('Noviembre');
  });

  it('opens the details of an event and sends an answer', async () => {
    const { fixture, el, fakes } = setup();
    await settle(fixture);
    el.querySelector<HTMLButtonElement>(
      '[data-day="2026-11-25"] button[aria-label^="Evento"]',
    )?.click();
    await settle(fixture);
    expect(fakes.get).toHaveBeenCalledWith('ev-1');
    el.querySelector<HTMLButtonElement>('[data-testid="rsvp-accepted"]')?.click();
    await settle(fixture);
    expect(fakes.respond).toHaveBeenCalledWith('ev-1', 'accepted');
  });

  it('asks occurrence or series before deleting a recurring event', async () => {
    const { fixture, el, fakes } = setup({
      event: detail({ recurrenceRule: 'FREQ=DAILY;COUNT=5' }),
      recurring: true,
    });
    await settle(fixture);
    el.querySelector<HTMLButtonElement>(
      '[data-day="2026-11-25"] button[aria-label^="Evento"]',
    )?.click();
    await settle(fixture);
    el.querySelector<HTMLElement>('[data-testid="event-delete"] button')?.click();
    await settle(fixture);
    expect(el.querySelector('[data-testid="delete-choice"]')).not.toBeNull();
    el.querySelector<HTMLElement>('[data-testid="delete-occurrence"] button')?.click();
    await settle(fixture);
    expect(fakes.remove).toHaveBeenCalledWith('ev-1', '2026-11-25T10:00:00Z');
  });

  it('deletes a whole series without an occurrence', async () => {
    const { fixture, el, fakes } = setup({
      event: detail({ recurrenceRule: 'FREQ=DAILY;COUNT=5' }),
      recurring: true,
    });
    await settle(fixture);
    el.querySelector<HTMLButtonElement>(
      '[data-day="2026-11-25"] button[aria-label^="Evento"]',
    )?.click();
    await settle(fixture);
    el.querySelector<HTMLElement>('[data-testid="event-delete"] button')?.click();
    await settle(fixture);
    el.querySelector<HTMLElement>('[data-testid="delete-series"] button')?.click();
    await settle(fixture);
    expect(fakes.remove).toHaveBeenCalledWith('ev-1', undefined);
  });

  it('creates the subscription address and shows it once', async () => {
    const { fixture, el, fakes } = setup();
    await settle(fixture);
    el.querySelector<HTMLElement>('[data-testid="calendar-feed"] button')?.click();
    await settle(fixture);
    expect(el.querySelector('[data-testid="feed-disabled"]')).not.toBeNull();
    el.querySelector<HTMLElement>('[data-testid="feed-generate"] button')?.click();
    await settle(fixture);
    expect(fakes.createFeed).toHaveBeenCalled();
    expect(el.querySelector<HTMLInputElement>('[data-testid="feed-url"]')?.value).toContain(
      'export.ics?token=t',
    );
  });
});
