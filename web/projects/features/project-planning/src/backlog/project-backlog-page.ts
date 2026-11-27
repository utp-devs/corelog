import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  BacklogItem,
  BacklogStore,
  ProjectStore,
  WorkItemStatus,
  projectAccessVariantOf,
} from '@corelog/core';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { Lozenge, LozengeCategory } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { trackProjectScope } from '../shared/project-scope';

const STATUS_LABELS: Record<WorkItemStatus, string> = {
  open: 'Abierta',
  in_progress: 'En curso',
  resolved: 'Resuelta',
  closed: 'Cerrada',
};

const STATUS_CATEGORIES: Record<WorkItemStatus, LozengeCategory> = {
  open: 'todo',
  in_progress: 'progress',
  resolved: 'done',
  closed: 'done',
};

const COLUMNS: DataTableColumn[] = [
  { key: 'position', title: '#', align: 'left' },
  { key: 'summary', title: 'Incidencia', mobilePrimary: true },
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
    (projectId) => this.store.load(projectId),
  );

  protected statusLabel(status: WorkItemStatus): string {
    return STATUS_LABELS[status] ?? status;
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

  protected retry(): void {
    this.scope.reload();
  }
}
