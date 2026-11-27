import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject } from '@angular/core';
import { RouterLink } from '@angular/router';

import {
  AsyncState,
  BreakpointService,
  DashboardIssue,
  DashboardSprint,
  DashboardStore,
  Project,
  ProjectApiService,
  RecentItem,
  getRecent,
} from '@corelog/core';
import { CardGrid } from '@corelog/ui/card-grid';
import { ChartCard } from '@corelog/ui/chart-card';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { Section } from '@corelog/ui/section';
import { StatTile } from '@corelog/ui/stat-tile';

/** Below 768 px the panels stack in one column. */
const MOBILE_QUERY = '(max-width: 767px)';

/**
 * Personal dashboard: assigned work, urgent issues, active sprints, projects and recent items.
 * Without assigned work or recent issues it shows the first steps instead.
 */
@Component({
  selector: 'app-my-dashboard-page',
  imports: [
    CardGrid,
    ChartCard,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    PageContainer,
    PageHeader,
    ProjectAvatar,
    RouterLink,
    Section,
    StatTile,
  ],
  templateUrl: './my-dashboard-page.html',
  styleUrl: './my-dashboard-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MyDashboardPage {
  protected readonly dashboard = inject(DashboardStore);
  private readonly projectApi = inject(ProjectApiService);
  private readonly breakpoints = inject(BreakpointService);

  private readonly projectsRead = new AsyncState<Project[]>();

  protected readonly isMobile = this.breakpoints.matches(MOBILE_QUERY);

  /** Recent items from the local history, read once when the page opens. */
  protected readonly recent: readonly RecentItem[] = getRecent();

  protected readonly summary = computed(() => this.dashboard.summary.value());
  protected readonly ready = computed(() => this.dashboard.summary.hasValue());
  protected readonly showSkeleton = computed(
    () => !this.dashboard.summary.hasValue() && !this.dashboard.errorMessage(),
  );

  protected readonly statusRows = this.dashboard.statusRows;
  protected readonly statusLabels = computed(() => this.statusRows().map((row) => row.label));
  protected readonly statusCounts = computed(() => this.statusRows().map((row) => row.count));
  protected readonly statusSeries = computed(() => [
    { label: 'Incidencias', data: this.statusCounts() },
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

  protected sprintLabel(sprint: DashboardSprint): string {
    return `${sprint.projectKey} · ${sprint.name}${sprint.endDate ? ` (fin ${sprint.endDate})` : ''}`;
  }

  protected issueMeta(issue: DashboardIssue): string {
    return `${issue.projectKey}${issue.dueDate ? ` · vence ${issue.dueDate}` : ''}`;
  }

  private loadProjects(): void {
    this.projectsRead.load(this.projectApi.list());
  }
}
