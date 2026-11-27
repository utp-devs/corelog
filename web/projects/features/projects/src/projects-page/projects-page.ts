import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  OnInit,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NgTemplateOutlet } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import {
  AuthStore,
  FavoritesStore,
  NotificationService,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  OrganizationStore,
  Project,
  ProjectArchiveFilter,
  ProjectStore,
  projectAccessVariantOf,
  projectPath,
} from '@corelog/core';
import { CardGrid } from '@corelog/ui/card-grid';
import {
  DataTable,
  DataTableCell,
  DataTableColumn,
  DataTablePageChange,
} from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { SearchInput } from '@corelog/ui/search-input';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Toolbar, ToolbarLeft } from '@corelog/ui/toolbar';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';

import { PROJECT_TEMPLATES, TEMPLATE_LABELS } from '../project-wizard/project-templates';
import {
  ALL,
  ARCHIVED_OPTIONS,
  INITIAL_LIST_STATE,
  ProjectListState,
  ProjectView,
  SORT_OPTIONS,
  VISIBILITY_LABELS,
  clearedFilters,
  createProjectRights,
  hasActiveFilters,
  toListFilters,
} from './projects-page.utils';

const VIEW_STORAGE_KEY = 'corelog.projects.view';
const SKELETON_CARDS = 6;
const DATE_FORMAT = new Intl.DateTimeFormat('es', { dateStyle: 'medium' });

const COLUMNS: DataTableColumn[] = [
  { key: 'name', title: 'Proyecto', mobilePrimary: true },
  { key: 'key', title: 'Clave', mobilePrimary: true },
  { key: 'template', title: 'Plantilla', hideable: true },
  { key: 'lead', title: 'Responsable', hideable: true },
  { key: 'updated', title: 'Última actividad', hideable: true },
];

const TEMPLATE_OPTIONS: readonly SelectFieldOption[] = [
  { value: ALL, label: 'Todas las plantillas' },
  ...PROJECT_TEMPLATES.map((item) => ({ value: item.value, label: item.label })),
];

function readStoredView(): ProjectView {
  try {
    return localStorage.getItem(VIEW_STORAGE_KEY) === 'cards' ? 'cards' : 'table';
  } catch {
    return 'table';
  }
}

