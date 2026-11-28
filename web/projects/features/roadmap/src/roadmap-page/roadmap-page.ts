import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import {
  BreakpointService,
  CurrentProjectStore,
  NotificationService,
  RoadmapBar,
  RoadmapFilter,
  RoadmapGroupBy,
  RoadmapStore,
  TIMELINE_ZOOMS,
  TimelineZoom,
  buildScale,
  dayNumber,
  defaultRange,
  issuePath,
  projectPath,
} from '@corelog/core';
import { ArchivedNotice } from '@corelog/ui/archived-notice';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { SearchInput } from '@corelog/ui/search-input';
import { Timeline } from '@corelog/ui/timeline';

import {
  GROUP_LABELS,
  describeBar,
  formatDay,
  formatRange,
  statusLabel,
  toBand,
  toTimelineRow,
} from '../roadmap-format';

type RoadmapView = 'timeline' | 'table';

interface Choice<T extends string> {
  readonly value: T;
  readonly label: string;
}

const ZOOM_LABELS: Readonly<Record<TimelineZoom, string>> = {
  week: 'Semanas',
  month: 'Meses',
  quarter: 'Trimestres',
};

/** Phones show a list with progress bars instead of the timeline. */
const PHONE_QUERY = '(max-width: 767.98px)';

/**
 * Roadmap of a project: epics (or versions or sprints) on a timeline with progress, dependencies and a
 * manual order, a section for the items without dates and an equivalent table view.
 */
@Component({
  selector: 'app-roadmap-page',
  imports: [
    ArchivedNotice,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    ProgressBar,
    RouterLink,
    SearchInput,
    Timeline,
  ],
  templateUrl: './roadmap-page.html',
  styleUrl: './roadmap-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RoadmapPage {
  protected readonly store = inject(RoadmapStore);
  private readonly current = inject(CurrentProjectStore);
  private readonly router = inject(Router);
  private readonly notifications = inject(NotificationService);
  private readonly phone = inject(BreakpointService).matches(PHONE_QUERY);

  protected readonly zoom = signal<TimelineZoom>('month');
  protected readonly view = signal<RoadmapView>('timeline');

  protected readonly groupChoices: readonly Choice<RoadmapGroupBy>[] = (
    Object.keys(GROUP_LABELS) as RoadmapGroupBy[]
  ).map((value) => ({ value, label: GROUP_LABELS[value] }));
  protected readonly filterChoices: readonly Choice<RoadmapFilter>[] = [
    { value: 'all', label: 'Todas' },
    { value: 'open', label: 'Abiertas' },
    { value: 'done', label: 'Terminadas' },
  ];
  protected readonly zoomChoices: readonly Choice<TimelineZoom>[] = TIMELINE_ZOOMS.map((value) => ({
    value,
    label: ZOOM_LABELS[value],
  }));
  protected readonly viewChoices: readonly Choice<RoadmapView>[] = [
    { value: 'timeline', label: 'Línea de tiempo' },
    { value: 'table', label: 'Vista de tabla' },
  ];

  protected readonly ready = computed(() => this.store.roadmap.hasValue());
  protected readonly failed = computed(() => this.store.roadmap.error() !== null);
  protected readonly isArchived = this.current.isArchived;

  /** Phones always get the list, and the table view is the same list wide. */
  protected readonly showTimeline = computed(() => this.view() === 'timeline' && !this.phone());
  protected readonly isPhone = this.phone;

  protected readonly hasAny = computed(() => this.store.bars().length > 0);
  protected readonly filtering = computed(
    () => this.store.filter() !== 'all' || this.store.query().trim() !== '',
  );

  protected readonly rows = computed(() => this.store.dated().map(toTimelineRow));
  protected readonly undated = computed(() => this.store.undated());
  protected readonly bands = computed(() => {
    const roadmap = this.store.roadmap.value();
    if (!roadmap) return [];
    const group = roadmap.groupBy;
    const bars = group === 'epic' ? [...roadmap.versions, ...roadmap.sprints] : [];
    const other =
      group === 'version' ? roadmap.sprints : group === 'sprint' ? roadmap.versions : [];
    return [...bars, ...other].filter((bar) => bar.dated).map(toBand);
  });
  protected readonly dependencies = computed(() =>
    this.store.groupBy() === 'epic' ? (this.store.roadmap.value()?.dependencies ?? []) : [],
  );

  protected readonly scale = computed(() => {
    const range = defaultRange([...this.store.dated(), ...this.bands()]);
    return buildScale(range.from, range.to, this.zoom());
  });

  protected readonly reorderable = computed(
    () => this.store.groupBy() === 'epic' && this.current.can('edit_issue') && !this.isArchived(),
  );

  protected readonly columnTitle = computed(() => GROUP_LABELS[this.store.groupBy()]);
  protected readonly emptyTitle = computed(() => {
    switch (this.store.groupBy()) {
      case 'version':
        return 'Aún no hay versiones';
      case 'sprint':
        return 'Aún no hay sprints';
      default:
        return 'Aún no hay épicas';
    }
  });

  constructor() {
    effect(() => {
      const id = this.current.projectId();
      if (id) untracked(() => this.store.load(id));
    });
  }

  protected retry(): void {
    this.current.reload();
    this.store.reload();
  }

  protected bar(id: string): RoadmapBar | undefined {
    return this.store.bars().find((bar) => bar.id === id);
  }

  /** Epics open their issue and sprints their report; versions have no page of their own. */
  protected activate(id: string): void {
    const bar = this.bar(id);
    if (!bar) return;
    void this.router.navigateByUrl(this.linkOf(bar) ?? this.router.url);
  }

  protected linkOf(bar: RoadmapBar): string | null {
    if (bar.kind === 'epic') return bar.key ? issuePath(bar.key) : issuePath(bar.id);
    if (bar.kind === 'sprint') {
      const key = this.current.key();
      return key ? `${projectPath(key, 'sprints')}/${bar.id}/report` : null;
    }
    return null;
  }

  protected reorder(order: readonly string[]): void {
    this.store.rank(order).subscribe({
      error: () =>
        this.notifications.error('No se pudo guardar el orden. Se restauró el orden anterior.'),
    });
  }

  protected print(): void {
    window.print();
  }

  protected describe(bar: RoadmapBar): string {
    return describeBar(bar);
  }

  protected status(bar: RoadmapBar): string {
    return statusLabel(bar);
  }

  protected range(bar: RoadmapBar): string {
    return formatRange(bar.start, bar.end);
  }

  protected day(iso: string | null): string {
    return dayNumber(iso) === null ? 'Sin fecha' : formatDay(iso);
  }

  protected tone(bar: RoadmapBar): 'brand' | 'success' {
    if (bar.statusCategory === 'done') return 'success';
    return 'brand';
  }
}
