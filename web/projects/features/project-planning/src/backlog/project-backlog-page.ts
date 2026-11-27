import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
} from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  AuthStore,
  BacklogItem,
  BacklogSprint,
  BacklogStore,
  OrganizationStore,
  ProjectStore,
  SprintStatus,
  WorkItemStatus,
  projectAccessVariantOf,
  shortIssueKeyOf,
} from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { Lozenge, LozengeCategory, LozengeTone } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { trackProjectScope } from '../shared/project-scope';
import {
  SPRINT_STATUS_LABELS,
  SPRINT_STATUS_TONES,
  STATUS_CATEGORIES,
  isReadOnlyRole,
  isResolvedStatus,
  sprintDatesOf,
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

/** Id of the collapsible block that lists the issues no open sprint holds. */
const BACKLOG_SECTION_ID = 'backlog';

/**
 * Project backlog: one collapsible block per active or planned sprint, with its dates, goal, progress and
 * issues, then the block of the issues no sprint holds, in rank order. Those move one position at a time
 * with the up and down controls; the server answers with the renumbered list, which replaces the rows.
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
    ProgressBar,
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

  protected readonly items = computed(() => [...this.store.unplanned()]);
  protected readonly sprintBlocks = this.store.sprints.value;
  protected readonly backlogSectionId = BACKLOG_SECTION_ID;
  private readonly collapsedIds = signal<ReadonlySet<string>>(new Set());
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

  protected isOpen(sectionId: string): boolean {
    return !this.collapsedIds().has(sectionId);
  }

  protected toggle(sectionId: string): void {
    this.collapsedIds.update((current) => {
      const next = new Set(current);
      if (!next.delete(sectionId)) {
        next.add(sectionId);
      }
      return next;
    });
  }

  protected sprintStatusLabel(status: SprintStatus): string {
    return SPRINT_STATUS_LABELS[status];
  }

  protected sprintStatusTone(status: SprintStatus): LozengeTone {
    return SPRINT_STATUS_TONES[status];
  }

  protected sprintDates(block: BacklogSprint): string {
    return sprintDatesOf(block.sprint);
  }

  protected resolvedOf(block: BacklogSprint): number {
    return block.issues.filter((issue) => isResolvedStatus(issue.status)).length;
  }

  /** Share of the sprint issues that are resolved or closed; an empty sprint has none. */
  protected percentOf(block: BacklogSprint): number {
    return block.issues.length === 0 ? 0 : (this.resolvedOf(block) / block.issues.length) * 100;
  }

  protected statusLabel(status: WorkItemStatus): string {
    return statusName(status, status);
  }

  protected typeLabel(type: string): string {
    return typeName(type);
  }

  protected shortKey(item: { readonly id: string }): string {
    return shortIssueKeyOf(item.id);
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
