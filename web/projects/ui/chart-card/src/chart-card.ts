import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  DOCUMENT,
  ElementRef,
  PLATFORM_ID,
  computed,
  effect,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { Chart, ChartConfiguration, registerables } from 'chart.js';

import { resolveTokenColor, seriesColor } from './palette';

Chart.register(...registerables);

export type ChartTable = {
  headers: string[];
  rows: Array<Array<string | number>>;
};

export type ChartCardType = 'bar' | 'line' | 'doughnut';

export type ChartCardSeries = {
  readonly label: string;
  readonly data: readonly number[];
};

type ThemeMode = 'light' | 'dark';

let nextCardId = 0;

function readThemeMode(document: Document): ThemeMode {
  return document.documentElement.dataset['theme'] === 'dark' ? 'dark' : 'light';
}

/**
 * Chart card with a title, colors from design tokens and an accessible data table.
 * The table mirrors the chart data: it is visually hidden until "Ver datos" is pressed,
 * and screen readers always reach it. The chart is redrawn when the theme changes.
 */
@Component({
  selector: 'app-chart-card',
  templateUrl: './chart-card.html',
  styleUrl: './chart-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChartCard {
  private readonly document = inject(DOCUMENT);
  private readonly platformId = inject(PLATFORM_ID);
  private readonly idBase = `cl-chart-card-${nextCardId++}`;

  protected readonly titleId = `${this.idBase}-title`;
  protected readonly tableId = `${this.idBase}-table`;

  readonly title = input.required<string>();
  readonly description = input<string>();
  readonly table = input.required<ChartTable>();
  readonly chartType = input<ChartCardType>('bar');
  readonly labels = input<readonly string[]>([]);
  readonly series = input<readonly ChartCardSeries[]>([]);
  readonly isEmpty = input(false);
  readonly emptyText = input('Sin datos para mostrar');
  readonly testId = input<string>(undefined, { alias: 'data-testid' });

  protected readonly showData = signal(false);

  private readonly canvas = viewChild<ElementRef<HTMLCanvasElement>>('canvas');
  private readonly themeMode = signal<ThemeMode>(readThemeMode(this.document));

  private readonly chartConfig = computed<ChartConfiguration>(() =>
    this.buildConfig(this.chartType(), this.labels(), this.series(), this.themeMode()),
  );

  constructor() {
    if (typeof MutationObserver === 'function') {
      const observer = new MutationObserver(() => this.themeMode.set(readThemeMode(this.document)));
      observer.observe(this.document.documentElement, {
        attributes: true,
        attributeFilter: ['data-theme'],
      });
      inject(DestroyRef).onDestroy(() => observer.disconnect());
    }

    effect((onCleanup) => {
      const element = this.canvas()?.nativeElement;
      const config = this.chartConfig();
      if (!element || this.isEmpty() || !isPlatformBrowser(this.platformId)) return;
      const chart = new Chart(element, config);
      onCleanup(() => chart.destroy());
    });
  }

  protected toggleData(): void {
    this.showData.update((visible) => !visible);
  }

  private buildConfig(
    type: ChartCardType,
    labels: readonly string[],
    series: readonly ChartCardSeries[],
    // The mode is part of the input so that a theme change rebuilds the colors.
    _mode: ThemeMode,
  ): ChartConfiguration {
    const root = this.document.documentElement;
    const color = (token: string) => resolveTokenColor(token, root);
    const axis = {
      ticks: { color: color('var(--cl-text-subtle)') },
      grid: { color: color('var(--cl-border)') },
    };

    return {
      type,
      data: {
        labels: [...labels],
        datasets: series.map((entry, index) => {
          if (type === 'doughnut') {
            return {
              label: entry.label,
              data: [...entry.data],
              backgroundColor: labels.map((_, pointIndex) => color(seriesColor(pointIndex))),
            };
          }
          const seriesToken = color(seriesColor(index));
          return {
            label: entry.label,
            data: [...entry.data],
            backgroundColor: seriesToken,
            borderColor: seriesToken,
          };
        }),
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        plugins: {
          legend: {
            display: type === 'doughnut' || series.length > 1,
            labels: { color: color('var(--cl-text)') },
          },
        },
        scales: type === 'doughnut' ? undefined : { x: axis, y: axis },
      },
    };
  }
}
