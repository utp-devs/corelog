import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { Button } from 'primeng/button';
import { DatePicker } from 'primeng/datepicker';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import { forkJoin, switchMap } from 'rxjs';
import {
  ISSUE_STATUS_META,
  IssueStatus,
  AuthStore,
  CatalogStore,
  ListIssuesFilter,
  NotificationService,
  SavedFilter,
  SavedFilterApiService,
  SavedFilterCriteria,
  Issue,
  IssueStore,
  ISSUES_PATH,
  UNASSIGNED_FILTER,
  findPriorityByLegacyKey,
  findTypeByLegacyKey,
  issueKeyOf,
  issuePath,
  legacyPriorityKeyOf,
  legacyTypeKeyOf,
} from '@corelog/core';
import { DataTablePageChange, DataTablePagination, RowMenuItem } from '@corelog/ui/data-table';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { FilterBar, FilterDefinition } from '@corelog/ui/filter-bar';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { Toolbar, ToolbarLeft, ToolbarRight, ToolbarSecondary } from '@corelog/ui/toolbar';

import { injectMemberNames } from '../hooks/member-names';
import { IssueList } from '../issue-list/issue-list';

const DEFAULT_PAGE_SIZE = 20;
const SAVED_FILTER_NAME_MAX = 80;
const NEW_ISSUE_PATH = `${ISSUES_PATH}/new`;
/** Query parameter that keeps the assignee filter in the address. */
const ASSIGNEE_PARAM = 'assignee';
/** Value of the assignee filter for the issues assigned to the signed-in user. */
const ASSIGNED_TO_ME = 'me';

const STATUS_OPTIONS = (Object.keys(ISSUE_STATUS_META) as IssueStatus[]).map((value) => ({
  value,
  label: ISSUE_STATUS_META[value].label,
}));

/** Padded rank, so that ranks sort as strings in the same order as the list positions. */
function rankOf(index: number): string {
  return index.toString().padStart(8, '0');
}

/** Start of the local day of a date, as an ISO string. */
function startOfDayIso(date: Date): string {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy.toISOString();
}

/** End of the local day of a date, as an ISO string. */
function endOfDayIso(date: Date): string {
  const copy = new Date(date);
  copy.setHours(23, 59, 59, 999);
  return copy.toISOString();
}