/** Projects of the active organization: search, filters, sorting, table or cards, stars and paging. */
@Component({
  selector: 'app-projects-page',
  imports: [
    FormsModule,
    NgTemplateOutlet,
    RouterLink,
    Button,
    AppTooltip,
    CardGrid,
    DataTable,
    DataTableCell,
    EmptyState,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    ProjectAvatar,
    SearchInput,
    SelectField,
    Toolbar,
    ToolbarLeft,
  ],
  templateUrl: './projects-page.html',
  styleUrl: './projects-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'projects-page' },
})
export class ProjectsPage implements OnInit {
  private readonly store = inject(ProjectStore);
  private readonly favorites = inject(FavoritesStore);
  private readonly auth = inject(AuthStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly orgApi = inject(OrganizationApiService);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly router = inject(Router);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly columns = COLUMNS;
  protected readonly templateOptions = TEMPLATE_OPTIONS;
  protected readonly sortOptions = SORT_OPTIONS;
  protected readonly archivedOptions = ARCHIVED_OPTIONS;
  protected readonly templates = PROJECT_TEMPLATES;
  protected readonly skeletonCards = Array.from({ length: SKELETON_CARDS }, (_, index) => index);

  protected readonly state = signal<ProjectListState>(INITIAL_LIST_STATE);
  protected readonly view = signal<ProjectView>(readStoredView());
  private readonly leads = signal<ReadonlyMap<string, string>>(new Map());

  protected readonly items = this.store.pageItems;
  protected readonly meta = this.store.pageMeta;
  protected readonly loading = this.store.pageLoading;
  protected readonly loaded = this.store.pageLoaded;
  protected readonly error = this.store.pageError;

  protected readonly rights = computed(() =>
    createProjectRights(
      this.auth.user()?.role,
      this.orgs.currentRole(),
      this.orgs.currentOrg()?.settings.whoCanCreateProjects,
    ),
  );
  protected readonly canCreate = computed(() => this.rights().allowed);
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.error()));
  protected readonly filtersActive = computed(() => hasActiveFilters(this.state()));
  protected readonly total = computed(() => this.meta()?.total ?? 0);
  protected readonly firstLoad = computed(() => this.loading() && !this.loaded());
  protected readonly failed = computed(() => this.error() !== null && this.items().length === 0);
  protected readonly isEmpty = computed(
    () => this.loaded() && !this.failed() && this.total() === 0 && this.items().length === 0,
  );
  protected readonly neverCreated = computed(() => this.isEmpty() && !this.filtersActive());
  protected readonly errorMessage = computed(
    () => this.error()?.message ?? 'No se pudieron cargar los proyectos.',
  );
  protected readonly pagination = computed(() => ({
    page: this.state().page,
    pageSize: this.state().pageSize,
    total: this.total(),
  }));
  protected readonly totalPages = computed(() =>
    Math.max(1, Math.ceil(this.total() / this.state().pageSize)),
  );
  protected readonly leadOptions = computed<SelectFieldOption[]>(() => [
    { value: ALL, label: 'Cualquier responsable' },
    ...[...this.leads()].map(([value, label]) => ({ value, label })),
  ]);

  constructor() {
    effect(() => {
      const filters = toListFilters(this.state());
      untracked(() => this.store.fetchPage(filters));
    });
  }

  ngOnInit(): void {
    this.favorites.fetchFavorites();
    this.loadLeads();
  }

  protected leadName(project: Project): string {
    return this.leads().get(project.leadId) ?? '-';
  }

  protected templateLabel(project: Project): string {
    return TEMPLATE_LABELS[project.template ?? project.type];
  }

  protected visibilityLabel(project: Project): string {
    return VISIBILITY_LABELS[project.visibility];
  }

  protected updated(project: Project): string {
    return DATE_FORMAT.format(new Date(project.updatedAt));
  }

  protected path(project: Project): string {
    return projectPath(project);
  }

  protected isStarred(project: Project): boolean {
    return this.favorites.isStarred(project.id);
  }

  protected create(): void {
    if (this.canCreate()) void this.router.navigateByUrl('/projects/new');
  }

  protected openProject(project: Project): void {
    void this.router.navigateByUrl(projectPath(project));
  }

  protected setQuery(q: string): void {
    this.patch({ q });
  }

  protected setTemplate(value: string | null): void {
    this.patch({ template: (value ?? ALL) as ProjectListState['template'] });
  }

  protected setLead(value: string | null): void {
    this.patch({ leadId: value ?? ALL });
  }

  protected setArchived(value: string | null): void {
    this.patch({ archived: (value ?? 'exclude') as ProjectArchiveFilter });
  }

  protected setSort(value: string | null): void {
    this.patch({ sort: value ?? INITIAL_LIST_STATE.sort });
  }

  protected toggleStarredFilter(): void {
    this.patch({ starred: !this.state().starred });
  }

  protected clearFilters(): void {
    this.state.update((state) => clearedFilters(state));
  }

  protected setView(view: ProjectView): void {
    this.view.set(view);
    try {
      localStorage.setItem(VIEW_STORAGE_KEY, view);
    } catch {
      // The choice only lasts for this visit when storage is blocked.
    }
  }

  protected changePage(change: DataTablePageChange): void {
    this.state.update((state) => ({ ...state, page: change.page, pageSize: change.pageSize }));
  }

  protected goToPage(page: number): void {
    const target = Math.min(Math.max(1, page), this.totalPages());
    this.state.update((state) => ({ ...state, page: target }));
  }

  protected toggleStar(project: Project): void {
    const wasStarred = this.isStarred(project);
    const request = wasStarred
      ? this.favorites.unstar(project.id)
      : this.favorites.star({
          id: project.id,
          name: project.name,
          key: project.key,
          color: project.color,
        });
    request.pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => {
        if (wasStarred && this.state().starred) this.store.fetchPage(toListFilters(this.state()));
      },
      error: () =>
        this.notify.error(
          wasStarred
            ? 'No se pudo quitar el proyecto de favoritos.'
            : 'No se pudo marcar el proyecto como favorito.',
        ),
    });
  }

  protected retry(): void {
    this.store.fetchPage(toListFilters(this.state()));
  }

  /** Any change of a filter or of the sort goes back to the first page. */
  private patch(change: Partial<ProjectListState>): void {
    this.state.update((state) => ({ ...state, ...change, page: 1 }));
  }

  private loadLeads(): void {
    const organizationId = this.organizationId();
    if (!organizationId) return;
    this.orgApi
      .listMembers(organizationId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (members) =>
          this.leads.set(
            new Map(members.map((member) => [member.userId, member.displayName || member.name])),
          ),
        // Guests cannot read the directory: leads then show as unknown and the filter stays short.
        error: () => this.leads.set(new Map()),
      });
  }
}
