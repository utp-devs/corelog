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
import {
  BasicFilters,
  CoreqlProblem,
  FilterStore,
  ISSUES_PATH,
  IssueSearchPage,
  NotificationService,
  OrganizationStore,
  SearchQueryApiService,
  AsyncState,
  asApiError,
  basicFilterChips,
  basicFiltersToJql,
  EMPTY_BASIC_FILTERS,
  formatOrder,
  hasBasicFilters,
  issueKeyOf,
  issuePath,
  parseOrder,
  removeChip,
  splitOrderBy,
  withOrderBy,
  type QueryChip,
  type SavedQuery,
  type Issue,
  type SortSpec,
} from '@corelog/core';
import { SaveFilterDialog, type SaveFilterRequest } from '@corelog/features/filters';
import { FilterSubscriptionDialog } from '@corelog/features/filters';
import { BulkAction } from '@corelog/ui/bulk-bar';
import {
  CoreqlInput,
  type CoreqlSuggestProvider,
  type CoreqlSuggestion,
} from '@corelog/ui/coreql-input';
import { DataTablePageChange, DataTablePagination, RowMenuItem } from '@corelog/ui/data-table';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { QueryChips } from '@corelog/ui/query-chips';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { MultiSelect } from 'primeng/multiselect';
import { Select } from 'primeng/select';
import { SelectButton } from 'primeng/selectbutton';
import { Subject, debounceTime, forkJoin, map, of, switchMap, catchError } from 'rxjs';

import { BasicFiltersPanel } from '../issue-navigator/basic-filters';
import { FiltersSidebar } from '../issue-navigator/filters-sidebar';
import {
  BASIC_PARAM_NAMES,
  DEFAULT_ORDER,
  NAVIGATOR_COLUMNS,
  NavigatorMode,
  SORT_FIELDS,
  csvCell,
  filtersToParams,
  orderedColumns,
  parseColumns,
  paramsToFilters,
} from '../issue-navigator/navigator-state';
import { IssueList } from '../issue-list/issue-list';
import { BulkKind, IssueBulkDialog } from '../issue-bulk/issue-bulk-dialog';
import { IssueStatusDialog } from '../status-change/issue-status-dialog';

const DEFAULT_PAGE_SIZE = 50;
/** Delay between the last keystroke of the advanced query and its validation, in milliseconds. */
export const VALIDATE_DEBOUNCE_MS = 400;

const MODES: Array<{ value: NavigatorMode; label: string }> = [
  { value: 'basic', label: 'Básico' },
  { value: 'advanced', label: 'Avanzado' },
];

/** Basic parameters cleared when the query moves to the advanced text, and the other way around. */
const CLEAR_BASIC: Readonly<Record<string, null>> = Object.fromEntries(
  BASIC_PARAM_NAMES.map((name) => [name, null]),
);

/**
 * Issue navigator: basic filters or a CoreQL query, results with a column chooser, sorting, pagination,
 * selection, export and saved filters. The state lives in the address so a reload keeps the search.
 */
