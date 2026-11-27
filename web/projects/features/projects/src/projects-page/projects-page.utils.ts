import type {
  ProjectArchiveFilter,
  ProjectListFilters,
  ProjectSortColumn,
  ProjectTemplate,
  ProjectType,
  ProjectVisibility,
} from '@corelog/core';

export const VISIBILITY_LABELS: Record<ProjectVisibility, string> = {
  open: 'Abierto',
  private: 'Privado',
};

export const TYPE_LABELS: Record<ProjectType, string> = {
  kanban: 'Kanban',
  scrum: 'Scrum',
};

/** Value of a filter select that means "no filter". */
export const ALL = 'all';

export const DEFAULT_PAGE_SIZE = 20;

export type ProjectView = 'table' | 'cards';

/** Everything the list page lets the user change. Turned into server filters by `toListFilters`. */
export interface ProjectListState {
  readonly q: string;
  readonly template: ProjectTemplate | typeof ALL;
  readonly leadId: string;
  readonly starred: boolean;
  readonly archived: ProjectArchiveFilter;
  readonly sort: string;
  readonly page: number;
  readonly pageSize: number;
}

export const INITIAL_LIST_STATE: ProjectListState = {
  q: '',
  template: ALL,
  leadId: ALL,
  starred: false,
  archived: 'exclude',
  sort: 'name:asc',
  page: 1,
  pageSize: DEFAULT_PAGE_SIZE,
};

export interface SortOption {
  readonly value: string;
  readonly label: string;
}

export const SORT_OPTIONS: readonly SortOption[] = [
  { value: 'name:asc', label: 'Nombre (A a Z)' },
  { value: 'name:desc', label: 'Nombre (Z a A)' },
  { value: 'updated_at:desc', label: 'Actividad reciente' },
  { value: 'created_at:desc', label: 'Creados recientemente' },
  { value: 'key:asc', label: 'Clave (A a Z)' },
];

export const ARCHIVED_OPTIONS: readonly { value: ProjectArchiveFilter; label: string }[] = [
  { value: 'exclude', label: 'Solo activos' },
  { value: 'include', label: 'Activos y archivados' },
  { value: 'only', label: 'Solo archivados' },
];

/** Whether any filter differs from the default list (sorting and paging do not count). */
export function hasActiveFilters(state: ProjectListState): boolean {
  return (
    state.q.trim() !== '' ||
    state.template !== ALL ||
    state.leadId !== ALL ||
    state.starred ||
    state.archived !== 'exclude'
  );
}

/** Same filters as before, back to the defaults; keeps the chosen sort and page size. */
export function clearedFilters(state: ProjectListState): ProjectListState {
  return {
    ...INITIAL_LIST_STATE,
    sort: state.sort,
    pageSize: state.pageSize,
  };
}

export function toListFilters(state: ProjectListState): ProjectListFilters {
  const [column, direction] = state.sort.split(':');
  const filters: ProjectListFilters = {
    archived: state.archived,
    sort: column as ProjectSortColumn,
    desc: direction === 'desc',
    page: state.page,
    pageSize: state.pageSize,
  };
  const q = state.q.trim();
  if (q) filters.q = q;
  if (state.template !== ALL) filters.template = state.template;
  if (state.leadId !== ALL) filters.leadId = state.leadId;
  if (state.starred) filters.starred = true;
  return filters;
}

export { createProjectRights, type CreateProjectRights } from '@corelog/core';
