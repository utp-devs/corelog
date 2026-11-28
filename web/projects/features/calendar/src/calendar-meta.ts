import type {
  CalendarItem,
  CalendarItemKind,
  CalendarKindGroup,
  RsvpResponse,
} from '@corelog/core';

export interface KindMeta {
  readonly label: string;
  readonly icon: string;
}

export const KIND_META: Readonly<Record<CalendarItemKind, KindMeta>> = {
  event: { label: 'Evento', icon: 'pi pi-calendar' },
  issue_due: { label: 'Vence una incidencia', icon: 'pi pi-clock' },
  issue_start: { label: 'Empieza una incidencia', icon: 'pi pi-play' },
  sprint_start: { label: 'Inicio de sprint', icon: 'pi pi-play-circle' },
  sprint_end: { label: 'Fin de sprint', icon: 'pi pi-flag' },
  version_release: { label: 'Lanzamiento de versión', icon: 'pi pi-box' },
};

export const GROUP_META: Readonly<
  Record<CalendarKindGroup, { readonly label: string; readonly color: string }>
> = {
  events: { label: 'Eventos', color: 'var(--cl-chart-1)' },
  issues: { label: 'Incidencias', color: 'var(--cl-chart-2)' },
  sprints: { label: 'Sprints', color: 'var(--cl-chart-4)' },
  versions: { label: 'Versiones', color: 'var(--cl-chart-3)' },
};

/** Chart token of each event color key. Derived dates are colored by their kind. */
const EVENT_COLORS: Readonly<Record<string, string>> = {
  gray: 'var(--cl-chart-6)',
  red: 'var(--cl-chart-5)',
  orange: 'var(--cl-chart-2)',
  amber: 'var(--cl-chart-7)',
  green: 'var(--cl-chart-3)',
  teal: 'var(--cl-chart-8)',
  blue: 'var(--cl-chart-1)',
  indigo: 'var(--cl-chart-1)',
  purple: 'var(--cl-chart-4)',
  pink: 'var(--cl-chart-5)',
};

const KIND_COLORS: Readonly<Record<Exclude<CalendarItemKind, 'event'>, string>> = {
  issue_due: 'var(--cl-chart-2)',
  issue_start: 'var(--cl-chart-8)',
  sprint_start: 'var(--cl-chart-4)',
  sprint_end: 'var(--cl-chart-4)',
  version_release: 'var(--cl-chart-3)',
};

/** CSS color (a token reference) of an item, fed to the chip as a custom property. */
export function itemColor(item: Pick<CalendarItem, 'kind' | 'color'>): string {
  if (item.kind === 'event') return EVENT_COLORS[item.color] ?? 'var(--cl-chart-1)';
  return KIND_COLORS[item.kind];
}

export const COLOR_OPTIONS: readonly { readonly value: string; readonly label: string }[] = [
  { value: 'blue', label: 'Azul' },
  { value: 'indigo', label: 'Índigo' },
  { value: 'purple', label: 'Morado' },
  { value: 'pink', label: 'Rosa' },
  { value: 'red', label: 'Rojo' },
  { value: 'orange', label: 'Naranja' },
  { value: 'amber', label: 'Ámbar' },
  { value: 'green', label: 'Verde' },
  { value: 'teal', label: 'Turquesa' },
  { value: 'gray', label: 'Gris' },
];

export const RSVP_LABEL: Readonly<Record<RsvpResponse, string>> = {
  needs_action: 'Sin responder',
  accepted: 'Asistiré',
  declined: 'No asistiré',
  tentative: 'Quizá',
};

export const VISIBILITY_LABEL = {
  private: 'Solo yo y los invitados',
  project: 'Miembros del proyecto',
  organization: 'Toda la organización',
} as const;

/** Time range for an accessible name or a chip: "10:00 - 11:00", or "Todo el día". */
export function timeLabel(item: CalendarItem): string {
  if (item.allDay) return 'Todo el día';
  const format = (iso: string): string =>
    new Date(iso).toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' });
  return `${format(item.startsAt)} - ${format(item.endsAt)}`;
}

/** Name read by a screen reader for one item. */
export function itemAriaLabel(item: CalendarItem): string {
  const kind = KIND_META[item.kind].label;
  const project = item.projectKey ? `, proyecto ${item.projectKey}` : '';
  return `${kind}: ${item.title}, ${timeLabel(item)}${project}`;
}