@Component({
  selector: 'app-issues-list-page',
  imports: [
    BasicFiltersPanel,
    Button,
    CoreqlInput,
    Dialog,
    EmptyState,
    FilterSubscriptionDialog,
    FiltersSidebar,
    FormsModule,
    IssueList,
    IssueBulkDialog,
    IssueStatusDialog,
    Message,
    MultiSelect,
    PageContainer,
    PageHeader,
    QueryChips,
    SaveFilterDialog,
    Section,
    Select,
    SelectButton,
  ],
  templateUrl: './issues-list-page.html',
  styleUrl: './issues-list-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssuesListPage {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly api = inject(SearchQueryApiService);
  private readonly filterStore = inject(FilterStore);
  private readonly organizations = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly validateRequests = new Subject<string>();

  private readonly params = toSignal(this.route.queryParamMap, {
    initialValue: this.route.snapshot.queryParamMap,
  });
  private readonly results = new AsyncState<IssueSearchPage>();
  private lastRunKey = '';
  private loadedFilterId: string | null = null;

  protected readonly modes = MODES;
  protected readonly sortFields = [...SORT_FIELDS];
  protected readonly columnOptions = [...NAVIGATOR_COLUMNS];

  // State kept in the address.
  protected readonly mode = computed<NavigatorMode>(() =>
    this.params().has('jql') || this.params().get('mode') === 'advanced' ? 'advanced' : 'basic',
  );
  protected readonly filters = computed<BasicFilters>(() =>
    paramsToFilters(
      Object.fromEntries(this.params().keys.map((key) => [key, this.params().get(key)])),
    ),
  );
  private readonly appliedJql = computed(() => this.params().get('jql') ?? '');
  private readonly order = computed(() => this.params().get('order') ?? DEFAULT_ORDER);
  protected readonly page = computed(() => Math.max(1, Number(this.params().get('page')) || 1));
  protected readonly pageSize = computed(
    () => Number(this.params().get('size')) || DEFAULT_PAGE_SIZE,
  );
  protected readonly columnKeys = computed(() => parseColumns(this.params().get('cols')));
  protected readonly filterId = computed(() => this.params().get('filter'));

  // Advanced editor.
  protected readonly draft = signal('');
  protected readonly problems = signal<readonly CoreqlProblem[]>([]);
  protected readonly switchWarningOpen = signal(false);

  // Saved filters.
  protected readonly savedFilters = this.filterStore.filters;
  protected readonly savedLoading = this.filterStore.list.loading;
  protected readonly savedError = computed(() => this.filterStore.list.error()?.message ?? null);
  protected readonly currentFilter = computed(() => this.filterStore.find(this.filterId()));
  protected readonly editableFilter = computed(() => {
    const filter = this.currentFilter();
    return filter && filter.access !== 'view' ? filter : null;
  });
  protected readonly saveOpen = signal(false);
  protected readonly saveBusy = signal(false);
  protected readonly saveError = signal<string | null>(null);
  protected readonly subscriptionTarget = signal<SavedQuery | null>(null);
  protected readonly sidebarOpen = signal(false);

  // Results.
  protected readonly issues = computed<Issue[]>(() => [...(this.results.value()?.items ?? [])]);
  protected readonly total = computed(() => this.results.value()?.total ?? 0);
  protected readonly warnings = computed(() => this.results.value()?.warnings ?? []);
  protected readonly loading = this.results.loading;
  protected readonly runError = computed(() => {
    const error = this.results.error();
    if (!error) return null;
    return error.code.startsWith('search.')
      ? error.message
      : 'No se pudieron cargar las incidencias.';
  });
  protected readonly queryError = computed(() => this.runError());
  /** Errors that are not about the query itself (network, server): the list offers a retry. */
  protected readonly loadError = computed(() => {
    const error = this.results.error();
    return error && !error.code.startsWith('search.')
      ? 'No se pudieron cargar las incidencias.'
      : null;
  });
  /** The advanced editor already lists the problems of the query; the banner is for the rest. */
  protected readonly showQueryBanner = computed(
    () => this.queryError() !== null && !(this.mode() === 'advanced' && this.problems().length > 0),
  );
  protected readonly pagination = computed<DataTablePagination | null>(() =>
    this.total() > 0 ? { page: this.page(), pageSize: this.pageSize(), total: this.total() } : null,
  );
  protected readonly selectedKeys = signal<string[]>([]);
  protected readonly bulkKind = signal<BulkKind | null>(null);
  protected readonly statusIssues = signal<readonly Issue[]>([]);
  protected readonly statusOpen = signal(false);

  // Derived views of the query.
  protected readonly effectiveJql = computed(() =>
    this.mode() === 'advanced' ? this.appliedJql() : basicFiltersToJql(this.filters()),
  );
  protected readonly effectiveOrder = computed(() =>
    this.mode() === 'advanced' ? '' : this.order(),
  );
  protected readonly chips = computed(() => basicFilterChips(this.filters()));
  protected readonly hasQuery = computed(() =>
    this.mode() === 'advanced' ? this.appliedJql().trim() !== '' : hasBasicFilters(this.filters()),
  );
  protected readonly sort = computed<SortSpec | null>(() => {
    const order = this.mode() === 'advanced' ? splitOrderBy(this.appliedJql()).order : this.order();
    return parseOrder(order)[0] ?? null;
  });
  protected readonly visibleColumns = computed(() => orderedColumns(this.columnKeys()));
  protected readonly selectedColumns = computed(() => this.columnKeys());

  protected readonly bulkActions = computed<BulkAction[]>(() => [
    {
      key: 'status',
      label: 'Cambiar estado',
      icon: 'pi pi-arrow-right-arrow-left',
      onClick: () => {
        const keys = new Set(this.selectedKeys());
        this.openStatusDialog(this.issues().filter((issue) => keys.has(issue.id)));
      },
    },
    {
      key: 'assign',
      label: 'Asignar',
      icon: 'pi pi-user',
      onClick: () => this.bulkKind.set('assign'),
    },
    {
      key: 'priority',
      label: 'Prioridad',
      icon: 'pi pi-flag',
      onClick: () => this.bulkKind.set('set_priority'),
    },
    {
      key: 'move',
      label: 'Mover',
      icon: 'pi pi-arrow-right',
      onClick: () => this.bulkKind.set('move'),
    },
    {
      key: 'delete',
      label: 'Eliminar',
      icon: 'pi pi-trash',
      danger: true,
      onClick: () => this.bulkKind.set('delete'),
    },
    {
      key: 'export',
      label: 'Exportar selección',
      icon: 'pi pi-download',
      onClick: () => this.exportCsv(true),
    },
  ]);

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
      key: 'status',
      label: 'Cambiar estado',
      icon: 'pi pi-arrow-right-arrow-left',
      onClick: () => this.openStatusDialog([issue]),
    },
  ];

  /** Autocomplete of the advanced editor: values of the field being written, or completions at the cursor. */
  protected readonly provider: CoreqlSuggestProvider = ({ text, cursor, context }) => {
    const completions = this.api.complete(text, cursor, 10);
    if (context.kind === 'value' && context.field) {
      return forkJoin([this.api.values(context.field, context.prefix, 10), completions]).pipe(
        map(([found, rest]) => [
          ...found.values.map((option): CoreqlSuggestion => ({
            label:
              option.value === option.label ? option.label : `${option.label} (${option.value})`,
            insert: option.value,
            kind: 'value',
            detail: option.detail && option.detail !== option.value ? option.detail : 'Valor',
          })),
          ...rest.completions
            .filter((item) => item.kind === 'function')
            .map((item) => ({
              label: item.label,
              insert: item.label,
              kind: 'function',
              detail: 'Función',
            })),
        ]),
      );
    }
    return completions.pipe(
      map((found) =>
        found.completions.map((item) => ({
          label: item.label,
          insert: item.label,
          kind: item.kind,
          detail: item.detail,
        })),
      ),
    );
  };

  constructor() {
    this.filterStore.load('all');

    // The editor follows the address (a saved filter, the browser buttons).
    effect(() => {
      const applied = this.appliedJql();
      untracked(() => {
        if (this.draft() !== applied) this.draft.set(applied);
      });
    });

    // Runs the query whenever what is searched, the page or the page size changes.
    effect(() => {
      const request = {
        jql: this.effectiveJql(),
        order: this.effectiveOrder(),
        page: this.page(),
        pageSize: this.pageSize(),
      };
      untracked(() => this.run(request));
    });

    // A saved filter in the address is loaded into the advanced editor.
    effect(() => {
      const id = this.filterId();
      const known = this.savedFilters();
      untracked(() => this.loadSavedFilter(id, known));
    });

    this.validateRequests
      .pipe(
        debounceTime(VALIDATE_DEBOUNCE_MS),
        switchMap((text) =>
          text.trim() === '' ? of(null) : this.api.validate(text).pipe(catchError(() => of(null))),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((result) => this.problems.set(result?.errors ?? []));
  }

  // Mode.
  protected setMode(mode: NavigatorMode): void {
    if (mode === this.mode()) return;
    if (mode === 'advanced') {
      const jql = withOrderBy(basicFiltersToJql(this.filters()), this.order());
      this.go({ ...CLEAR_BASIC, mode: 'advanced', jql, order: null, page: null, filter: null });
    } else if (this.appliedJql().trim() === '') {
      this.go({ mode: null, jql: null, page: null, filter: null });
    } else {
      this.switchWarningOpen.set(true);
    }
  }

  protected confirmBasicMode(): void {
    this.switchWarningOpen.set(false);
    this.problems.set([]);
    this.go({ mode: null, jql: null, order: null, page: null, filter: null });
  }

  // Basic filters.
  protected setFilters(filters: BasicFilters): void {
    this.go({ ...filtersToParams(filters), page: null, filter: null });
  }

  protected removeFilterChip(chip: QueryChip): void {
    this.setFilters(removeChip(this.filters(), chip));
  }

  protected clearFilters(): void {
    this.setFilters(EMPTY_BASIC_FILTERS);
  }

  // Advanced query.
  protected onDraft(text: string): void {
    this.draft.set(text);
    this.validateRequests.next(text);
  }

  protected runAdvanced(): void {
    const text = this.draft().trim();
    this.go({ jql: text, mode: 'advanced', page: null, filter: this.filterId() });
    if (text === this.appliedJql().trim()) this.refresh();
  }

  // Results.
  protected setSortField(field: string | null): void {
    if (!field) return;
    this.applySort(field, this.sort()?.direction ?? 'DESC');
  }

  protected toggleSortDirection(): void {
    const sort = this.sort();
    this.applySort(sort?.field ?? 'updated', sort?.direction === 'ASC' ? 'DESC' : 'ASC');
  }

  protected setColumns(keys: string[] | null): void {
    const chosen = keys && keys.length > 0 ? keys : ['summary'];
    this.go({ cols: chosen.join(',') });
  }

  protected changePage(change: DataTablePageChange): void {
    this.go({
      page: change.page > 1 ? String(change.page) : null,
      size: change.pageSize !== DEFAULT_PAGE_SIZE ? String(change.pageSize) : null,
    });
  }

  protected refresh(): void {
    this.lastRunKey = '';
    this.run({
      jql: this.effectiveJql(),
      order: this.effectiveOrder(),
      page: this.page(),
      pageSize: this.pageSize(),
    });
  }

  protected openIssue(issue: Issue): void {
    void this.router.navigateByUrl(issuePath(issue.key));
  }

  protected openNew(): void {
    void this.router.navigateByUrl(`${ISSUES_PATH}/new`);
  }

  protected openStatusDialog(issues: readonly Issue[]): void {
    if (issues.length === 0) return;
    this.statusIssues.set(issues);
    this.statusOpen.set(true);
  }

  /** A bulk change finished for some issues: the list is reloaded and the selection cleared. */
  protected onBulkDone(): void {
    this.selectedKeys.set([]);
    this.refresh();
  }

  protected onStatusChanged(): void {
    this.selectedKeys.set([]);
    this.refresh();
  }

  /** Downloads the rows of the page (or the ticked ones) as a CSV file with the visible columns. */
  protected exportCsv(onlySelected = false): void {
    const keys = new Set(this.selectedKeys());
    const rows = this.issues().filter((issue) => !onlySelected || keys.has(issue.id));
    const columns = this.visibleColumns();
    const titles = new Map(NAVIGATOR_COLUMNS.map((column) => [column.key, column.title]));
    const lines = [
      columns.map((key) => csvCell(titles.get(key) ?? key)).join(','),
      ...rows.map((issue) => columns.map((key) => csvCell(this.cellText(issue, key))).join(',')),
    ];
    try {
      const blob = new Blob([`﻿${lines.join('\r\n')}\r\n`], { type: 'text/csv;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `issues-${new Date().toISOString().slice(0, 10)}.csv`;
      link.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      this.notify.error(error);
    }
  }

  // Saved filters.
  protected selectSaved(filter: SavedQuery): void {
    this.sidebarOpen.set(false);
    this.loadedFilterId = null;
    this.go({ filter: filter.id });
    this.loadSavedFilter(filter.id, [filter]);
  }

  protected openSave(): void {
    this.saveError.set(null);
    this.saveOpen.set(true);
  }

  protected async save(request: SaveFilterRequest): Promise<void> {
    this.saveBusy.set(true);
    this.saveError.set(null);
    const query = withOrderBy(this.effectiveJql(), this.mode() === 'advanced' ? '' : this.order());
    const sort = this.sort();
    const payload = {
      name: request.name,
      description: request.description,
      query: this.mode() === 'advanced' ? this.appliedJql() : query,
      columns: this.columnKeys(),
      sort: sort ? formatOrder([sort]) : '',
    };
    const current = this.editableFilter();
    try {
      const saved =
        request.replace && current
          ? await this.filterStore.update(current.id, payload)
          : await this.filterStore.save({
              ...payload,
              organizationId: this.organizations.currentOrgId() ?? undefined,
            });
      this.loadedFilterId = saved.id;
      this.saveOpen.set(false);
      this.notify.success('Filtro guardado');
      this.go({
        mode: 'advanced',
        jql: saved.query,
        filter: saved.id,
        order: null,
        page: null,
        ...CLEAR_BASIC,
      });
    } catch (error) {
      this.saveError.set(asApiError(error).message);
    } finally {
      this.saveBusy.set(false);
    }
  }

  protected toggleSubscription(filter: SavedQuery): void {
    this.subscriptionTarget.set(filter);
  }

  private applySort(field: string, direction: 'ASC' | 'DESC'): void {
    const order = formatOrder([{ field, direction }]);
    if (this.mode() === 'advanced') {
      this.go({ jql: withOrderBy(this.appliedJql(), order), page: null });
    } else {
      this.go({ order: order === DEFAULT_ORDER ? null : order, page: null });
    }
  }

  private loadSavedFilter(id: string | null, known: readonly SavedQuery[]): void {
    if (!id) {
      this.loadedFilterId = null;
      return;
    }
    if (this.loadedFilterId === id) return;
    const filter = known.find((item) => item.id === id);
    if (!filter) return;
    this.loadedFilterId = id;
    const jql =
      filter.sort && !splitOrderBy(filter.query).order
        ? withOrderBy(filter.query, filter.sort)
        : filter.query;
    this.go({
      ...CLEAR_BASIC,
      mode: 'advanced',
      jql,
      order: null,
      cols: filter.columns.length > 0 ? filter.columns.join(',') : null,
      page: null,
    });
  }

  private run(request: { jql: string; order: string; page: number; pageSize: number }): void {
    const key = JSON.stringify(request);
    if (key === this.lastRunKey) return;
    this.lastRunKey = key;
    this.selectedKeys.set([]);
    this.results.load(
      this.api.searchIssues({
        jql: request.jql,
        order: request.order || undefined,
        page: request.page,
        pageSize: request.pageSize,
      }),
    );
    // A rejected query shows its problems (position, message, fixes) next to the editor.
    if (this.mode() === 'advanced' && request.jql.trim() !== '')
      this.validateRequests.next(request.jql);
  }

  /** Merges parameters into the address, replacing the history entry so Back does not step through every edit. */
  private go(changes: Record<string, string | null>): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: changes,
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  private async copyKey(issue: Issue): Promise<void> {
    try {
      await navigator.clipboard.writeText(issueKeyOf(issue));
    } catch {
      return;
    }
    this.notify.success('Clave copiada');
  }

  private cellText(issue: Issue, key: string): string {
    switch (key) {
      case 'key':
        return issueKeyOf(issue);
      case 'summary':
        return issue.summary;
      case 'type':
        return issue.issueType.name;
      case 'status':
        return issue.workflowStatus?.name ?? issue.status;
      case 'priority':
        return issue.priority.name;
      case 'assignee':
        return issue.assigneeName ?? '';
      case 'reporter':
        return issue.reporterName;
      case 'project':
        return issue.projectKey;
      case 'labels':
        return ((issue as { labels?: readonly string[] }).labels ?? []).join('; ');
      case 'points':
        return issue.storyPoints === null ? '' : String(issue.storyPoints);
      case 'due':
        return issue.dueDate ?? '';
      case 'created':
        return issue.createdAt;
      case 'updated':
        return issue.updatedAt;
      default:
        return '';
    }
  }
}
