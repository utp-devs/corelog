import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import {
  ApiError,
  Dashboard,
  DashboardScope,
  DashboardShareInput,
  DashboardV2ApiService,
  DashboardV2Store,
  NotificationService,
} from '@corelog/core';
import { ConfirmDangerModal } from '@corelog/ui/confirm-danger-modal';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { SearchInput } from '@corelog/ui/search-input';
import { AppTooltip } from '@corelog/ui/tooltip';
import type { MenuItem } from 'primeng/api';
import { Menu } from 'primeng/menu';

import { DashboardFormDialog, DashboardFormValue } from '../form-dialog/dashboard-form-dialog';
import { DashboardShareDialog } from '../share-dialog/share-dialog';

type ManagerTab = 'all' | 'mine' | 'shared' | 'favorites';

interface TabChoice {
  readonly value: ManagerTab;
  readonly label: string;
}

const TABS: readonly TabChoice[] = [
  { value: 'all', label: 'Todos' },
  { value: 'mine', label: 'Míos' },
  { value: 'shared', label: 'Compartidos conmigo' },
  { value: 'favorites', label: 'Favoritos' },
];

const DATE = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short', year: 'numeric' });

function messageOf(error: unknown, fallback: string): string {
  const message = (error as Partial<ApiError> | null)?.message;
  return message && message.trim() ? message : fallback;
}

