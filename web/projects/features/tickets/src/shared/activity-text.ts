import {
  ISSUE_PRIORITY_LABELS,
  ISSUE_STATUS_META,
  ISSUE_TYPE_LABELS,
  IssueActivity,
} from '@corelog/core';

/** Reads the display name of a user id. */
export type NameOf = (userId: string) => string;

const EMPTY_VALUE = 'Sin valor';

/** Field names written by the server, with the noun phrase used in the sentence. */
const FIELD_LABELS: Readonly<Record<string, string>> = {
  status: 'el estado',
  priority: 'la prioridad',
  title: 'el título',
  description: 'la descripción',
  assignee_id: 'el responsable',
  issue_type: 'el tipo',
  due_date: 'la fecha límite',
  story_points: 'los puntos de historia',
  parent_id: 'la incidencia padre',
  rank: 'el orden',
};

/** Actions that carry no field: the whole sentence is fixed. */
const FIXED_TEXTS: Readonly<Record<string, string>> = {
  created: 'creó la incidencia',
  create: 'creó la incidencia',
  updated: 'actualizó la incidencia',
  update: 'actualizó la incidencia',
  commented: 'añadió un comentario',
  comment: 'añadió un comentario',
  comment_added: 'añadió un comentario',
  comment_updated: 'editó un comentario',
  comment_deleted: 'eliminó un comentario',
  link_added: 'vinculó otra incidencia',
  link_removed: 'quitó un vínculo',
  attachment_added: 'adjuntó un archivo',
  attachment_removed: 'quitó un archivo adjunto',
  deleted: 'eliminó la incidencia',
  closed: 'cerró la incidencia',
  reopened: 'reabrió la incidencia',
};

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (part: number): string => String(part).padStart(2, '0');
  return `${pad(date.getUTCDate())}/${pad(date.getUTCMonth() + 1)}/${date.getUTCFullYear()}`;
}

/** Human text of a stored value of the given field. Values the UI knows (status, priority...) use their Spanish label. */
function formatValue(field: string, value: string | null, nameOf: NameOf): string {
  if (value === null || value === '') return EMPTY_VALUE;
  switch (field) {
    case 'status':
      return ISSUE_STATUS_META[value as keyof typeof ISSUE_STATUS_META]?.label ?? value;
    case 'priority':
      return ISSUE_PRIORITY_LABELS[value as keyof typeof ISSUE_PRIORITY_LABELS] ?? value;
    case 'issue_type':
      return ISSUE_TYPE_LABELS[value as keyof typeof ISSUE_TYPE_LABELS] ?? value;
    case 'assignee_id':
      return nameOf(value);
    case 'due_date':
      return formatDate(value);
    default:
      return value;
  }
}

/** Field an activity is about: the stored field name, or the one the action name points at ("status_changed"). */
function fieldOf(activity: IssueActivity): string | null {
  if (activity.fieldName) return activity.fieldName;
  const match = /^(.+?)_(changed|updated)$/.exec(activity.action);
  return match ? match[1] : null;
}

function assigneeText(activity: IssueActivity, nameOf: NameOf): string {
  const { oldValue, newValue } = activity;
  if (newValue && !oldValue) return `asignó la incidencia a ${nameOf(newValue)}`;
  if (!newValue && oldValue) return `quitó al responsable (${nameOf(oldValue)})`;
  if (newValue && oldValue) {
    return `cambió el responsable de ${nameOf(oldValue)} a ${nameOf(newValue)}`;
  }
  return 'cambió el responsable';
}

function changeText(activity: IssueActivity, field: string, nameOf: NameOf): string {
  if (field === 'assignee_id') return assigneeText(activity, nameOf);
  const label = FIELD_LABELS[field] ?? 'un dato de la incidencia';
  // Long free text is not quoted back: the change is enough.
  if (field === 'description') return 'actualizó la descripción';
  const { oldValue, newValue } = activity;
  const from = formatValue(field, oldValue, nameOf);
  const to = formatValue(field, newValue, nameOf);
  if (oldValue !== null && newValue !== null) return `cambió ${label} de ${from} a ${to}`;
  if (newValue !== null) return `definió ${label}: ${to}`;
  if (oldValue !== null) return `quitó ${label} (${from})`;
  return `actualizó ${label}`;
}

/**
 * Spanish sentence of a history entry without the actor ("cambió el estado de Abierto a En progreso").
 * The server stores English action names; none of them reaches the screen.
 */
export function describeActivity(activity: IssueActivity, nameOf: NameOf): string {
  const field = fieldOf(activity);
  if (field !== null && (activity.fieldName || !(activity.action in FIXED_TEXTS))) {
    return changeText(activity, field, nameOf);
  }
  return FIXED_TEXTS[activity.action] ?? 'hizo un cambio en la incidencia';
}
