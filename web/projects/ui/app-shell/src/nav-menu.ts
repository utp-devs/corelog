import { NgTemplateOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { ProjectAvatar } from '@corelog/ui/project-avatar';

import { DropdownMenu } from './dropdown-menu';
import type { MenuEntry } from './menu-model';
import type { NavMenu as NavMenuModel } from './navigation';
import { ShellIcon } from './shell-icon';
import { ShellMenuData } from './shell-menu-data';

/** Query string of an entry that opens another application. */
export function hrefOf(entry: MenuEntry): string {
  if (!entry.queryParams) return entry.path;
  const query = new URLSearchParams(entry.queryParams).toString();
  return `${entry.path}?${query}`;
}

/**
 * One top menu (Proyectos, Filtros, Paneles, Equipos): a trigger that opens a panel with recent or
 * own items grouped under small headers, a "Ver todos" footer and, when allowed, a primary
 * "Crear" action.
 */
@Component({
  selector: 'app-nav-menu',
  imports: [DropdownMenu, NgTemplateOutlet, ProjectAvatar, RouterLink, RouterLinkActive, ShellIcon],
  templateUrl: './nav-menu.html',
  styleUrl: './nav-menu.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TopMenu {
  private readonly data = inject(ShellMenuData);

  readonly menu = input.required<NavMenuModel>();
  /** Path of the page in view, to mark the menu of the current area. */
  readonly currentPath = input('');

  protected readonly open = signal(false);
  protected readonly model = computed(() => this.data.modelOf(this.menu()));
  protected readonly active = computed(() => {
    const path = this.menu().path;
    const current = this.currentPath();
    return !this.menu().external && (current === path || current.startsWith(`${path}/`));
  });
  protected readonly hrefOf = hrefOf;
  protected readonly skeletonRows = [1, 2, 3];

  protected onOpened(): void {
    this.data.refresh(this.menu());
  }
}
