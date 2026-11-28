import type {
  ApiError,
  NotificationChannel,
  NotificationEvent,
  NotificationRecipient,
  NotificationRecipientType,
  NotificationRule,
  NotificationRuleInput,
} from '@corelog/core';

export interface EventGroup {
  readonly id: string;
  readonly title: string;
  readonly events: readonly EventInfo[];
}

export interface EventInfo {
  readonly event: NotificationEvent;
  readonly label: string;
  readonly hint: string;
}

/** The 13 events grouped for the matrix, in the order they are shown. */
export const EVENT_GROUPS: readonly EventGroup[] = [
  {
    id: 'issues',
    title: 'Incidencias',
    events: [
      {
        event: 'issue_created',
        label: 'Se crea una incidencia',
        hint: 'Alguien registra una incidencia nueva.',
      },
      {
        event: 'issue_assigned',
        label: 'Se asigna una incidencia',
        hint: 'Una incidencia cambia de responsable.',
      },
      {
        event: 'issue_updated',
        label: 'Se edita una incidencia',
        hint: 'Cambian campos como el título, la prioridad o la fecha.',
      },
      {
        event: 'issue_transitioned',
        label: 'Cambia de estado',
        hint: 'La incidencia pasa a otro estado del flujo.',
      },
      {
        event: 'issue_resolved',
        label: 'Se resuelve una incidencia',
        hint: 'La incidencia se marca como resuelta.',
      },
    ],
  },
  {
    id: 'comments',
    title: 'Comentarios y menciones',
    events: [
      {
        event: 'issue_commented',
        label: 'Nuevo comentario',
        hint: 'Alguien comenta en una incidencia.',
      },
      {
        event: 'issue_mentioned',
        label: 'Te mencionan',
        hint: 'Alguien escribe una mención con @.',
      },
      {
        event: 'comment_edited',
        label: 'Se edita un comentario',
        hint: 'Un comentario ya publicado cambia.',
      },
    ],
  },
  {
    id: 'time',
    title: 'Tiempo',
    events: [
      {
        event: 'worklog_logged',
        label: 'Se registra tiempo',
        hint: 'Alguien anota tiempo trabajado en una incidencia.',
      },
    ],
  },
  {
    id: 'releases',
    title: 'Versiones y sprints',
    events: [
      {
        event: 'version_released',
        label: 'Se publica una versión',
        hint: 'Una versión del proyecto se marca como publicada.',
      },
      { event: 'sprint_started', label: 'Empieza un sprint', hint: 'Se inicia un sprint.' },
      {
        event: 'sprint_completed',
        label: 'Termina un sprint',
        hint: 'Se cierra un sprint.',
      },
    ],
  },
  {
    id: 'filters',
    title: 'Filtros',
    events: [
      {
        event: 'filter_subscription',
        label: 'Suscripción de un filtro',
        hint: 'Llega el resumen programado de un filtro guardado.',
      },
    ],
  },
];

export const CHANNEL_LABELS: Readonly<Record<NotificationChannel, string>> = {
  in_app: 'En la aplicación',
  email: 'Correo',
};

/** Recipients that need no extra data, in the order the add menu lists them. */
export const SIMPLE_RECIPIENTS: readonly { type: NotificationRecipientType; label: string }[] = [
  { type: 'reporter', label: 'Quien reportó' },
  { type: 'assignee', label: 'Responsable' },
  { type: 'watchers', label: 'Seguidores' },
  { type: 'project_lead', label: 'Líder del proyecto' },
  { type: 'component_lead', label: 'Líder del componente' },
  { type: 'mentioned', label: 'Personas mencionadas' },
  { type: 'reply_target', label: 'Autor del comentario respondido' },
];

export const ROLE_LABELS: Readonly<Record<string, string>> = {
  lead: 'Líderes del proyecto',
  contributor: 'Colaboradores',
  viewer: 'Observadores',
};

/** One row of the matrix while it is being edited. */
export interface DraftRule {
  readonly event: NotificationEvent;
  readonly recipients: readonly NotificationRecipient[];
  readonly channels: readonly NotificationChannel[];
  readonly overridden: boolean;
}

export function toDraft(rules: readonly NotificationRule[]): readonly DraftRule[] {
  return rules.map((rule) => ({ ...rule }));
}

function update(
  draft: readonly DraftRule[],
  event: NotificationEvent,
  change: (rule: DraftRule) => DraftRule,
): readonly DraftRule[] {
  return draft.map((rule) => (rule.event === event ? change(rule) : rule));
}

