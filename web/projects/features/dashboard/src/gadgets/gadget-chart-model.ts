import type { GadgetData } from '@corelog/core';
import type { ChartCardSeries, ChartCardType, ChartTable } from '@corelog/ui/chart-card';

import { formatShortDay, groupLabel } from './gadget-format';

/** Everything a chart card needs to draw a chart gadget. */
export interface ChartModel {
  readonly chartType: ChartCardType;
  readonly labels: readonly string[];
  readonly series: readonly ChartCardSeries[];
  readonly table: ChartTable;
  readonly isEmpty: boolean;
  readonly emptyText: string;
  readonly emptyDescription: string;
}

const STATUS_COLORS: Readonly<Record<string, string>> = {
  todo: 'var(--cl-cat-todo)',
  in_progress: 'var(--cl-cat-progress)',
  done: 'var(--cl-cat-done)',
};

/** Whether the type of a gadget is drawn as a chart (the others are lists, tables or text). */
export function isChartGadget(type: string): boolean {
  return (
    type === 'issue_statistics' ||
    type === 'pie' ||
    type === 'created_vs_resolved' ||
    type === 'sprint_burndown'
  );
}

/** Maps the data of a chart gadget to the inputs of the chart card; null for the other types. */
export function toChartModel(content: GadgetData): ChartModel | null {
  switch (content.type) {
    case 'issue_statistics':
    case 'pie': {
      const data = content.data;
      const groups = [...data.groups];
      if (data.other > 0) groups.push({ key: '__other', label: 'Otros', count: data.other });
      const labels = groups.map((group) => groupLabel(group.label));
      const colors =
        data.groupBy === 'status' ? groups.map((g) => STATUS_COLORS[g.key]) : undefined;
      const useColors = colors?.every((color) => !!color) ?? false;
      return {
        chartType: content.type === 'pie' ? 'doughnut' : 'progress',
        labels,
        series: [
          {
            label: 'Incidencias',
            data: groups.map((group) => group.count),
            colors: useColors ? (colors as string[]) : undefined,
          },
        ],
        table: {
          headers: ['Grupo', 'Incidencias'],
          rows: groups.map((group, index) => [labels[index], group.count]),
        },
        isEmpty: data.total === 0,
        emptyText: 'Sin incidencias para mostrar.',
        emptyDescription: 'Cuando haya incidencias que coincidan, verás aquí el reparto.',
      };
    }
    case 'created_vs_resolved': {
      const series = content.data.series;
      const labels = series.map((point) => formatShortDay(point.date));
      return {
        chartType: 'line',
        labels,
        series: [
          { label: 'Creadas', data: series.map((point) => point.created) },
          { label: 'Resueltas', data: series.map((point) => point.resolved) },
        ],
        table: {
          headers: ['Día', 'Creadas', 'Resueltas'],
          rows: series.map((point, index) => [labels[index], point.created, point.resolved]),
        },
        isEmpty: content.data.totalCreated === 0 && content.data.totalResolved === 0,
        emptyText: 'Sin actividad en este periodo.',
        emptyDescription: 'No se crearon ni resolvieron incidencias en los días elegidos.',
      };
    }
    case 'sprint_burndown': {
      const points = content.data.points;
      const labels = points.map((point) => formatShortDay(point.date));
      return {
        chartType: 'line',
        labels,
        series: [
          {
            label: 'Pendiente',
            data: points.map((point) => point.remaining ?? Number.NaN),
          },
          { label: 'Ideal', data: points.map((point) => point.ideal), dashed: true },
        ],
        table: {
          headers: ['Día', 'Pendiente', 'Ideal'],
          rows: points.map((point, index) => [
            labels[index],
            point.remaining ?? '-',
            Math.round(point.ideal * 10) / 10,
          ]),
        },
        isEmpty: content.data.sprint === null || points.length === 0,
        emptyText: 'No hay un sprint activo.',
        emptyDescription: 'Cuando un sprint de tus proyectos esté activo, verás su burndown aquí.',
      };
    }
    default:
      return null;
  }
}
