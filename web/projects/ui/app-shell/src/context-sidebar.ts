import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BreakpointService, UiPreferencesService } from '@corelog/core';
import { ProjectAvatar } from '@corelog/ui/project-avatar';

import { isSidebarItemActive, type NavItem } from './navigation';
import { ShellIcon } from './shell-icon';
import { NARROW_SIDEBAR_QUERY } from './shell-queries';

/** Project shown in the sidebar header. */
export interface SidebarProject {
  readonly id: string;
  readonly name: string;
  readonly key: string;
}

/**
 * Sidebar of the current context: project header, item list and a collapse toggle. The collapsed
 * state is persisted once the user has chosen; until then it follows the viewport width.
 */
@Component({
  selector: 'app-context-sidebar',
  imports: [ProjectAvatar, RouterLink, ShellIcon],
  templateUrl: './context-sidebar.html',
  styleUrl: './context-sidebar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ContextSidebar {
  private readonly preferences = inject(UiPreferencesService);
  private readonly breakpoints = inject(BreakpointService);

  /** Context name: the accessible name of the aside, and the header when there is no project. */
  readonly title = input.required<string>();
  readonly items = input.required<readonly NavItem[]>();
  readonly project = input<SidebarProject | null>(null);
  /** Current route path, used to mark the active item. */
  readonly currentPath = input.required<string>();

  private readonly narrow = this.breakpoints.matches(NARROW_SIDEBAR_QUERY);
  protected readonly collapsed = computed(
    () => this.preferences.sidebarCollapsed() ?? this.narrow(),
  );

  protected isActive(item: NavItem): boolean {
    return isSidebarItemActive(item, this.currentPath());
  }

  protected toggle(): void {
    this.preferences.setSidebarCollapsed(!this.collapsed());
  }
}
