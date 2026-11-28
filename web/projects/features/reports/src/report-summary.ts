import { EMPTY_CELL, formatNumber } from './report-format';

export interface SummaryTile {
  readonly key: string;
  readonly label: string;
  readonly value: string;
  readonly hint?: string;
}

type Format = 'number' | 'percent' | 'days' | 'hours';

interface TileDef {
  readonly key: string;
  readonly label: string;
  readonly format?: Format;
}

const SPRINT_PROGRESS: readonly TileDef[] = [
  { key: 'committed', label: 'Comprometido' },
  { key: 'scope', label: 'Alcance actual' },
  { key: 'completed', label: 'Completado' },
  { key: 'remaining', label: 'Restante' },
  { key: 'scope_change', label: 'Cambio de alcance' },
];

const TILES: Readonly<Record<string, readonly TileDef[]>> = {
  burndown: SPRINT_PROGRESS,
  burnup: SPRINT_PROGRESS,
  velocity: [
    { key: 'sprints', label: 'Sprints' },
    { key: 'average_committed', label: 'Comprometido medio' },
    { key: 'average_completed', label: 'Completado medio' },
    { key: 'commitment_ratio', label: 'Cumplimiento', format: 'percent' },
  ],
  'cumulative-flow': [
    { key: 'todo', label: 'Por hacer' },
    { key: 'in_progress', label: 'En curso' },
    { key: 'done', label: 'Hecho' },
    { key: 'total', label: 'Total' },
    { key: 'average_in_progress', label: 'En curso (promedio)' },
  ],
  control: [
    { key: 'count', label: 'Incidencias resueltas' },
    { key: 'average_days', label: 'Promedio', format: 'days' },
    { key: 'median_days', label: 'Mediana', format: 'days' },
    { key: 'p85_days', label: 'Percentil 85', format: 'days' },
    { key: 'std_dev_days', label: 'Desviación estándar', format: 'days' },
  ],
  'created-vs-resolved': [
    { key: 'created', label: 'Creadas' },
    { key: 'resolved', label: 'Resueltas' },
    { key: 'net', label: 'Neto' },
  ],
  'resolution-time': [
    { key: 'count', label: 'Incidencias resueltas' },
    { key: 'average_days', label: 'Promedio', format: 'days' },
    { key: 'median_days', label: 'Mediana', format: 'days' },
    { key: 'p90_days', label: 'Percentil 90', format: 'days' },
  ],
  'time-tracking': [
    { key: 'estimate_hours', label: 'Estimado', format: 'hours' },
    { key: 'spent_hours', label: 'Dedicado', format: 'hours' },
    { key: 'accuracy_percent', label: 'Precisión', format: 'percent' },
    { key: 'entries', label: 'Registros' },
    { key: 'users', label: 'Personas' },
  ],
  workload: [
    { key: 'open', label: 'Incidencias abiertas' },
    { key: 'unassigned', label: 'Sin asignar' },
    { key: 'remaining_hours', label: 'Estimación restante', format: 'hours' },
    { key: 'assignees', label: 'Personas' },
  ],
  version: [
    { key: 'total', label: 'Incidencias' },
    { key: 'done', label: 'Hechas' },
    { key: 'in_progress', label: 'En curso' },
    { key: 'todo', label: 'Por hacer' },
    { key: 'percent_done', label: 'Avance', format: 'percent' },
    { key: 'days_to_release', label: 'Días para el lanzamiento', format: 'days' },
  ],
  'sprint-report': [
    { key: 'committed', label: 'Comprometido' },
    { key: 'completed', label: 'Completado' },
    { key: 'incomplete', label: 'Incompleto' },
    { key: 'added', label: 'Añadido' },
    { key: 'removed', label: 'Retirado' },
    { key: 'completion_percent', label: 'Cumplimiento', format: 'percent' },
  ],
};

function formatValue(value: unknown, format: Format | undefined): string {
  if (typeof value !== 'number') return EMPTY_CELL;
  const text = formatNumber(value);
  switch (format) {
    case 'percent':
      return `${text} %`;
    case 'days':
      return `${text} ${Math.abs(value) === 1 ? 'día' : 'días'}`;
    case 'hours':
      return `${text} h`;
    default:
      return text;
  }
}

/** Name of the sprint or version the report is about, when the summary names one. */
export function summarySubject(summary: Readonly<Record<string, unknown>>): string | null {
  for (const key of ['sprint', 'version']) {
    const value = summary[key];
    if (value && typeof value === 'object' && 'name' in value) return String(value.name);
  }
  return null;
}

/** The summary of a report as stat tiles: label, formatted value and, for sprint progress, the unit. */
export function summaryTiles(
  kind: string,
  summary: Readonly<Record<string, unknown>>,
): SummaryTile[] {
  const unit =
    summary['metric'] === 'issues'
      ? 'incidencias'
      : summary['metric'] === 'points'
        ? 'puntos'
        : undefined;
  return (TILES[kind] ?? [])
    .filter((def) => def.key in summary)
    .map((def) => ({
      key: def.key,
      label: def.label,
      value: formatValue(summary[def.key], def.format),
      hint: kind === 'burndown' || kind === 'burnup' ? unit : undefined,
    }));
}
