import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import {
  CatalogItem,
  CatalogKind,
  CatalogStore,
  NotificationService,
  OrganizationStore,
  canManageOrg,
  catalogErrorMessage,
} from '@corelog/core';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { CATALOG_COPY, CATALOG_TABS } from './catalog-copy';
import { CatalogDeleteDialog } from './catalog-delete-dialog';
import { CatalogFormDialog, CatalogFormValue } from './catalog-form-dialog';
import { CatalogTable } from './catalog-table';

/**
 * Issue types, priorities and resolutions of the organization, in one tab each. Owners and admins create,
 * edit, reorder, mark as default and delete (moving the issues that use an item to another one); other
 * members only read. Every refusal of the API is shown in Spanish next to what was attempted.
 */
@Component({
  selector: 'app-org-catalogs-page',
  imports: [
    Button,
    CatalogDeleteDialog,
    CatalogFormDialog,
    CatalogTable,
    ErrorState,
    LoadingSkeleton,
    Message,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    Section,
  ],
  templateUrl: './org-catalogs-page.html',
  styleUrl: './org-catalogs-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgCatalogsPage {
  private readonly store = inject(CatalogStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);

  protected readonly tabs = CATALOG_TABS;
  protected readonly active = signal<CatalogKind>('issue-types');
  protected readonly copy = computed(() => CATALOG_COPY[this.active()]);
  protected readonly items = computed(() => this.store.items(this.active()));
  protected readonly canManage = computed(() => canManageOrg(this.orgs.currentRole()));
  protected readonly subtitle = computed(() => {
    const org = this.orgs.currentOrg();
    return org ? `Catálogos de ${org.name}` : undefined;
  });

  protected readonly loading = computed(() => this.store.loading() && !this.store.loaded());
  protected readonly loadError = computed(() => (this.store.loaded() ? null : this.store.error()));

  protected readonly formOpen = signal(false);
  /** Item being edited; null while the form creates a new one. */
  protected readonly editing = signal<CatalogItem | null>(null);
  protected readonly deleteOpen = signal(false);
  protected readonly deleting = signal<CatalogItem | null>(null);
  protected readonly busy = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly deleteError = signal<string | null>(null);
  /** Failure of an action made from the list (reorder, default): shown above the table. */
  protected readonly listError = signal<string | null>(null);
  /** Text for assistive technology after an action that has no other visible feedback. */
  protected readonly announcement = signal('');

  constructor() {
    effect(() => {
      this.orgs.currentOrgId();
      untracked(() => this.store.ensureLoaded());
    });
  }

  protected select(kind: CatalogKind): void {
    this.active.set(kind);
    this.listError.set(null);
  }

  protected onTabKeydown(event: KeyboardEvent, index: number): void {
    const count = this.tabs.length;
    let next: number;
    if (event.key === 'ArrowRight') next = (index + 1) % count;
    else if (event.key === 'ArrowLeft') next = (index - 1 + count) % count;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = count - 1;
    else return;
    event.preventDefault();
    this.select(this.tabs[next].kind);
    document.getElementById(`catalog-tab-${this.tabs[next].kind}`)?.focus();
  }

  protected retry(): void {
    this.store.refreshAll();
  }

  protected openCreate(): void {
    this.editing.set(null);
    this.formError.set(null);
    this.formOpen.set(true);
  }

  protected openEdit(item: CatalogItem): void {
    this.editing.set(item);
    this.formError.set(null);
    this.formOpen.set(true);
  }

  protected closeForm(): void {
    if (!this.busy()) this.formOpen.set(false);
  }

  protected save(value: CatalogFormValue): void {
    const kind = this.active();
    const item = this.editing();
    this.busy.set(true);
    this.formError.set(null);
    const request = item
      ? this.store.update(kind, item.id, {
          name: value.name,
          description: value.description,
          icon: value.icon ?? undefined,
          color: value.color ?? undefined,
          isDefault: value.isDefault && !item.isDefault ? true : undefined,
        })
      : this.store.create(kind, {
          name: value.name,
          description: value.description || undefined,
          icon: value.icon ?? undefined,
          color: value.color ?? undefined,
          isDefault: value.isDefault || undefined,
        });
    request.subscribe({
      next: (saved) => {
        this.busy.set(false);
        this.formOpen.set(false);
        this.notify.success(
          item ? `Se guardaron los cambios de ${saved.name}` : `Se creó ${saved.name}`,
        );
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.formError.set(catalogErrorMessage(error, 'No se pudo guardar. Inténtalo de nuevo.'));
      },
    });
  }

  protected reorder(ids: string[]): void {
    this.listError.set(null);
    this.busy.set(true);
    this.store.reorder(this.active(), ids).subscribe({
      next: () => {
        this.busy.set(false);
        this.announcement.set('Orden actualizado');
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.listError.set(catalogErrorMessage(error, 'No se pudo cambiar el orden.'));
      },
    });
  }

  protected makeDefault(item: CatalogItem): void {
    this.listError.set(null);
    this.busy.set(true);
    this.store.update(this.active(), item.id, { isDefault: true }).subscribe({
      next: () => {
        this.busy.set(false);
        this.announcement.set(`${item.name} es ahora el predeterminado`);
        this.notify.success(`${item.name} es ahora el predeterminado`);
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.listError.set(catalogErrorMessage(error, 'No se pudo cambiar el predeterminado.'));
      },
    });
  }

  protected openDelete(item: CatalogItem): void {
    this.deleting.set(item);
    this.deleteError.set(null);
    this.deleteOpen.set(true);
  }

  protected closeDelete(): void {
    if (!this.busy()) this.deleteOpen.set(false);
  }

  protected confirmDelete(reassignTo: string | undefined): void {
    const item = this.deleting();
    if (!item) return;
    this.busy.set(true);
    this.deleteError.set(null);
    this.store.remove(this.active(), item.id, reassignTo).subscribe({
      next: () => {
        this.busy.set(false);
        this.deleteOpen.set(false);
        this.notify.success(`Se eliminó ${item.name}`);
        // The issues moved to another item changed its usage count.
        if (reassignTo) this.store.refresh(this.active());
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.deleteError.set(
          catalogErrorMessage(error, 'No se pudo eliminar. Inténtalo de nuevo.'),
        );
      },
    });
  }
}
