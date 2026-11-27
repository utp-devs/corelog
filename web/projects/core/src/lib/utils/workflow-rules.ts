import { WorkflowRule } from '../models/workflow.model';

export type RuleKind = 'condition' | 'validator' | 'postFunction';

export interface RuleOption {
  readonly value: string;
  readonly label: string;
}

/** Kind of control that edits one parameter of a rule. */
export type RuleParamControl =
  'roles' | 'permission' | 'field' | 'text' | 'textarea' | 'assignTarget' | 'resolution';

export interface RuleParamSpec {
  readonly key: string;
  readonly label: string;
  readonly control: RuleParamControl;
  readonly help?: string;
  readonly placeholder?: string;
}

export interface RuleSpec {
  readonly type: string;
  readonly kind: RuleKind;
  readonly label: string;
  readonly description: string;
  readonly params: readonly RuleParamSpec[];
}

export const PROJECT_ROLE_OPTIONS: readonly RuleOption[] = [
  { value: 'lead', label: 'Líder' },
  { value: 'contributor', label: 'Colaborador' },
  { value: 'viewer', label: 'Observador' },
];

export const ASSIGN_TARGET_OPTIONS: readonly RuleOption[] = [
  { value: 'reporter', label: 'Quien reportó la incidencia' },
  { value: 'current_user', label: 'Quien ejecuta la transición' },
  { value: 'lead', label: 'El líder del proyecto' },
  { value: 'unassigned', label: 'Nadie (sin responsable)' },
];

export const PERMISSION_OPTIONS: readonly RuleOption[] = [
  { value: 'edit_issue', label: 'Editar incidencias' },
  { value: 'assign_issue', label: 'Asignar incidencias' },
  { value: 'transition_issue', label: 'Cambiar de estado' },
  { value: 'comment_issue', label: 'Comentar' },
  { value: 'move_cards', label: 'Mover tarjetas' },
  { value: 'manage_sprints', label: 'Gestionar sprints' },
  { value: 'manage_workflow', label: 'Gestionar el flujo de trabajo' },
  { value: 'manage_members', label: 'Gestionar miembros' },
  { value: 'edit_project', label: 'Editar el proyecto' },
];

/** Value of the "done" resolution that the server maps to the organization default. */
export const DEFAULT_DONE_RESOLUTION = 'done';

/** Standard fields a transition may ask for, besides the custom ones (`cf_` plus 32 hex digits). */
export const STANDARD_FIELD_OPTIONS: readonly RuleOption[] = [
  { value: 'story_points', label: 'Puntos de historia' },
  { value: 'due_date', label: 'Fecha de vencimiento' },
  { value: 'start_date', label: 'Fecha de inicio' },
  { value: 'flagged', label: 'Marcada' },
];

const FIELD_PARAM: RuleParamSpec = {
  key: 'field',
  label: 'Campo',
  control: 'field',
  help: 'Un campo estándar o un campo personalizado (cf_…).',
};

/** Typed registry of every rule the engine understands. The editor builds its forms from it. */
export const RULE_SPECS: readonly RuleSpec[] = [
  {
    type: 'user_in_role',
    kind: 'condition',
    label: 'Quien ejecuta tiene un rol',
    description: 'Solo las personas con alguno de los roles marcados ven la transición.',
    params: [{ key: 'roles', label: 'Roles permitidos', control: 'roles' }],
  },
  {
    type: 'is_assignee',
    kind: 'condition',
    label: 'Quien ejecuta es el responsable',
    description: 'Solo la persona asignada a la incidencia puede ejecutarla.',
    params: [],
  },
  {
    type: 'is_reporter',
    kind: 'condition',
    label: 'Quien ejecuta reportó la incidencia',
    description: 'Solo quien reportó la incidencia puede ejecutarla.',
    params: [],
  },
  {
    type: 'permission',
    kind: 'condition',
    label: 'Quien ejecuta tiene un permiso',
    description: 'Exige un permiso del proyecto.',
    params: [{ key: 'permission', label: 'Permiso', control: 'permission' }],
  },
  {
    type: 'field_equals',
    kind: 'condition',
    label: 'Un campo tiene un valor',
    description: 'La transición solo está disponible si el campo vale lo indicado.',
    params: [FIELD_PARAM, { key: 'value', label: 'Valor', control: 'text' }],
  },
  {
    type: 'required_field',
    kind: 'validator',
    label: 'Un campo es obligatorio',
    description: 'Pide un valor para el campo antes de cambiar de estado.',
    params: [FIELD_PARAM],
  },
  {
    type: 'comment_required',
    kind: 'validator',
    label: 'Exigir un comentario',
    description: 'Quien ejecuta debe escribir un comentario.',
    params: [],
  },
  {
    type: 'resolution_required',
    kind: 'validator',
    label: 'Exigir una resolución',
    description: 'Quien ejecuta debe elegir una resolución.',
    params: [],
  },
  {
    type: 'subtasks_done',
    kind: 'validator',
    label: 'Subtareas completadas',
    description: 'Todas las subtareas deben estar en un estado de categoría Hecho.',
    params: [],
  },
  {
    type: 'assignee_set',
    kind: 'validator',
    label: 'Exigir un responsable',
    description: 'La incidencia debe tener responsable o se pedirá uno.',
    params: [],
  },
  {
    type: 'set_resolution',
    kind: 'postFunction',
    label: 'Establecer la resolución',
    description: 'Fija la resolución de la incidencia al ejecutar la transición.',
    params: [{ key: 'resolution', label: 'Resolución', control: 'resolution' }],
  },
  {
    type: 'clear_resolution',
    kind: 'postFunction',
    label: 'Borrar la resolución',
    description: 'Quita la resolución, por ejemplo al reabrir.',
    params: [],
  },
  {
    type: 'assign',
    kind: 'postFunction',
    label: 'Asignar la incidencia',
    description: 'Cambia el responsable al ejecutar la transición.',
    params: [{ key: 'to', label: 'Asignar a', control: 'assignTarget' }],
  },
  {
    type: 'set_field',
    kind: 'postFunction',
    label: 'Fijar el valor de un campo',
    description: 'Escribe un valor fijo en un campo.',
    params: [FIELD_PARAM, { key: 'value', label: 'Valor', control: 'text' }],
  },
  {
    type: 'add_comment',
    kind: 'postFunction',
    label: 'Añadir un comentario',
    description: 'Deja un comentario automático en la incidencia.',
    params: [
      {
        key: 'text',
        label: 'Texto del comentario',
        control: 'textarea',
        help: 'Máximo 2000 caracteres.',
      },
    ],
  },
  {
    type: 'emit',
    kind: 'postFunction',
    label: 'Emitir un evento',
    description: 'Publica un evento con nombre para las automatizaciones.',
    params: [
      {
        key: 'event',
        label: 'Nombre del evento',
        control: 'text',
        placeholder: 'issue.reviewed',
        help: 'Minúsculas, números y guiones bajos separados por puntos.',
      },
    ],
  },
];

