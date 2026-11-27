import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
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
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { DateField } from '@corelog/ui/date-field';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { Lozenge, LozengeCategory, LozengeTone } from '@corelog/ui/lozenge';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section, SectionActions } from '@corelog/ui/section';
import { StatTile } from '@corelog/ui/stat-tile';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';

import { projectIdParam } from '../shared/project-scope';
import { STATUS_CATEGORIES, statusName } from '../shared/work-labels';
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
    ConfirmModal,
    DateField,
    FieldControl,
    FieldRow,
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
  /** The confirmation of the close action is open. */
  protected readonly closeOpen = signal(false);
  protected readonly closeDescription = computed(() => {
    const sprint = this.selected();
    const target =
      this.closePolicy() === 'sprint'
        ? this.plannedTargets().find((item) => item.id === this.closeTargetId())?.name
        : undefined;
    const destination = target ? `al sprint ${target}` : 'al backlog';
    return `Se cerrará ${sprint?.name ?? 'el sprint'}. Las incidencias sin terminar pasarán ${destination}. Esta acción no se puede deshacer.`;
  });
  protected readonly assignableOptions = computed(() =>
    this.store.assignableBacklog().map((item) => ({ value: item.id, label: item.summary })),
  );
  protected readonly ready = computed(
    () => this.store.overview.hasValue() && !this.loadErrorMessage(),
  );
  protected readonly accessVariant = computed(() =>
    projectAccessVariantOf(this.store.overview.error()),
  );

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
    return statusName(status, status);
  }

  protected issueCategory(status: WorkItemStatus): LozengeCategory {
    return STATUS_CATEGORIES[status] ?? 'todo';
  }

  /** Dates of a sprint as dd/MM/yyyy. */
  protected dateRangeOf(sprint: Sprint): string {
    return sprint.startDate && sprint.endDate
      ? `${formatDay(sprint.startDate)} - ${formatDay(sprint.endDate)}`
      : 'Sin fechas';
  }

  protected setDraft(field: keyof SprintDraft, event: Event): void {
    this.store.setDraft({ [field]: (event.target as HTMLInputElement).value });
  }

  /** Date fields give ISO days or null; the draft keeps an empty text for no date. */
  protected setDraftDate(field: 'startDate' | 'endDate', value: string | null): void {
    this.store.setDraft({ [field]: value ?? '' });
  }

  protected askClose(): void {
    if (this.store.canClose()) this.closeOpen.set(true);
  }

  protected confirmClose(): void {
    const sprint = this.selected();
    this.closeOpen.set(false);
    if (sprint) this.store.close(sprint.id);
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

/** ISO day (2026-11-19) as dd/MM/yyyy. Other text is returned unchanged. */
function formatDay(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  return match ? `${match[3]}/${match[2]}/${match[1]}` : value;
}
