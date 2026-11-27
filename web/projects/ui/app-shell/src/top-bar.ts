import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  TemplateRef,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import { BreakpointService, ThemeService } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';

import { DEFAULT_SHELL_BRAND, type ShellBrand } from './brand';
import { CreateIssueDialog } from './create-issue-dialog';
import { GlobalSearch } from './global-search';
import { HelpMenu } from './help-menu';
import { TopMenu } from './nav-menu';
import { NotificationsBell } from './notifications-bell';
import type { NavMenu } from './navigation';
import { ShellIcon } from './shell-icon';
import { COLLAPSED_NAV_QUERY, ICON_SEARCH_QUERY, MOBILE_QUERY } from './shell-queries';
import { UserMenu } from './user-menu';

/** Global header: logo, primary menus with dropdowns, Create, search, bell, help and user menu. */
@Component({
  selector: 'app-top-bar',
  imports: [
    AppTooltip,
    CreateIssueDialog,
    GlobalSearch,
    HelpMenu,
    NgTemplateOutlet,
    NotificationsBell,
    RouterLink,
    ShellIcon,
    TopMenu,
    UserMenu,
  ],
  templateUrl: './top-bar.html',
  styleUrl: './top-bar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TopBar {
  private readonly breakpoints = inject(BreakpointService);
  private readonly theme = inject(ThemeService);

  readonly menus = input.required<readonly NavMenu[]>();
  readonly canCreate = input(false);
  readonly orgName = input<string | null>(null);
  /** Organization switcher from the consumer. Replaces the plain organization name when given. */
  readonly orgSwitcher = input<TemplateRef<unknown> | null>(null);
  /** Custom create button area from the consumer. Replaces the default Crear button when given. */
  readonly createSlot = input<TemplateRef<unknown> | null>(null);
  readonly brand = input<ShellBrand>(DEFAULT_SHELL_BRAND);
  /** Shows the Menú button that opens the navigation drawer. */
  readonly showMenuButton = input(false);
  /** Path of the page in view, to mark the menu of the current area. */
  readonly currentPath = input('');
  /** Documentation, release notes and support addresses of the help menu. */
  readonly docsUrl = input<string | null>(null);
  readonly whatsNewUrl = input<string | null>(null);
  readonly supportUrl = input<string | null>(null);

  readonly openMenu = output<void>();
  readonly openShortcuts = output<void>();
  /** Opens the personal scratchpad. */
  readonly openNotes = output<void>();
  /** Shows the scratchpad button (apps that mount the work pages). */
  readonly showNotes = input(false);

  protected readonly isMobile = this.breakpoints.matches(MOBILE_QUERY);
  protected readonly collapsedNav = this.breakpoints.matches(COLLAPSED_NAV_QUERY);
  protected readonly iconSearch = this.breakpoints.matches(ICON_SEARCH_QUERY);
  protected readonly isDark = this.theme.isDark;
  protected readonly createOpen = signal(false);
}
