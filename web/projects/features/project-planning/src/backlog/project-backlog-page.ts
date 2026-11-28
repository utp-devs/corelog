import { CdkDropListGroup } from '@angular/cdk/drag-drop';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
} from '@angular/core';
import { Router } from '@angular/router';
import {
  AgileSprint,
  BacklogSection,
  BacklogViewStore,
  BreakpointService,
  CompleteSprintInput,
  CurrentProjectStore,
  NotificationService,
  PROJECT_MOBILE_QUERY,
  SelectionMode,
  SprintPlanStore,
  asApiError,
  projectAccessVariantOf,
  projectPath,
  sprintErrorMessage,
} from '@corelog/core';
import { ArchivedNotice } from '@corelog/ui/archived-notice';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { SearchInput } from '@corelog/ui/search-input';
import { SidePanel } from '@corelog/ui/side-panel';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { Skeleton } from 'primeng/skeleton';

import { trackProjectScope } from '../shared/project-scope';
import { CompleteSprintDialog, CompleteSprintSummary } from '../sprints/complete-sprint-dialog';
import { SprintDialog, SprintDialogMode, SprintDialogValue } from '../sprints/sprint-dialog';
import { BacklogIssuePanel } from './backlog-issue-panel';
import { BacklogDestination, BacklogRankRequest, BacklogSectionView } from './backlog-section';

const LOAD_ERROR_MESSAGE =
  'No se pudo cargar el backlog. Verifica que tengas acceso al proyecto e intenta de nuevo.';
const WIDE_QUERY = '(min-width: 1100px)';

/**
 * Backlog of the project: the epics panel, the sprint sections (active, planned) and the backlog as ranked
 * lists, and the selected issue in a side panel. It also runs the sprint lifecycle: create, edit, start,
 * complete and delete.
 */
