import type { GadgetConfig, GadgetType } from '../models/dashboard-v2.model';

/** Kind of input a configuration field needs. */
export type GadgetFieldKind =
  'number' | 'boolean' | 'select' | 'text' | 'textarea' | 'project' | 'filter' | 'multi';

export interface GadgetFieldOption {
  readonly value: string;
  readonly label: string;
}

export interface GadgetField {
  readonly key: string;
  readonly label: string;
  readonly kind: GadgetFieldKind;
  readonly help?: string;
  readonly min?: number;
  readonly max?: number;
  readonly options?: readonly GadgetFieldOption[];
  /** Placeholder or empty option text of selects. */
  readonly placeholder?: string;
}

export type GadgetCategory = 'issues' | 'charts' | 'activity' | 'other';

export interface GadgetDefinition {
  readonly type: GadgetType;
  readonly label: string;
  readonly description: string;
  readonly category: GadgetCategory;
  /** PrimeIcons class. */
  readonly icon: string;
  readonly defaultConfig: GadgetConfig;
  readonly fields: readonly GadgetField[];
}

export const GADGET_CATEGORY_LABELS: Readonly<Record<GadgetCategory, string>> = {
  issues: 'Incidencias',
  charts: 'Gráficos y estadísticas',
  activity: 'Actividad y calendario',
  other: 'Otros',
};

export const STAT_FIELD_OPTIONS: readonly GadgetFieldOption[] = [
  { value: 'status', label: 'Estado' },
  { value: 'assignee', label: 'Responsable' },
  { value: 'priority', label: 'Prioridad' },
  { value: 'type', label: 'Tipo' },
  { value: 'label', label: 'Etiqueta' },
  { value: 'project', label: 'Proyecto' },
];

export const ISSUE_COLUMN_OPTIONS: readonly GadgetFieldOption[] = [
  { value: 'key', label: 'Clave' },
  { value: 'summary', label: 'Resumen' },
  { value: 'status', label: 'Estado' },
  { value: 'priority', label: 'Prioridad' },
  { value: 'type', label: 'Tipo' },
  { value: 'assignee', label: 'Responsable' },
  { value: 'due', label: 'Vencimiento' },
  { value: 'updated', label: 'Actualizada' },
];

export const FAVORITE_TYPE_OPTIONS: readonly GadgetFieldOption[] = [
  { value: 'project', label: 'Proyectos' },
  { value: 'filter', label: 'Filtros' },
  { value: 'board', label: 'Tableros' },
  { value: 'dashboard', label: 'Paneles' },
  { value: 'issue', label: 'Incidencias' },
];

const ROWS: GadgetField = {
  key: 'limit',
  label: 'Filas',
  kind: 'number',
  min: 1,
  max: 50,
  help: 'Entre 1 y 50.',
};
const GROUPS: GadgetField = {
  key: 'limit',
  label: 'Grupos',
  kind: 'number',
  min: 1,
  max: 20,
  help: 'Entre 1 y 20.',
};
const PROJECT: GadgetField = {
  key: 'project_id',
  label: 'Proyecto',
  kind: 'project',
  placeholder: 'Todos mis proyectos',
};
const INCLUDE_DONE: GadgetField = {
  key: 'include_done',
  label: 'Incluir incidencias terminadas',
  kind: 'boolean',
};