/** Issue list with filters, server-side pagination, saved filters and manual ordering. */
@Component({
  selector: 'app-issues-list-page',
  imports: [
    Button,
    DatePicker,
    Dialog,
    EmptyState,
    FilterBar,
    FormsModule,
    InputText,
    Message,
    PageContainer,
    PageHeader,
    Section,
    Select,
    IssueList,
    Toolbar,
    ToolbarLeft,
    ToolbarRight,
    ToolbarSecondary,
  ],
  templateUrl: './issues-list-page.html',
  styleUrl: './issues-list-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssuesListPage {
  private readonly store = inject(IssueStore);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly auth = inject(AuthStore);
  private readonly members = injectMemberNames();
  private readonly notify = inject(NotificationService);
  private readonly savedFilterApi = inject(SavedFilterApiService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly catalog = inject(CatalogStore);

  protected readonly statusFilter = signal<IssueStatus | undefined>(undefined);
  /** Catalog id of the selected priority and type. */
  protected readonly priorityFilter = signal<string | undefined>(undefined);
  protected readonly issueTypeFilter = signal<string | undefined>(undefined);
  /** Kept in the address (`?assignee=`): `me`, `none` or a member id. */
  private readonly queryParams = toSignal(this.route.queryParamMap, {
    initialValue: this.route.snapshot.queryParamMap,
  });
  protected readonly assigneeFilter = computed(
    () => this.queryParams().get(ASSIGNEE_PARAM) || undefined,
  );
  protected readonly dueFrom = signal<string | undefined>(undefined);
  protected readonly dueTo = signal<string | undefined>(undefined);
  protected readonly page = signal(1);
  protected readonly pageSize = signal(DEFAULT_PAGE_SIZE);
  protected readonly search = signal('');

  protected readonly advancedOpen = signal(false);
  protected readonly savedFilters = signal<SavedFilter[]>([]);
  protected readonly selectedSavedFilter = signal<string | undefined>(undefined);
  protected readonly savedFilterName = signal('');
  protected readonly saveDialogOpen = signal(false);
  protected readonly savedFilterError = signal<string | undefined>(undefined);

  protected readonly statusOptions = STATUS_OPTIONS;
  protected readonly priorityOptions = computed(() =>
    this.catalog.priorities().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly issueTypeOptions = computed(() =>
    this.catalog.issueTypes().map((item) => ({ value: item.id, label: item.name })),
  );
  /**
   * The list endpoint filters by the legacy keys. A selected catalog item narrows the request through its
   * legacy key when it has one, and the loaded page is always filtered by the exact item id, so renamed
   * or custom items and shared legacy keys still show the right rows (totals may count the extras).
   */
  private readonly priorityLegacyKey = computed(() => {
    const item = this.catalog.priorities().find((entry) => entry.id === this.priorityFilter());
    return item ? legacyPriorityKeyOf(item) : undefined;
  });
  private readonly typeLegacyKey = computed(() => {
    const item = this.catalog.issueTypes().find((entry) => entry.id === this.issueTypeFilter());
    return item ? legacyTypeKeyOf(item) : undefined;
  });

  protected readonly query = computed(() => this.search().trim() || undefined);
  protected readonly hasActiveFilters = computed(() =>
    Boolean(
      this.search().trim() ||
      this.statusFilter() ||
      this.priorityFilter() ||
      this.issueTypeFilter() ||
      this.assigneeFilter() ||
      this.dueFrom() ||
      this.dueTo(),
    ),
  );
  protected readonly advancedVisible = computed(
    () =>
      this.advancedOpen() || Boolean(this.dueFrom() || this.dueTo() || this.selectedSavedFilter()),
  );
  protected readonly criteria = computed<SavedFilterCriteria>(() => ({
    status: this.statusFilter(),
    priority: this.priorityLegacyKey(),
    issueType: this.typeLegacyKey(),
    query: this.query(),
    dueFrom: this.dueFrom(),
    dueTo: this.dueTo(),
  }));
  /** The value sent to the API: `me` becomes the signed-in user's id. */
  private readonly assigneeIdForApi = computed(() => {
    const value = this.assigneeFilter();
    return value === ASSIGNED_TO_ME ? this.auth.user()?.id : value;
  });
  private readonly assigneeOptions = computed(() => [
    { value: ASSIGNED_TO_ME, label: 'Asignadas a mí' },
    { value: UNASSIGNED_FILTER, label: 'Sin asignar' },
    ...this.members.members().map((member) => ({ value: member.id, label: member.name })),
  ]);
  protected readonly filter = computed<ListIssuesFilter>(() => ({
    status: this.statusFilter(),
    priority: this.priorityLegacyKey(),
    issueType: this.typeLegacyKey(),
    assigneeId: this.assigneeIdForApi(),
    query: this.query(),
    dueFrom: this.dueFrom(),
    dueTo: this.dueTo(),
    page: this.page(),
    pageSize: this.pageSize(),
  }));

  protected readonly issues = computed<Issue[]>(() => {
    const priorityId = this.priorityFilter();
    const typeId = this.issueTypeFilter();
    return this.store
      .issues()
      .filter(
        (issue) =>
          (!priorityId || issue.priority.id === priorityId) &&
          (!typeId || issue.issueType.id === typeId),
      )
      .sort((a, b) => a.rank.localeCompare(b.rank));
  });
  protected readonly loading = this.store.list.loading;
  protected readonly error = computed(() => this.store.list.error()?.message ?? null);
  protected readonly pagination = computed<DataTablePagination | null>(() =>
    this.store.total() > 0
      ? { page: this.page(), pageSize: this.pageSize(), total: this.store.total() }
      : null,
  );

  protected readonly filterDefinitions = computed<FilterDefinition[]>(() => [
    {
      key: 'status',
      label: 'Filtrar por estado',
      options: this.statusOptions,
      value: this.statusFilter(),
      onChange: (value) =>
        this.changeFilter(() => this.statusFilter.set(value as IssueStatus | undefined)),
    },
    {
      key: 'priority',
      label: 'Filtrar por prioridad',
      options: this.priorityOptions(),
      value: this.priorityFilter(),
      onChange: (value) => this.changeFilter(() => this.priorityFilter.set(value)),
    },
    {
      key: 'issueType',
      label: 'Filtrar por tipo',
      options: this.issueTypeOptions(),
      value: this.issueTypeFilter(),
      onChange: (value) => this.changeFilter(() => this.issueTypeFilter.set(value)),
    },
    {
      key: 'assignee',
      label: 'Filtrar por asignado',
      options: this.assigneeOptions(),
      value: this.assigneeFilter(),
      onChange: (value) => this.changeFilter(() => this.setAssignee(value)),
    },
  ]);

  protected readonly savedFilterOptions = computed(() =>
    this.savedFilters().map((filter) => ({ value: filter.id, label: filter.name })),
  );
  protected readonly dueFromDate = computed(() => this.toDate(this.dueFrom()));
  protected readonly dueToDate = computed(() => this.toDate(this.dueTo()));
  protected readonly canSaveName = computed(() => {
    const name = this.savedFilterName();
    return name.trim().length > 0 && name.length <= SAVED_FILTER_NAME_MAX;
  });

  protected readonly rowActions = (issue: Issue): RowMenuItem[] => [
    {
      key: 'open',
      label: 'Abrir incidencia',
      icon: 'pi pi-external-link',
      onClick: () => this.openIssue(issue),
    },
    {
      key: 'copy',
      label: 'Copiar clave',
      icon: 'pi pi-copy',
      onClick: () => void this.copyKey(issue),
    },
    {
      key: 'up',
      label: 'Mover arriba',
      icon: 'pi pi-arrow-up',
      onClick: () => this.moveIssue(issue, -1),
    },
    {
      key: 'down',
      label: 'Mover abajo',
      icon: 'pi pi-arrow-down',
      onClick: () => this.moveIssue(issue, 1),
    },
  ];

  constructor() {
    this.catalog.ensureLoaded();
    // The list reloads whenever a filter, the search text, the page or the page size changes.
    effect(() => {
      const filter = this.filter();
      untracked(() => this.store.fetchIssues(filter));
    });

    this.savedFilterApi
      .list()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (filters) => this.savedFilters.set(filters),
        error: () => this.savedFilterError.set('No se pudieron cargar los filtros guardados.'),
      });
  }

  protected onSearch(value: string): void {
    this.search.set(value);
    this.page.set(1);
  }

  protected openIssue(issue: Issue): void {
    void this.router.navigateByUrl(issuePath(issue.key));
  }

  private async copyKey(issue: Issue): Promise<void> {
    try {
      await navigator.clipboard.writeText(issueKeyOf(issue));
    } catch {
      return;
    }
    this.notify.success('Clave copiada');
  }

  protected openNew(): void {
    void this.router.navigateByUrl(NEW_ISSUE_PATH);
  }

  protected changePage(change: DataTablePageChange): void {
    this.page.set(change.page);
    this.pageSize.set(change.pageSize);
  }

  protected retry(): void {
    this.store.fetchIssues(this.filter());
  }

  protected toggleAdvanced(): void {
    this.advancedOpen.update((open) => !open);
  }

  protected onDueFrom(value: Date | null): void {
    this.changeFilter(() => this.dueFrom.set(value ? startOfDayIso(value) : undefined));
  }

  protected onDueTo(value: Date | null): void {
    this.changeFilter(() => this.dueTo.set(value ? endOfDayIso(value) : undefined));
  }

  protected clearFilters(): void {
    this.search.set('');
    this.statusFilter.set(undefined);
    this.priorityFilter.set(undefined);
    this.issueTypeFilter.set(undefined);
    this.setAssignee(undefined);
    this.dueFrom.set(undefined);
    this.dueTo.set(undefined);
    this.selectedSavedFilter.set(undefined);
    this.page.set(1);
  }

  /** Loads a saved filter's criteria into the issue filters. Clearing the selection resets the filters. */
  protected applySavedFilter(filterId: string | null): void {
    const id = filterId ?? undefined;
    this.selectedSavedFilter.set(id);
    const criteria = this.savedFilters().find((filter) => filter.id === id)?.criteria;
    this.statusFilter.set(criteria?.status);
    this.priorityFilter.set(
      criteria?.priority
        ? findPriorityByLegacyKey(this.catalog.priorities(), criteria.priority)?.id
        : undefined,
    );
    this.issueTypeFilter.set(
      criteria?.issueType
        ? findTypeByLegacyKey(this.catalog.issueTypes(), criteria.issueType)?.id
        : undefined,
    );
    this.search.set(criteria?.query ?? '');
    this.dueFrom.set(criteria?.dueFrom);
    this.dueTo.set(criteria?.dueTo);
    this.page.set(1);
  }

  protected openSaveDialog(): void {
    this.saveDialogOpen.set(true);
  }

  protected closeSaveDialog(): void {
    this.saveDialogOpen.set(false);
  }

  protected onSavedFilterName(value: string): void {
    this.savedFilterName.set(value);
  }

  protected saveCurrentFilter(): void {
    if (!this.canSaveName()) return;
    this.savedFilterApi.create(this.savedFilterName(), this.criteria()).subscribe({
      next: (filter) => {
        this.savedFilters.update((current) =>
          [...current, filter].sort((a, b) => a.name.localeCompare(b.name)),
        );
        this.selectedSavedFilter.set(filter.id);
        this.savedFilterName.set('');
        this.saveDialogOpen.set(false);
        this.savedFilterError.set(undefined);
        this.notify.success('Filtro guardado');
      },
      error: () =>
        this.savedFilterError.set(
          'No se pudo guardar el filtro. Comprueba que el nombre no esté repetido.',
        ),
    });
  }

  protected deleteSelectedFilter(): void {
    const id = this.selectedSavedFilter();
    if (!id) return;
    this.savedFilterApi.delete(id).subscribe({
      next: () => {
        this.savedFilters.update((current) => current.filter((filter) => filter.id !== id));
        this.selectedSavedFilter.set(undefined);
        this.savedFilterError.set(undefined);
        this.notify.success('Filtro eliminado');
      },
      error: () => this.savedFilterError.set('No se pudo eliminar el filtro guardado.'),
    });
  }

  protected dismissSavedFilterError(): void {
    this.savedFilterError.set(undefined);
  }

  /** Swaps the rank of a issue with its neighbour. Every issue first receives a dense rank by position. */
  private moveIssue(issue: Issue, direction: -1 | 1): void {
    const ordered = this.issues();
    const currentIndex = ordered.findIndex((item) => item.id === issue.id);
    const targetIndex = currentIndex + direction;
    if (currentIndex < 0 || targetIndex < 0 || targetIndex >= ordered.length) return;

    forkJoin(ordered.map((item, index) => this.store.updateRank(item.id, rankOf(index))))
      .pipe(
        switchMap(() =>
          forkJoin([
            this.store.updateRank(issue.id, rankOf(targetIndex)),
            this.store.updateRank(ordered[targetIndex].id, rankOf(currentIndex)),
          ]),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({ error: () => this.notify.error('No se pudo actualizar la incidencia') });
  }

  /** Writes the assignee filter to the address, replacing the history entry so Back does not step through filters. */
  private setAssignee(value: string | undefined): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { [ASSIGNEE_PARAM]: value ?? null },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  private changeFilter(apply: () => void): void {
    apply();
    this.page.set(1);
  }

  private toDate(value: string | undefined): Date | null {
    return value ? new Date(value) : null;
  }
}