/** Manager of the dashboards the person can see: create, rename, duplicate, share, favorites and delete. */
@Component({
  selector: 'app-dashboards-page',
  imports: [
    AppTooltip,
    ConfirmDangerModal,
    DashboardFormDialog,
    DashboardShareDialog,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    Menu,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    RouterLink,
    SearchInput,
  ],
  templateUrl: './dashboards-page.html',
  styleUrl: './dashboards-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardsPage {
  protected readonly store = inject(DashboardV2Store);
  private readonly api = inject(DashboardV2ApiService);
  private readonly router = inject(Router);
  private readonly notifications = inject(NotificationService);

  protected readonly tabs = TABS;
  protected readonly tab = signal<ManagerTab>('all');
  protected readonly query = signal('');

  protected readonly createOpen = signal(false);
  protected readonly formBusy = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly renaming = signal<Dashboard | null>(null);
  protected readonly sharing = signal<Dashboard | null>(null);
  protected readonly shareDetail = signal<Dashboard | null>(null);
  protected readonly shareBusy = signal(false);
  protected readonly shareError = signal<string | null>(null);
  protected readonly deleting = signal<Dashboard | null>(null);
  protected readonly deleteBusy = signal(false);
  protected readonly menuItems = signal<MenuItem[]>([]);

  protected readonly ready = computed(() => this.store.page.hasValue() || !!this.store.listError());
  protected readonly visible = computed(() =>
    this.tab() === 'favorites'
      ? this.store.items().filter((item) => item.isFavorite)
      : this.store.items(),
  );
  protected readonly isFiltering = computed(
    () => this.query().trim() !== '' || this.tab() !== 'all',
  );

  constructor() {
    effect(() => {
      const tab = this.tab();
      const q = this.query().trim();
      const scope: DashboardScope = tab === 'mine' ? 'mine' : tab === 'shared' ? 'shared' : 'all';
      untracked(() => this.store.loadList({ scope, q: q || undefined }));
    });
  }

  protected date(value: string): string {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? '' : DATE.format(parsed).replace('.', '');
  }

  protected retry(): void {
    this.store.loadList({
      scope: this.tab() === 'mine' ? 'mine' : this.tab() === 'shared' ? 'shared' : 'all',
    });
  }

  protected openMenu(event: Event, dashboard: Dashboard, menu: Menu): void {
    const items: MenuItem[] = [];
    if (dashboard.canEdit) {
      items.push({
        label: 'Cambiar nombre y descripción',
        icon: 'pi pi-pencil',
        command: () => {
          this.formError.set(null);
          this.renaming.set(dashboard);
        },
      });
    }
    items.push({ label: 'Duplicar', icon: 'pi pi-copy', command: () => this.duplicate(dashboard) });
    if (dashboard.canManage) {
      items.push({
        label: 'Compartir',
        icon: 'pi pi-share-alt',
        command: () => this.openShare(dashboard),
      });
      if (!dashboard.isDefault) {
        items.push({
          label: 'Usar como panel principal',
          icon: 'pi pi-home',
          command: () => this.makeDefault(dashboard),
        });
      }
      items.push({ separator: true });
      items.push({
        label: 'Eliminar',
        icon: 'pi pi-trash',
        command: () => this.deleting.set(dashboard),
      });
    }
    this.menuItems.set(items);
    menu.toggle(event);
  }

  protected toggleFavorite(dashboard: Dashboard): void {
    this.store.toggleFavorite(dashboard).subscribe({
      error: (error: unknown) => this.notifications.error(error),
    });
  }

  protected create(value: DashboardFormValue): void {
    this.formBusy.set(true);
    this.formError.set(null);
    this.store.create(value).subscribe({
      next: (created) => {
        this.formBusy.set(false);
        this.createOpen.set(false);
        this.notifications.success('Panel creado.');
        void this.router.navigate(['/dashboards', created.id]);
      },
      error: (error: unknown) => {
        this.formBusy.set(false);
        this.formError.set(messageOf(error, 'No se pudo crear el panel.'));
      },
    });
  }

  protected rename(value: DashboardFormValue): void {
    const target = this.renaming();
    if (!target) return;
    this.formBusy.set(true);
    this.formError.set(null);
    this.store.update(target.id, { name: value.name, description: value.description }).subscribe({
      next: () => {
        this.formBusy.set(false);
        this.renaming.set(null);
        this.notifications.success('Panel actualizado.');
      },
      error: (error: unknown) => {
        this.formBusy.set(false);
        this.formError.set(messageOf(error, 'No se pudo actualizar el panel.'));
      },
    });
  }

  protected duplicate(dashboard: Dashboard): void {
    this.store.duplicate(dashboard.id).subscribe({
      next: (copy) => {
        this.notifications.success('Panel duplicado.');
        void this.router.navigate(['/dashboards', copy.id]);
      },
      error: (error: unknown) => this.notifications.error(error),
    });
  }

  protected makeDefault(dashboard: Dashboard): void {
    this.store.makeDefault(dashboard.id).subscribe({
      next: () => this.notifications.success('Este panel es ahora tu panel principal.'),
      error: (error: unknown) => this.notifications.error(error),
    });
  }

  protected openShare(dashboard: Dashboard): void {
    this.shareError.set(null);
    this.sharing.set(dashboard);
    // The list does not carry the shares: only the detail of the owner does.
    this.shareDetail.set(null);
    this.api.get(dashboard.id).subscribe({
      next: (detail) => this.shareDetail.set(detail),
      error: (error: unknown) =>
        this.shareError.set(messageOf(error, 'No se pudieron cargar los accesos.')),
    });
  }

  protected saveShares(shares: readonly DashboardShareInput[]): void {
    const target = this.sharing();
    if (!target) return;
    this.shareBusy.set(true);
    this.shareError.set(null);
    this.store.setShares(target.id, shares).subscribe({
      next: () => {
        this.shareBusy.set(false);
        this.sharing.set(null);
        this.notifications.success('Accesos actualizados.');
        this.store.loadList({
          scope: this.tab() === 'mine' ? 'mine' : this.tab() === 'shared' ? 'shared' : 'all',
        });
      },
      error: (error: unknown) => {
        this.shareBusy.set(false);
        this.shareError.set(messageOf(error, 'No se pudieron guardar los accesos.'));
      },
    });
  }

  protected confirmDelete(): void {
    const target = this.deleting();
    if (!target) return;
    this.deleteBusy.set(true);
    this.store.remove(target.id).subscribe({
      next: () => {
        this.deleteBusy.set(false);
        this.deleting.set(null);
        this.notifications.success('Panel eliminado.');
      },
      error: (error: unknown) => {
        this.deleteBusy.set(false);
        this.notifications.error(error);
      },
    });
  }
}
