import type { RoadmapBar, RoadmapGroupBy } from '@corelog/core';
import type { TimelineBand, TimelineRow } from '@corelog/ui/timeline';

const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];
const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})/;

/** Short Spanish date ("3 dic 2026") of an ISO day; an em-free dash for a missing date. */
export function formatDay(iso: string | null): string {
  const match = iso ? ISO_DAY.exec(iso) : null;
  if (!match) return 'Sin fecha';
  return `${Number(match[3])} ${MONTHS[Number(match[2]) - 1]} ${match[1]}`;
}

/** "3 dic 2026 - 20 dic 2026", or a single date when both ends are the same. */
export function formatRange(start: string | null, end: string | null): string {
  if (!start && !end) return 'Sin fechas';
  if (!start || !end || start === end) return formatDay(start ?? end);
  return `${formatDay(start)} - ${formatDay(end)}`;
}

export const CATEGORY_LABELS: Readonly<Record<string, string>> = {
  todo: 'Por hacer',
  in_progress: 'En curso',
  done: 'Terminada',
};

const SPRINT_STATUS: Readonly<Record<string, string>> = {
  planned: 'Planificado',
  future: 'Planificado',
  active: 'Activo',
  closed: 'Cerrado',
  completed: 'Cerrado',
};

const VERSION_STATUS: Readonly<Record<string, string>> = {
  unreleased: 'Sin publicar',
  released: 'Publicada',
  archived: 'Archivada',
};

/** Status text of a bar for its kind. */
export function statusLabel(bar: RoadmapBar): string {
  if (bar.kind === 'sprint') return SPRINT_STATUS[bar.status] ?? bar.status;
  if (bar.kind === 'version') return VERSION_STATUS[bar.status] ?? bar.status;
  return CATEGORY_LABELS[bar.statusCategory] ?? bar.status;
}

/** Sentence read by assistive technology and shown in tooltips. */
export function describeBar(bar: RoadmapBar): string {
  const parts = [statusLabel(bar), formatRange(bar.start, bar.end)];
  if (bar.progress.total > 0) {
    parts.push(
      `${bar.progress.percent} % completado (${bar.progress.done} de ${bar.progress.total})`,
    );
  }
  return parts.join(', ');
}

export function toTimelineRow(bar: RoadmapBar): TimelineRow {
  return {
    id: bar.id,
    title: bar.title,
    key: bar.key || undefined,
    category: bar.statusCategory,
    percent: bar.progress.percent,
    start: bar.start,
    end: bar.end,
    description: describeBar(bar),
  };
}

export function toBand(bar: RoadmapBar): TimelineBand {
  return {
    id: bar.id,
    label: bar.title,
    kind: bar.kind === 'sprint' ? 'sprint' : 'version',
    start: bar.start,
    end: bar.end,
  };
}

export const GROUP_LABELS: Readonly<Record<RoadmapGroupBy, string>> = {
  epic: 'Épicas',
  version: 'Versiones',
  sprint: 'Sprints',
};
