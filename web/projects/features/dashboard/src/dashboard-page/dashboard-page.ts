import { CdkDrag, CdkDragDrop, CdkDropList } from '@angular/cdk/drag-drop';
import { LiveAnnouncer } from '@angular/cdk/a11y';
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
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import {
  ApiError,
  AuthStore,
  Dashboard,
  DashboardLayout,
  DashboardShareInput,
  DashboardV2Store,
  GadgetConfig,
  GadgetType,
  NotificationService,
  UNSAVED_CHANGES_PROMPT,
  UnsavedChangesAware,
  gadgetDefinition,
  gadgetsByColumn,
  layoutTracks,
  warnBeforeUnload,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { AppTooltip } from '@corelog/ui/tooltip';
import type { MenuItem } from 'primeng/api';
import { Menu } from 'primeng/menu';

import { DashboardEditSession } from '../edit/dashboard-edit-session';
import { GadgetCatalogDialog } from '../edit/gadget-catalog-dialog';
import { GadgetConfigDialog, GadgetConfigValue } from '../edit/gadget-config-dialog';
import {
  DashboardFormDialog,
  DashboardFormValue,
  LAYOUT_OPTIONS,
} from '../form-dialog/dashboard-form-dialog';
import { GadgetHost } from '../gadgets/gadget-host';
import { DashboardShareDialog } from '../share-dialog/share-dialog';

const LONG_DATE = new Intl.DateTimeFormat('es', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
});

interface ConfigTarget {
  readonly mode: 'add' | 'edit';
  readonly type: GadgetType;
  readonly gadgetId: string | null;
  readonly title: string;
  readonly config: GadgetConfig;
}

function messageOf(error: unknown, fallback: string): string {
  const message = (error as Partial<ApiError> | null)?.message;
  return message && message.trim() ? message : fallback;
}

/**
 * A dashboard of gadgets. With `data.home` it shows the personal default dashboard (with the greeting
 * banner); with an `id` route parameter it shows that dashboard. Owners and editors can switch to edit
 * mode to add, configure, move and remove gadgets; viewers see it read-only.
 */
