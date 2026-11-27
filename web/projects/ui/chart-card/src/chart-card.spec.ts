import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Chart } from 'chart.js';

import { ChartCard, type ChartCardSeries, type ChartTable } from './chart-card';

const table: ChartTable = {
  headers: ['Estado', 'Incidencias'],
  rows: [
    ['Abiertas', 12],
    ['Cerradas', 5],
  ],
};

@Component({
  imports: [ChartCard],
  template: `
    <app-chart-card
      [title]="title()"
      [table]="table()"
      [isEmpty]="isEmpty()"
      [labels]="labels()"
      [series]="series()"
      [chartType]="chartType()"
    />
  `,
})
class Host {
  readonly title = signal('Incidencias por estado');
  readonly table = signal<ChartTable>(table);
  readonly isEmpty = signal(false);
  readonly labels = signal<string[]>(['Abiertas', 'Cerradas']);
  readonly series = signal<ChartCardSeries[]>([{ label: 'Incidencias', data: [12, 5] }]);
  readonly chartType = signal<'bar' | 'progress'>('bar');
}

function create(): ComponentFixture<Host> {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return fixture;
}

function toggleButton(el: HTMLElement): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === 'Ver datos',
  );
}

describe('ChartCard', () => {
  beforeEach(() => {
    // jsdom has no 2D canvas context: keep chart.js from logging the missing context.
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('renders the title, the chart and a labelled section', () => {
    const el = create().nativeElement as HTMLElement;
    const region = el.querySelector('section');
    const labelId = region?.getAttribute('aria-labelledby');
    expect(region?.getAttribute('aria-labelledby')).toBeTruthy();
    expect(document.getElementById(labelId ?? '')?.textContent).toBe('Incidencias por estado');
    expect(el.querySelector('canvas')).not.toBeNull();
  });

  it('toggles the data table with the "Ver datos" button', () => {
    const fixture = create();
    const el = fixture.nativeElement as HTMLElement;
    const button = toggleButton(el);
    expect(button?.getAttribute('aria-expanded')).toBe('false');
    const tableEl = el.querySelector('table');
    expect(tableEl?.getAttribute('aria-label')).toBe('Datos: Incidencias por estado');
    expect(tableEl?.classList.contains('chart-card__table--hidden')).toBe(true);

    button?.click();
    fixture.detectChanges();
    expect(toggleButton(el)?.getAttribute('aria-expanded')).toBe('true');
    expect(tableEl?.classList.contains('chart-card__table--hidden')).toBe(false);
  });

  it('exposes the table rows with real headers', () => {
    const el = create().nativeElement as HTMLElement;
    const columnHeaders = Array.from(el.querySelectorAll('th[scope="col"]')).map(
      (th) => th.textContent,
    );
    expect(columnHeaders).toContain('Incidencias');
    const rowHeader = el.querySelector('th[scope="row"]');
    expect(rowHeader?.textContent).toBe('Abiertas');
    const cells = Array.from(el.querySelectorAll('td')).map((td) => td.textContent);
    expect(cells).toContain('12');
  });

  it('shows the empty text instead of the chart and the data toggle', () => {
    const fixture = create();
    fixture.componentInstance.isEmpty.set(true);
    fixture.componentInstance.title.set('Vacío');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.chart-card__empty')?.textContent).toContain('Sin datos para mostrar');
    expect(el.querySelector('canvas')).toBeNull();
    expect(toggleButton(el)).toBeUndefined();
  });

  it('draws progress bars scaled to the largest value without a canvas', () => {
    const fixture = create();
    fixture.componentInstance.chartType.set('progress');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;

    expect(el.querySelector('canvas')).toBeNull();
    const fills = Array.from(el.querySelectorAll<HTMLElement>('.chart-card__progress-fill'));
    expect(fills).toHaveLength(2);
    expect(fills[0].style.getPropertyValue('--cl-progress')).toBe('100%');
    expect(parseFloat(fills[1].style.getPropertyValue('--cl-progress'))).toBeCloseTo(41.6667, 3);
    expect(fills[0].getAttribute('style')).not.toMatch(/background-color|width:/);
    expect(el.querySelector('.chart-card__progress')?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelector('th[scope="row"]')?.textContent).toBe('Abiertas');
  });

  it('shows zero-width bars when every value is zero', () => {
    const fixture = create();
    fixture.componentInstance.chartType.set('progress');
    fixture.componentInstance.series.set([{ label: 'Incidencias', data: [0, 0] }]);
    fixture.detectChanges();
    const fills = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>(
        '.chart-card__progress-fill',
      ),
    );
    expect(fills.map((fill) => fill.style.getPropertyValue('--cl-progress'))).toEqual(['0%', '0%']);
  });

  it('colors each progress bar from its own token through a custom property', () => {
    const fixture = create();
    fixture.componentInstance.chartType.set('progress');
    fixture.componentInstance.series.set([
      { label: 'Incidencias', data: [12, 5], colors: ['var(--cl-cat-todo)', 'var(--cl-cat-done)'] },
    ]);
    fixture.detectChanges();
    const fills = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>(
        '.chart-card__progress-fill',
      ),
    );
    expect(fills.map((fill) => fill.style.getPropertyValue('--cl-bar-color'))).toEqual([
      'var(--cl-cat-todo)',
      'var(--cl-cat-done)',
    ]);
  });

  it('keeps the data toggle keyboard reachable with its name and controlled table', () => {
    const el = create().nativeElement as HTMLElement;
    const button = toggleButton(el);
    expect(button?.getAttribute('type')).toBe('button');
    expect(button?.getAttribute('aria-controls')).toBe(el.querySelector('table')?.id);
  });

  it('shows the empty description when there is nothing to draw', () => {
    const fixture = create();
    fixture.componentInstance.isEmpty.set(true);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('app-empty-state')).not.toBeNull();
  });

  it('redraws the chart when the theme changes', async () => {
    const destroy = vi.spyOn(Chart.prototype, 'destroy');
    create();
    const before = destroy.mock.calls.length;

    document.documentElement.dataset['theme'] = 'dark';
    await Promise.resolve();
    TestBed.flushEffects();

    expect(destroy.mock.calls.length).toBeGreaterThan(before);
    delete document.documentElement.dataset['theme'];
  });
});
