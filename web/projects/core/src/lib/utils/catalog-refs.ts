import {
  CatalogItem,
  HIERARCHY_EPIC,
  HIERARCHY_SUBTASK,
  IssueTypeRef,
  PriorityRef,
} from '../models/catalog.model';
import { IssuePriority, IssueType } from '../models/issue.model';

/**
 * Display data of the values that the board, backlog, sprint and dashboard endpoints still return as
 * plain keys (`bug`, `high`, ...). Each key is shown with the palette entry of the default catalogs.
 */
const LEGACY_TYPE_REFS: Readonly<Record<IssueType, IssueTypeRef>> = {
  epic: { id: '', name: 'Épica', icon: 'epic', color: 'purple', hierarchyLevel: 1 },
  story: { id: '', name: 'Historia', icon: 'story', color: 'green', hierarchyLevel: 0 },
  task: { id: '', name: 'Tarea', icon: 'task', color: 'blue', hierarchyLevel: 0 },
  bug: { id: '', name: 'Error', icon: 'bug', color: 'red', hierarchyLevel: 0 },
  subtask: { id: '', name: 'Subtarea', icon: 'subtask', color: 'teal', hierarchyLevel: -1 },
};

const LEGACY_PRIORITY_REFS: Readonly<Record<IssuePriority, PriorityRef>> = {
  critical: { id: '', name: 'Máxima', icon: 'chevrons-up', color: 'red' },
  high: { id: '', name: 'Alta', icon: 'arrow-up', color: 'orange' },
  medium: { id: '', name: 'Media', icon: 'minus', color: 'amber' },
  low: { id: '', name: 'Baja', icon: 'arrow-down', color: 'blue' },
};

/** Display reference of a legacy issue type key; undefined when the key is not a known type. */
export function issueTypeRefFromKey(key: string | null | undefined): IssueTypeRef | undefined {
  return key ? LEGACY_TYPE_REFS[key as IssueType] : undefined;
}

/** Display reference of a legacy priority key; undefined when the key is not a known priority. */
export function priorityRefFromKey(key: string | null | undefined): PriorityRef | undefined {
  return key ? LEGACY_PRIORITY_REFS[key as IssuePriority] : undefined;
}

function normalize(name: string): string {
  return name.normalize('NFD').replace(/[̀-ͯ]/g, '').trim().toLowerCase();
}

const LEGACY_TYPE_BY_NAME: Readonly<Record<string, IssueType>> = {
  epica: 'epic',
  historia: 'story',
  tarea: 'task',
  error: 'bug',
  subtarea: 'subtask',
};

const LEGACY_PRIORITY_BY_NAME: Readonly<Record<string, IssuePriority>> = {
  maxima: 'critical',
  alta: 'high',
  media: 'medium',
  baja: 'low',
};

/**
 * Legacy filter key of an issue type: the hierarchy level for epics and subtasks, the default name
 * for the standard types. Custom types have no key (undefined).
 */
export function legacyTypeKeyOf(
  item: Pick<CatalogItem, 'name' | 'hierarchyLevel'>,
): IssueType | undefined {
  if (item.hierarchyLevel === HIERARCHY_EPIC) return 'epic';
  if (item.hierarchyLevel === HIERARCHY_SUBTASK) return 'subtask';
  const byName = LEGACY_TYPE_BY_NAME[normalize(item.name)];
  return byName === 'epic' || byName === 'subtask' ? undefined : byName;
}

/** Legacy filter key of a priority, from the default names. Custom priorities have no key. */
export function legacyPriorityKeyOf(item: Pick<CatalogItem, 'name'>): IssuePriority | undefined {
  return LEGACY_PRIORITY_BY_NAME[normalize(item.name)];
}

/** Item of the catalog that matches a legacy key, used to restore saved filters. */
export function findTypeByLegacyKey(
  items: readonly CatalogItem[],
  key: IssueType,
): CatalogItem | undefined {
  return items.find((item) => legacyTypeKeyOf(item) === key);
}

export function findPriorityByLegacyKey(
  items: readonly CatalogItem[],
  key: IssuePriority,
): CatalogItem | undefined {
  return items.find((item) => legacyPriorityKeyOf(item) === key);
}

/** Embedded reference of a catalog item, as an issue carries it. */
export function toIssueTypeRef(item: CatalogItem): IssueTypeRef {
  return {
    id: item.id,
    name: item.name,
    icon: item.icon ?? 'task',
    color: item.color ?? 'gray',
    hierarchyLevel: item.hierarchyLevel ?? 0,
  };
}

export function toPriorityRef(item: CatalogItem): PriorityRef {
  return { id: item.id, name: item.name, icon: item.icon ?? 'minus', color: item.color ?? 'gray' };
}

/** Option of a select: the item id as value and its name as label, with the palette keys for the icon. */
export interface CatalogSelectOption {
  readonly value: string;
  readonly label: string;
  readonly icon: string | null;
  readonly color: string | null;
}

export function toCatalogOption(
  item: Pick<CatalogItem, 'id' | 'name' | 'icon' | 'color'>,
): CatalogSelectOption {
  return { value: item.id, label: item.name, icon: item.icon, color: item.color };
}

const FALLBACK_PRIORITIES = new Map<string, PriorityRef>();
const FALLBACK_TYPES = new Map<string, IssueTypeRef>();

/**
 * Priority to display for a legacy key. Known keys use the default palette entry; any other value is
 * shown under its own text with a neutral icon. The same object is returned for the same key, so
 * template bindings stay stable.
 */
export function priorityDisplayOf(key: string): PriorityRef {
  const known = priorityRefFromKey(key);
  if (known) return known;
  let fallback = FALLBACK_PRIORITIES.get(key);
  if (!fallback) {
    fallback = { id: '', name: key, icon: 'circle', color: 'gray' };
    FALLBACK_PRIORITIES.set(key, fallback);
  }
  return fallback;
}

/** Issue type to display for a legacy key; see `priorityDisplayOf`. */
export function issueTypeDisplayOf(key: string): IssueTypeRef {
  const known = issueTypeRefFromKey(key);
  if (known) return known;
  let fallback = FALLBACK_TYPES.get(key);
  if (!fallback) {
    fallback = { id: '', name: key, icon: 'circle', color: 'gray', hierarchyLevel: 0 };
    FALLBACK_TYPES.set(key, fallback);
  }
  return fallback;
}
