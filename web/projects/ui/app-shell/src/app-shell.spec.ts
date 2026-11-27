import { Component, inject, signal, type Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, Routes, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  LOCATION_ASSIGN,
  AuthStore,
  AvatarApiService,
  NotificationStore,
  ProjectStore,
  SearchApiService,
  TeamStore,
  ThemeService,
  UiPreferencesService,
  type Project,
  type User,
} from '@corelog/core';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AppShell } from './app-shell';
import { CreateIssueLauncher } from './create-issue-dialog';
import type { ShellOrg } from './navigation';
import { AppShellOrgSwitcher } from './shell-slots';

const adminUser: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
  role: 'admin',
  teamId: null,
  displayName: '',
  jobTitle: '',
  department: '',
  avatarKey: null,
  timezone: 'UTC',
  createdAt: '2026-01-01T00:00:00Z',
  lastLoginAt: null,
  emailVerifiedAt: null,
};

const demoProject: Project = {
  id: 'p1',
  organizationId: 'o1',
  key: 'DEM',
  name: 'Proyecto demo',
  description: '',
  type: 'scrum',
  leadId: 'u1',
  visibility: 'private',
  archivedAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const acme: ShellOrg = { id: 'o1', name: 'Acme', slug: 'acme' };

/** Minimal matchMedia: "(max-width: Npx)" matches when the viewport is at most N px wide. */
function setViewport(width: number): void {
  window.matchMedia = ((query: string) => {
    const max = /max-width:\s*([\d.]+)px/.exec(query);
    return {
      matches: max ? width <= Number(max[1]) : false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    } as MediaQueryList;
  }) as typeof window.matchMedia;
}

@Component({ selector: 'app-route-page', template: '<p>{{ text }}</p>' })
class RoutePage {
  readonly text = inject(ActivatedRoute).snapshot.data['text'] as string;
}

@Component({
  imports: [AppShell],
  template: `<app-shell [org]="org" [canCreate]="true" [docsUrl]="docsUrl" />`,
})
class PlainHost {
  org: ShellOrg | null = null;
  docsUrl: string | null = null;
}

@Component({
  imports: [AppShell],
  template: `<app-shell [org]="acme" [canCreate]="true" />`,
})
class OrgHost {
  readonly acme = acme;
}

@Component({
  imports: [AppShell, AppShellOrgSwitcher],
  template: `<app-shell [org]="acme" [canCreate]="true"
    ><span appShellOrgSwitcher>Switcher real</span></app-shell
  >`,
})
class SwitcherHost {
  readonly acme = acme;
}

function routesFor(host: Type<unknown>): Routes {
  return [
    {
      path: '',
      component: host,
      children: [
        { path: 'issues', component: RoutePage, data: { text: 'Lista de incidencias' } },
        {
          path: 'projects/:id/board',
          component: RoutePage,
          data: { text: 'Tablero del proyecto' },
        },
        { path: 'profile', component: RoutePage, data: { text: 'Perfil' } },
      ],
    },
    { path: 'login', component: RoutePage, data: { text: 'Pantalla de acceso' } },
  ];
}

interface Fakes {
  auth: {
    user: ReturnType<typeof signal<User | null>>;
    isAuthenticated: ReturnType<typeof signal<boolean>>;
    logout: ReturnType<typeof vi.fn>;
  };
  assign: ReturnType<typeof vi.fn>;
  projects: {
    projects: ReturnType<typeof signal<Project[]>>;
    fetchProjects: ReturnType<typeof vi.fn>;
  };
}

async function render(
  path: string,
  options: { host?: Type<unknown>; width?: number; projects?: Project[]; user?: User | null } = {},
): Promise<{ root: HTMLElement; fakes: Fakes; harness: RouterTestingHarness }> {
  setViewport(options.width ?? 1440);
  const fakes: Fakes = {
    auth: {
      user: signal<User | null>(options.user === undefined ? adminUser : options.user),
      isAuthenticated: signal(true),
      logout: vi.fn(),
    },
    assign: vi.fn(),
    projects: {
      projects: signal<Project[]>(options.projects ?? [demoProject]),
      fetchProjects: vi.fn(),
    },
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter(routesFor(options.host ?? PlainHost)),
      { provide: AuthStore, useValue: fakes.auth },
      { provide: LOCATION_ASSIGN, useValue: fakes.assign },
      { provide: ProjectStore, useValue: fakes.projects },
      { provide: TeamStore, useValue: { teams: signal([]), fetchTeams: vi.fn() } },
      {
        provide: NotificationStore,
        useValue: {
          unreadCount: signal(0),
          items: signal([]),
          startPolling: vi.fn(),
          stopPolling: vi.fn(),
          loadList: vi.fn(),
          markRead: vi.fn(() => of(undefined)),
          markAllRead: vi.fn(() => of(undefined)),
        },
      },
      { provide: AvatarApiService, useValue: { getAvatar: () => of(null) } },
      { provide: SearchApiService, useValue: { search: () => of([]) } },
    ],
  });
  const harness = await RouterTestingHarness.create(path);
  harness.fixture.detectChanges();
  return { root: harness.fixture.nativeElement as HTMLElement, fakes, harness };
}

