import { DOCUMENT, NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  HostListener,
  Injector,
  TemplateRef,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  model,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';

import { MENU_ICONS, type MenuEntry } from './menu-model';
import { hrefOf } from './nav-menu';
import { isSidebarItemActive, type NavItem, type NavMenu } from './navigation';
import { ShellIcon, type ShellIconName } from './shell-icon';
import { ShellMenuData } from './shell-menu-data';

/**
 * Navigation drawer for phones and tablets: the organization switcher (phones), the four top
 * menus as expandable groups with their real items, and the links of the current context.
 * The drawer stays in the DOM while closed so the organization switcher keeps its state; opening
 * moves focus to the close button and closing returns it to the control that opened it.
 */
@Component({
  selector: 'app-nav-drawer',
  imports: [NgTemplateOutlet, RouterLink, RouterLinkActive, ShellIcon],
  templateUrl: './nav-drawer.html',
  styleUrl: './nav-drawer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NavDrawer {
  private readonly data = inject(ShellMenuData);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  readonly open = model(false);
  readonly menus = input.required<readonly NavMenu[]>();
  readonly sidebarTitle = input('');
  readonly items = input<readonly NavItem[]>([]);
  readonly currentPath = input('');
  /** Phones show the organization switcher inside the drawer. */
  readonly showOrgSwitcher = input(false);
  readonly orgSwitcher = input<TemplateRef<unknown> | null>(null);

  private readonly closeButton = viewChild<ElementRef<HTMLButtonElement>>('closeButton');
  private opener: HTMLElement | null = null;
  private wasOpen = false;

  protected readonly expanded = signal<string | null>(null);
  protected readonly models = computed(
    () => new Map(this.menus().map((menu) => [menu.key, this.data.modelOf(menu)])),
  );
  protected readonly hrefOf = hrefOf;

  constructor() {
    effect(() => {
      const open = this.open();
      untracked(() => this.onVisibleChange(open));
    });
  }

  protected iconOf(menu: NavMenu): ShellIconName {
    return MENU_ICONS[menu.key] ?? 'folder';
  }

  protected modelOf(menu: NavMenu) {
    return this.models().get(menu.key) ?? null;
  }

  protected toggleMenu(menu: NavMenu): void {
    const opening = this.expanded() !== menu.key;
    this.expanded.set(opening ? menu.key : null);
    if (opening) this.data.refresh(menu);
  }

  protected isItemActive(item: NavItem): boolean {
    return isSidebarItemActive(item, this.currentPath());
  }

  protected entryHref(entry: MenuEntry): string {
    return hrefOf(entry);
  }

  protected close(): void {
    this.open.set(false);
  }

  @HostListener('document:keydown.escape')
  protected onEscape(): void {
    if (this.open()) this.close();
  }

  private onVisibleChange(open: boolean): void {
    if (open && !this.wasOpen) {
      const active = this.document.activeElement;
      this.opener = active instanceof HTMLElement ? active : null;
      afterNextRender(() => this.closeButton()?.nativeElement.focus(), {
        injector: this.injector,
      });
    } else if (!open && this.wasOpen) {
      this.opener?.focus();
      this.opener = null;
    }
    this.wasOpen = open;
  }
}
