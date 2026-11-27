import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  TemplateRef,
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
import { NavigationEnd, Router, RouterOutlet } from '@angular/router';
import {
  AuthStore,
  BreakpointService,
  CurrentProjectStore,
  HotkeysService,
  permissionsFor,
  projectPath,
} from '@corelog/core';
import { trackRecent } from '@corelog/ui/command-palette';
import { ScratchpadDrawer } from '@corelog/ui/scratchpad-drawer';
import { ShortcutsDialog } from '@corelog/ui/shortcuts-dialog';
import { filter, map } from 'rxjs';

import { CreateIssueLauncher } from './create-issue-dialog';
import { DEFAULT_SHELL_BRAND, type ShellBrand } from './brand';
import { ContextSidebar, type SidebarProject } from './context-sidebar';
import {
  DEFAULT_SHELL_NAV_CONFIG,
  buildSidebar,
  buildTopNav,
  resolveSidebarContext,
  sidebarTitleOf,
  type NavItem,
  type ShellNavConfig,
  type ShellOrg,
} from './navigation';
import type { ShellIconName } from './shell-icon';
import { NavDrawer } from './nav-drawer';
import { AppShellCreate, AppShellOrgSwitcher } from './shell-slots';
import { COLLAPSED_NAV_QUERY, MOBILE_QUERY } from './shell-queries';
import { TopBar } from './top-bar';

/** Icon of each section of a project in the sidebar. */
const PROJECT_SECTION_ICONS: Readonly<Record<string, ShellIconName>> = {
  board: 'board',
  backlog: 'backlog',
  sprints: 'sprints',
  calendar: 'calendar',
  permisos: 'permissions',
  settings: 'settings',
};

/**
 * Shell of the signed-in apps: skip link, top bar, collapsible context sidebar (or the mobile
 * drawer), the main landmark with the router outlet, and a live region for route changes.
 * Navigation, brand, organization and the create action come from inputs and projected slots.
 */
@Component({
  selector: 'app-shell',
  imports: [ContextSidebar, NavDrawer, RouterOutlet, ScratchpadDrawer, ShortcutsDialog, TopBar],
  templateUrl: './app-shell.html',
  styleUrl: './app-shell.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppShell {
  private readonly router = inject(Router);
  private readonly title = inject(Title);
  private readonly auth = inject(AuthStore);
  protected readonly hotkeys = inject(HotkeysService);
  private readonly currentProject = inject(CurrentProjectStore);
  private readonly breakpoints = inject(BreakpointService);
  private readonly createLauncher = inject(CreateIssueLauncher);

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
  /** Release notes address. The help menu lists the entry only when it is set. */
  readonly whatsNewUrl = input<string | null>(null);
  /** Support address. The help menu lists the entry only when it is set. */
  readonly supportUrl = input<string | null>(null);

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

  /**
   * The project of the route once it has loaded. The store is cleared when another project opens,
   * so a project that does not match the route is never shown.
   */
  private readonly routeProject = computed(() => {
    const id = this.projectId();
    const project = this.currentProject.project();
    if (!id || !project) return null;
    const matches = project.id === id || project.key.toLowerCase() === id.toLowerCase();
    return matches ? project : null;
  });
  protected readonly sidebarTitle = computed(
    () => this.routeProject()?.name ?? sidebarTitleOf(this.context(), this.navConfig()),
  );
  protected readonly sidebarProject = computed<SidebarProject | null>(() => {
    const project = this.routeProject();
    if (!project) return null;
    return {
      id: project.id,
      name: project.name,
      key: project.key,
      archived: this.currentProject.isArchived(),
    };
  });
  /** A project route whose project is still being read. */
  protected readonly sidebarLoading = computed(
    () => this.projectId() !== null && !this.routeProject() && !this.currentProject.error(),
  );
  protected readonly items = computed<NavItem[]>(() => {
    if (this.context().kind === 'project') {
      return this.currentProject.navItems().map((item) => ({
        key: item.key,
        label: item.label,
        path: item.path,
        icon: PROJECT_SECTION_ICONS[item.key] ?? 'folder',
      }));
    }
    return buildSidebar(
      this.context(),
      permissionsFor(this.auth.user(), this.org()?.role),
      this.navConfig(),
    );
  });
  /** Path used to mark the active entry: a project opened by its UUID matches its key paths. */
  protected readonly activePath = computed(() => {
    const project = this.routeProject();
    if (!project) return this.pathname();
    const [first, , ...rest] = this.pathname().split('/').filter(Boolean);
    return `/${[first, encodeURIComponent(project.key), ...rest].join('/')}`;
  });
  /** The project context shows the sidebar while it loads; the others only when they list items. */
  protected readonly showSidebar = computed(() => {
    if (this.isMobile()) return false;
    if (this.context().kind === 'project')
      return this.sidebarProject() !== null || this.sidebarLoading();
    return this.items().length > 0;
  });
  private readonly nav = computed(() => buildTopNav(this.org(), this.navConfig()));
  protected readonly topMenus = computed(() => this.nav().menus);
  protected readonly orgName = computed(() => this.nav().orgName);

  protected readonly helpOpen = signal(false);
  protected readonly notesOpen = signal(false);
  protected readonly drawerOpen = signal(false);
  protected readonly announcement = signal('');
  private routeCount = 0;

  constructor() {
    // Shell shortcuts follow the project in view ('g b' opens its board).
    effect((onCleanup) => {
      const projectId = this.projectId();
      const unregister = this.hotkeys.registerGlobalShortcuts({
        projectId,
        workPages: this.workPages(),
        // Without the Crear button there is no dialog to open: the shortcut then opens the page.
        onCreate: this.canCreate() ? () => this.createLauncher.open() : undefined,
        onHelp: () => this.helpOpen.update((open) => !open),
        onNotes: this.workPages() ? () => this.notesOpen.update((open) => !open) : undefined,
      });
      onCleanup(unregister);
    });

    // Remember the project in view for the recent items of the command palette.
    effect(() => {
      const current = this.routeProject();
      const path = this.pathname();
      untracked(() => {
        if (current && path) {
          trackRecent({
            kind: 'project',
            id: current.id,
            title: current.name,
            subtitle: current.key,
            link: projectPath(current, 'board'),
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
    this.drawerOpen.set(true);
  }
}
