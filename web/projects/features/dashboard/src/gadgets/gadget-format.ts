import type { ActivityGadgetItem } from '@corelog/core';

const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];

function pad(value: number): string {
  return String(value).padStart(2, '0');
}

/** "3 dic, 14:05" for an ISO timestamp; the text itself when it is not a date. */
export function formatDateTime(value: string, allDay = false): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const day = `${date.getDate()} ${MONTHS[date.getMonth()]}`;
  return allDay ? day : `${day}, ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** "3 dic" for an ISO day or timestamp. */
export function formatShortDay(value: string | null): string {
  if (!value) return '';
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) return value;
  return `${Number(match[3])} ${MONTHS[Number(match[2]) - 1]}`;
}

/** "14:05" of an ISO timestamp. */
export function formatClock(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** Whether an ISO day lies before today (local calendar). */
export function isPastDay(value: string | null, now: Date = new Date()): boolean {
  const match = value ? /^(\d{4})-(\d{2})-(\d{2})/.exec(value) : null;
  if (!match) return false;
  const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
  return match[0] < today;
}

/** Tone of a status category as the API names it. */
export function categoryTone(category: string): 'neutral' | 'blue' | 'green' {
  if (category === 'done') return 'green';
  return category === 'in_progress' || category === 'progress' ? 'blue' : 'neutral';
}

/** What the person did in an activity entry, as a short Spanish phrase. */
export function activityVerb(item: ActivityGadgetItem): string {
  switch (item.action) {
    case 'created':
      return 'creó';
    case 'status_changed':
      return item.newValue ? `movió a "${item.newValue}"` : 'cambió el estado de';
    case 'assignee_changed':
      return item.newValue ? `asignó a ${item.newValue}` : 'cambió el responsable de';
    case 'comment':
    case 'commented':
      return 'comentó en';
    case 'field_changed':
      return item.field ? `cambió ${item.field} de` : 'actualizó';
    default:
      return 'actualizó';
  }
}

/** Names of the favorite types in Spanish. */
export const FAVORITE_TYPE_LABELS: Readonly<Record<string, string>> = {
  project: 'Proyecto',
  filter: 'Filtro',
  board: 'Tablero',
  dashboard: 'Panel',
  issue: 'Incidencia',
};

export const COLUMN_LABELS: Readonly<Record<string, string>> = {
  key: 'Clave',
  summary: 'Resumen',
  status: 'Estado',
  priority: 'Prioridad',
  type: 'Tipo',
  assignee: 'Responsable',
  due: 'Vence',
  updated: 'Actualizada',
};

/** Label of a statistics group that has no value ("Sin asignar" and similar). */
export function groupLabel(label: string): string {
  return label.trim() === '' ? 'Sin valor' : label;
}
