import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  BacklogItem,
  AuthStore,
  BacklogStore,
  OrganizationStore,
  ProjectStore,
  WorkItemStatus,
  projectAccessVariantOf,
} from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { Lozenge, LozengeCategory } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { Section } from '@corelog/ui/section';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { trackProjectScope } from '../shared/project-scope';
import {
  STATUS_CATEGORIES,
  isReadOnlyRole,
  shortKeyOf,
  statusName,
  typeName,
} from '../shared/work-labels';

const COLUMNS: DataTableColumn[] = [
  { key: 'position', title: '#', align: 'left' },
  { key: 'summary', title: 'Incidencia', mobilePrimary: true },
  { key: 'type', title: 'Tipo' },
  { key: 'priority', title: 'Prioridad' },
  { key: 'status', title: 'Estado' },
  { key: 'points', title: 'Puntos', align: 'right' },
  { key: 'order', title: 'Orden', align: 'right' },
];

const LOAD_ERROR_MESSAGE =
  'No se pudo cargar el backlog. Verifica que tengas acceso al proyecto e intenta de nuevo.';

/**
 * Project backlog in rank order. Items move one position at a time with the up and down controls; the
 * server answers with the renumbered list, which replaces the rows.
 */
@Component({
  selector: 'app-project-backlog-page',
  imports: [
    NgTemplateOutlet,
    RouterLink,
    Button,
    AppTooltip,
    DropdownMenu,
    PageHeaderActions,
    Message,
    DataTable,
    DataTableCell,
    EmptyState,
    ErrorPage,
    Lozenge,
    PageContainer,
    PageHeader,
    PriorityTag,
    Section,
  ],
  templateUrl: './project-backlog-page.html',
  styleUrl: './project-backlog-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectBacklogPage {
  protected readonly store = inject(BacklogStore);
  private readonly auth = inject(AuthStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly projects = inject(ProjectStore);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly items = computed(() => [...this.store.ordered()]);
  protected readonly columns = COLUMNS;
  protected readonly loadErrorMessage = LOAD_ERROR_MESSAGE;

  /** Failure of the project read or of the backlog load. */
  private readonly failure = computed(() => this.scope.accessError() ?? this.store.items.error());
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.failure()));
  protected readonly tableError = computed(() => (this.failure() ? LOAD_ERROR_MESSAGE : null));
  protected readonly loading = computed(
    () => !this.store.items.hasValue() && this.failure() === null,
  );
  protected readonly ready = computed(() => this.store.items.hasValue() && this.failure() === null);

  protected readonly scope = trackProjectScope(
    inject(ActivatedRoute),
    inject(ProjectStore),
    this.destroyRef,
    (projectId) => {
      this.store.load(projectId);
      this.projects.fetchMembers(projectId);
    },
  );

  /** Viewers and guests read the backlog; only editors reorder it. */
  protected readonly readOnly = computed(() =>
    isReadOnlyRole(
      this.auth.user()?.role,
      this.orgs.currentRole(),
      this.auth.user()?.id,
      this.projects.members(),
    ),
  );

  protected statusLabel(status: WorkItemStatus): string {
    return statusName(status, status);
  }

  protected typeLabel(type: string): string {
    return typeName(type);
  }

  protected shortKey(item: BacklogItem): string {
    return shortKeyOf(item.id);
  }

  protected statusCategory(status: WorkItemStatus): LozengeCategory {
    return STATUS_CATEGORIES[status] ?? 'todo';
  }

  /** 1-based position of the item in the backlog. */
  protected positionOf(item: BacklogItem): number {
    return this.items().indexOf(item) + 1;
  }

  protected canMoveUp(item: BacklogItem): boolean {
    return this.store.canMoveUp(this.items().indexOf(item));
  }

  protected canMoveDown(item: BacklogItem): boolean {
    return this.store.canMoveDown(this.items().indexOf(item));
  }

  protected moveUp(item: BacklogItem): void {
    const index = this.items().indexOf(item);
    this.store.move(index, index - 1);
  }

  protected moveDown(item: BacklogItem): void {
    const index = this.items().indexOf(item);
    this.store.move(index, index + 1);
  }

  protected moveToStart(item: BacklogItem): void {
    this.store.move(this.items().indexOf(item), 0);
  }

  protected moveToEnd(item: BacklogItem): void {
    this.store.move(this.items().indexOf(item), this.items().length - 1);
  }

  protected retry(): void {
    this.scope.reload();
  }
}