/** Gadgets a dashboard can hold, in the order of the catalog. */
export const GADGET_CATALOG: readonly GadgetDefinition[] = [
  {
    type: 'assigned_to_me',
    label: 'Asignadas a mí',
    description: 'Tus incidencias abiertas, de la más reciente a la más antigua.',
    category: 'issues',
    icon: 'pi-user',
    defaultConfig: { limit: 10, include_done: false },
    fields: [ROWS, INCLUDE_DONE],
  },
  {
    type: 'created_by_me',
    label: 'Creadas por mí',
    description: 'Las incidencias que has creado.',
    category: 'issues',
    icon: 'pi-pencil',
    defaultConfig: { limit: 10, include_done: false },
    fields: [ROWS, INCLUDE_DONE],
  },
  {
    type: 'filter_results',
    label: 'Resultados de un filtro',
    description: 'Incidencias de un filtro guardado o de una consulta CoreQL.',
    category: 'issues',
    icon: 'pi-filter',
    defaultConfig: { query: 'statusCategory != done ORDER BY updated DESC', limit: 10 },
    fields: [
      {
        key: 'filter_id',
        label: 'Filtro guardado',
        kind: 'filter',
        placeholder: 'Usar una consulta',
      },
      {
        key: 'query',
        label: 'Consulta CoreQL',
        kind: 'textarea',
        help: 'Se usa solo cuando no eliges un filtro guardado.',
      },
      { key: 'columns', label: 'Columnas', kind: 'multi', options: ISSUE_COLUMN_OPTIONS },
      ROWS,
    ],
  },
  {
    type: 'issue_statistics',
    label: 'Estadísticas de incidencias',
    description: 'Cuántas incidencias hay por estado, responsable, prioridad u otro campo.',
    category: 'charts',
    icon: 'pi-chart-bar',
    defaultConfig: { group_by: 'status', include_done: true, limit: 10 },
    fields: [
      { key: 'group_by', label: 'Agrupar por', kind: 'select', options: STAT_FIELD_OPTIONS },
      PROJECT,
      INCLUDE_DONE,
      GROUPS,
    ],
  },
  {
    type: 'pie',
    label: 'Gráfico circular',
    description: 'El reparto de incidencias por un campo, como un gráfico circular.',
    category: 'charts',
    icon: 'pi-chart-pie',
    defaultConfig: { group_by: 'status', include_done: true, limit: 10 },
    fields: [
      { key: 'group_by', label: 'Agrupar por', kind: 'select', options: STAT_FIELD_OPTIONS },
      PROJECT,
      INCLUDE_DONE,
      GROUPS,
    ],
  },
  {
    type: 'two_dimensional',
    label: 'Estadísticas en dos dimensiones',
    description: 'Una tabla que cruza dos campos, por ejemplo estado y responsable.',
    category: 'charts',
    icon: 'pi-table',
    defaultConfig: { x_axis: 'status', y_axis: 'assignee', include_done: true, limit: 10 },
    fields: [
      { key: 'x_axis', label: 'Columnas', kind: 'select', options: STAT_FIELD_OPTIONS },
      { key: 'y_axis', label: 'Filas', kind: 'select', options: STAT_FIELD_OPTIONS },
      PROJECT,
      INCLUDE_DONE,
      GROUPS,
    ],
  },
  {
    type: 'created_vs_resolved',
    label: 'Creadas frente a resueltas',
    description: 'Cuántas incidencias se crean y se resuelven cada día.',
    category: 'charts',
    icon: 'pi-chart-line',
    defaultConfig: { days: 30 },
    fields: [
      PROJECT,
      { key: 'days', label: 'Días', kind: 'number', min: 7, max: 90, help: 'Entre 7 y 90.' },
    ],
  },
  {
    type: 'sprint_burndown',
    label: 'Burndown del sprint',
    description: 'Trabajo pendiente del sprint activo frente a la línea ideal.',
    category: 'charts',
    icon: 'pi-arrow-down-right',
    defaultConfig: {},
    fields: [PROJECT],
  },
  {
    type: 'activity_stream',
    label: 'Actividad reciente',
    description: 'Los últimos cambios en las incidencias de tus proyectos.',
    category: 'activity',
    icon: 'pi-history',
    defaultConfig: { limit: 10 },
    fields: [PROJECT, ROWS],
  },
  {
    type: 'calendar_upcoming',
    label: 'Próximos eventos',
    description: 'Eventos del calendario, fechas de sprints y vencimientos próximos.',
    category: 'activity',
    icon: 'pi-calendar',
    defaultConfig: { days: 14, limit: 10 },
    fields: [
      {
        key: 'days',
        label: 'Días por delante',
        kind: 'number',
        min: 1,
        max: 60,
        help: 'Entre 1 y 60.',
      },
      ROWS,
    ],
  },
  {
    type: 'recent_projects',
    label: 'Proyectos recientes',
    description: 'Los proyectos que has abierto hace poco.',
    category: 'other',
    icon: 'pi-folder-open',
    defaultConfig: { limit: 10 },
    fields: [ROWS],
  },
  {
    type: 'favorites',
    label: 'Favoritos',
    description: 'Proyectos, filtros, tableros y paneles que has marcado con una estrella.',
    category: 'other',
    icon: 'pi-star',
    defaultConfig: { types: [], limit: 10 },
    fields: [
      {
        key: 'types',
        label: 'Tipos',
        kind: 'multi',
        options: FAVORITE_TYPE_OPTIONS,
        help: 'Sin marcar, se muestran todos.',
      },
      ROWS,
    ],
  },
  {
    type: 'text',
    label: 'Texto',
    description: 'Una nota en Markdown: instrucciones, enlaces o avisos para el equipo.',
    category: 'other',
    icon: 'pi-align-left',
    defaultConfig: { markdown: '' },
    fields: [
      {
        key: 'markdown',
        label: 'Contenido (Markdown)',
        kind: 'textarea',
        help: 'Hasta 10000 caracteres.',
      },
    ],
  },
];

const BY_TYPE = new Map<GadgetType, GadgetDefinition>(GADGET_CATALOG.map((d) => [d.type, d]));

export function gadgetDefinition(type: GadgetType): GadgetDefinition {
  const found = BY_TYPE.get(type);
  if (!found) throw new Error(`Unknown gadget type: ${type}`);
  return found;
}

/**
 * Configuration to send: nulls and empty strings are dropped for optional ids, and for a filter gadget
 * a saved filter wins over the query (the API accepts only one of them).
 */
export function cleanGadgetConfig(type: GadgetType, config: GadgetConfig): GadgetConfig {
  const cleaned: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(config)) {
    if (value === null || value === undefined) continue;
    if (typeof value === 'string' && value.trim() === '' && key !== 'markdown') continue;
    if (key === 'columns' && Array.isArray(value) && value.length === 0) continue;
    cleaned[key] = typeof value === 'string' && key === 'query' ? value.trim() : value;
  }
  if (type === 'filter_results' && cleaned['filter_id']) delete cleaned['query'];
  return cleaned;
}