@Component({
  selector: 'app-dashboard-page',
  imports: [
    AppTooltip,
    CdkDrag,
    CdkDropList,
    DashboardFormDialog,
    DashboardShareDialog,
    EmptyState,
    ErrorState,
    GadgetCatalogDialog,
    GadgetConfigDialog,
    GadgetHost,
    LoadingSkeleton,
    Menu,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    RouterLink,
  ],
  templateUrl: './dashboard-page.html',
  styleUrl: './dashboard-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardPage implements UnsavedChangesAware {
  protected readonly store = inject(DashboardV2Store);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly auth = inject(AuthStore);
  private readonly notifications = inject(NotificationService);
  private readonly announcer = inject(LiveAnnouncer);
  private readonly prompt = inject(UNSAVED_CHANGES_PROMPT);

  protected readonly session = new DashboardEditSession();
  protected readonly editing = signal(false);

  protected readonly catalogOpen = signal(false);
  protected readonly configTarget = signal<ConfigTarget | null>(null);
  protected readonly shareOpen = signal(false);
  protected readonly shareBusy = signal(false);
  protected readonly shareError = signal<string | null>(null);
  protected readonly renameOpen = signal(false);
  protected readonly renameBusy = signal(false);
  protected readonly renameError = signal<string | null>(null);
  protected readonly saveError = signal<string | null>(null);

  protected readonly layoutOptions = LAYOUT_OPTIONS;
  protected readonly today = LONG_DATE.format(new Date());

  private readonly params = toSignal(this.route.paramMap, { requireSync: true });
  protected readonly isHome = computed(() => this.params().get('id') === null);

  protected readonly dashboard = computed(() => this.store.current.value() ?? null);
  protected readonly ready = computed(() => this.dashboard() !== null);
  protected readonly firstName = computed(
    () => (this.auth.displayName() ?? '').trim().split(/\s+/)[0] ?? '',
  );

  /** Edits not saved yet; the route guard and the browser ask before they are lost. */
  readonly dirty = computed(() => this.editing() && this.session.dirty());

  private readonly storedColumns = computed(() => {
    const dashboard = this.dashboard();
    return dashboard ? gadgetsByColumn(dashboard.gadgets, dashboard.layout) : [];
  });
  protected readonly columns = computed(() =>
    this.editing() ? this.session.columns() : this.storedColumns(),
  );
  protected readonly layout = computed(() =>
    this.editing() ? this.session.layout() : (this.dashboard()?.layout ?? '2'),
  );
  protected readonly tracks = computed(() => layoutTracks(this.layout()));
  protected readonly dropListIds = computed(() =>
    this.columns().map((_, index) => `dashboard-column-${index}`),
  );
  protected readonly gadgetTotal = computed(() => this.columns().reduce((n, c) => n + c.length, 0));

  protected readonly menuItems = computed<MenuItem[]>(() => {
    const dashboard = this.dashboard();
    if (!dashboard) return [];
    const items: MenuItem[] = [];
    if (dashboard.canEdit) {
      items.push({
        label: 'Cambiar nombre y descripción',
        icon: 'pi pi-pencil',
        command: () => this.renameOpen.set(true),
      });
    }
    items.push({ label: 'Duplicar', icon: 'pi pi-copy', command: () => this.duplicate() });
    if (dashboard.canManage && !dashboard.isDefault) {
      items.push({
        label: 'Usar como panel principal',
        icon: 'pi pi-home',
        command: () => this.makeDefault(),
      });
    }
    items.push({
      label: 'Todos los paneles',
      icon: 'pi pi-th-large',
      command: () => void this.router.navigateByUrl('/dashboards'),
    });
    return items;
  });

  constructor() {
    warnBeforeUnload(this.dirty);
    effect(() => {
      const id = this.params().get('id');
      untracked(() => {
        this.editing.set(false);
        if (id) this.store.loadOne(id);
        else this.store.loadMine();
      });
    });
    inject(DestroyRef).onDestroy(() => this.store.current.cancel());
  }

  protected retry(): void {
    if (this.isHome()) this.store.loadMine();
    else this.store.loadOne(this.params().get('id') ?? '');
  }

  // Edit mode

  protected startEdit(): void {
    const dashboard = this.dashboard();
    if (!dashboard?.canEdit) return;
    this.session.begin(dashboard);
    this.saveError.set(null);
    this.editing.set(true);
  }

  protected async cancelEdit(): Promise<void> {
    if (this.session.dirty()) {
      const leave = await this.prompt({
        title: 'Descartar los cambios',
        description: 'Los cambios que no has guardado se perderán.',
        confirmText: 'Descartar',
        cancelText: 'Seguir editando',
      });
      if (!leave) return;
    }
    this.session.discard();
    this.editing.set(false);
  }

  protected save(): void {
    const dashboard = this.dashboard();
    if (!dashboard || this.store.saving()) return;
    this.saveError.set(null);
    this.store.saveGadgets(dashboard.id, this.session.layout(), this.session.gadgets()).subscribe({
      next: (saved) => {
        this.session.commit(saved);
        this.editing.set(false);
        this.notifications.success('Panel guardado.');
      },
      error: (error: unknown) =>
        this.saveError.set(messageOf(error, 'No se pudo guardar el panel. Intenta de nuevo.')),
    });
  }

  protected setLayout(value: string | null): void {
    if (value) this.session.setLayout(value as DashboardLayout);
  }

  // Gadgets

  protected openCatalog(): void {
    if (this.session.canAdd()) this.catalogOpen.set(true);
    else this.notifications.info('El panel ya tiene el máximo de 20 gadgets.');
  }

  protected pickGadget(type: GadgetType): void {
    const definition = gadgetDefinition(type);
    this.catalogOpen.set(false);
    this.configTarget.set({
      mode: 'add',
      type,
      gadgetId: null,
      title: definition.label,
      config: definition.defaultConfig,
    });
  }

  protected configureGadget(id: string): void {
    const gadget = this.session.gadgets().find((entry) => entry.id === id);
    if (!gadget) return;
    this.configTarget.set({
      mode: 'edit',
      type: gadget.type,
      gadgetId: id,
      title: gadget.title,
      config: gadget.config,
    });
  }

  protected applyConfig(value: GadgetConfigValue): void {
    const target = this.configTarget();
    if (!target) return;
    this.configTarget.set(null);
    if (target.mode === 'add') {
      const added = this.session.add(target.type, value.config, value.title);
      if (added)
        this.announce(`Gadget ${added.title} añadido a la columna ${added.columnIndex + 1}.`);
    } else if (target.gadgetId) {
      this.session.update(target.gadgetId, value.title, value.config);
      this.announce('Gadget actualizado.');
    }
  }

  protected removeGadget(id: string, title: string): void {
    this.session.remove(id);
    this.announce(`Gadget ${title} quitado del panel.`);
  }

  protected moveToColumn(id: string, title: string, column: number): void {
    this.session.move(id, column, Number.MAX_SAFE_INTEGER);
    this.announce(`Gadget ${title} movido a la columna ${column + 1}.`);
  }

  protected nudge(id: string, title: string, delta: -1 | 1): void {
    if (!this.session.nudge(id, delta)) return;
    const spot = this.session.where(id);
    if (spot) {
      this.announce(
        `Gadget ${title} movido a la posición ${spot.index + 1} de la columna ${spot.column + 1}.`,
      );
    }
  }

  protected drop(event: CdkDragDrop<number, number, string>): void {
    const id = event.item.data;
    const title = this.session.gadgets().find((gadget) => gadget.id === id)?.title ?? 'Gadget';
    const column = event.container.data;
    this.session.move(id, column, event.currentIndex);
    this.announce(
      `Gadget ${title} soltado en la posición ${event.currentIndex + 1} de la columna ${column + 1}.`,
    );
  }

  private announce(message: string): void {
    void this.announcer.announce(message, 'polite');
  }

  protected unsavedFor(id: string): boolean {
    const gadget = this.session.gadgets().find((entry) => entry.id === id);
    return this.editing() && !!gadget && this.session.isUnsaved(gadget);
  }

  // Dashboard actions

  protected toggleFavorite(): void {
    const dashboard = this.dashboard();
    if (!dashboard) return;
    this.store.toggleFavorite(dashboard).subscribe({
      next: (starred) =>
        this.announce(starred ? 'Panel marcado como favorito.' : 'Panel quitado de favoritos.'),
      error: (error: unknown) => this.notifications.error(error),
    });
  }

  protected rename(value: DashboardFormValue): void {
    const dashboard = this.dashboard();
    if (!dashboard) return;
    this.renameBusy.set(true);
    this.renameError.set(null);
    this.store
      .update(dashboard.id, { name: value.name, description: value.description })
      .subscribe({
        next: () => {
          this.renameBusy.set(false);
          this.renameOpen.set(false);
          this.notifications.success('Panel actualizado.');
        },
        error: (error: unknown) => {
          this.renameBusy.set(false);
          this.renameError.set(messageOf(error, 'No se pudo actualizar el panel.'));
        },
      });
  }

  protected duplicate(): void {
    const dashboard = this.dashboard();
    if (!dashboard) return;
    this.store.duplicate(dashboard.id).subscribe({
      next: (copy) => {
        this.notifications.success('Panel duplicado.');
        void this.router.navigate(['/dashboards', copy.id]);
      },
      error: (error: unknown) => this.notifications.error(error),
    });
  }

  protected makeDefault(): void {
    const dashboard = this.dashboard();
    if (!dashboard) return;
    this.store.makeDefault(dashboard.id).subscribe({
      next: () => this.notifications.success('Este panel es ahora tu panel principal.'),
      error: (error: unknown) => this.notifications.error(error),
    });
  }

  protected saveShares(shares: readonly DashboardShareInput[]): void {
    const dashboard = this.dashboard();
    if (!dashboard) return;
    this.shareBusy.set(true);
    this.shareError.set(null);
    this.store.setShares(dashboard.id, shares).subscribe({
      next: () => {
        this.shareBusy.set(false);
        this.shareOpen.set(false);
        this.notifications.success('Accesos actualizados.');
      },
      error: (error: unknown) => {
        this.shareBusy.set(false);
        this.shareError.set(messageOf(error, 'No se pudieron guardar los accesos.'));
      },
    });
  }
}
