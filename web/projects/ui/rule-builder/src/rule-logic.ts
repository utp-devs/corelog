import type {
  AutomationAction,
  AutomationActionType,
  AutomationCondition,
  AutomationConditionType,
  AutomationRuleDraft,
  AutomationTrigger,
  AutomationTriggerType,
} from '@corelog/core';

export interface Choice<T extends string = string> {
  readonly value: T;
  readonly label: string;
}

export const TRIGGER_CHOICES: readonly Choice<AutomationTriggerType>[] = [
  { value: 'issue_created', label: 'Se crea una incidencia' },
  { value: 'issue_updated', label: 'Se actualiza una incidencia' },
  { value: 'issue_transitioned', label: 'Una incidencia cambia de estado' },
  { value: 'comment_added', label: 'Se añade un comentario' },
  { value: 'sprint_started', label: 'Empieza un sprint' },
  { value: 'sprint_completed', label: 'Termina un sprint' },
  { value: 'scheduled', label: 'Cada cierto tiempo' },
];

export const CONDITION_CHOICES: readonly Choice<AutomationConditionType>[] = [
  { value: 'field_equals', label: 'Un campo tiene un valor' },
  { value: 'field_in', label: 'Un campo es uno de varios valores' },
  { value: 'issue_type', label: 'La incidencia es de ciertos tipos' },
  { value: 'actor_role', label: 'Quien actúa tiene un rol' },
  { value: 'query', label: 'Cumple una consulta CoreQL' },
];

export const ACTION_CHOICES: readonly Choice<AutomationActionType>[] = [
  { value: 'add_comment', label: 'Añadir un comentario' },
  { value: 'transition_issue', label: 'Cambiar de estado' },
  { value: 'assign_issue', label: 'Asignar la incidencia' },
  { value: 'set_field', label: 'Cambiar un campo' },
  { value: 'add_label', label: 'Añadir una etiqueta' },
  { value: 'remove_label', label: 'Quitar una etiqueta' },
  { value: 'create_subtask', label: 'Crear una subtarea' },
  { value: 'send_webhook', label: 'Enviar un webhook' },
  { value: 'send_notification', label: 'Enviar un aviso' },
];

/** Fields of an issue a condition can compare. */
export const COMPARABLE_FIELDS: readonly Choice[] = [
  { value: 'status', label: 'Estado' },
  { value: 'status_category', label: 'Categoría del estado' },
  { value: 'priority', label: 'Prioridad' },
  { value: 'type', label: 'Tipo' },
  { value: 'summary', label: 'Resumen' },
  { value: 'assignee', label: 'Responsable' },
  { value: 'reporter', label: 'Informador' },
];

/** Fields an action can write. */
export const SETTABLE_FIELDS: readonly Choice[] = [
  { value: 'summary', label: 'Resumen' },
  { value: 'description', label: 'Descripción' },
  { value: 'priority', label: 'Prioridad' },
  { value: 'story_points', label: 'Puntos de historia' },
  { value: 'due_date', label: 'Fecha de vencimiento' },
];

/** Fields whose change can fire an update trigger. */
export const UPDATED_FIELDS: readonly Choice[] = [
  { value: '', label: 'Cualquier campo' },
  { value: 'summary', label: 'Resumen' },
  { value: 'description', label: 'Descripción' },
  { value: 'priority', label: 'Prioridad' },
  { value: 'assignee', label: 'Responsable' },
  { value: 'labels', label: 'Etiquetas' },
];

export const ROLE_CHOICES: readonly Choice[] = [
  { value: 'lead', label: 'Líder del proyecto' },
  { value: 'contributor', label: 'Colaborador' },
  { value: 'viewer', label: 'Observador' },
  { value: 'org_admin', label: 'Administrador de la organización' },
];

export const ASSIGN_CHOICES: readonly Choice[] = [
  { value: 'reporter', label: 'Quien informó la incidencia' },
  { value: 'lead', label: 'Líder del proyecto' },
  { value: 'user', label: 'Una persona concreta' },
  { value: 'round_robin', label: 'Reparto por turnos en un equipo' },
];

export const NOTIFY_CHOICES: readonly Choice[] = [
  { value: 'assignee', label: 'Responsable' },
  { value: 'reporter', label: 'Informador' },
  { value: 'lead', label: 'Líder del proyecto' },
  { value: 'user', label: 'Una persona concreta' },
];

