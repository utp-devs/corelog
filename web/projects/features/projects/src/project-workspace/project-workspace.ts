import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  DOCUMENT,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterOutlet } from '@angular/router';
import {
  CurrentProjectStore,
  Project,
  ProjectAccessVariant,
  projectPath,
  leadNameOf,
  projectAccessVariantOf,
  pushRecent,
  useOrgMembersById,
} from '@corelog/core';
import { ErrorPage } from '@corelog/ui/error-page';
import { PageContainer } from '@corelog/ui/page-container';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { filter, map } from 'rxjs';

import { TYPE_LABELS, VISIBILITY_LABELS } from '../projects-page/projects-page.utils';

/** Descriptions longer than this show the expand toggle. */
const DESCRIPTION_PREVIEW_CHARS = 70;

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
    PageContainer,
    RouterLink,
    RouterOutlet,
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
  private readonly current = inject(CurrentProjectStore);
  private readonly router = inject(Router);
  private readonly document = inject(DOCUMENT);

  protected readonly loadFailedMessage = LOAD_FAILED_MESSAGE;

  /** Section tabs allowed for the project: template and permissions decide which ones are listed. */
  protected readonly sections = this.current.navItems;

  /** Loading, failure or the project, derived from the store read of the route project. */
  protected readonly state = computed<LoadState>(() => {
    const error = this.current.error();
    if (error) {
      return { status: 'error', variant: projectAccessVariantOf(error) };
    }
    const project = this.current.project();
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
    // /projects/<key or id>/<section>: the section is the fourth segment of the path.
    const segment = this.url().split(/[?#]/)[0].split('/')[3];
    return this.sections().find((section) => section.key === segment)?.key ?? 'board';
  });

  protected readonly descriptionOpen = signal(false);

  protected readonly lead = useOrgMembersById(computed(() => this.project()?.organizationId));

  constructor() {
    // Leaving the project area clears the project and its permissions.
    inject(DestroyRef).onDestroy(() => this.current.reset());

    // Keeps the active tab visible when the tabs scroll (phones).
    effect(() => {
      this.activeKey();
      queueMicrotask(() => {
        const active = this.document.querySelector<HTMLElement>('.project-workspace__tab--active');
        active?.scrollIntoView?.({ inline: 'nearest', block: 'nearest' });
      });
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
          link: projectPath(project),
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

  protected isLongDescription(project: Project): boolean {
    return (
      project.description.length > DESCRIPTION_PREVIEW_CHARS || project.description.includes('\n')
    );
  }

  protected toggleDescription(): void {
    this.descriptionOpen.update((open) => !open);
  }

  protected isActive(key: string): boolean {
    return this.activeKey() === key;
  }

  protected retry(): void {
    this.current.reload();
  }
}