/** Makes a row own by this scope, keeping the values it inherited as the starting point. */
export function customize(
  draft: readonly DraftRule[],
  event: NotificationEvent,
): readonly DraftRule[] {
  return update(draft, event, (rule) => ({ ...rule, overridden: true }));
}

/** Turns a channel on or off. Editing a row customizes it. */
export function setChannel(
  draft: readonly DraftRule[],
  event: NotificationEvent,
  channel: NotificationChannel,
  enabled: boolean,
): readonly DraftRule[] {
  return update(draft, event, (rule) => {
    const without = rule.channels.filter((c) => c !== channel);
    return { ...rule, overridden: true, channels: enabled ? [...without, channel] : without };
  });
}

export function recipientKey(recipient: NotificationRecipient): string {
  return `${recipient.type}|${recipient.id ?? ''}|${recipient.role ?? ''}`;
}

/** Adds a recipient unless the row has it already. Editing a row customizes it. */
export function addRecipient(
  draft: readonly DraftRule[],
  event: NotificationEvent,
  recipient: NotificationRecipient,
): readonly DraftRule[] {
  return update(draft, event, (rule) => {
    const exists = rule.recipients.some((r) => recipientKey(r) === recipientKey(recipient));
    return {
      ...rule,
      overridden: true,
      recipients: exists ? rule.recipients : [...rule.recipients, recipient],
    };
  });
}

export function removeRecipient(
  draft: readonly DraftRule[],
  event: NotificationEvent,
  recipient: NotificationRecipient,
): readonly DraftRule[] {
  return update(draft, event, (rule) => ({
    ...rule,
    overridden: true,
    recipients: rule.recipients.filter((r) => recipientKey(r) !== recipientKey(recipient)),
  }));
}

function sameRule(a: DraftRule, b: NotificationRule): boolean {
  if (a.overridden !== b.overridden) return false;
  if (!a.overridden) return true;
  const channels = (rule: { channels: readonly NotificationChannel[] }): string =>
    [...rule.channels].sort().join(',');
  const recipients = (rule: { recipients: readonly NotificationRecipient[] }): string =>
    rule.recipients.map(recipientKey).sort().join(',');
  return channels(a) === channels(b) && recipients(a) === recipients(b);
}

/** True when the draft differs from what the server holds. */
export function isDirty(draft: readonly DraftRule[], saved: readonly NotificationRule[]): boolean {
  const byEvent = new Map(saved.map((rule) => [rule.event, rule] as const));
  return draft.some((rule) => {
    const original = byEvent.get(rule.event);
    return !original || !sameRule(rule, original);
  });
}

/** Rules to send: only the ones this scope owns, so the rest keep inheriting. */
export function toInputs(draft: readonly DraftRule[]): readonly NotificationRuleInput[] {
  return draft
    .filter((rule) => rule.overridden)
    .map(({ event, recipients, channels }) => ({ event, recipients, channels }));
}

/** How many rows the scope owns. */
export function overrideCount(draft: readonly DraftRule[]): number {
  return draft.filter((rule) => rule.overridden).length;
}

/**
 * Maps the details of a validation error to the rows that were sent. Details look like `rules[2].recipients`
 * (the index counts the rules of the request); anything else stays a general message.
 */
export function rowErrors(
  failure: ApiError,
  sent: readonly NotificationRuleInput[],
): { readonly byEvent: ReadonlyMap<NotificationEvent, string>; readonly general: string | null } {
  const byEvent = new Map<NotificationEvent, string>();
  for (const detail of failure.details) {
    const match = /rules\D*(\d+)/.exec(detail.field);
    const rule = match ? sent[Number(match[1])] : undefined;
    if (rule && !byEvent.has(rule.event)) byEvent.set(rule.event, detail.message);
  }
  return { byEvent, general: byEvent.size > 0 ? null : failure.message };
}

/** Label of a recipient chip; people and teams are looked up by id. */
export function recipientLabel(
  recipient: NotificationRecipient,
  names: { person(id: string): string; team(id: string): string },
): string {
  switch (recipient.type) {
    case 'user':
      return names.person(recipient.id ?? '');
    case 'team':
      return `Equipo ${names.team(recipient.id ?? '')}`;
    case 'role':
      return ROLE_LABELS[recipient.role ?? ''] ?? `Rol ${recipient.role ?? ''}`;
    default:
      return SIMPLE_RECIPIENTS.find((s) => s.type === recipient.type)?.label ?? recipient.type;
  }
}
