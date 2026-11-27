import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  linkedSignal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import {
  Sprint,
  SprintDraft,
  SprintIssue,
  SprintStatus,
  SprintStore,
  SprintUnfinishedPolicy,
  WorkItemStatus,
  projectAccessVariantOf,
} from '@corelog/core';
import { projectIdParam } from '../shared/project-scope';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge, LozengeTone } from '@corelog/ui/lozenge';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section, SectionActions, SectionFooter } from '@corelog/ui/section';
import { StatTile } from '@corelog/ui/stat-tile';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';

import { BurndownChart } from './burndown-chart';

const STATUS_LABELS: Record<SprintStatus, string> = {
  planned: 'Planificado',
  active: 'Activo',
  closed: 'Cerrado',
};
const STATUS_TONES: Record<SprintStatus, LozengeTone> = {
  planned: 'info',
  active: 'success',
  closed: 'neutral',
};
const ISSUE_STATUS_LABELS: Record<WorkItemStatus, string> = {
  open: 'Abierta',
  in_progress: 'En curso',
  resolved: 'Resuelta',
  closed: 'Cerrada',
};

const ISSUE_COLUMNS: DataTableColumn[] = [
  { key: 'title', title: 'Incidencia', mobilePrimary: true },
  { key: 'status', title: 'Estado' },
  { key: 'points', title: 'Puntos', align: 'right' },
  { key: 'actions', title: '', align: 'right' },
];

/**
 * Sprint planning: the sprint list, the selected sprint with its issues and burndown, the lifecycle
 * actions (plan, start, close), the issues that can join the sprint and the velocity of closed sprints.
 * Every action reloads the server state through the sprint store.
 */
@Component({
  selector: 'app-project-sprints-page',
  imports: [
    FormsModule,
    Button,
    InputText,
    Message,
    Select,
    BurndownChart,
    DataTable,
    DataTableCell,
    EmptyState,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    PageContainer,
    PageHeader,
    Section,
    SectionActions,
    SectionFooter,
    StatTile,
  ],
  templateUrl: './project-sprints-page.html',
  styleUrl: './project-sprints-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectSprintsPage {
  protected readonly store = inject(SprintStore);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly sprints = this.store.sprints;
  protected readonly selected = this.store.selected;
  protected readonly selectedId = this.store.selectedId;
  protected readonly issues = computed(() => [...this.store.issues()]);
  protected readonly burndown = this.store.burndown;
  protected readonly velocity = this.store.velocity;
  protected readonly totals = this.store.issueTotals;
  protected readonly draft = this.store.draft;
  protected readonly closePolicy = this.store.closePolicy;
  protected readonly closeTargetId = this.store.closeTargetId;
  protected readonly issueToAdd = this.store.issueToAdd;
  protected readonly plannedTargets = this.store.plannedTargets;
  protected readonly actionError = this.store.actionError;
  protected readonly loadErrorMessage = this.store.loadErrorMessage;
  protected readonly issueColumns = ISSUE_COLUMNS;
  protected readonly assignableOptions = computed(() =>
    this.store.assignableBacklog().map((item) => ({ value: item.id, label: item.summary })),
  );
  protected readonly ready = computed(
    () => this.store.overview.hasValue() && !this.loadErrorMessage(),
  );
  protected readonly accessVariant = computed(() =>
    projectAccessVariantOf(this.store.overview.error()),
  );

  /** Whether the action error banner is open. It opens again whenever a new error is stored. */
  protected readonly alertOpen = linkedSignal(() => this.store.actionError() !== null);

  constructor() {
    projectIdParam(inject(ActivatedRoute))
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((projectId) => this.store.load(projectId));
  }

  protected statusLabel(status: SprintStatus): string {
    return STATUS_LABELS[status];
  }

  protected statusTone(status: SprintStatus): LozengeTone {
    return STATUS_TONES[status];
  }

  protected issueStatusLabel(status: WorkItemStatus): string {
    return ISSUE_STATUS_LABELS[status] ?? status;
  }

  protected dateRangeOf(sprint: Sprint): string {
    return sprint.startDate && sprint.endDate
      ? `${sprint.startDate} → ${sprint.endDate}`
      : 'Sin fechas';
  }

  protected setDraft(field: keyof SprintDraft, event: Event): void {
    this.store.setDraft({ [field]: (event.target as HTMLInputElement).value });
  }

  protected setCloseMode(policy: SprintUnfinishedPolicy): void {
    this.store.setCloseMode(policy);
  }

  protected removeIssue(sprint: Sprint, issue: SprintIssue): void {
    this.store.removeIssue(sprint.id, issue.id);
  }

  protected canRemove(sprint: Sprint): boolean {
    return sprint.status !== 'closed';
  }
}