export function specsOfKind(kind: RuleKind): readonly RuleSpec[] {
  return RULE_SPECS.filter((spec) => spec.kind === kind);
}

export function ruleSpec(type: string): RuleSpec | undefined {
  return RULE_SPECS.find((spec) => spec.type === type);
}

/** New rule of a type with the default value of every parameter. */
export function newRule(type: string): WorkflowRule {
  const spec = ruleSpec(type);
  const params: Record<string, unknown> = {};
  for (const param of spec?.params ?? []) {
    if (param.control === 'roles') params[param.key] = ['lead'];
    else if (param.control === 'permission') params[param.key] = 'edit_issue';
    else if (param.control === 'assignTarget') params[param.key] = 'current_user';
    else if (param.control === 'resolution') params[param.key] = DEFAULT_DONE_RESOLUTION;
    else params[param.key] = '';
  }
  return { type, params };
}

/** Name of a field for display: standard fields by name, custom ones as "Campo personalizado". */
export function fieldLabel(field: string): string {
  const standard = STANDARD_FIELD_OPTIONS.find((option) => option.value === field);
  if (standard) return standard.label;
  return field.startsWith('cf_') ? 'Campo personalizado' : field;
}

function optionLabel(options: readonly RuleOption[], value: unknown): string {
  return options.find((option) => option.value === value)?.label ?? String(value ?? '');
}

/** One-line description of a rule with its parameters, for tables and chips. */
export function describeRule(rule: WorkflowRule): string {
  const spec = ruleSpec(rule.type);
  const label = spec?.label ?? rule.type;
  const params = rule.params;
  switch (rule.type) {
    case 'user_in_role': {
      const roles = Array.isArray(params['roles']) ? (params['roles'] as string[]) : [];
      return `${label}: ${roles.map((role) => optionLabel(PROJECT_ROLE_OPTIONS, role)).join(', ')}`;
    }
    case 'permission':
      return `${label}: ${optionLabel(PERMISSION_OPTIONS, params['permission'])}`;
    case 'field_equals':
    case 'set_field':
      return `${label}: ${fieldLabel(String(params['field'] ?? ''))} = ${String(params['value'] ?? '')}`;
    case 'required_field':
      return `${label}: ${fieldLabel(String(params['field'] ?? ''))}`;
    case 'assign':
      return `${label}: ${optionLabel(ASSIGN_TARGET_OPTIONS, params['to'])}`;
    case 'emit':
      return `${label}: ${String(params['event'] ?? '')}`;
    default:
      return label;
  }
}

/** Client-side check of a rule form: a Spanish message for the first missing parameter, or null. */
export function ruleFieldError(rule: WorkflowRule): string | null {
  const spec = ruleSpec(rule.type);
  if (!spec) return 'Tipo de regla desconocido.';
  for (const param of spec.params) {
    const value = rule.params[param.key];
    if (param.control === 'roles') {
      if (!Array.isArray(value) || value.length === 0) return 'Elige al menos un rol.';
    } else if (typeof value !== 'string' || value.trim() === '') {
      if (param.control !== 'text' || spec.type !== 'field_equals') {
        return `Indica «${param.label.toLowerCase()}».`;
      }
    }
  }
  return null;
}
