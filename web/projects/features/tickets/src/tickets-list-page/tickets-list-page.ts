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
  ISSUE_PRIORITY_LABELS,
  ISSUE_STATUS_META,
  ISSUE_TYPE_LABELS,
  IssuePriority,
  IssueStatus,
  IssueType,
  AuthStore,
  ListTicketsFilter,
  NotificationService,
  SavedFilter,
  SavedFilterApiService,
  SavedFilterCriteria,
  Ticket,
  TicketStore,
  ISSUES_PATH,
  UNASSIGNED_FILTER,
  issueKeyOf,
  issuePath,
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
import { TicketList } from '../ticket-list/ticket-list';

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
const PRIORITY_OPTIONS = (Object.keys(ISSUE_PRIORITY_LABELS) as IssuePriority[]).map((value) => ({
  value,
  label: ISSUE_PRIORITY_LABELS[value],
}));
const ISSUE_TYPE_OPTIONS = (Object.keys(ISSUE_TYPE_LABELS) as IssueType[]).map((value) => ({
  value,
  label: ISSUE_TYPE_LABELS[value],
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
  selector: 'app-tickets-list-page',
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
    TicketList,
    Toolbar,
    ToolbarLeft,
    ToolbarRight,
    ToolbarSecondary,
  ],
  templateUrl: './tickets-list-page.html',
  styleUrl: './tickets-list-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TicketsListPage {
  private readonly store = inject(TicketStore);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly auth = inject(AuthStore);
  private readonly members = injectMemberNames();
  private readonly notify = inject(NotificationService);
  private readonly savedFilterApi = inject(SavedFilterApiService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly statusFilter = signal<IssueStatus | undefined>(undefined);
  protected readonly priorityFilter = signal<IssuePriority | undefined>(undefined);
  protected readonly issueTypeFilter = signal<IssueType | undefined>(undefined);
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
  protected readonly priorityOptions = PRIORITY_OPTIONS;
  protected readonly issueTypeOptions = ISSUE_TYPE_OPTIONS;

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
    priority: this.priorityFilter(),
    issueType: this.issueTypeFilter(),
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
  protected readonly filter = computed<ListTicketsFilter>(() => ({
    status: this.statusFilter(),
    priority: this.priorityFilter(),
    issueType: this.issueTypeFilter(),
    assigneeId: this.assigneeIdForApi(),
    query: this.query(),
    dueFrom: this.dueFrom(),
    dueTo: this.dueTo(),
    page: this.page(),
    pageSize: this.pageSize(),
  }));

  protected readonly tickets = computed<Ticket[]>(() =>
    [...this.store.tickets()].sort((a, b) => a.rank.localeCompare(b.rank)),
  );
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
      options: this.priorityOptions,
      value: this.priorityFilter(),
      onChange: (value) =>
        this.changeFilter(() => this.priorityFilter.set(value as IssuePriority | undefined)),
    },
    {
      key: 'issueType',
      label: 'Filtrar por tipo',
      options: this.issueTypeOptions,
      value: this.issueTypeFilter(),
      onChange: (value) =>
        this.changeFilter(() => this.issueTypeFilter.set(value as IssueType | undefined)),
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

  protected readonly rowActions = (ticket: Ticket): RowMenuItem[] => [
    {
      key: 'open',
      label: 'Abrir incidencia',
      icon: 'pi pi-external-link',
      onClick: () => this.openIssue(ticket),
    },
    {
      key: 'copy',
      label: 'Copiar clave',
      icon: 'pi pi-copy',
      onClick: () => void this.copyKey(ticket),
    },
    {
      key: 'up',
      label: 'Mover arriba',
      icon: 'pi pi-arrow-up',
      onClick: () => this.moveTicket(ticket, -1),
    },
    {
      key: 'down',
      label: 'Mover abajo',
      icon: 'pi pi-arrow-down',
      onClick: () => this.moveTicket(ticket, 1),
    },
  ];

  constructor() {
    // The list reloads whenever a filter, the search text, the page or the page size changes.
    effect(() => {
      const filter = this.filter();
      untracked(() => this.store.fetchTickets(filter));
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

  protected openIssue(ticket: Ticket): void {
    void this.router.navigateByUrl(issuePath(ticket.id));
  }

  private async copyKey(ticket: Ticket): Promise<void> {
    try {
      await navigator.clipboard.writeText(issueKeyOf(ticket.id));
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
    this.store.fetchTickets(this.filter());
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
    this.priorityFilter.set(criteria?.priority);
    this.issueTypeFilter.set(criteria?.issueType);
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

  /** Swaps the rank of a ticket with its neighbour. Every ticket first receives a dense rank by position. */
  private moveTicket(ticket: Ticket, direction: -1 | 1): void {
    const ordered = this.tickets();
    const currentIndex = ordered.findIndex((item) => item.id === ticket.id);
    const targetIndex = currentIndex + direction;
    if (currentIndex < 0 || targetIndex < 0 || targetIndex >= ordered.length) return;

    forkJoin(ordered.map((item, index) => this.store.updateRank(item.id, rankOf(index))))
      .pipe(
        switchMap(() =>
          forkJoin([
            this.store.updateRank(ticket.id, rankOf(targetIndex)),
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
