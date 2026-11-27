import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { BurndownPoint } from '@corelog/core';
import { guardConsole } from '@corelog/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { BurndownChart } from './burndown-chart';

@Component({
  imports: [BurndownChart],
  template: '<app-burndown-chart [points]="points()" />',
})
class BurndownHost {
  readonly points = signal<readonly BurndownPoint[]>([]);
}

const points: BurndownPoint[] = [
  { date: '2026-11-02', ideal: 10, remaining: 10 },
  { date: '2026-11-03', ideal: 5, remaining: 7 },
];

function render(initial: readonly BurndownPoint[]) {
  const fixture = TestBed.createComponent(BurndownHost);
  fixture.componentInstance.points.set(initial);
  fixture.detectChanges();
  return fixture;
}

describe('BurndownChart', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => {
    // jsdom has no 2D canvas context: keep chart.js from logging the missing context.
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('shows the burndown card with its title and the data table', () => {
    const fixture = render(points);

    expect(fixture.nativeElement.querySelector('h3')?.textContent).toContain('Burndown');
    const headers = Array.from(fixture.nativeElement.querySelectorAll('th')).map((th) =>
      (th as HTMLElement).textContent?.trim(),
    );
    expect(headers).toEqual(expect.arrayContaining(['Fecha', 'Ideal', 'Restante']));
    const rows = fixture.nativeElement.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
    // jsdom has no canvas context, so chart.js reports it; the chart is covered by the chart card spec.
  });

  it('explains that the chart needs dates when there are no points', () => {
    const fixture = render([]);

    expect(fixture.nativeElement.textContent).toContain(
      'El burndown necesita fechas de inicio y fin.',
    );
    consoleGuard.expectNoConsoleOutput();
  });
});
