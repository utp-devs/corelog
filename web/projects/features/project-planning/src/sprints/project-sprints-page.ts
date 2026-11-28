import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  AgileSprint,
  CurrentProjectStore,
  SprintApiService,
  SprintPlanStore,
  SprintStatus,
  VelocityPoint,
  projectAccessVariantOf,
  projectPath,
} from '@corelog/core';
import { ArchivedNotice } from '@corelog/ui/archived-notice';
import { ChartCard, ChartCardSeries, ChartTable } from '@corelog/ui/chart-card';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';

import {
  SPRINT_STATUS_LABELS,
  SPRINT_STATUS_TONES,
  formatPoints,
  sprintDatesOf,
} from '../shared/planning-format';
import { trackProjectScope } from '../shared/project-scope';

const LOAD_ERROR_MESSAGE =
  'No se pudieron cargar los sprints. Verifica que tengas acceso al proyecto e intenta de nuevo.';

interface SprintGroup {
  readonly status: SprintStatus;
  readonly title: string;
  readonly sprints: readonly AgileSprint[];
}

/**
 * Sprints of the project in three groups (active, planned, closed), each with its dates, goal and points, and
 * a link to its report. Planning itself happens in the backlog.
 */
@Component({
  selector: 'app-project-sprints-page',
  imports: [
    ArchivedNotice,
    ChartCard,
    EmptyState,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    PageContainer,
    PageHeader,
    RouterLink,
  ],
  templateUrl: './project-sprints-page.html',
  styleUrl: './project-sprints-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectSprintsPage {
  private readonly store = inject(SprintPlanStore);
  private readonly velocityApi = inject(SprintApiService);
  private readonly current = inject(CurrentProjectStore);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly loadErrorMessage = LOAD_ERROR_MESSAGE;
  protected readonly velocity = signal<readonly VelocityPoint[]>([]);

  protected readonly scope = trackProjectScope(this.current, this.destroyRef, (projectId) => {
    this.store.loadSprints(projectId);
    this.velocityApi.velocity(projectId).subscribe({
      next: (points) => this.velocity.set([...points].reverse()),
      error: () => this.velocity.set([]),
    });
  });

  private readonly failure = computed(() => this.scope.accessError() ?? this.store.sprints.error());
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.failure()));
  protected readonly failed = computed(() => this.failure() !== null);
  protected readonly ready = computed(
    () => this.store.sprints.value() !== undefined && !this.failed(),
  );

  protected readonly groups = computed<SprintGroup[]>(() => {
    const sprints = this.store.sprints.value() ?? [];
    const order: [SprintStatus, string][] = [
      ['active', 'Sprint activo'],
      ['planned', 'Planificados'],
      ['closed', 'Cerrados'],
    ];
    return order
      .map(([status, title]) => ({
        status,
        title,
        sprints: sprints.filter((sprint) => sprint.status === status),
      }))
      .filter((group) => group.sprints.length > 0);
  });
  protected readonly isEmpty = computed(() => (this.store.sprints.value()?.length ?? 0) === 0);

  protected readonly velocityLabels = computed(() => this.velocity().map((point) => point.name));
  protected readonly velocitySeries = computed<ChartCardSeries[]>(() => [
    { label: 'Puntos completados', data: this.velocity().map((point) => point.points) },
  ]);
  protected readonly velocityTable = computed<ChartTable>(() => ({
    headers: ['Sprint', 'Puntos completados'],
    rows: this.velocity().map((point) => [point.name, point.points]),
  }));

  protected readonly backlogPath = computed(() => this.pathOf('backlog'));
  protected readonly boardPath = computed(() => this.pathOf('board'));

  protected statusLabel(status: SprintStatus): string {
    return SPRINT_STATUS_LABELS[status];
  }

  protected tone(status: SprintStatus): (typeof SPRINT_STATUS_TONES)[SprintStatus] {
    return SPRINT_STATUS_TONES[status];
  }

  protected dates(sprint: AgileSprint): string {
    return sprintDatesOf(sprint);
  }

  protected points(sprint: AgileSprint): string {
    if (sprint.status === 'closed') {
      return `${formatPoints(sprint.completedPoints ?? 0)} de ${formatPoints(sprint.committedPoints ?? 0)} pts`;
    }
    return sprint.committedPoints !== null
      ? `${formatPoints(sprint.committedPoints)} pts comprometidos`
      : '';
  }

  protected reportPath(sprint: AgileSprint): string {
    return this.pathOf(`sprints/${sprint.id}/report`);
  }

  protected retry(): void {
    this.scope.reload();
  }

  private pathOf(section: string): string {
    const project = this.current.project();
    return project ? projectPath(project, section) : '/';
  }
}
