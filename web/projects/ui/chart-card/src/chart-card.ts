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
import { Chart, ChartConfiguration, Plugin, registerables } from 'chart.js';
import { EmptyState } from '@corelog/ui/empty-state';

import { resolveTokenColor, seriesColor } from './palette';

Chart.register(...registerables);

export type ChartTable = {
  headers: string[];
  rows: Array<Array<string | number>>;
};

/** Chart.js types, plus `progress`: horizontal bars drawn with tokens (no canvas). */
export type ChartCardType = 'bar' | 'line' | 'doughnut' | 'progress';

export type ChartJsType = Exclude<ChartCardType, 'progress'>;

export interface ProgressRow {
  readonly label: string;
  readonly value: number;
  /** Share of the largest value, 0 to 100. */
  readonly percent: number;
  /** Token reference (for example `var(--cl-cat-done)`) the bar is drawn with. */
  readonly color: string;
}

/** Vertical reference line drawn at one label of a line or bar chart (for example today). */
export interface ChartMarker {
  /** Index into the labels. */
  readonly index: number;
  readonly label: string;
}

export type ChartCardSeries = {
  readonly label: string;
  readonly data: readonly number[];
  /** Draws the series with a dashed stroke (for example an ideal or target line). */
  readonly dashed?: boolean;
  /** Token references (`var(--cl-...)`), one per point, that replace the palette for this series. */
  readonly colors?: readonly string[];
  /** Draws this series as a line or as bars inside a bar chart (a combined chart). */
  readonly type?: 'line' | 'bar';
  /** Token reference of the whole series (for example a chart token); wins over the palette order. */
  readonly color?: string;
  /** Points only, without the connecting line (scatter-like series). */
  readonly pointsOnly?: boolean;
  /** Fills the area under a line series. */
  readonly fill?: boolean;
  /** Draws a line series as steps. */
  readonly stepped?: boolean;
  /** Radius of the points of a line series. */
  readonly pointRadius?: number;
};

type ThemeMode = 'light' | 'dark';

interface ChartLayout {
  readonly stacked: boolean;
  readonly horizontal: boolean;
  readonly zeroBased: boolean;
  readonly xTitle?: string;
  readonly yTitle?: string;
}

let nextCardId = 0;
const DASHED_PATTERN = [6, 4];
const FILL_ALPHA = 0.22;