function byLabel(root: ParentNode, selector: string, label: string): HTMLElement | null {
  return root.querySelector<HTMLElement>(`${selector}[aria-label="${label}"]`);
}

/** Clicks the open menu row whose text matches. */
function clickMenuItem(text: string): void {
  const rows = document.querySelectorAll<HTMLElement>(
    '[role="menuitem"], [role="menuitemradio"], [role="menuitemcheckbox"]',
  );
  const match = Array.from(rows).find((item) => (item.textContent ?? '').trim().includes(text));
  if (!match) throw new Error(`menu item "${text}" not found`);
  match.click();
}

describe('AppShell', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    delete (window as Partial<Window>).matchMedia;
    document.querySelectorAll('[role="dialog"], [role="menu"]').forEach((node) => node.remove());
  });

  it('puts the skip link first in the tab order and it moves focus to main', async () => {
    const { root } = await render('/issues');
    const focusable = Array.from(root.querySelectorAll<HTMLElement>('a[href], button, input'));
    expect(focusable[0]?.textContent).toContain('Saltar al contenido');
    root.querySelector<HTMLAnchorElement>('a.app-shell__skip')?.click();
    expect(document.getElementById('main-content')).toBe(document.activeElement);
  });

  it('renders the landmarks: banner, main navigation, main and the context aside', async () => {
    const { root } = await render('/projects/p1/board');
    expect(root.querySelector('header')).not.toBeNull();
    expect(root.querySelector('nav[aria-label="Principal"]')).not.toBeNull();
    expect(root.querySelector('main')?.id).toBe('main-content');
    expect(root.querySelector('aside[aria-label="Proyecto"]')).not.toBeNull();
    expect(root.textContent).toContain('Tablero del proyecto');
  });

  it('shows the project header and marks the active item with aria-current', async () => {
    const { root } = await render('/projects/p1/board');
    const aside = root.querySelector<HTMLElement>('aside[aria-label="Proyecto"]');
    expect(aside?.textContent).toContain('Proyecto demo');
    expect(aside?.textContent).toContain('DEM');
    expect(byLabel(aside as HTMLElement, 'a', 'Tablero')?.getAttribute('aria-current')).toBe(
      'page',
    );
    expect(byLabel(aside as HTMLElement, 'a', 'Permisos')?.hasAttribute('aria-current')).toBe(
      false,
    );
  });

  it('has no sidebar in the global context', async () => {
    const { root } = await render('/issues');
    expect(root.querySelector('aside')).toBeNull();
  });

  it('collapses below 1280 px by default and persists a manual choice', async () => {
    const { root } = await render('/projects/p1/board', { width: 1100 });
    expect(root.querySelector('aside')?.getAttribute('data-collapsed')).toBe('true');
    byLabel(root, 'button', 'Expandir menú lateral')?.click();
    expect(window.localStorage.getItem('corelog.sidebar')).toBe('expanded');
    expect(TestBed.inject(UiPreferencesService).sidebarCollapsed()).toBe(false);
  });

  it('starts the sidebar expanded when the user chose that before', async () => {
    window.localStorage.setItem('corelog.sidebar', 'expanded');
    const { root } = await render('/projects/p1/board', { width: 1100 });
    expect(root.querySelector('aside')?.getAttribute('data-collapsed')).toBe('false');
  });

  it('replaces the sidebar with a drawer on mobile and opens it from the top bar', async () => {
    const { root, harness } = await render('/projects/p1/board', { width: 390 });
    expect(root.querySelector('aside')).toBeNull();
    expect(root.querySelector('nav[aria-label="Principal"]')).toBeNull();
    byLabel(root, 'button', 'Abrir menú de navegación')?.click();
    harness.fixture.detectChanges();
    const drawer = document.querySelector<HTMLElement>('nav[aria-label="Navegación móvil"]');
    expect(drawer).not.toBeNull();
    expect(drawer?.textContent).toContain('Tablero');
  });

  it('groups the drawer: the four menus expand with real entries and the context links follow', async () => {
    const { root, harness } = await render('/projects/p1/board', { width: 390 });
    byLabel(root, 'button', 'Abrir menú de navegación')?.click();
    harness.fixture.detectChanges();
    const drawer = document.querySelector<HTMLElement>('nav[aria-label="Navegación móvil"]');
    const toggles = Array.from(
      drawer?.querySelectorAll<HTMLButtonElement>('button[aria-expanded]') ?? [],
    );
    expect(toggles.map((item) => (item.textContent ?? '').trim())).toEqual([
      'Proyectos',
      'Filtros',
      'Paneles',
      'Equipos',
    ]);
    expect(toggles.every((item) => item.getAttribute('aria-expanded') === 'false')).toBe(true);
    toggles[0].click();
    harness.fixture.detectChanges();
    expect(toggles[0].getAttribute('aria-expanded')).toBe('true');
    expect(drawer?.textContent).toContain('Ver todos los proyectos');
    expect(drawer?.textContent).toContain('Tablero');
  });

  it('moves focus into the drawer when it opens and back to the opener when it closes', async () => {
    const { root, harness } = await render('/projects/p1/board', { width: 390 });
    const opener = byLabel(root, 'button', 'Abrir menú de navegación') as HTMLButtonElement;
    opener.focus();
    opener.click();
    harness.fixture.detectChanges();
    TestBed.tick();
    expect(document.activeElement).toBe(byLabel(root, 'button', 'Cerrar navegación'));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    harness.fixture.detectChanges();
    TestBed.tick();
    expect(document.activeElement).toBe(opener);
  });

  it('closes the phone drawer on Escape and keeps its organization switcher mounted', async () => {
    const { root, harness } = await render('/projects/p1/board', { width: 390 });
    byLabel(root, 'button', 'Abrir menú de navegación')?.click();
    harness.fixture.detectChanges();
    const drawer = root.querySelector<HTMLElement>('.nav-drawer');
    expect(drawer?.classList.contains('nav-drawer--open')).toBe(true);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    harness.fixture.detectChanges();
    expect(drawer?.classList.contains('nav-drawer--open')).toBe(false);
    expect(drawer?.getAttribute('aria-hidden')).toBe('true');
  });

  it('moves the primary menus into the drawer at 820 px and keeps the sidebar', async () => {
    const { root, harness } = await render('/projects/p1/board', { width: 820 });
    expect(root.querySelector('nav[aria-label="Principal"]')).toBeNull();
    expect(root.querySelector('aside[aria-label="Proyecto"]')).not.toBeNull();
    byLabel(root, 'button', 'Abrir menú de navegación')?.click();
    harness.fixture.detectChanges();
    const drawer = document.querySelector<HTMLElement>('nav[aria-label="Navegación móvil"]');
    expect(drawer?.textContent).toContain('Proyectos');
    expect(drawer?.textContent).toContain('Equipos');
    expect(drawer?.textContent).not.toContain('Organización');
  });

  it('puts the organization switcher inside the drawer on phones, not in the top bar', async () => {
    const { root, harness } = await render('/issues', { width: 390, host: SwitcherHost });
    expect(root.querySelector('header')?.textContent).not.toContain('Switcher real');
    byLabel(root, 'button', 'Abrir menú de navegación')?.click();
    harness.fixture.detectChanges();
    const drawer = document.querySelector<HTMLElement>('nav[aria-label="Navegación móvil"]');
    expect(drawer?.textContent).toContain('Switcher real');
    expect(drawer?.textContent).toContain('Equipos');
  });

  it('opens the shortcuts dialog from the help menu', async () => {
    const { root, harness } = await render('/issues');
    byLabel(root, 'button', 'Ayuda')?.click();
    harness.fixture.detectChanges();
    clickMenuItem('Atajos de teclado');
    harness.fixture.detectChanges();
    const dialog = document.querySelector<HTMLElement>('[role="dialog"][aria-modal="true"]');
    expect(dialog?.textContent).toContain('Atajos de teclado');
  });
});

