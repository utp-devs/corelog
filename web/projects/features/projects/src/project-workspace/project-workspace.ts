import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, NavigationEnd, Router, RouterLink, RouterOutlet } from '@angular/router';
import {
  Project,
  ProjectAccessVariant,
  ProjectStore,
  leadNameOf,
  projectAccessVariantOf,
  pushRecent,
  useOrgMembersById,
} from '@corelog/core';
import { BreadcrumbItem, Breadcrumbs } from '@corelog/ui/breadcrumbs';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { filter, map } from 'rxjs';

import { TYPE_LABELS, VISIBILITY_LABELS } from '../projects-page/projects-page.utils';

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
  private readonly projects = inject(ProjectStore);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  protected readonly sections = SECTIONS;
  protected readonly loadFailedMessage = LOAD_FAILED_MESSAGE;
  protected readonly id = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('id') ?? '')),
    { requireSync: true },
  );

  /** Loading, failure or the project, derived from the store read of the route project. */
  protected readonly state = computed<LoadState>(() => {
    const error = this.projects.projectError();
    if (error) {
      return { status: 'error', variant: projectAccessVariantOf(error) };
    }
    const project = this.projects.project();
    return project ? { status: 'ready', project } : { status: 'loading' };
  });

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

  protected readonly lead = useOrgMembersById(computed(() => this.project()?.organizationId));

  constructor() {
    // Loads the project for the route id. The store keeps the failure so the access status survives.
    effect(() => {
      const id = this.id();
      if (id) {
        this.projects.loadOne(id);
      }
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
    const id = this.id();
    if (id) {
      this.projects.loadOne(id);
    }
  }
}
