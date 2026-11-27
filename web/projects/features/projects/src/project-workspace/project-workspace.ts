import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, NavigationEnd, Router, RouterLink, RouterOutlet } from '@angular/router';
import {
  Project,
  ProjectAccessVariant,
  ProjectApiService,
  asApiError,
  leadNameOf,
  projectAccessVariantOf,
  pushRecent,
} from '@corelog/core';
import { BreadcrumbItem, Breadcrumbs } from '@corelog/ui/breadcrumbs';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { filter, map } from 'rxjs';

import { TYPE_LABELS, VISIBILITY_LABELS } from '../projects-page/projects-page.utils';
import { projectLeadLookup } from '../shared/project-lead-lookup';

/** Sections of the project. Each one is a child route of the workspace. */
const SECTIONS: { key: string; label: string }[] = [
  { key: 'board', label: 'Tablero' },
  { key: 'backlog', label: 'Backlog' },
  { key: 'sprints', label: 'Sprints' },
  { key: 'calendar', label: 'Calendario' },
  { key: 'permisos', label: 'Permisos' },
];

const LOAD_FAILED_MESSAGE =
  'No se pudo abrir el proyecto. Verifica que tengas acceso e intenta de nuevo.';

type LoadState =
  | { readonly status: 'loading' }
  | { readonly status: 'ready'; readonly project: Project }
  | { readonly status: 'error'; readonly variant: ProjectAccessVariant | null };

/** Shared header of a project: identity, breadcrumb and the section navigation. Sections render below it. */
@Component({
  selector: 'app-project-workspace',
  imports: [
    RouterLink,
    RouterOutlet,
    Breadcrumbs,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    ProjectAvatar,
  ],
  templateUrl: './project-workspace.html',
  styleUrl: './project-workspace.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectWorkspace {
  private readonly api = inject(ProjectApiService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  protected readonly sections = SECTIONS;
  protected readonly loadFailedMessage = LOAD_FAILED_MESSAGE;
  protected readonly id = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('id') ?? '')),
    { requireSync: true },
  );

  /** Incremented by the retry action to load the project again. */
  private readonly attempt = signal(0);
  protected readonly state = signal<LoadState>({ status: 'loading' });

  protected readonly project = computed(() => {
    const state = this.state();
    return state.status === 'ready' ? state.project : null;
  });

  /** Access failures replace the whole workspace with the full-page error. */
  protected readonly accessVariant = computed(() => {
    const state = this.state();
    return state.status === 'error' ? state.variant : null;
  });

  protected readonly failed = computed(() => this.state().status === 'error');

  private readonly url = toSignal(
    this.router.events.pipe(
      filter((event): event is NavigationEnd => event instanceof NavigationEnd),
      map(() => this.router.url),
    ),
    { initialValue: this.router.url },
  );

  protected readonly activeKey = computed(() => {
    const prefix = `/projects/${this.id()}/`;
    const url = this.url();
    return SECTIONS.find((section) => url.startsWith(prefix + section.key))?.key ?? 'board';
  });

  private readonly activeLabel = computed(
    () => SECTIONS.find((section) => section.key === this.activeKey())?.label ?? 'Tablero',
  );

  protected readonly breadcrumbs = computed<BreadcrumbItem[]>(() => [
    { label: 'Proyectos', to: '/projects' },
    { label: this.project()?.key ?? this.activeLabel() },
  ]);

  protected readonly lead = projectLeadLookup(computed(() => this.project()?.organizationId));

  constructor() {
    // Loads the project for the route id. Errors are kept unwrapped so the access status survives.
    effect((onCleanup) => {
      const id = this.id();
      this.attempt();
      if (!id) return;
      this.state.set({ status: 'loading' });
      const subscription = this.api.get(id).subscribe({
        next: (project) => this.state.set({ status: 'ready', project }),
        error: (error: unknown) =>
          this.state.set({ status: 'error', variant: projectAccessVariantOf(asApiError(error)) }),
      });
      onCleanup(() => subscription.unsubscribe());
    });

    // Recents: the project once its header has loaded.
    effect(() => {
      const project = this.project();
      if (project) {
        pushRecent({
          kind: 'project',
          id: project.id,
          title: project.name,
          subtitle: project.key,
          link: `/projects/${project.id}`,
        });
      }
    });
  }

  protected leadName(project: Project): string {
    return leadNameOf(project.leadId, this.lead.members(), this.lead.status());
  }

  protected visibilityLabel(project: Project): string {
    return VISIBILITY_LABELS[project.visibility];
  }

  protected typeLabel(project: Project): string {
    return TYPE_LABELS[project.type];
  }

  protected isActive(key: string): boolean {
    return this.activeKey() === key;
  }

  protected retry(): void {
    this.attempt.update((value) => value + 1);
  }
}