/** Closed list of placeholders the server accepts inside texts. */
export const SMART_VALUES: readonly Choice[] = [
  { value: 'issue.key', label: 'Clave' },
  { value: 'issue.summary', label: 'Resumen' },
  { value: 'issue.status', label: 'Estado' },
  { value: 'issue.priority', label: 'Prioridad' },
  { value: 'issue.type', label: 'Tipo' },
  { value: 'actor.name', label: 'Quien actúa' },
  { value: 'project.key', label: 'Clave del proyecto' },
  { value: 'project.name', label: 'Proyecto' },
  { value: 'trigger.from', label: 'Estado anterior' },
  { value: 'trigger.to', label: 'Estado nuevo' },
  { value: 'rule.name', label: 'Nombre de la regla' },
];

export const MAX_ACTIONS = 10;
export const MAX_CONDITIONS = 10;
export const MIN_EVERY_MINUTES = 5;
export const MAX_EVERY_MINUTES = 7 * 24 * 60;
export const MAX_NAME_LENGTH = 120;
export const MAX_TEXT_LENGTH = 2000;

const labelOf = (list: readonly Choice[], value: string | undefined): string =>
  list.find((c) => c.value === value)?.label ?? value ?? '';

export const triggerLabel = (type: AutomationTriggerType): string => labelOf(TRIGGER_CHOICES, type);
export const actionLabel = (type: AutomationActionType): string => labelOf(ACTION_CHOICES, type);
export const conditionLabel = (type: AutomationConditionType): string =>
  labelOf(CONDITION_CHOICES, type);

export function emptyRule(): AutomationRuleDraft {
  return {
    name: '',
    enabled: true,
    trigger: { type: 'issue_created' },
    conditions: [],
    actions: [{ type: 'add_comment', text: '' }],
  };
}

/** A trigger of the type with its defaults; parameters of other types are dropped. */
export function triggerOfType(type: AutomationTriggerType): AutomationTrigger {
  return type === 'scheduled' ? { type, everyMinutes: 60, query: '' } : { type };
}

export function conditionOfType(type: AutomationConditionType): AutomationCondition {
  switch (type) {
    case 'field_equals':
      return { type, field: 'priority', value: '' };
    case 'field_in':
      return { type, field: 'status', values: [] };
    case 'issue_type':
      return { type, values: [] };
    case 'actor_role':
      return { type, role: 'lead' };
    default:
      return { type: 'query', query: '' };
  }
}

export function actionOfType(type: AutomationActionType): AutomationAction {
  switch (type) {
    case 'transition_issue':
      return { type, status: '' };
    case 'assign_issue':
      return { type, who: 'reporter' };
    case 'set_field':
      return { type, field: 'priority', value: '' };
    case 'add_comment':
    case 'create_subtask':
      return { type, text: '' };
    case 'add_label':
    case 'remove_label':
      return { type, label: '' };
    case 'send_webhook':
      return { type, url: '' };
    default:
      return { type: 'send_notification', who: 'assignee', text: '' };
  }
}

// List operations. All of them return a new list.

export function addItem<T>(list: readonly T[], item: T, max: number): readonly T[] {
  return list.length >= max ? list : [...list, item];
}

export function removeAt<T>(list: readonly T[], index: number): readonly T[] {
  return list.filter((_, i) => i !== index);
}

export function replaceAt<T>(list: readonly T[], index: number, item: T): readonly T[] {
  return list.map((current, i) => (i === index ? item : current));
}

