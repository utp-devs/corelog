import type { ReportKind } from '@corelog/core';

export type ReportGroupId = 'agile' | 'flow' | 'versions' | 'other';

export interface ReportMeta {
  readonly kind: ReportKind;
  /** Short name, used while the catalog of the server is not available. */
  readonly title: string;
  readonly description: string;
  /** PrimeIcons class of the card icon. */
  readonly icon: string;
}

export interface ReportGroup {
  readonly id: ReportGroupId;
  readonly title: string;
  readonly kinds: readonly ReportKind[];
}

export const REPORT_META: Readonly<Record<ReportKind, ReportMeta>> = {
  burndown: {
    kind: 'burndown',
    title: 'Burndown del sprint',
    description: 'Trabajo restante día a día frente a la línea ideal.',
    icon: 'pi pi-chart-line',
  },
  burnup: {
    kind: 'burnup',
    title: 'Burnup del sprint',
    description: 'Alcance total y trabajo completado día a día.',
    icon: 'pi pi-arrow-up-right',
  },
  velocity: {
    kind: 'velocity',
    title: 'Velocidad',
    description: 'Puntos comprometidos y completados en los últimos sprints.',
    icon: 'pi pi-gauge',
  },
  'cumulative-flow': {
    kind: 'cumulative-flow',
    title: 'Flujo acumulado',
    description: 'Incidencias por categoría de estado y día.',
    icon: 'pi pi-chart-bar',
  },
  'sprint-report': {
    kind: 'sprint-report',
    title: 'Resumen del sprint',
    description: 'Comprometido, completado, añadido y retirado de un sprint.',
    icon: 'pi pi-flag',
  },
  control: {
    kind: 'control',
    title: 'Gráfico de control',
    description: 'Tiempo de ciclo de las incidencias resueltas.',
    icon: 'pi pi-stopwatch',
  },
  'created-vs-resolved': {
    kind: 'created-vs-resolved',
    title: 'Creadas frente a resueltas',
    description: 'Incidencias creadas y resueltas por día, semana o mes.',
    icon: 'pi pi-arrows-h',
  },
  'resolution-time': {
    kind: 'resolution-time',
    title: 'Tiempo de resolución',
    description: 'Promedio, mediana y percentil 90 hasta resolver.',
    icon: 'pi pi-clock',
  },
  'time-tracking': {
    kind: 'time-tracking',
    title: 'Seguimiento de tiempo',
    description: 'Tiempo estimado frente a dedicado.',
    icon: 'pi pi-hourglass',
  },
  workload: {
    kind: 'workload',
    title: 'Carga de trabajo',
    description: 'Incidencias abiertas y estimación restante por persona.',
    icon: 'pi pi-users',
  },
  version: {
    kind: 'version',
    title: 'Versión',
    description: 'Progreso y detalle de las incidencias de una versión.',
    icon: 'pi pi-tag',
  },
};

/** Groups of the hub, in display order. */
export const REPORT_GROUPS: readonly ReportGroup[] = [
  {
    id: 'agile',
    title: 'Ágil',
    kinds: ['burndown', 'burnup', 'velocity', 'cumulative-flow', 'sprint-report'],
  },
  {
    id: 'flow',
    title: 'Tiempo y flujo',
    kinds: ['control', 'created-vs-resolved', 'resolution-time', 'time-tracking', 'workload'],
  },
  { id: 'versions', title: 'Versiones', kinds: ['version'] },
];

export function isReportKind(value: string): value is ReportKind {
  return value in REPORT_META;
}
