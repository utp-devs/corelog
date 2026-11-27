import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject } from '@angular/core';
import { RouterLink } from '@angular/router';

import {
  AsyncState,
  AuthStore,
  DashboardIssue,
  DashboardSprint,
  DashboardStore,
  IssuePriority,
  IssueStatus,
  Project,
  ProjectApiService,
  RecentItem,
  getRecent,
} from '@corelog/core';
import { ChartCard } from '@corelog/ui/chart-card';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { Section } from '@corelog/ui/section';
import { StatTile } from '@corelog/ui/stat-tile';
import { StatusTag } from '@corelog/ui/status-tag';

/** One step of the getting-started checklist; `done` comes from real data. */
export interface GettingStartedStep {
  readonly id: string;
  readonly done: boolean;
}

/** Chart color of each API status, taken from the status category tokens (the lozenge colors). */
const STATUS_COLORS: Readonly<Record<string, string>> = {
  open: 'var(--cl-cat-todo)',
  in_progress: 'var(--cl-cat-progress)',
  resolved: 'var(--cl-cat-done)',
  closed: 'var(--cl-cat-done)',
};
const FALLBACK_STATUS_COLOR = 'var(--cl-cat-todo)';

const STATUSES: readonly string[] = ['open', 'in_progress', 'resolved', 'closed'];
const PRIORITIES: readonly string[] = ['low', 'medium', 'high', 'critical'];

const SHORT_DATE = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short' });
const LONG_DATE = new Intl.DateTimeFormat('es', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
});
const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})/;

/** Short Spanish date ("25 nov") for an ISO day; other text is returned unchanged. */
function shortDate(value: string): string {
  const match = ISO_DAY.exec(value);
  if (!match) return value;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return SHORT_DATE.format(date).replace('.', '');
}

function todayIso(): string {
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/**
 * Personal dashboard: greeting, figures, assigned work, status chart, recent items, projects and a
 * getting-started checklist that disappears once its steps are done.
 */
@Component({
  selector: 'app-my-dashboard-page',
  imports: [
    ChartCard,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    PageContainer,
    PageHeader,
    PriorityTag,
    ProgressBar,
    ProjectAvatar,
    RouterLink,
    Section,
    StatTile,
    StatusTag,
  ],
  templateUrl: './my-dashboard-page.html',
  styleUrl: './my-dashboard-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MyDashboardPage {
  protected readonly dashboard = inject(DashboardStore);
  private readonly auth = inject(AuthStore);
  private readonly projectApi = inject(ProjectApiService);

  private readonly projectsRead = new AsyncState<Project[]>();

  /** Recent items from the local history, read once when the page opens. */
  protected readonly recent: readonly RecentItem[] = getRecent();

  protected readonly summary = computed(() => this.dashboard.summary.value());
  protected readonly ready = computed(() => this.dashboard.summary.hasValue());
  protected readonly showSkeleton = computed(
    () => !this.dashboard.summary.hasValue() && !this.dashboard.errorMessage(),
  );

  protected readonly subtitle = computed(() => {
    const name = this.auth.displayName();
    const today = LONG_DATE.format(new Date());
    return `${name ? `Hola, ${name}` : 'Hola'}. Hoy es ${today}.`;
  });

  protected readonly statusRows = this.dashboard.statusRows;
  protected readonly statusLabels = computed(() => this.statusRows().map((row) => row.label));
  protected readonly statusSeries = computed(() => [
    {
      label: 'Incidencias',
      data: this.statusRows().map((row) => row.count),
      colors: this.statusRows().map((row) => STATUS_COLORS[row.status] ?? FALLBACK_STATUS_COLOR),
    },
  ]);
  protected readonly statusTable = computed(() => ({
    headers: ['Estado', 'Incidencias'],
    rows: this.statusRows().map((row) => [row.label, row.count] as Array<string | number>),
  }));

  protected readonly projectsSkeleton = computed(
    () => !this.projectsRead.hasValue() && !this.projectsRead.error(),
  );
  protected readonly projectsFailed = computed(() => this.projectsRead.error() !== null);
  protected readonly projects = computed(() => this.projectsRead.value() ?? []);

  /** Checklist from real data; empty until the projects are known, so it never flashes. */
  protected readonly steps = computed<readonly GettingStartedStep[]>(() => {
    const data = this.summary();
    if (!data || !this.projectsRead.hasValue()) return [];
    const hasProject = this.projects().length > 0;
    const hasIssue =
      data.assignedOpen > 0 ||
      data.recentIssues.length > 0 ||
      this.recent.some((item) => item.kind === 'issue');
    const hasExplored = hasIssue || this.recent.some((item) => item.kind === 'project');
    return [
      { id: 'project', done: hasProject },
      { id: 'issue', done: hasIssue },
      { id: 'board', done: hasExplored },
    ];
  });
  protected readonly stepsDone = computed(() => this.steps().filter((step) => step.done).length);
  protected readonly stepsPercent = computed(() =>
    this.steps().length === 0 ? 0 : (this.stepsDone() / this.steps().length) * 100,
  );
  protected readonly showGettingStarted = computed(
    () => this.steps().length > 0 && this.stepsDone() < this.steps().length,
  );

  constructor() {
    this.dashboard.load();
    this.loadProjects();
    inject(DestroyRef).onDestroy(() => {
      this.dashboard.summary.cancel();
      this.projectsRead.cancel();
    });
  }

  protected retry(): void {
    this.dashboard.retry();
    this.loadProjects();
  }

  protected stepDone(id: string): boolean {
    return this.steps().find((step) => step.id === id)?.done ?? false;
  }

  protected sprintLabel(sprint: DashboardSprint): string {
    return `${sprint.projectKey} · ${sprint.name}`;
  }

  protected sprintEnd(sprint: DashboardSprint): string | null {
    return sprint.endDate ? `Termina el ${shortDate(sprint.endDate)}` : null;
  }

  protected dueLabel(issue: DashboardIssue): string | null {
    return issue.dueDate ? `Vence el ${shortDate(issue.dueDate)}` : null;
  }

  protected isOverdue(issue: DashboardIssue): boolean {
    return issue.dueDate !== undefined && issue.dueDate.slice(0, 10) < todayIso();
  }

  protected statusOf(issue: DashboardIssue): IssueStatus | null {
    return STATUSES.includes(issue.status) ? (issue.status as IssueStatus) : null;
  }

  protected priorityOf(issue: DashboardIssue): IssuePriority | null {
    return PRIORITIES.includes(issue.priority) ? (issue.priority as IssuePriority) : null;
  }

  private loadProjects(): void {
    this.projectsRead.load(this.projectApi.list());
  }
}
