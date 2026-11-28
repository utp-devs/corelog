import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  untracked,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  CurrentProjectStore,
  ReportStore,
  projectAccessVariantOf,
  projectPath,
} from '@corelog/core';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';

import { REPORT_GROUPS, REPORT_META, isReportKind, type ReportGroupId } from '../report-catalog';
import { REPORT_LOAD_FAILED } from '../report-errors';

interface HubCard {
  readonly kind: string;
  readonly title: string;
  readonly description: string;
  readonly icon: string;
  readonly path: string;
}

interface HubGroup {
  readonly id: ReportGroupId;
  readonly title: string;
  readonly cards: readonly HubCard[];
}

/** Gallery of the reports of a project, grouped by theme. Only the reports the server lists are shown. */
@Component({
  selector: 'app-reports-hub-page',
  imports: [RouterLink, ErrorPage, ErrorState, LoadingSkeleton, PageContainer, PageHeader],
  templateUrl: './reports-hub-page.html',
  styleUrl: './reports-hub-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReportsHubPage {
  private readonly current = inject(CurrentProjectStore);
  private readonly store = inject(ReportStore);

  protected readonly loadFailedMessage = REPORT_LOAD_FAILED;
  protected readonly catalog = this.store.catalog;

  /** 403 or 404 as a full page: from the permissions of the project or from the catalog request. */
  protected readonly accessVariant = computed(() => {
    if (this.current.permissionsLoaded() && !this.current.can('view_reports')) return 403;
    return projectAccessVariantOf(this.catalog.error());
  });
  protected readonly failed = computed(
    () => this.catalog.error() !== null && this.accessVariant() === null,
  );
  protected readonly loading = computed(
    () => !this.failed() && this.accessVariant() === null && !this.catalog.hasValue(),
  );

  protected readonly groups = computed<HubGroup[]>(() => {
    const project = this.current.project();
    const listed = this.catalog.value();
    if (!project || !listed) return [];
    const byKind = new Map(listed.map((descriptor) => [descriptor.kind, descriptor]));
    const known: HubGroup[] = REPORT_GROUPS.map((group) => ({
      id: group.id,
      title: group.title,
      cards: group.kinds
        .filter((kind) => byKind.has(kind))
        .map((kind): HubCard => {
          const descriptor = byKind.get(kind);
          const meta = REPORT_META[kind];
          return {
            kind,
            title: descriptor?.title || meta.title,
            description: descriptor?.description || meta.description,
            icon: meta.icon,
            path: projectPath(project, `reports/${kind}`),
          };
        }),
    }));
    return [...known, ...this.unknownCards(byKind, project)].filter(
      (group) => group.cards.length > 0,
    );
  });

  private lastId: string | null = null;

  constructor() {
    effect(() => {
      const id = this.current.projectId();
      untracked(() => this.load(id));
    });
  }

  protected retry(): void {
    this.lastId = null;
    this.load(this.current.projectId());
  }

  private load(id: string | null): void {
    if (!id || id === this.lastId) return;
    this.lastId = id;
    this.store.loadCatalog(id);
  }

  /** Reports the server lists that this client does not know yet go to a trailing group. */
  private unknownCards(
    byKind: Map<string, { kind: string; title: string; description: string }>,
    project: { id: string; key?: string | null },
  ): HubGroup[] {
    const extra = [...byKind.values()].filter((descriptor) => !isReportKind(descriptor.kind));
    return [
      {
        id: 'other',
        title: 'Otros',
        cards: extra.map((descriptor) => ({
          kind: descriptor.kind,
          title: descriptor.title,
          description: descriptor.description,
          icon: 'pi pi-chart-bar',
          path: projectPath(project, `reports/${descriptor.kind}`),
        })),
      },
    ];
  }
}