/** Moves the item one place (delta -1 up, +1 down); out of range leaves the list as it is. */
export function moveItem<T>(list: readonly T[], index: number, delta: -1 | 1): readonly T[] {
  const target = index + delta;
  if (index < 0 || index >= list.length || target < 0 || target >= list.length) return list;
  const next = [...list];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

/** Duplicate of a rule for "Duplicar": same definition, new name, disabled. */
export function duplicateDraft(rule: AutomationRuleDraft): AutomationRuleDraft {
  return {
    name: `${rule.name} (copia)`.slice(0, MAX_NAME_LENGTH),
    enabled: false,
    trigger: { ...rule.trigger },
    conditions: rule.conditions.map((c) => ({
      ...c,
      values: c.values ? [...c.values] : undefined,
    })),
    actions: rule.actions.map((a) => ({ ...a })),
  };
}

/** Splits a comma or line separated list into trimmed non-empty values. */
export function parseList(text: string): readonly string[] {
  return text
    .split(/[,\n]/)
    .map((part) => part.trim())
    .filter((part) => part !== '');
}

// Summary sentence.

const q = (text: string | undefined): string => (text?.trim() ? `«${text.trim()}»` : '…');
const listOf = (values: readonly string[] | undefined): string =>
  values && values.length > 0 ? values.map((v) => `«${v}»`).join(' o ') : '…';

export function summarizeTrigger(t: AutomationTrigger): string {
  switch (t.type) {
    case 'issue_created':
      return 'cuando se cree una incidencia';
    case 'issue_updated':
      return t.field
        ? `cuando cambie ${labelOf(UPDATED_FIELDS, t.field).toLowerCase()} de una incidencia`
        : 'cuando se actualice una incidencia';
    case 'issue_transitioned':
      if (t.from && t.to) return `cuando una incidencia pase de ${q(t.from)} a ${q(t.to)}`;
      if (t.to) return `cuando una incidencia pase a ${q(t.to)}`;
      if (t.from) return `cuando una incidencia salga de ${q(t.from)}`;
      return 'cuando una incidencia cambie de estado';
    case 'comment_added':
      return 'cuando se añada un comentario';
    case 'sprint_started':
      return 'cuando empiece un sprint';
    case 'sprint_completed':
      return 'cuando termine un sprint';
    case 'scheduled':
      return `cada ${t.everyMinutes ?? '…'} minutos, para las incidencias que cumplan ${q(t.query)}`;
  }
}

export function summarizeCondition(c: AutomationCondition): string {
  switch (c.type) {
    case 'query':
      return `cumple ${q(c.query)}`;
    case 'field_equals':
      return `${labelOf(COMPARABLE_FIELDS, c.field).toLowerCase()} es ${q(c.value)}`;
    case 'field_in':
      return `${labelOf(COMPARABLE_FIELDS, c.field).toLowerCase()} es ${listOf(c.values)}`;
    case 'actor_role':
      return `quien actúa es ${labelOf(ROLE_CHOICES, c.role).toLowerCase()}`;
    case 'issue_type':
      return `el tipo es ${listOf(c.values)}`;
  }
}

function whoText(a: AutomationAction, names: (id: string) => string): string {
  switch (a.who) {
    case 'reporter':
      return 'quien la informó';
    case 'assignee':
      return 'su responsable';
    case 'lead':
      return 'la persona líder del proyecto';
    case 'round_robin':
      return 'un equipo por turnos';
    case 'user':
      return a.userId ? names(a.userId) : '…';
    default:
      return '…';
  }
}

export function summarizeAction(
  a: AutomationAction,
  names: (id: string) => string = (id) => id,
): string {
  switch (a.type) {
    case 'transition_issue':
      return `pasar la incidencia a ${q(a.status)}`;
    case 'assign_issue':
      return `asignarla a ${whoText(a, names)}`;
    case 'set_field':
      return `cambiar ${labelOf(SETTABLE_FIELDS, a.field).toLowerCase()} a ${q(a.value)}`;
    case 'add_comment':
      return `añadir el comentario ${q(a.text)}`;
    case 'add_label':
      return `añadir la etiqueta ${q(a.label)}`;
    case 'remove_label':
      return `quitar la etiqueta ${q(a.label)}`;
    case 'create_subtask':
      return `crear la subtarea ${q(a.text)}`;
    case 'send_webhook':
      return `enviar un webhook a ${q(a.url)}`;
    case 'send_notification':
      return `avisar a ${whoText(a, names)}: ${q(a.text)}`;
  }
}

function joinAnd(parts: readonly string[]): string {
  if (parts.length <= 1) return parts[0] ?? '';
  return `${parts.slice(0, -1).join(', ')} y ${parts[parts.length - 1]}`;
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** The rule as one Spanish sentence: "Cuando ..., si ..., entonces ...". */
export function summarizeRule(
  rule: Pick<AutomationRuleDraft, 'trigger' | 'conditions' | 'actions'>,
  names?: (id: string) => string,
): string {
  const parts = [capitalize(summarizeTrigger(rule.trigger))];
  if (rule.conditions.length > 0) {
    parts.push(`si ${joinAnd(rule.conditions.map(summarizeCondition))}`);
  }
  const actions =
    rule.actions.length > 0 ? joinAnd(rule.actions.map((a) => summarizeAction(a, names))) : '…';
  parts.push(`entonces ${actions}`);
  return `${parts.join(', ')}.`;
}

// Local validation (the server validates again).

export interface RuleProblem {
  /** Where it is: "name", "trigger", "conditions[0]", "actions[1]". */
  readonly path: string;
  readonly message: string;
}

const SMART_KEYS = new Set(SMART_VALUES.map((s) => s.value));

/** Message when a text uses braces that are not a known smart value; null when it is fine. */
export function smartValueProblem(text: string | undefined): string | null {
  if (!text) return null;
  const stripped = text.replace(/\{\{\s*([^{}]*?)\s*\}\}/g, (_, key: string) =>
    SMART_KEYS.has(key) ? '' : '\u0000',
  );
  if (stripped.includes('\u0000')) return 'Hay un valor inteligente que no existe.';
  if (stripped.includes('{{') || stripped.includes('}}')) return 'Hay llaves dobles sin cerrar.';
  return null;
}

function textProblem(text: string | undefined, required: boolean): string | null {
  if (required && !text?.trim()) return 'El texto es obligatorio.';
  if ((text?.length ?? 0) > MAX_TEXT_LENGTH) return `Admite hasta ${MAX_TEXT_LENGTH} caracteres.`;
  return smartValueProblem(text);
}

export function validateRule(rule: AutomationRuleDraft): readonly RuleProblem[] {
  const out: RuleProblem[] = [];
  const name = rule.name.trim();
  if (!name) out.push({ path: 'name', message: 'Escribe un nombre para la regla.' });
  else if (name.length > MAX_NAME_LENGTH)
    out.push({ path: 'name', message: `El nombre admite hasta ${MAX_NAME_LENGTH} caracteres.` });

  const t = rule.trigger;
  if (t.type === 'scheduled') {
    const every = t.everyMinutes ?? 0;
    if (!Number.isInteger(every) || every < MIN_EVERY_MINUTES || every > MAX_EVERY_MINUTES)
      out.push({
        path: 'trigger',
        message: `El intervalo va de ${MIN_EVERY_MINUTES} minutos a ${MAX_EVERY_MINUTES} (una semana).`,
      });
    if (!t.query?.trim())
      out.push({ path: 'trigger', message: 'Indica la consulta de las incidencias a revisar.' });
  }

  if (rule.conditions.length > MAX_CONDITIONS)
    out.push({ path: 'conditions', message: `Máximo ${MAX_CONDITIONS} condiciones.` });
  rule.conditions.forEach((c, i) => {
    const path = `conditions[${i}]`;
    if (c.type === 'query' && !c.query?.trim())
      out.push({ path, message: 'Escribe la consulta CoreQL.' });
    if (c.type === 'field_equals' && !c.value?.trim())
      out.push({ path, message: 'Indica el valor a comparar.' });
    if (c.type === 'field_in' && (c.values?.length ?? 0) === 0)
      out.push({ path, message: 'Indica al menos un valor.' });
    if (c.type === 'issue_type' && (c.values?.length ?? 0) === 0)
      out.push({ path, message: 'Indica al menos un tipo de incidencia.' });
  });

  if (rule.actions.length === 0)
    out.push({ path: 'actions', message: 'Añade al menos una acción.' });
  if (rule.actions.length > MAX_ACTIONS)
    out.push({ path: 'actions', message: `Máximo ${MAX_ACTIONS} acciones.` });
  rule.actions.forEach((a, i) => {
    const path = `actions[${i}]`;
    const fail = (message: string | null): void => {
      if (message) out.push({ path, message });
    };
    switch (a.type) {
      case 'transition_issue':
        if (!a.status?.trim()) fail('Indica el estado de destino.');
        break;
      case 'assign_issue':
        if (a.who === 'user' && !a.userId) fail('Elige la persona.');
        if (a.who === 'round_robin' && !a.teamId) fail('Elige el equipo.');
        break;
      case 'set_field':
        fail(textProblem(a.value, false));
        break;
      case 'add_comment':
      case 'create_subtask':
        fail(textProblem(a.text, true));
        break;
      case 'add_label':
      case 'remove_label':
        if (!a.label?.trim()) fail('Escribe la etiqueta.');
        else if (a.label.length > 50) fail('La etiqueta admite hasta 50 caracteres.');
        break;
      case 'send_webhook':
        if (!/^https?:\/\/\S+$/i.test(a.url ?? ''))
          fail('La dirección debe empezar por http o https.');
        break;
      case 'send_notification':
        if (a.who === 'user' && !a.userId) fail('Elige la persona.');
        else fail(textProblem(a.text, true));
        break;
    }
  });
  return out;
}

/** Problems of one path prefix, for showing them next to the item. */
export function problemsOf(problems: readonly RuleProblem[], path: string): readonly string[] {
  return problems.filter((p) => p.path === path).map((p) => p.message);
}
