import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import {
  CurrentProjectStore,
  NotificationService,
  ReportApiService,
  ReportStore,
  SprintV2ApiService,
  VersionApiService,
  isoDate,
  projectAccessVariantOf,
  projectPath,
  type ReportQuery,
} from '@corelog/core';
import { ChartCard } from '@corelog/ui/chart-card';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import type { SelectFieldOption } from '@corelog/ui/select-field';
import { StatTile } from '@corelog/ui/stat-tile';
import { Button } from 'primeng/button';
import { map } from 'rxjs';

import { REPORT_META, isReportKind } from '../report-catalog';
import { buildReportChart, isReportEmpty } from '../report-charts';
import { REPORT_EXPORT_FAILED, isReportParamError, reportErrorMessage } from '../report-errors';
import { formatDate } from '../report-format';
import { ReportParamsBar } from '../report-params-bar/report-params-bar';
import { queryFromParams, queryToParams } from '../report-query';
import { summarySubject, summaryTiles } from '../report-summary';
import { ReportTable } from '../report-table/report-table';

const EMPTY_TABLE = { headers: [], rows: [] };

/**
 * One report of the project: parameters in the URL, summary tiles, chart, full data table and CSV export.
 * The table is the text alternative of the chart.
 */
@Component({
  selector: 'app-report-page',
  imports: [
    FormsModule,
    RouterLink,
    Button,
    ChartCard,
    EmptyState,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    StatTile,
    ReportParamsBar,
    ReportTable,
  ],
  templateUrl: './report-page.html',
  styleUrl: './report-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReportPage {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly current = inject(CurrentProjectStore);
  private readonly store = inject(ReportStore);
  private readonly api = inject(ReportApiService);
  private readonly sprintApi = inject(SprintV2ApiService);
  private readonly versionApi = inject(VersionApiService);
  private readonly notifications = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly emptyTable = EMPTY_TABLE;
  protected readonly today = isoDate(new Date());

  protected readonly kind = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('kind') ?? '')),
    {
      initialValue: '',
    },
  );
  protected readonly query = toSignal(
    this.route.queryParamMap.pipe(map((params) => queryFromParams(params))),
    { initialValue: {} as ReportQuery },
  );

  protected readonly hubPath = computed(() => {
    const project = this.current.project();
    return project ? projectPath(project, 'reports') : '/projects';
  });

  protected readonly descriptor = computed(() =>
    this.store.catalog.value()?.find((entry) => entry.kind === this.kind()),
  );
  protected readonly meta = computed(() =>
    isReportKind(this.kind()) ? REPORT_META[this.kind() as keyof typeof REPORT_META] : null,
  );
  protected readonly title = computed(
    () => this.descriptor()?.title ?? this.meta()?.title ?? 'Reporte',
  );
  protected readonly description = computed(
    () => this.descriptor()?.description ?? this.meta()?.description ?? '',
  );
  protected readonly params = computed(() => this.descriptor()?.params ?? []);

  protected readonly sprints = signal<SelectFieldOption[]>([]);
  protected readonly versions = signal<SelectFieldOption[]>([]);
  private readonly versionsLoaded = signal(false);
  /** Created-vs-resolved: also draw the accumulated difference. */
  protected readonly cumulative = signal(true);
  protected readonly exporting = signal(false);

  protected readonly report = this.store.report.value;
  protected readonly reportError = this.store.report.error;

  /** The version report needs a version: it is filled in from the project versions or explained when there are none. */
  protected readonly needsVersion = computed(
    () => this.kind() === 'version' && !this.query().version,
  );
  protected readonly noVersions = computed(
    () => this.needsVersion() && this.versionsLoaded() && this.versions().length === 0,
  );

  /** Full-page failure: no permission, unknown report, or the project/report is not visible. */
  protected readonly accessVariant = computed(() => {
    if (this.current.permissionsLoaded() && !this.current.can('view_reports')) return 403;
    const catalogError = projectAccessVariantOf(this.store.catalog.error());
    if (catalogError) return catalogError;
    if (this.store.catalog.hasValue() && !this.descriptor()) return 404;
    const error = this.reportError();
    // report.* failures (unknown sprint, version...) are explained inline, not as a missing page.
    return error && !error.code?.startsWith('report.') ? projectAccessVariantOf(error) : null;
  });

  protected readonly loading = computed(
    () =>
      this.accessVariant() === null &&
      !this.reportError() &&
      !this.noVersions() &&
      !this.store.report.hasValue(),
  );
  protected readonly errorMessage = computed(() => reportErrorMessage(this.reportError()));
  protected readonly canRetry = computed(() => !isReportParamError(this.reportError()));
  protected readonly isEmpty = computed(() => {
    const report = this.report();
    return !!report && isReportEmpty(report);
  });
  protected readonly chart = computed(() => {
    const report = this.report();
    return report && !this.isEmpty()
      ? buildReportChart(report, { cumulative: this.cumulative() })
      : null;
  });
  protected readonly tiles = computed(() => {
    const report = this.report();
    return report ? summaryTiles(report.kind, report.summary) : [];
  });
  protected readonly subject = computed(() => {
    const report = this.report();
    return report ? summarySubject(report.summary) : null;
  });
  protected readonly generated = computed(() => {
    const at = this.report()?.generatedAt;
    return at ? formatDate(at) : '';
  });

  private catalogRequestedFor: string | null = null;
  private optionsFor: string | null = null;
  private lastRun = '';

  constructor() {
    effect(() => {
      const id = this.current.projectId();
      untracked(() => {
        if (!id) return;
        if (this.catalogRequestedFor !== id && !this.store.catalog.hasValue()) {
          this.catalogRequestedFor = id;
          this.store.loadCatalog(id);
        }
      });
    });

    effect(() => {
      const id = this.current.projectId();
      const params = this.params();
      untracked(() => this.loadOptions(id, params));
    });

    effect(() => {
      const needs = this.needsVersion();
      const loaded = this.versionsLoaded();
      const first = this.versions()[0];
      if (needs && loaded && first) {
        untracked(() => this.pickVersion(first.value));
      }
    });

    effect(() => {
      const id = this.current.projectId();
      const kind = this.kind();
      const descriptor = this.descriptor();
      const query = this.query();
      const waitingForVersion = this.needsVersion();
      const denied = this.accessVariant() !== null && this.accessVariant() !== 404;
      if (!id || !kind || !descriptor || waitingForVersion || denied) return;
      const signature = JSON.stringify([id, kind, query]);
      untracked(() => {
        if (signature === this.lastRun) return;
        this.lastRun = signature;
        this.store.run(id, kind, query);
      });
    });
  }

  protected setQuery(query: ReportQuery): void {
    void this.router.navigate([], { relativeTo: this.route, queryParams: queryToParams(query) });
  }

  protected retry(): void {
    const id = this.current.projectId();
    if (!id) return;
    this.lastRun = '';
    this.store.run(id, this.kind(), this.query());
  }

  /** Downloads the table of the report as CSV with the parameters on screen. */
  protected exportCsv(): void {
    const id = this.current.projectId();
    if (!id || this.exporting()) return;
    this.exporting.set(true);
    this.api
      .csv(id, this.kind(), this.query())
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (blob) => {
          this.download(
            blob,
            `${this.current.key() ?? 'proyecto'}-${this.kind()}-${this.today}.csv`,
          );
          this.exporting.set(false);
        },
        error: (error: unknown) => {
          this.exporting.set(false);
          const code = (error as { error?: { error?: { code?: string } } })?.error?.error?.code;
          this.notifications.error(reportErrorMessage({ code }, REPORT_EXPORT_FAILED));
        },
      });
  }

  private download(blob: Blob, filename: string): void {
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  private pickVersion(version: string): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { version },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  /** Sprints and versions for the selects, only for the reports that take them. */
  private loadOptions(id: string | null, params: readonly string[]): void {
    if (!id || params.length === 0) return;
    const signature = `${id}|${params.includes('sprint')}|${params.includes('version')}`;
    if (signature === this.optionsFor) return;
    this.optionsFor = signature;
    if (params.includes('sprint')) {
      this.sprintApi
        .list(id)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({
          next: (sprints) =>
            this.sprints.set(
              [...sprints].reverse().map((sprint) => ({
                value: sprint.id,
                label: `${sprint.name}${sprint.status === 'active' ? ' (activo)' : ''}`,
              })),
            ),
          error: () => this.sprints.set([]),
        });
    }
    if (params.includes('version')) {
      this.versionApi
        .list(id)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({
          next: (versions) => {
            const open = versions.filter((version) => version.status === 'unreleased');
            const ordered = [...open, ...versions.filter((version) => !open.includes(version))];
            this.versions.set(
              ordered.map((version) => ({ value: version.id, label: version.name })),
            );
            this.versionsLoaded.set(true);
          },
          error: () => {
            this.versions.set([]);
            this.versionsLoaded.set(true);
          },
        });
    }
  }
}
