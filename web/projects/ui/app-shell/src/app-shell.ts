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
  contentChild,
  effect,
  inject,
  input,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { Title } from '@angular/platform-browser';
import { NavigationEnd, Router, RouterLink, RouterOutlet } from '@angular/router';
import {
  AuthStore,
  BreakpointService,
  HotkeysService,
  ProjectStore,
  permissionsFor,
} from '@corelog/core';
import { trackRecent } from '@corelog/ui/command-palette';
import { ShortcutsDialog } from '@corelog/ui/shortcuts-dialog';
import { filter, map } from 'rxjs';

import { DEFAULT_SHELL_BRAND, type ShellBrand } from './brand';
import { ContextSidebar, type SidebarProject } from './context-sidebar';
import {
  DEFAULT_SHELL_NAV_CONFIG,
  buildSidebar,
  buildTopNav,
  isSidebarItemActive,
  resolveSidebarContext,
  sidebarTitleOf,
  type NavItem,
  type ShellNavConfig,
  type ShellOrg,
} from './navigation';
import { ShellIcon } from './shell-icon';
import { AppShellCreate, AppShellOrgSwitcher } from './shell-slots';
import { COLLAPSED_NAV_QUERY, MOBILE_QUERY } from './shell-queries';
import { TopBar } from './top-bar';

/**
 * Shell of the signed-in apps: skip link, top bar, collapsible context sidebar (or the mobile
 * drawer), the main landmark with the router outlet, and a live region for route changes.
 * Navigation, brand, organization and the create action come from inputs and projected slots.
 */
