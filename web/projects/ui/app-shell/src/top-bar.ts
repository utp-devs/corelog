import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  TemplateRef,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { BreakpointService } from '@corelog/core';
import type { MenuItem } from 'primeng/api';
import { Menu } from 'primeng/menu';

import { DEFAULT_SHELL_BRAND, type ShellBrand } from './brand';
import { CreateIssueDialog } from './create-issue-dialog';
import { GlobalSearch } from './global-search';
import { HelpMenu } from './help-menu';
import { NotesButton } from './notes-button';
import { NotificationsBell } from './notifications-bell';
import { VIEW_ALL_LABEL, type NavMenu } from './navigation';
import { ShellIcon } from './shell-icon';
import { COLLAPSED_NAV_QUERY, ICON_SEARCH_QUERY, MOBILE_QUERY } from './shell-queries';
import { UserMenu } from './user-menu';

/** Global header: logo, primary menus with dropdowns, Create, search, bell, notes, help and user menu. */
@Component({
  selector: 'app-top-bar',
  imports: [
    CreateIssueDialog,
    GlobalSearch,
    HelpMenu,
    Menu,
    NgTemplateOutlet,
    NotesButton,
    NotificationsBell,
    RouterLink,
    ShellIcon,
    UserMenu,
  ],
  templateUrl: './top-bar.html',
  styleUrl: './top-bar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TopBar {
  private readonly breakpoints = inject(BreakpointService);
  private readonly router = inject(Router);

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
  readonly docsUrl = input<string | null>(null);

  readonly openMenu = output<void>();
  readonly openShortcuts = output<void>();

  protected readonly isMobile = this.breakpoints.matches(MOBILE_QUERY);
  protected readonly collapsedNav = this.breakpoints.matches(COLLAPSED_NAV_QUERY);
  protected readonly iconSearch = this.breakpoints.matches(ICON_SEARCH_QUERY);
  protected readonly createOpen = signal(false);

  /** Dropdown entries of every top menu, keyed by the menu key. */
  protected readonly menuModels = computed<ReadonlyMap<string, MenuItem[]>>(
    () => new Map(this.menus().map((menu) => [menu.key, this.dropdownItems(menu)])),
  );

  protected itemsOf(menu: NavMenu): MenuItem[] {
    return this.menuModels().get(menu.key) ?? [];
  }

  protected go(path: string): void {
    void this.router.navigateByUrl(path);
  }

  private dropdownItems(menu: NavMenu): MenuItem[] {
    return [
      { label: 'Recientes', disabled: true, styleClass: 'menu-group' },
      { label: 'Sin elementos recientes', disabled: true },
      { separator: true },
      { label: VIEW_ALL_LABEL, command: () => this.go(menu.path) },
      ...(menu.extras ?? []).map((link) => ({
        label: link.label,
        command: () => this.go(link.path),
      })),
    ];
  }
}
