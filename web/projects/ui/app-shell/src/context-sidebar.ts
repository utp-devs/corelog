import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  untracked,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  BreakpointService,
  FavoritesStore,
  NotificationService,
  UiPreferencesService,
} from '@corelog/core';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { AppTooltip } from '@corelog/ui/tooltip';

import { isSidebarItemActive, type NavItem } from './navigation';
import { ShellIcon } from './shell-icon';
import { NARROW_SIDEBAR_QUERY } from './shell-queries';

/** Project shown at the top of the sidebar of a project. */
export interface SidebarProject {
  readonly id: string;
  readonly name: string;
  readonly key: string;
  readonly archived: boolean;
}

/** Rows of the placeholder shown while the project loads. */
const SKELETON_ROWS = [1, 2, 3, 4];

/**
 * Sidebar of the current context: title, item list and a collapse toggle. The collapsed
 * state is persisted once the user has chosen; until then it follows the viewport width. In the
 * project context it also shows the project (avatar, name, key, star, archived badge) or a
 * placeholder while it loads.
 */
@Component({
  selector: 'app-context-sidebar',
  imports: [AppTooltip, ProjectAvatar, RouterLink, ShellIcon],
  templateUrl: './context-sidebar.html',
  styleUrl: './context-sidebar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ContextSidebar {
  private readonly preferences = inject(UiPreferencesService);
  private readonly breakpoints = inject(BreakpointService);
  private readonly favorites = inject(FavoritesStore);
  private readonly notifications = inject(NotificationService);

  /** Context name: the accessible name of the aside and its header. */
  readonly title = input.required<string>();
  readonly items = input.required<readonly NavItem[]>();
  /** Current route path, used to mark the active item. */
  readonly currentPath = input.required<string>();
  /** Project of the project context; null in the other contexts and while it loads. */
  readonly project = input<SidebarProject | null>(null);
  /** The context is a project that has not loaded yet: shows the placeholder. */
  readonly loading = input(false);

  protected readonly skeletonRows = SKELETON_ROWS;
  protected readonly starred = computed(() => {
    const project = this.project();
    return project ? this.favorites.isStarred(project.id) : false;
  });
  private starsRequested = false;

  private readonly narrow = this.breakpoints.matches(NARROW_SIDEBAR_QUERY);
  protected readonly collapsed = computed(
    () => this.preferences.sidebarCollapsed() ?? this.narrow(),
  );

  constructor() {
    // The star needs the starred projects: read them once the first project shows.
    effect(() => {
      if (this.project() && !this.starsRequested) {
        this.starsRequested = true;
        untracked(() => this.favorites.fetchFavorites());
      }
    });
  }

  protected toggleStar(project: SidebarProject): void {
    const request = this.starred()
      ? this.favorites.unstar(project.id)
      : this.favorites.star({ id: project.id, name: project.name, key: project.key });
    request.subscribe({ error: (error: unknown) => this.notifications.error(error) });
  }

  protected isActive(item: NavItem): boolean {
    return isSidebarItemActive(item, this.currentPath());
  }

  protected toggle(): void {
    this.preferences.setSidebarCollapsed(!this.collapsed());
  }
}