@Component({
  selector: 'app-shell',
  imports: [
    ContextSidebar,
    NgTemplateOutlet,
    RouterLink,
    RouterOutlet,
    ShellIcon,
    ShortcutsDialog,
    TopBar,
  ],
  templateUrl: './app-shell.html',
  styleUrl: './app-shell.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppShell {
  private readonly router = inject(Router);
  private readonly title = inject(Title);
  private readonly auth = inject(AuthStore);
  protected readonly hotkeys = inject(HotkeysService);
  private readonly projectStore = inject(ProjectStore);
  private readonly breakpoints = inject(BreakpointService);
  private readonly document = inject(DOCUMENT);

  /** Menus, sidebar items and sidebar titles. */
  readonly navConfig = input<ShellNavConfig>(DEFAULT_SHELL_NAV_CONFIG);
  readonly brand = input<ShellBrand>(DEFAULT_SHELL_BRAND);
  /** Shows the Crear button. Set it for signed-in users. */
  readonly canCreate = input(false);
  /** The app mounts the work pages (issues, projects, dashboard). Set it false in other applications. */
  readonly workPages = input(true);
  readonly org = input<ShellOrg | null>(null);
  /** Documentation address. The help menu lists the entry only when it is set. */
  readonly docsUrl = input<string | null>(null);

  private readonly orgSwitcherSlot = contentChild(AppShellOrgSwitcher);
  private readonly createSlotContent = contentChild(AppShellCreate);
  private readonly orgSwitcherTemplate = viewChild<TemplateRef<unknown>>('orgSwitcherTpl');
  private readonly createTemplate = viewChild<TemplateRef<unknown>>('createTpl');
  private readonly mainRef = viewChild.required<ElementRef<HTMLElement>>('main');

  protected readonly orgSwitcher = computed(() =>
    this.orgSwitcherSlot() ? (this.orgSwitcherTemplate() ?? null) : null,
  );
  protected readonly createSlot = computed(() =>
    this.createSlotContent() ? (this.createTemplate() ?? null) : null,
  );

  protected readonly isMobile = this.breakpoints.matches(MOBILE_QUERY);
  protected readonly collapsedNav = this.breakpoints.matches(COLLAPSED_NAV_QUERY);

  private readonly currentUrl = toSignal(
    this.router.events.pipe(
      filter((event): event is NavigationEnd => event instanceof NavigationEnd),
      map(() => this.router.url),
    ),
    { initialValue: this.router.url },
  );

  protected readonly pathname = computed(() => this.currentUrl().split(/[?#]/)[0] ?? '/');
  private readonly context = computed(() => resolveSidebarContext(this.pathname()));
  private readonly projectId = computed(() => {
    const current = this.context();
    return current.kind === 'project' ? current.projectId : null;
  });

  protected readonly project = computed<SidebarProject | null>(() => {
    const id = this.projectId();
    if (!id) return null;
    const found = this.projectStore.projects().find((item) => item.id === id);
    return found ? { id: found.id, name: found.name, key: found.key } : null;
  });
  private readonly hasProject = computed(() => this.project() !== null);

  protected readonly sidebarTitle = computed(() =>
    sidebarTitleOf(this.context(), this.navConfig()),
  );
  protected readonly items = computed(() =>
    buildSidebar(
      this.context(),
      permissionsFor(this.auth.user(), this.org()?.role),
      this.navConfig(),
    ),
  );
  protected readonly showSidebar = computed(
    () => !this.isMobile() && this.context().kind !== 'global',
  );
  private readonly nav = computed(() => buildTopNav(this.org(), this.navConfig()));
  protected readonly topMenus = computed(() => this.nav().menus);
  protected readonly orgName = computed(() => this.nav().orgName);

  protected readonly helpOpen = signal(false);
  protected readonly drawerOpen = signal(false);
  protected readonly announcement = signal('');
  private drawerOpener: HTMLElement | null = null;
  private readonly injector = inject(Injector);
  protected readonly drawerClose = viewChild<ElementRef<HTMLButtonElement>>('drawerClose');
  private routeCount = 0;

  constructor() {
    // Shell shortcuts follow the project in view ('g b' opens its board).
    effect((onCleanup) => {
      const projectId = this.projectId();
      const unregister = this.hotkeys.registerGlobalShortcuts({
        projectId,
        workPages: this.workPages(),
        onHelp: () => this.helpOpen.update((open) => !open),
      });
      onCleanup(unregister);
    });

    // The project header needs the projects list; load it when the URL points at a project not loaded yet.
    effect(() => {
      const id = this.projectId();
      const loaded = this.hasProject();
      if (id && !loaded) {
        untracked(() => this.projectStore.fetchProjects());
      }
    });

    // Remember the project in view for the recent items of the command palette.
    effect(() => {
      const current = this.project();
      const path = this.pathname();
      untracked(() => {
        if (current && path) {
          trackRecent({
            kind: 'project',
            id: current.id,
            title: current.name,
            subtitle: current.key,
            link: `/projects/${encodeURIComponent(current.id)}/board`,
          });
        }
      });
    });

    // Route changes move focus to the main landmark and are announced; the first load is left alone.
    this.router.events
      .pipe(
        filter((event): event is NavigationEnd => event instanceof NavigationEnd),
        takeUntilDestroyed(),
      )
      .subscribe(() => {
        this.routeCount += 1;
        if (this.routeCount === 1) return;
        this.mainRef().nativeElement.focus();
        const pageTitle = this.title.getTitle();
        this.announcement.set(pageTitle ? `Página: ${pageTitle}` : '');
      });
  }

  protected skipToContent(event: Event): void {
    event.preventDefault();
    this.mainRef().nativeElement.focus();
  }

  protected openDrawer(): void {
    const active = this.document.activeElement;
    this.drawerOpener = active instanceof HTMLElement ? active : null;
    this.onDrawerVisible(true);
  }

  /**
   * Opening or closing the drawer by any means. The drawer stays in the DOM, so closing only hides it;
   * closing returns focus to the element that opened it, opening moves focus to the drawer's close button.
   */
  protected onDrawerVisible(visible: boolean): void {
    this.drawerOpen.set(visible);
    if (visible) {
      afterNextRender(() => this.drawerClose()?.nativeElement.focus(), { injector: this.injector });
    } else {
      this.drawerOpener?.focus();
      this.drawerOpener = null;
    }
  }

  @HostListener('document:keydown.escape')
  protected onEscape(): void {
    if (this.drawerOpen()) this.closeDrawer();
  }

  protected closeDrawer(): void {
    this.onDrawerVisible(false);
  }

  protected isItemActive(item: NavItem): boolean {
    return isSidebarItemActive(item, this.pathname());
  }
}
