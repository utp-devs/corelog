import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { RouterOutlet } from '@angular/router';
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
import { projectCover } from '../project-cover';

import { TYPE_LABELS, VISIBILITY_LABELS } from '../projects-page/projects-page.utils';

/** Descriptions longer than this show the expand toggle. */
const DESCRIPTION_PREVIEW_CHARS = 70;

const LOAD_FAILED_MESSAGE =
  'No se pudo abrir el proyecto. Verifica que tengas acceso e intenta de nuevo.';

type LoadState =
  | { readonly status: 'loading' }
  | { readonly status: 'ready'; readonly project: Project }
  | { readonly status: 'error'; readonly variant: ProjectAccessVariant | null };

/** Shared header of a project: identity. The section navigation lives in the app shell sidebar. Sections render below it. */
@Component({
  selector: 'app-project-workspace',
  imports: [
    PageContainer,
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

  protected readonly loadFailedMessage = LOAD_FAILED_MESSAGE;

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

  protected readonly descriptionOpen = signal(false);

  protected readonly lead = useOrgMembersById(computed(() => this.project()?.organizationId));

  constructor() {
    // Leaving the project area clears the project and its permissions.
    inject(DestroyRef).onDestroy(() => this.current.reset());

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

  protected cover(project: Project): string {
    return projectCover(project.color, project.key);
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

  protected retry(): void {
    this.current.reload();
  }
}