/** Same color with an opacity, for the area under a line (the input is the computed rgb or hex color). */
function withAlpha(value: string, alpha: number): string {
  const hex = /^#([0-9a-f]{6})$/i.exec(value.trim());
  if (hex) {
    const n = parseInt(hex[1], 16);
    return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
  }
  const rgb = /^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/.exec(value.trim());
  return rgb ? `rgba(${rgb[1]}, ${rgb[2]}, ${rgb[3]}, ${alpha})` : value;
}

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
  imports: [EmptyState],
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
  /** Helper line under the empty text. */
  readonly emptyDescription = input<string>();
  readonly marker = input<ChartMarker>();
  /** Stacks the series (stacked areas or bars). */
  readonly stacked = input(false);
  /** Draws bars horizontally. */
  readonly horizontal = input(false);
  /** Text alternative of the picture: the chart is exposed as an image with this label. */
  readonly ariaLabel = input<string>();
  /** Shows the "Ver datos" button and its table. Off when the page already shows the full table. */
  readonly showDataToggle = input(true);
  /** Taller plot for pages whose main content is the chart. */
  readonly size = input<'md' | 'lg'>('md');
  /** Starts the value axis at zero (lines default to the data range). */
  readonly zeroBased = input(false);
  readonly xTitle = input<string>();
  readonly yTitle = input<string>();
  readonly testId = input<string>(undefined, { alias: 'data-testid' });

  protected readonly showData = signal(false);

  private readonly canvas = viewChild<ElementRef<HTMLCanvasElement>>('canvas');
  private readonly themeMode = signal<ThemeMode>(readThemeMode(this.document));

  /** Progress bars have no canvas, so they have no chart configuration. */
  private readonly chartConfig = computed<ChartConfiguration | null>(() => {
    const type = this.chartType();
    if (type === 'progress') return null;
    return this.buildConfig(type, this.labels(), this.series(), this.themeMode(), this.marker(), {
      stacked: this.stacked(),
      horizontal: this.horizontal(),
      zeroBased: this.zeroBased(),
      xTitle: this.xTitle(),
      yTitle: this.yTitle(),
    });
  });

  protected readonly isProgress = computed(() => this.chartType() === 'progress');
  /** Rows of the progress mode: the first series against the labels, scaled to the largest value. */
  protected readonly progressRows = computed<ProgressRow[]>(() => {
    const values = this.series()[0]?.data ?? [];
    const max = Math.max(0, ...values);
    return this.labels().map((label, index) => {
      const value = values[index] ?? 0;
      const color = this.series()[0]?.colors?.[index] ?? seriesColor(0);
      return { label, value, percent: max > 0 ? (value / max) * 100 : 0, color };
    });
  });

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
      if (!element || !config || this.isEmpty() || !isPlatformBrowser(this.platformId)) return;
      const chart = new Chart(element, config);
      onCleanup(() => chart.destroy());
    });
  }

  protected toggleData(): void {
    this.showData.update((visible) => !visible);
  }

  private buildConfig(
    type: ChartJsType,
    labels: readonly string[],
    series: readonly ChartCardSeries[],
    // The mode is part of the input so that a theme change rebuilds the colors.
    _mode: ThemeMode,
    marker: ChartMarker | undefined,
    layout: ChartLayout,
  ): ChartConfiguration {
    const root = this.document.documentElement;
    const color = (token: string) => resolveTokenColor(token, root);
    const axisBase = {
      ticks: { color: color('var(--cl-text-subtle)') },
      grid: { color: color('var(--cl-divider)') },
      border: { color: color('var(--cl-border)') },
    };
    const axisTitle = (text: string | undefined) => ({
      display: !!text,
      text: text ?? '',
      color: color('var(--cl-text-subtle)'),
    });
    const xAxis = {
      ...axisBase,
      stacked: layout.stacked,
      beginAtZero: layout.zeroBased,
      title: axisTitle(layout.xTitle),
    };
    const yAxis = {
      ...axisBase,
      stacked: layout.stacked,
      beginAtZero: layout.zeroBased,
      title: axisTitle(layout.yTitle),
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
          const seriesToken = color(entry.color ?? seriesColor(index));
          const pointColors = entry.colors?.map((token) => color(token));
          const isLine = (entry.type ?? type) === 'line';
          return {
            ...(entry.type ? { type: entry.type } : {}),
            label: entry.label,
            data: [...entry.data],
            backgroundColor:
              pointColors ?? (entry.fill ? withAlpha(seriesToken, FILL_ALPHA) : seriesToken),
            borderColor: pointColors ?? seriesToken,
            borderWidth: isLine ? 2 : 0,
            pointRadius: isLine ? (entry.pointRadius ?? 3) : undefined,
            pointBackgroundColor: seriesToken,
            showLine: !entry.pointsOnly,
            fill: entry.fill ? 'origin' : false,
            stepped: entry.stepped ? 'before' : false,
            tension: 0,
            borderRadius: !isLine ? 4 : undefined,
            borderDash: entry.dashed ? DASHED_PATTERN : [],
            order: isLine && entry.type ? 0 : 1,
          };
        }),
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        indexAxis: layout.horizontal ? ('y' as const) : ('x' as const),
        interaction: { mode: 'index' as const, intersect: false },
        layout: { padding: { top: marker ? 16 : 0 } },
        plugins: {
          legend: {
            display: type === 'doughnut' || series.length > 1,
            position: 'bottom',
            labels: {
              color: color('var(--cl-text)'),
              boxWidth: 24,
              boxHeight: 8,
              padding: 16,
              // Lines show a line sample (dashes included); bars show a small rounded box.
              usePointStyle: type !== 'doughnut',
              generateLabels: (chart) =>
                Chart.defaults.plugins.legend.labels.generateLabels(chart).map((item) => {
                  const entry = series[item.datasetIndex ?? 0];
                  const line = (entry?.type ?? type) === 'line';
                  return {
                    ...item,
                    pointStyle: line ? ('line' as const) : ('rectRounded' as const),
                  };
                }),
            },
          },
          tooltip: {
            backgroundColor: color('var(--cl-bg-elevated)'),
            titleColor: color('var(--cl-text)'),
            bodyColor: color('var(--cl-text)'),
            borderColor: color('var(--cl-border)'),
            borderWidth: 1,
            padding: 10,
            boxPadding: 4,
          },
        },
        scales: type === 'doughnut' ? undefined : { x: xAxis, y: yAxis },
      },
      plugins: marker ? [this.markerPlugin(marker, color('var(--cl-text-subtle)'))] : [],
    };
  }

  /** Draws a dashed vertical line with a small caption at the marked label. */
  private markerPlugin(marker: ChartMarker, lineColor: string): Plugin {
    return {
      id: 'clMarker',
      afterDatasetsDraw: (chart) => {
        const x = chart.scales['x']?.getPixelForValue(marker.index);
        const area = chart.chartArea;
        if (x === undefined || Number.isNaN(x) || !area) return;
        const ctx = chart.ctx;
        ctx.save();
        ctx.strokeStyle = lineColor;
        ctx.fillStyle = lineColor;
        ctx.lineWidth = 1;
        ctx.setLineDash([2, 3]);
        ctx.beginPath();
        ctx.moveTo(x, area.top);
        ctx.lineTo(x, area.bottom);
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.textAlign = 'center';
        ctx.fillText(marker.label, x, area.top - 4);
        ctx.restore();
      },
    };
  }
}
