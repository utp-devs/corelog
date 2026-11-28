import { BasicFilters, EMPTY_BASIC_FILTERS } from '@corelog/core';

export type NavigatorMode = 'basic' | 'advanced';

/** Column of the results table that the user can show or hide. */
export interface NavigatorColumn {
  readonly key: string;
  readonly title: string;
}

export const NAVIGATOR_COLUMNS: readonly NavigatorColumn[] = [
  { key: 'key', title: 'Clave' },
  { key: 'summary', title: 'Resumen' },
  { key: 'type', title: 'Tipo' },
  { key: 'status', title: 'Estado' },
  { key: 'priority', title: 'Prioridad' },
  { key: 'assignee', title: 'Responsable' },
  { key: 'reporter', title: 'Informante' },
  { key: 'project', title: 'Proyecto' },
  { key: 'labels', title: 'Etiquetas' },
  { key: 'points', title: 'Puntos' },
  { key: 'due', title: 'Vencimiento' },
  { key: 'created', title: 'Creada' },
  { key: 'updated', title: 'Actualizada' },
];

export const DEFAULT_COLUMNS: readonly string[] = [
  'key',
  'summary',
  'status',
  'priority',
  'assignee',
  'updated',
];

/** Fields the results can be sorted by, with the CoreQL field name. */
export const SORT_FIELDS: ReadonlyArray<{ readonly value: string; readonly label: string }> = [
  { value: 'updated', label: 'Actualizada' },
  { value: 'created', label: 'Creada' },
  { value: 'key', label: 'Clave' },
  { value: 'priority', label: 'Prioridad' },
  { value: 'status', label: 'Estado' },
  { value: 'assignee', label: 'Responsable' },
  { value: 'summary', label: 'Resumen' },
  { value: 'due', label: 'Vencimiento' },
];

export const DEFAULT_ORDER = 'updated DESC';

/** Address parameter of each basic filter. */
const FILTER_PARAMS = {
  projects: 'project',
  types: 'type',
  statusCategories: 'status',
  assignees: 'assignee',
  reporters: 'reporter',
  labels: 'label',
  components: 'component',
  fixVersions: 'version',
  text: 'text',
  createdFrom: 'createdFrom',
  createdTo: 'createdTo',
  updatedFrom: 'updatedFrom',
  updatedTo: 'updatedTo',
} as const satisfies Record<keyof BasicFilters, string>;

type ParamMap = Readonly<Record<string, string | null | undefined>>;

const LIST_KEYS = [
  'projects',
  'types',
  'statusCategories',
  'assignees',
  'reporters',
  'labels',
  'components',
  'fixVersions',
] as const;

const TEXT_KEYS = ['text', 'createdFrom', 'createdTo', 'updatedFrom', 'updatedTo'] as const;

/** Query parameters that hold the basic filters; empty filters give null so the router drops them. */
export function filtersToParams(filters: BasicFilters): Record<string, string | null> {
  const params: Record<string, string | null> = {};
  for (const key of LIST_KEYS) {
    params[FILTER_PARAMS[key]] = filters[key].length > 0 ? filters[key].join(',') : null;
  }
  for (const key of TEXT_KEYS) {
    params[FILTER_PARAMS[key]] = filters[key] || null;
  }
  return params;
}

/** Basic filters from the address. Unknown parameters are ignored. */
export function paramsToFilters(params: ParamMap): BasicFilters {
  const list = (name: string): string[] => (params[name] ?? '').split(',').filter(Boolean);
  const text = (name: string): string => params[name] ?? '';
  return {
    ...EMPTY_BASIC_FILTERS,
    projects: list(FILTER_PARAMS.projects),
    types: list(FILTER_PARAMS.types),
    statusCategories: list(FILTER_PARAMS.statusCategories),
    assignees: list(FILTER_PARAMS.assignees),
    reporters: list(FILTER_PARAMS.reporters),
    labels: list(FILTER_PARAMS.labels),
    components: list(FILTER_PARAMS.components),
    fixVersions: list(FILTER_PARAMS.fixVersions),
    text: text(FILTER_PARAMS.text),
    createdFrom: text(FILTER_PARAMS.createdFrom),
    createdTo: text(FILTER_PARAMS.createdTo),
    updatedFrom: text(FILTER_PARAMS.updatedFrom),
    updatedTo: text(FILTER_PARAMS.updatedTo),
  };
}

/** Parameter names that belong to the basic filters, to clear them when the mode changes. */
export const BASIC_PARAM_NAMES: readonly string[] = Object.values(FILTER_PARAMS);

/** Valid column keys from a comma list; falls back to the defaults when none is valid. */
export function parseColumns(value: string | null | undefined): string[] {
  const known = new Set(NAVIGATOR_COLUMNS.map((column) => column.key));
  const picked = (value ?? '').split(',').filter((key) => known.has(key));
  return picked.length > 0 ? picked : [...DEFAULT_COLUMNS];
}

/** Columns in table order, always with the summary so a row stays recognizable. */
export function orderedColumns(keys: readonly string[]): string[] {
  const wanted = new Set([...keys, 'summary']);
  return NAVIGATOR_COLUMNS.map((column) => column.key).filter((key) => wanted.has(key));
}

/** Escapes a value for a CSV cell. */
export function csvCell(value: string): string {
  return /[",\n\r]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}
