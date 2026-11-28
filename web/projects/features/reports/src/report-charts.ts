import type { Report, ReportSeries } from '@corelog/core';
import type { ChartCardSeries } from '@corelog/ui/chart-card';

import { formatDate, formatNumber } from './report-format';

const ISO_DAY = /^\d{4}-\d{2}-\d{2}$/;

/** How a report is drawn: the inputs of the chart card, built from the series of the report. */
export interface ReportChartSpec {
  readonly chartType: 'bar' | 'line';
  readonly labels: string[];
  readonly series: ChartCardSeries[];
  readonly stacked: boolean;
  readonly horizontal: boolean;
  readonly xTitle?: string;
  readonly yTitle?: string;
  /** Text alternative of the picture. */
  readonly ariaLabel: string;
}

export interface ReportChartOptions {
  /** Created-vs-resolved: also draws the accumulated difference. */
  readonly cumulative?: boolean;
}

const C1 = 'var(--cl-chart-1)';
const C2 = 'var(--cl-chart-2)';
const C3 = 'var(--cl-chart-3)';
const C4 = 'var(--cl-chart-4)';
const C6 = 'var(--cl-chart-6)';
const NEUTRAL = 'var(--cl-text-subtle)';

/** Whether there is nothing to draw: no series, or series without points. */
export function isReportEmpty(report: Report): boolean {
  return (
    report.series.every((series) => series.points.length === 0) && report.table.rows.length === 0
  );
}

function values(series: ReportSeries): number[] {
  return series.points.map((point) => point.y);
}

function mapSeries(
  report: Report,
  style: (series: ReportSeries, index: number) => Partial<ChartCardSeries>,
): ChartCardSeries[] {
  return report.series.map((series, index) => ({
    label: series.name,
    data: values(series),
    ...style(series, index),
  }));
}

function labelsOf(report: Report): string[] {
  const longest = report.series.reduce<ReportSeries | undefined>(
    (best, series) => (!best || series.points.length > best.points.length ? series : best),
    undefined,
  );
  return (
    longest?.points.map((point) => (ISO_DAY.test(point.x) ? formatDate(point.x) : point.x)) ?? []
  );
}

/** One-sentence summary of every series (points, minimum, maximum and last value) for screen readers. */
export function describeSeries(report: Report): string {
  const parts = report.series
    .filter((series) => series.points.length > 0)
    .map((series) => {
      const ys = values(series);
      const last = ys[ys.length - 1];
      return `${series.name}: ${ys.length} ${ys.length === 1 ? 'valor' : 'valores'}, de ${formatNumber(Math.min(...ys))} a ${formatNumber(Math.max(...ys))}, último ${formatNumber(last)}`;
    });
  if (parts.length === 0) return `${report.title}. Sin datos.`;
  return `${report.title}. ${parts.join('. ')}. Los datos completos están en la tabla siguiente.`;
}

const SPRINT_POINT_COLORS: Readonly<Record<string, string>> = {
  Comprometido: C1,
  Completado: C3,
  Incompleto: C2,
  Añadido: C4,
  Retirado: C6,
};

/** Chart of a report, or null when the report has no series to draw. */
export function buildReportChart(
  report: Report,
  options: ReportChartOptions = {},
): ReportChartSpec | null {
  if (report.series.every((series) => series.points.length === 0)) return null;
  const base = {
    labels: labelsOf(report),
    stacked: false,
    horizontal: false,
    ariaLabel: describeSeries(report),
  };

  switch (report.kind) {
    case 'burndown':
      return {
        ...base,
        chartType: 'line',
        series: mapSeries(report, (series) =>
          series.name === 'Ideal'
            ? { color: NEUTRAL, dashed: true, pointRadius: 0 }
            : { color: C1, fill: true, pointRadius: 2 },
        ),
        yTitle: 'Trabajo restante',
      };
    case 'burnup':
      return {
        ...base,
        chartType: 'line',
        series: mapSeries(report, (series) =>
          series.name === 'Alcance'
            ? { color: C2, dashed: true, pointRadius: 0 }
            : { color: C3, fill: true, pointRadius: 2 },
        ),
        yTitle: 'Trabajo acumulado',
      };
    case 'velocity':
      return {
        ...base,
        chartType: 'bar',
        series: mapSeries(report, (series) => ({
          color: series.name === 'Comprometido' ? C1 : C3,
        })),
        yTitle: 'Puntos',
      };
    case 'cumulative-flow': {
      const color: Record<string, string> = { 'Por hacer': C6, 'En curso': C1, Hecho: C3 };
      return {
        ...base,
        chartType: 'line',
        stacked: true,
        // The first dataset sits at the bottom of the stack: finished work goes there.
        series: mapSeries(report, (series) => ({
          color: color[series.name] ?? C4,
          fill: true,
          pointRadius: 0,
        })).reverse(),
        yTitle: 'Incidencias',
      };
    }
    case 'control': {
      return {
        ...base,
        chartType: 'line',
        series: mapSeries(report, (series) => {
          if (series.name.startsWith('Tiempo de ciclo')) {
            return { color: C1, pointsOnly: true, pointRadius: 4 };
          }
          if (series.name === 'Media móvil') return { color: C2, pointRadius: 0 };
          return { color: C6, dashed: true, pointRadius: 0 };
        }),
        yTitle: 'Días',
      };
    }
    case 'created-vs-resolved': {
      const series = mapSeries(report, (entry) => {
        if (entry.name === 'Creadas') return { type: 'bar' as const, color: C2 };
        if (entry.name === 'Resueltas') return { type: 'bar' as const, color: C3 };
        return { type: 'line' as const, color: C1, pointRadius: 2 };
      }).filter((entry) => options.cumulative !== false || entry.type === 'bar');
      return { ...base, chartType: 'bar', series, yTitle: 'Incidencias' };
    }
    case 'resolution-time':
      return {
        ...base,
        chartType: 'bar',
        series: mapSeries(report, (_, index) => ({ color: index === 0 ? C1 : C3 })),
        yTitle: 'Días',
      };
    case 'time-tracking':
      return {
        ...base,
        chartType: 'bar',
        series: mapSeries(report, (series) => ({
          color: series.name.startsWith('Estimado') ? C1 : C2,
        })),
        yTitle: 'Horas',
      };
    case 'workload':
      return {
        ...base,
        chartType: 'bar',
        horizontal: true,
        series: mapSeries(report, (_, index) => ({ color: index === 0 ? C1 : C2 })),
        xTitle: 'Incidencias y horas',
      };
    case 'version':
      return {
        ...base,
        chartType: 'bar',
        series: mapSeries(report, (_, index) => ({ color: index === 0 ? C1 : C2 })),
        yTitle: 'Cantidad',
      };
    case 'sprint-report':
      return {
        ...base,
        chartType: 'bar',
        series: mapSeries(report, (series) => ({
          colors: series.points.map((point) => SPRINT_POINT_COLORS[point.x] ?? C1),
        })),
        yTitle: 'Puntos',
      };
    default:
      return {
        ...base,
        chartType: 'line',
        series: mapSeries(report, () => ({})),
      };
  }
}