@Component({
  selector: 'app-project-backlog-page',
  imports: [
    ArchivedNotice,
    BacklogIssuePanel,
    BacklogSectionView,
    Button,
    CdkDropListGroup,
    CompleteSprintDialog,
    ConfirmModal,
    EmptyState,
    ErrorPage,
    ErrorState,
    Message,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    ProgressBar,
    SearchInput,
    SidePanel,
    Skeleton,
    SprintDialog,
  ],
  templateUrl: './project-backlog-page.html',
  styleUrl: './project-backlog-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectBacklogPage {
  protected readonly store = inject(BacklogViewStore);
  private readonly sprintPlan = inject(SprintPlanStore);
  private readonly current = inject(CurrentProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly router = inject(Router);
  private readonly breakpoints = inject(BreakpointService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly isMobile = this.breakpoints.matches(PROJECT_MOBILE_QUERY);
  protected readonly isWide = this.breakpoints.matches(WIDE_QUERY);
  protected readonly view = this.store.view.value;
  protected readonly loadErrorMessage = LOAD_ERROR_MESSAGE;
  protected readonly skeletonRows = [0, 1, 2, 3, 4];

  protected readonly scope = trackProjectScope(this.current, this.destroyRef, (projectId) =>
    this.store.open(projectId),
  );

  private readonly failure = computed(() => this.scope.accessError() ?? this.store.view.error());
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.failure()));
  protected readonly failed = computed(() => this.failure() !== null);
  protected readonly ready = computed(() => this.view() !== undefined && !this.failed());

  protected readonly archived = this.current.isArchived;
  protected readonly canMove = computed(() => this.store.canMove() && !this.archived());
  protected readonly canManage = computed(() => this.store.canManageSprints() && !this.archived());
  protected readonly sections = computed(() => this.view()?.sections ?? []);
  protected readonly epics = computed(() => this.view()?.epics ?? []);
  protected readonly hasActive = computed(() => this.sections().some((s) => s.kind === 'active'));
  protected readonly orderedIds = computed(() =>
    this.sections().flatMap((section) => section.issues.map((issue) => issue.id)),
  );
  protected readonly totalIssues = computed(() => this.orderedIds().length);
  protected readonly destinations = computed<BacklogDestination[]>(() =>
    this.sections().map((section) => ({
      sprintId: section.sprint?.id ?? null,
      name: section.sprint?.name ?? 'Backlog',
    })),
  );
  protected readonly inspected = this.store.inspected;
  protected readonly epicsOpen = signal(false);
  protected readonly backlogPath = computed(() => {
    const project = this.current.project();
    return project ? projectPath(project, 'board') : '/';
  });

  /* Sprint dialogs */

  protected readonly sprintDialogOpen = signal(false);
  protected readonly sprintDialogMode = signal<SprintDialogMode>('create');
  protected readonly dialogSprint = signal<AgileSprint | null>(null);
  protected readonly dialogIssueCount = signal(0);
  protected readonly sprintBusy = signal(false);
  protected readonly sprintError = signal<string | null>(null);

  protected readonly completeOpen = signal(false);
  protected readonly completeSprintTarget = signal<AgileSprint | null>(null);
  protected readonly completeSummary = signal<CompleteSprintSummary>({
    doneIssues: 0,
    openIssues: 0,
    donePoints: 0,
    openPoints: 0,
  });
  protected readonly plannedForComplete = signal<readonly AgileSprint[]>([]);
  protected readonly completeBusy = signal(false);
  protected readonly completeError = signal<string | null>(null);

  protected readonly deleteTarget = signal<AgileSprint | null>(null);
  protected readonly deleteBusy = signal(false);

  protected readonly suggestedName = computed(() => {
    const names = this.sections()
      .map((section) => section.sprint?.name ?? '')
      .filter((name) => name);
    const numbers = names
      .map((name) => /(\d+)\s*$/.exec(name))
      .filter((match): match is RegExpExecArray => match !== null)
      .map((match) => Number(match[1]));
    const next = numbers.length > 0 ? Math.max(...numbers) + 1 : names.length + 1;
    return `Sprint ${next}`;
  });

  /* Selection, ranking, points, create */

  protected onSelect(event: { id: string; mode: SelectionMode }): void {
    this.store.select(event.id, event.mode);
  }

  protected onRank(request: BacklogRankRequest): void {
    this.store.rank(request.issueIds, request.sprintId, request.anchor);
  }

  protected onPoints(event: { id: string; points: number | null }): void {
    this.store.setPoints(event.id, event.points);
  }

  protected onCreateIssue(summary: string, section: BacklogSection): void {
    this.store.createIssue(summary, section.sprint?.id ?? null).subscribe({
      next: () => this.notify.success('Incidencia creada'),
      error: () => this.notify.error('No se pudo crear la incidencia. Intenta de nuevo.'),
    });
  }

  /* Sprint lifecycle */

  protected openCreateSprint(): void {
    this.sprintError.set(null);
    this.dialogSprint.set(null);
    this.sprintDialogMode.set('create');
    this.sprintDialogOpen.set(true);
  }

  protected openEditSprint(sprint: AgileSprint): void {
    this.sprintError.set(null);
    this.dialogSprint.set(sprint);
    this.sprintDialogMode.set('edit');
    this.sprintDialogOpen.set(true);
  }

  protected openStartSprint(section: BacklogSection): void {
    this.sprintError.set(null);
    this.dialogSprint.set(section.sprint);
    this.dialogIssueCount.set(section.issues.length);
    this.sprintDialogMode.set('start');
    this.sprintDialogOpen.set(true);
  }

  protected submitSprint(value: SprintDialogValue): void {
    const projectId = this.current.projectId();
    const sprint = this.dialogSprint();
    const mode = this.sprintDialogMode();
    if (!projectId) return;
    this.sprintBusy.set(true);
    this.sprintError.set(null);
    const request =
      mode === 'create'
        ? this.sprintPlan.create(projectId, {
            name: value.name,
            goal: value.goal,
            startDate: value.startDate,
            endDate: value.endDate,
          })
        : mode === 'edit' && sprint
          ? this.sprintPlan.update(sprint.id, {
              name: value.name,
              goal: value.goal,
              startDate: value.startDate,
              endDate: value.endDate,
            })
          : sprint
            ? this.sprintPlan.start(sprint.id, {
                name: value.name,
                goal: value.goal,
                startDate: value.startDate ?? undefined,
                endDate: value.endDate ?? undefined,
                force: value.force || undefined,
              })
            : null;
    if (!request) {
      this.sprintBusy.set(false);
      return;
    }
    request.subscribe({
      next: (saved) => {
        this.sprintBusy.set(false);
        this.sprintDialogOpen.set(false);
        this.store.refresh();
        this.notify.success(
          mode === 'create'
            ? `Sprint «${saved.name}» creado`
            : mode === 'edit'
              ? 'Sprint actualizado'
              : `Sprint «${saved.name}» iniciado`,
        );
      },
      error: (error: unknown) => {
        this.sprintBusy.set(false);
        this.sprintError.set(sprintErrorMessage(asApiError(error)));
      },
    });
  }

  protected openComplete(section: BacklogSection): void {
    if (!section.sprint) return;
    const done = section.totals.byCategory.done;
    this.completeError.set(null);
    this.completeSprintTarget.set(section.sprint);
    this.completeSummary.set({
      doneIssues: done?.issues ?? 0,
      donePoints: done?.points ?? 0,
      openIssues: section.totals.issues - (done?.issues ?? 0),
      openPoints: section.totals.points - (done?.points ?? 0),
    });
    this.plannedForComplete.set(
      this.sections()
        .filter((candidate) => candidate.kind === 'planned' && candidate.sprint)
        .map((candidate) => candidate.sprint as AgileSprint),
    );
    this.completeOpen.set(true);
  }

  protected confirmComplete(input: CompleteSprintInput): void {
    const sprint = this.completeSprintTarget();
    const project = this.current.project();
    if (!sprint || !project) return;
    this.completeBusy.set(true);
    this.sprintPlan.complete(sprint.id, input).subscribe({
      next: (result) => {
        this.completeBusy.set(false);
        this.completeOpen.set(false);
        this.store.refresh();
        this.notify.success(`Sprint «${result.sprint.name}» completado`);
        void this.router.navigateByUrl(projectPath(project, `sprints/${sprint.id}/report`));
      },
      error: (error: unknown) => {
        this.completeBusy.set(false);
        this.completeError.set(sprintErrorMessage(asApiError(error)));
      },
    });
  }

  protected confirmDelete(): void {
    const sprint = this.deleteTarget();
    if (!sprint) return;
    this.deleteBusy.set(true);
    this.sprintPlan.delete(sprint.id).subscribe({
      next: () => {
        this.deleteBusy.set(false);
        this.deleteTarget.set(null);
        this.store.refresh();
        this.notify.success(`Sprint «${sprint.name}» eliminado`);
      },
      error: (error: unknown) => {
        this.deleteBusy.set(false);
        this.deleteTarget.set(null);
        this.notify.error(sprintErrorMessage(asApiError(error)));
      },
    });
  }

  protected retry(): void {
    this.scope.reload();
  }
}
