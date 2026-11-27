import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter, RouterOutlet } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { ApiError, CalendarEvent, projectRouteResolver } from '@corelog/core';
import { guardConsole } from '@corelog/core/testing';
import { NEVER, Observable, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { projectTestEnvironment } from '../testing/projects-test-kit';
import { ProjectCalendarPage } from './project-calendar-page';

@Component({ imports: [RouterOutlet], template: '<router-outlet />' })
class WorkspaceStub {}

const ROUTES = [
  {
    path: 'projects/:key',
    resolve: { project: projectRouteResolver },
    component: WorkspaceStub,
    children: [{ path: 'calendar', component: ProjectCalendarPage }],
  },
];

const EXPORT_EVENT: CalendarEvent = {
  kind: 'issue_due',
  date: '2026-11-05',
  title: 'Entregar exportacion',
  sourceId: 'issue-1',
  link: '/issues/issue-1',
};

const SPRINT_EVENT: CalendarEvent = {
  kind: 'sprint_start',
  date: '2026-11-02',
  title: 'Sprint 1',
  sourceId: 'sprint-1',
  link: '/projects/p-1/sprints',
};

function apiError(status: number, message: string): ApiError {
  return { status, code: `http.${status}`, message, details: [] };
}

describe('ProjectCalendarPage', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => {
    // Fixed month (November 2026) so the milestones of the spec fall in the view. Only Date is faked.
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date(2026, 10, 15));
  });

  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  async function open(options: Parameters<typeof projectTestEnvironment>[0]): Promise<{
    fixture: ComponentFixture<unknown>;
    events: ReturnType<typeof projectTestEnvironment>['calendarApi']['events'];
  }> {
    const env = projectTestEnvironment(options);
    TestBed.configureTestingModule({ providers: [provideRouter(ROUTES), ...env.providers] });
    const harness = await RouterTestingHarness.create('/projects/p-1/calendar');
    await settle(harness.fixture as ComponentFixture<unknown>);
    return {
      fixture: harness.fixture as ComponentFixture<unknown>,
      events: env.calendarApi['events']!,
    };
  }

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 3; pass += 1) {
      fixture.detectChanges();
      await Promise.resolve();
    }
  }

  function text(fixture: ComponentFixture<unknown>): string {
    return (fixture.nativeElement as HTMLElement).textContent ?? '';
  }

  function find(fixture: ComponentFixture<unknown>, selector: string): HTMLElement | null {
    return (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(selector);
  }

  function buttonNamed(
    fixture: ComponentFixture<unknown>,
    name: string,
  ): HTMLButtonElement | undefined {
    return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === name,
    );
  }

  it('shows the month grid with each event on its day', async () => {
    const { fixture } = await open({
      calendarApi: { events: () => of([EXPORT_EVENT, SPRINT_EVENT]) },
    });

    expect(text(fixture)).toContain('Entregar exportacion');
    const fifth = find(fixture, '[role="gridcell"][aria-label^="5 de"]');
    expect(fifth?.textContent).toContain('Entregar exportacion');
    const second = find(fixture, '[role="gridcell"][aria-label^="2 de"]');
    expect(second?.textContent).toContain('Inicio de sprint');
  });

  it('writes the month in lowercase Spanish and marks today', async () => {
    const { fixture } = await open({ calendarApi: { events: () => of([EXPORT_EVENT]) } });

    expect(find(fixture, 'h2')?.textContent?.trim()).toBe('noviembre de 2026');
    expect(find(fixture, '[role="gridcell"][aria-current="date"]')?.textContent).toContain('hoy');
  });

  it('shows the short issue key on due-date chips', async () => {
    const { fixture } = await open({ calendarApi: { events: () => of([EXPORT_EVENT]) } });

    expect(find(fixture, '.project-calendar__chip--issue_due')?.textContent).toContain(
      'INC-ISSUE-',
    );
  });

  it('keeps the grid visible and shows a compact notice for an empty month', async () => {
    const { fixture } = await open({ calendarApi: { events: () => of([]) } });

    expect(find(fixture, '.project-calendar__empty')).not.toBeNull();
    expect(find(fixture, '[role="grid"]')).not.toBeNull();
  });

  it('lists the events grouped by date in list view', async () => {
    const { fixture } = await open({
      calendarApi: { events: () => of([EXPORT_EVENT, SPRINT_EVENT]) },
    });
    expect(text(fixture)).toContain('Entregar exportacion');

    const listButton = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('*'),
    ).find((el) => el.children.length === 0 && el.textContent?.trim() === 'Lista');
    listButton?.click();
    fixture.detectChanges();

    expect(find(fixture, 'section[aria-label="2026-11-05"]')?.textContent).toContain(
      'Entregar exportacion',
    );
    expect(find(fixture, 'section[aria-label="2026-11-02"]')?.textContent).toContain('Sprint 1');
  });

  it('reloads the events when moving to another month', async () => {
    const { fixture, events } = await open({ calendarApi: { events: () => of([EXPORT_EVENT]) } });
    const before = events.mock.calls.length;

    find(fixture, 'button[aria-label="Mes siguiente"]')?.click();
    await settle(fixture);

    expect(events.mock.calls.length).toBeGreaterThan(before);
  });

  it('shows a useful error when the calendar cannot be loaded', async () => {
    const { fixture } = await open({
      calendarApi: { events: () => throwError(() => new Error('network')) },
    });

    expect(find(fixture, '[role="alert"]')?.textContent).toContain(
      'No se pudo cargar el calendario',
    );
  });

  it('shows an empty state inside the calendar when the month has no milestones', async () => {
    const { fixture } = await open({ calendarApi: { events: () => of([]) } });

    expect(text(fixture)).toContain('No hay hitos en este mes');
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the 403 page when the calendar is forbidden', async () => {
    const { fixture } = await open({
      calendarApi: { events: () => throwError(() => apiError(403, 'forbidden')) },
    });

    expect(text(fixture)).toContain('No tienes acceso a esta página');
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the calendar skeleton while the events load, without the empty state', async () => {
    const { fixture } = await open({
      calendarApi: { events: () => NEVER as Observable<CalendarEvent[]> },
    });

    expect(find(fixture, 'h1')?.textContent).toContain('Calendario');
    expect(text(fixture)).not.toContain('No hay hitos en este mes');
    expect(find(fixture, '[role="alert"]')).toBeNull();
    consoleGuard.expectNoConsoleOutput();
  });

  it('retries loading the events from the error state', async () => {
    const { fixture, events } = await open({
      calendarApi: { events: () => throwError(() => new Error('network')) },
    });
    expect(find(fixture, '[role="alert"]')?.textContent).toContain(
      'No se pudo cargar el calendario',
    );

    events.mockReturnValue(of([EXPORT_EVENT]));
    buttonNamed(fixture, 'Reintentar')?.click();
    await settle(fixture);

    expect(text(fixture)).toContain('Entregar exportacion');
    expect(events).toHaveBeenCalledTimes(2);
    consoleGuard.expectNoConsoleOutput();
  });
});
