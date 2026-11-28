import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  FilterStore,
  ISSUES_PATH,
  NotificationService,
  asApiError,
  type FilterScope,
  type SavedQuery,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import {
  DataTable,
  DataTableCell,
  type DataTableColumn,
  type RowMenuItem,
} from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { SelectButton } from 'primeng/selectbutton';

import { FilterShareDialog } from '../filter-share-dialog/filter-share-dialog';
import { FilterSubscriptionDialog } from '../filter-subscription-dialog/filter-subscription-dialog';

const SCOPES: Array<{ value: FilterScope; label: string }> = [
  { value: 'all', label: 'Todos' },
  { value: 'mine', label: 'Míos' },
  { value: 'shared', label: 'Compartidos conmigo' },
  { value: 'favorite', label: 'Favoritos' },
];

const ACCESS_LABELS: Readonly<Record<SavedQuery['access'], string>> = {
  owner: 'Propietario',
  edit: 'Puede editar',
  view: 'Solo lectura',
};

/** Manager of saved filters: open, share, subscribe and delete. */
@Component({
  selector: 'app-filters-page',
  imports: [
    Button,
    ConfirmModal,
    DataTable,
    DataTableCell,
    EmptyState,
    FilterShareDialog,
    FilterSubscriptionDialog,
    FormsModule,
    PageContainer,
    PageHeader,
    Section,
    SelectButton,
  ],
  templateUrl: './filters-page.html',
  styleUrl: './filters-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FiltersPage {
  private readonly store = inject(FilterStore);
  private readonly router = inject(Router);
  private readonly notify = inject(NotificationService);

  protected readonly scopes = SCOPES;
  protected readonly scope = signal<FilterScope>('all');
  protected readonly columns: DataTableColumn[] = [
    { key: 'name', title: 'Nombre' },
    { key: 'query', title: 'Consulta', ellipsis: true, field: 'query' },
    { key: 'owner', title: 'Propietario' },
    { key: 'access', title: 'Acceso' },
    { key: 'subscription', title: 'Suscripción' },
  ];
  protected readonly rows = computed(() => this.store.filters() as SavedQuery[]);
  protected readonly loading = this.store.list.loading;
  protected readonly error = computed(() => this.store.list.error()?.message ?? null);

  protected readonly shareTarget = signal<SavedQuery | null>(null);
  protected readonly subscriptionTarget = signal<SavedQuery | null>(null);
  protected readonly deleteTarget = signal<SavedQuery | null>(null);
  protected readonly deleting = signal(false);

  protected readonly rowActions = (filter: SavedQuery): RowMenuItem[] => {
    const actions: RowMenuItem[] = [
      {
        key: 'open',
        label: 'Abrir',
        icon: 'pi pi-external-link',
        primary: true,
        onClick: () => this.open(filter),
      },
      {
        key: 'subscription',
        label: filter.isSubscribed ? 'Editar suscripción' : 'Suscribirme',
        icon: 'pi pi-bell',
        onClick: () => this.subscriptionTarget.set(filter),
      },
    ];
    if (filter.access === 'owner') {
      actions.push(
        {
          key: 'share',
          label: 'Compartir',
          icon: 'pi pi-share-alt',
          onClick: () => this.shareTarget.set(filter),
        },
        {
          key: 'delete',
          label: 'Eliminar',
          icon: 'pi pi-trash',
          danger: true,
          onClick: () => this.deleteTarget.set(filter),
        },
      );
    }
    return actions;
  };

  constructor() {
    this.store.load('all');
  }

  protected accessLabel(filter: SavedQuery): string {
    return ACCESS_LABELS[filter.access];
  }

  protected setScope(scope: FilterScope): void {
    this.scope.set(scope);
    this.store.load(scope);
  }

  protected open(filter: SavedQuery): void {
    void this.router.navigate([ISSUES_PATH], { queryParams: { filter: filter.id } });
  }

  protected goToIssues(): void {
    void this.router.navigate([ISSUES_PATH]);
  }

  protected retry(): void {
    this.store.load(this.scope());
  }

  protected async confirmDelete(): Promise<void> {
    const filter = this.deleteTarget();
    if (!filter) return;
    this.deleting.set(true);
    try {
      await this.store.remove(filter.id);
      this.notify.success('Filtro eliminado');
      this.deleteTarget.set(null);
    } catch (error) {
      this.notify.error(asApiError(error).message);
    } finally {
      this.deleting.set(false);
    }
  }
}