describe('AppShell user menu and shortcuts', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    delete (window as Partial<Window>).matchMedia;
    document.querySelectorAll('[role="dialog"], [role="menu"]').forEach((node) => node.remove());
  });

  /** Opens the user menu and lets the menu render its items. */
  function openUserMenu(root: HTMLElement, harness: RouterTestingHarness): void {
    byLabel(root, 'button', 'Menú de Ana Pérez')?.click();
    harness.fixture.detectChanges();
  }

  it('calls the store logout and moves the browser to the login page', async () => {
    const { root, fakes, harness } = await render('/issues');
    openUserMenu(root, harness);
    clickMenuItem('Cerrar sesión');
    expect(fakes.auth.logout).toHaveBeenCalledTimes(1);
    expect(fakes.assign).toHaveBeenCalledWith('/login');
  });

  it('opens the profile with a full-page move to the workspace', async () => {
    const { root, fakes, harness } = await render('/issues');
    openUserMenu(root, harness);
    clickMenuItem('Mi perfil');
    expect(fakes.assign).toHaveBeenCalledWith('/workspace/profile');
  });

  it('changes the theme from the user menu and keeps the menu open', async () => {
    const { root, harness } = await render('/issues');
    openUserMenu(root, harness);
    clickMenuItem('Oscuro');
    harness.fixture.detectChanges();
    expect(TestBed.inject(ThemeService).mode()).toBe('dark');
    const dark = Array.from(document.querySelectorAll('[role="menuitemradio"]')).find((row) =>
      (row.textContent ?? '').includes('Oscuro'),
    );
    expect(dark?.getAttribute('aria-checked')).toBe('true');
    clickMenuItem('Sistema');
    expect(TestBed.inject(ThemeService).mode()).toBe('system');
  });

  it('persists the density choice in corelog.density', async () => {
    const { root, harness } = await render('/issues');
    openUserMenu(root, harness);
    clickMenuItem('Densidad compacta');
    expect(window.localStorage.getItem('corelog.density')).toBe('"compact"');
    expect(document.documentElement.dataset['density']).toBe('compact');
  });

  it('shows the organization name as plain text until the switcher exists', async () => {
    const { root } = await render('/issues', { host: OrgHost });
    expect(root.querySelector('header')?.textContent).toContain('Acme');
  });

  it('mounts the command palette once and opens it with Ctrl+K and with "/"', async () => {
    const { harness } = await render('/issues');
    const refresh = (): void => harness.fixture.detectChanges();
    const palettes = (): number =>
      document.querySelectorAll('[role="dialog"][aria-label="Paleta de comandos"]').length;
    expect(palettes()).toBe(0);
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true }));
    refresh();
    expect(palettes()).toBe(1);
    document
      .querySelector('[role="dialog"][aria-label="Paleta de comandos"]')
      ?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    refresh();
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }));
    refresh();
    expect(palettes()).toBe(1);
  });

  it('asks the create dialog to open with the c key instead of leaving the page', async () => {
    const { harness } = await render('/issues');
    // The dialog form needs the full store graph, so only the request is observed here.
    const open = vi
      .spyOn(TestBed.inject(CreateIssueLauncher), 'open')
      .mockImplementation(() => undefined);
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'c', bubbles: true }));
    harness.fixture.detectChanges();
    expect(open).toHaveBeenCalledTimes(1);
    expect(TestBed.inject(Router).url).toBe('/issues');
  });

  it('leaves "/" to the palette only: the header search field does not take focus', async () => {
    const { root } = await render('/issues');
    const search = root.querySelector<HTMLInputElement>(
      'input[aria-label="Buscar proyectos e incidencias"]',
    );
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }));
    expect(search).not.toBe(document.activeElement);
  });
});

describe('AppShell project context', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    delete (window as Partial<Window>).matchMedia;
  });

  it('loads the projects list when the URL points at a project that is not loaded', async () => {
    const { fakes } = await render('/projects/p1/board', { projects: [] });
    expect(fakes.projects.fetchProjects).toHaveBeenCalledTimes(1);
  });

  it('does not fetch the projects in the global context', async () => {
    const { fakes } = await render('/issues', { projects: [] });
    expect(fakes.projects.fetchProjects).not.toHaveBeenCalled();
  });
});
