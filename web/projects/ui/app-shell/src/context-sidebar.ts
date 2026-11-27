import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BreakpointService, UiPreferencesService } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';

import { isSidebarItemActive, type NavItem } from './navigation';
import { ShellIcon } from './shell-icon';
import { NARROW_SIDEBAR_QUERY } from './shell-queries';

/**
 * Sidebar of the current context: title, item list and a collapse toggle. The collapsed
 * state is persisted once the user has chosen; until then it follows the viewport width.
 */
@Component({
  selector: 'app-context-sidebar',
  imports: [AppTooltip, RouterLink, ShellIcon],
  templateUrl: './context-sidebar.html',
  styleUrl: './context-sidebar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ContextSidebar {
  private readonly preferences = inject(UiPreferencesService);
  private readonly breakpoints = inject(BreakpointService);

  /** Context name: the accessible name of the aside and its header. */
  readonly title = input.required<string>();
  readonly items = input.required<readonly NavItem[]>();
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
