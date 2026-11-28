import type { WebhookDeliveryStatus } from '@corelog/core';
import type { Tone } from '@corelog/ui/tone-tag';

export interface EventOption {
  readonly value: string;
  readonly label: string;
}

export interface EventGroup {
  readonly key: string;
  readonly label: string;
  readonly events: readonly EventOption[];
}

/** The wildcard that subscribes to every event, present and future. */
export const ALL_EVENTS = '*';

export const EVENT_GROUPS: readonly EventGroup[] = [
  {
    key: 'issue',
    label: 'Incidencias',
    events: [
      { value: 'issue.created', label: 'Se crea una incidencia' },
      { value: 'issue.updated', label: 'Se modifica una incidencia' },
      { value: 'issue.transitioned', label: 'Cambia de estado' },
      { value: 'issue.deleted', label: 'Se elimina' },
      { value: 'issue.restored', label: 'Se restaura' },
    ],
  },
  {
    key: 'comment',
    label: 'Comentarios',
    events: [{ value: 'comment.created', label: 'Se añade un comentario' }],
  },
  {
    key: 'sprint',
    label: 'Sprints',
    events: [
      { value: 'sprint.started', label: 'Empieza un sprint' },
      { value: 'sprint.completed', label: 'Termina un sprint' },
    ],
  },
  {
    key: 'version',
    label: 'Versiones',
    events: [{ value: 'version.released', label: 'Se publica una versión' }],
  },
];

const ALL_VALUES: readonly string[] = EVENT_GROUPS.flatMap((g) => g.events.map((e) => e.value));

export function eventLabel(value: string): string {
  if (value === ALL_EVENTS) return 'Todos los eventos';
  if (value === 'ping') return 'Prueba (ping)';
  for (const group of EVENT_GROUPS) {
    const found = group.events.find((e) => e.value === value);
    if (found) return found.label;
  }
  return value;
}

/** Selection after toggling one event. Choosing every single event is stored as the wildcard. */
export function toggleEvent(selected: readonly string[], value: string): readonly string[] {
  const base = selected.includes(ALL_EVENTS) ? ALL_VALUES : selected;
  const next = base.includes(value) ? base.filter((v) => v !== value) : [...base, value];
  return next.length === ALL_VALUES.length ? [ALL_EVENTS] : next;
}

export function toggleAll(selected: readonly string[]): readonly string[] {
  return selected.includes(ALL_EVENTS) ? [] : [ALL_EVENTS];
}

export function isEventSelected(selected: readonly string[], value: string): boolean {
  return selected.includes(ALL_EVENTS) || selected.includes(value);
}

/** Events of a group chosen, for the "indeterminate" hint. */
export function groupCount(selected: readonly string[], group: EventGroup): number {
  return group.events.filter((e) => isEventSelected(selected, e.value)).length;
}

export interface UrlCheck {
  readonly valid: boolean;
  readonly message: string | null;
}

const PRIVATE_V4 = [
  /^127\./,
  /^10\./,
  /^192\.168\./,
  /^172\.(1[6-9]|2\d|3[01])\./,
  /^169\.254\./,
  /^0\./,
];

/** Whether a host name points at the machine itself or at a private network, which the server refuses. */
export function isPrivateHost(host: string): boolean {
  const name = host.toLowerCase().replace(/^\[|\]$/g, '');
  if (name === 'localhost' || name.endsWith('.localhost')) return true;
  if (name.endsWith('.local') || name.endsWith('.internal') || name.endsWith('.lan')) return true;
  if (
    name === '::1' ||
    name === '::' ||
    name.startsWith('fe80:') ||
    /^f[cd][0-9a-f]{2}:/.test(name)
  )
    return true;
  return PRIVATE_V4.some((pattern) => pattern.test(name));
}

/** Live check of the address field. An empty field is not an error yet (the form stays invalid). */
export function checkWebhookUrl(raw: string): UrlCheck {
  const text = raw.trim();
  if (text === '') return { valid: false, message: null };
  let url: URL;
  try {
    url = new URL(text);
  } catch {
    return {
      valid: false,
      message: 'Escribe una dirección completa, como https://ejemplo.com/hook.',
    };
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:')
    return { valid: false, message: 'La dirección debe empezar por https:// o http://.' };
  if (url.username || url.password)
    return { valid: false, message: 'La dirección no puede incluir usuario ni contraseña.' };
  if (isPrivateHost(url.hostname))
    return {
      valid: false,
      message: 'No se permiten direcciones privadas ni locales (localhost, 192.168.x.x, 10.x.x.x).',
    };
  return { valid: true, message: null };
}

/** Host of an address for the list; the whole text when it is not a valid address. */
export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export const DELIVERY_LABELS: Readonly<Record<WebhookDeliveryStatus, string>> = {
  pending: 'Pendiente',
  success: 'Correcta',
  failed: 'Fallida',
  dead: 'Agotada',
};

export const DELIVERY_TONES: Readonly<Record<WebhookDeliveryStatus, Tone>> = {
  pending: 'gold',
  success: 'green',
  failed: 'orange',
  dead: 'red',
};

/** Failed and exhausted deliveries can be sent again. */
export function canRedeliver(status: WebhookDeliveryStatus): boolean {
  return status === 'failed' || status === 'dead';
}

/** Payload as indented JSON for reading; shown as text, never as markup. */
export function prettyPayload(payload: unknown): string {
  try {
    return JSON.stringify(payload, null, 2) ?? '';
  } catch {
    return String(payload);
  }
}
