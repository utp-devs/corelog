import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  CurrentProjectStore,
  SprintPlanStore,
  SprintReport,
  SprintReportIssue,
  projectAccessVariantOf,
  projectPath,
} from '@corelog/core';
import { ChartCard, ChartCardSeries, ChartTable } from '@corelog/ui/chart-card';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { StatTile } from '@corelog/ui/stat-tile';
import { Skeleton } from 'primeng/skeleton';
import { map } from 'rxjs';

import {
  SPRINT_STATUS_LABELS,
  SPRINT_STATUS_TONES,
  formatDay,
  formatPoints,
  parseDay,
  sprintDatesOf,
} from '../shared/planning-format';

const SHORT_DAY = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short' });
const LOAD_ERROR_MESSAGE = 'No se pudo cargar el informe del sprint. Intenta de nuevo.';

/** One list of the report: its title, the issues and a line explaining it. */
interface ReportList {
  readonly id: 'completed' | 'incomplete' | 'removed' | 'added';
  readonly title: string;
  readonly description: string;
  readonly issues: readonly SprintReportIssue[];
  readonly points: number;
}

function shortDay(value: string): string {
  const date = parseDay(value);
  return date ? SHORT_DAY.format(date).replace('.', '') : value;
}

function pointsOf(issues: readonly SprintReportIssue[]): number {
  return issues.reduce((total, issue) => total + (issue.points ?? 0), 0);
}

/**
 * Report of a sprint: committed against completed points, the burndown with its scope line, what was
 * completed, left unfinished, removed and added after the start, and the velocity of the closed sprints.
 */
@Component({
  selector: 'app-sprint-report-page',
  imports: [
    ChartCard,
    EmptyState,
    ErrorPage,
    ErrorState,
    Lozenge,
    PageContainer,
    PageHeader,
    RouterLink,
    Skeleton,
    StatTile,
  ],
  templateUrl: './sprint-report-page.html',
  styleUrl: './sprint-report-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SprintReportPage {
  private readonly store = inject(SprintPlanStore);
  private readonly current = inject(CurrentProjectStore);
  private readonly route = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly report = this.store.report.value;
  protected readonly loadErrorMessage = LOAD_ERROR_MESSAGE;
  private readonly sprintId = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('id'))),
    {
      initialValue: this.route.snapshot.paramMap.get('id'),
    },
  );

  private readonly failure = computed(() => this.current.error() ?? this.store.report.error());
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.failure()));
  protected readonly failed = computed(() => this.failure() !== null);
  protected readonly ready = computed(() => this.report() !== undefined && !this.failed());

  protected readonly sprint = computed(() => this.report()?.sprint ?? null);
  protected readonly statusLabel = computed(() => {
    const sprint = this.sprint();
    return sprint ? SPRINT_STATUS_LABELS[sprint.status] : '';
  });
  protected readonly statusTone = computed(() => {
    const sprint = this.sprint();
    return sprint ? SPRINT_STATUS_TONES[sprint.status] : 'neutral';
  });
  protected readonly dates = computed(() => {
    const sprint = this.sprint();
    return sprint ? sprintDatesOf(sprint) : '';
  });
  protected readonly isPlanned = computed(() => this.sprint()?.status === 'planned');
  protected readonly backlogPath = computed(() => this.pathOf('backlog'));
  protected readonly boardPath = computed(() => this.pathOf('board'));
  protected readonly sprintsPath = computed(() => this.pathOf('sprints'));

  protected readonly tiles = computed(() => {
    const report = this.report();
    if (!report) return [];
    return [
      {
        label: 'Comprometidos',
        value: formatPoints(report.committedPoints),
        hint: 'puntos al iniciar',
      },
      {
        label: 'Completados',
        value: formatPoints(report.completedPoints),
        hint: 'puntos terminados',
      },
      {
        label: 'Sin terminar',
        value: formatPoints(report.incompletePoints),
        hint: 'puntos pendientes',
      },
      { label: 'Añadidos', value: formatPoints(report.addedPoints), hint: 'después de empezar' },
      {
        label: 'Retirados',
        value: formatPoints(report.removedPoints),
        hint: 'puntos fuera del sprint',
      },
    ];
  });

  protected readonly lists = computed<ReportList[]>(() => {
    const report = this.report();
    if (!report) return [];
    return [
      {
        id: 'completed',
        title: 'Completadas',
        description: 'Incidencias terminadas dentro del sprint.',
        issues: report.completed,
        points: pointsOf(report.completed),
      },
      {
        id: 'incomplete',
        title: 'Sin terminar',
        description: 'Siguen abiertas al final del sprint.',
        issues: report.incomplete,
        points: pointsOf(report.incomplete),
      },
      {
        id: 'added',
        title: 'Añadidas después de empezar',
        description: 'No estaban en el compromiso inicial.',
        issues: report.addedAfterStart,
        points: pointsOf(report.addedAfterStart),
      },
      {
        id: 'removed',
        title: 'Retiradas',
        description: 'Salieron del sprint antes de cerrarlo.',
        issues: report.removed,
        points: pointsOf(report.removed),
      },
    ];
  });

  protected readonly burndownLabels = computed(() =>
    (this.report()?.burndown ?? []).map((point) => shortDay(point.date)),
  );
  protected readonly burndownSeries = computed<ChartCardSeries[]>(() => {
    const points = this.report()?.burndown ?? [];
    const hasIdeal = points.some((point) => point.ideal !== null);
    const series: ChartCardSeries[] = [];
    if (hasIdeal) {
      series.push({ label: 'Ideal', data: points.map((point) => point.ideal ?? 0), dashed: true });
    }
    series.push({ label: 'Restante', data: points.map((point) => point.remaining) });
    series.push({ label: 'Alcance', data: points.map((point) => point.scope), dashed: true });
    return series;
  });
  protected readonly burndownTable = computed<ChartTable>(() => ({
    headers: ['Fecha', 'Restante', 'Alcance', 'Completado'],
    rows: (this.report()?.burndown ?? []).map((point) => [
      formatDay(point.date),
      point.remaining,
      point.scope,
      point.completed,
    ]),
  }));

  protected readonly velocityLabels = computed(() =>
    (this.report()?.velocity ?? []).map((entry) => entry.name),
  );
  protected readonly velocitySeries = computed<ChartCardSeries[]>(() => [
    {
      label: 'Puntos completados',
      data: (this.report()?.velocity ?? []).map((entry) => entry.points),
    },
  ]);
  protected readonly velocityTable = computed<ChartTable>(() => ({
    headers: ['Sprint', 'Puntos completados'],
    rows: (this.report()?.velocity ?? []).map((entry) => [entry.name, entry.points]),
  }));

  constructor() {
    effect(() => {
      const id = this.sprintId();
      if (id) untracked(() => this.store.loadReport(id));
    });
    this.destroyRef.onDestroy(() => this.store.report.reset());
  }

  protected href(issue: SprintReportIssue): string {
    return `/browse/${issue.key}`;
  }

  protected retry(): void {
    const id = this.sprintId();
    if (id) this.store.loadReport(id);
  }

  protected asReport(): SprintReport | undefined {
    return this.report();
  }

  private pathOf(section: string): string {
    const project = this.current.project();
    return project ? projectPath(project, section) : '/';
  }
}
