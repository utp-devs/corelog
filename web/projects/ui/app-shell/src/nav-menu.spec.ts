import { signal } from '@angular/core';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import {
  AuthStore,
  FavoritesStore,
  OrganizationStore,
  TeamStore,
  pushRecent,
  type FavoriteItem,
  type RecentProjectItem,
  type User,
} from '@corelog/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { NavMenu } from './navigation';
import { TopMenu } from './nav-menu';

const projectsMenu: NavMenu = { key: 'projects', label: 'Proyectos', path: '/projects' };
const teamsMenu: NavMenu = {
  key: 'teams',
  label: 'Equipos',
  path: '/teams',
  extras: [{ key: 'people', label: 'Ver personas', path: '/people' }],
};

function projectItem(id: string, name: string, key: string): FavoriteItem & RecentProjectItem {
  return {
    type: 'project',
    id,
    name,
    key,
    color: '',
    createdAt: '2026-01-01T00:00:00Z',
    viewedAt: '2026-01-02T00:00:00Z',
  };
}

const platformAdmin = { id: 'u1', role: 'admin' } as User;
const plainUser = { id: 'u2', role: 'user' } as User;

interface Setup {
  fixture: ComponentFixture<TopMenu>;
  el: HTMLElement;
  fetchFavorites: ReturnType<typeof vi.fn>;
  fetchRecent: ReturnType<typeof vi.fn>;
  favorites: ReturnType<typeof signal<FavoriteItem[]>>;
  recent: ReturnType<typeof signal<RecentProjectItem[]>>;
  loading: ReturnType<typeof signal<boolean>>;
  failed: ReturnType<typeof signal<unknown>>;
  fetchTeams: ReturnType<typeof vi.fn>;
}

function setup(
  menu: NavMenu,
  options: { user?: User; currentPath?: string; canCreateProject?: boolean } = {},
): Setup {
  const fetchFavorites = vi.fn();
  const fetchRecent = vi.fn();
  const favorites = signal<FavoriteItem[]>([]);
  const recent = signal<RecentProjectItem[]>([]);
  const loading = signal(false);
  const failed = signal<unknown>(null);
  const fetchTeams = vi.fn();
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      {
        provide: AuthStore,
        useValue: { user: signal(options.user ?? plainUser), isAuthenticated: signal(true) },
      },
      {
        provide: OrganizationStore,
        useValue: {
          canCreateProject: signal(options.canCreateProject ?? options.user === platformAdmin),
        },
      },
      {
        provide: FavoritesStore,
        useValue: {
          favorites,
          recent,
          favoritesLoading: loading,
          recentLoading: loading,
          favoritesError: failed,
          recentError: failed,
          fetchFavorites,
          fetchRecent,
        },
      },
      {
        provide: TeamStore,
        useValue: {
          teams: signal([{ id: 't1', name: 'Plataforma', memberCount: 3 }]),
          fetchTeams,
        },
      },
    ],
  });
  const fixture = TestBed.createComponent(TopMenu);
  fixture.componentRef.setInput('menu', menu);
  fixture.componentRef.setInput('currentPath', options.currentPath ?? '');
  fixture.detectChanges();
  return {
    fixture,
    el: fixture.nativeElement as HTMLElement,
    fetchFavorites,
    fetchRecent,
    favorites,
    recent,
    loading,
    failed,
    fetchTeams,
  };
}

const trigger = (el: HTMLElement): HTMLButtonElement =>
  el.querySelector<HTMLButtonElement>('button[aria-haspopup="menu"]') as HTMLButtonElement;

function open(s: Setup): HTMLElement {
  trigger(s.el).click();
  s.fixture.detectChanges();
  return document.querySelector('[role="menu"]') as HTMLElement;
}

function rows(menu: HTMLElement): HTMLElement[] {
  return Array.from(menu.querySelectorAll<HTMLElement>('[role="menuitem"]'));
}

function press(target: Element, key: string): void {
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
}

describe('TopMenu', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('is closed until the trigger is used and exposes its state', () => {
    const s = setup(projectsMenu);
    expect(document.querySelector('[role="menu"]')).toBeNull();
    expect(trigger(s.el).getAttribute('aria-expanded')).toBe('false');
    const menu = open(s);
    expect(menu.getAttribute('aria-label')).toBe('Proyectos');
    expect(trigger(s.el).getAttribute('aria-expanded')).toBe('true');
    expect(trigger(s.el).getAttribute('aria-controls')).toBe(menu.id);
  });

  it('lists the starred and the recent projects with avatar, name and key', () => {
    const s = setup(projectsMenu);
    s.favorites.set([projectItem('p1', 'Proyecto demo', 'DEM')]);
    s.recent.set([projectItem('p2', 'Otro proyecto', 'OTR')]);
    const menu = open(s);
    expect(menu.textContent).toContain('Destacados');
    expect(menu.textContent).toContain('Recientes');
    expect(menu.textContent).toContain('Proyecto demo');
    expect(menu.textContent).toContain('OTR');
    expect(menu.querySelectorAll('app-project-avatar')).toHaveLength(2);
    expect(menu.textContent).toContain('Ver todos los proyectos');
    const hrefs = rows(menu).map((row) => row.getAttribute('href'));
    expect(hrefs).toContain('/projects/DEM');
    expect(hrefs).toContain('/projects/OTR');
  });

  it('does not repeat a starred project among the recent ones', () => {
    const s = setup(projectsMenu);
    s.favorites.set([projectItem('p1', 'Proyecto demo', 'DEM')]);
    s.recent.set([projectItem('p1', 'Proyecto demo', 'DEM')]);
    const menu = open(s);
    expect(menu.querySelectorAll('app-project-avatar')).toHaveLength(1);
    expect(menu.textContent).not.toContain('Recientes');
  });

  it('shows the empty text, the footer and no skeleton when nothing was visited', () => {
    const menu = open(setup(projectsMenu));
    expect(menu.textContent).toContain('Aún no has visitado proyectos');
    expect(menu.textContent).toContain('Ver todos los proyectos');
    expect(menu.querySelector('[data-testid="nav-menu-loading"]')).toBeNull();
  });

  it('shows a skeleton while the lists load', () => {
    const s = setup(projectsMenu);
    s.loading.set(true);
    const menu = open(s);
    expect(menu.querySelector('[data-testid="nav-menu-loading"]')).not.toBeNull();
    expect(menu.textContent).not.toContain('Aún no has visitado proyectos');
  });

  it('shows an error line but keeps the footer actions when the lists fail', () => {
    const s = setup(projectsMenu, { user: platformAdmin });
    s.failed.set({ status: 500 });
    const menu = open(s);
    expect(menu.querySelector('[data-testid="nav-menu-error"]')).not.toBeNull();
    expect(menu.textContent).toContain('Ver todos los proyectos');
    expect(menu.textContent).toContain('Crear proyecto');
  });

  it('links the create action to the new project page', () => {
    const menu = open(setup(projectsMenu, { user: platformAdmin }));
    const create = menu.querySelector('.nav-menu__create');
    expect(create?.getAttribute('href')).toBe('/projects/new');
  });

  it('says so when there is nothing recent for the filters menu', () => {
    const menu = open(setup({ key: 'filters', label: 'Filtros', path: '/issues' }));
    expect(menu.textContent).toContain('Aún no has visitado incidencias.');
  });

  it('lists the teams of the user and the people link', () => {
    const s = setup(teamsMenu);
    const menu = open(s);
    expect(menu.textContent).toContain('Plataforma');
    expect(menu.textContent).toContain('3 personas');
    expect(menu.textContent).toContain('Ver personas');
    expect(s.fetchTeams).not.toHaveBeenCalled();
  });

  it('offers the primary create action only when it is permitted', () => {
    expect(open(setup(projectsMenu, { user: platformAdmin })).textContent).toContain(
      'Crear proyecto',
    );
    TestBed.resetTestingModule();
    expect(open(setup(projectsMenu)).textContent).not.toContain('Crear proyecto');
  });

  it('offers the create action to an organization member allowed by the policy', () => {
    const menu = open(setup(projectsMenu, { canCreateProject: true }));
    expect(menu.textContent).toContain('Crear proyecto');
  });

  it('marks the menu of the current area as active', () => {
    const s = setup(projectsMenu, { currentPath: '/projects/p1/board' });
    expect(trigger(s.el).classList.contains('nav-menu__trigger--active')).toBe(true);
  });

  it('opens with the keyboard on the first row and moves with the arrows, Home and End', () => {
    const s = setup(teamsMenu);
    trigger(s.el).click();
    s.fixture.detectChanges();
    TestBed.tick();
    const menu = document.querySelector('[role="menu"]') as HTMLElement;
    const all = rows(menu);
    expect(all.length).toBeGreaterThanOrEqual(3);
    expect(document.activeElement).toBe(all[0]);
    press(menu, 'ArrowDown');
    expect(document.activeElement).toBe(all[1]);
    press(menu, 'End');
    expect(document.activeElement).toBe(all[all.length - 1]);
    press(menu, 'ArrowDown');
    expect(document.activeElement).toBe(all[0]);
    press(menu, 'ArrowUp');
    expect(document.activeElement).toBe(all[all.length - 1]);
    press(menu, 'Home');
    expect(document.activeElement).toBe(all[0]);
  });

  it('closes on Escape and returns focus to the trigger', () => {
    const s = setup(teamsMenu);
    trigger(s.el).click();
    s.fixture.detectChanges();
    TestBed.tick();
    const menu = document.querySelector('[role="menu"]') as HTMLElement;
    press(menu, 'Escape');
    s.fixture.detectChanges();
    expect(document.querySelector('[role="menu"]')).toBeNull();
    expect(document.activeElement).toBe(trigger(s.el));
  });

  it('closes when a click lands outside', () => {
    const s = setup(teamsMenu);
    open(s);
    document.body.click();
    s.fixture.detectChanges();
    expect(document.querySelector('[role="menu"]')).toBeNull();
  });

  it('navigates and closes when a row is chosen', async () => {
    const s = setup(teamsMenu);
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
    const menu = open(s);
    const people = rows(menu).find((row) => (row.textContent ?? '').includes('Ver personas'));
    people?.click();
    s.fixture.detectChanges();
    expect(navigate).toHaveBeenCalled();
    expect(document.querySelector('[role="menu"]')).toBeNull();
  });

  it('reads the starred and recent projects each time the menu opens', () => {
    const s = setup(projectsMenu);
    open(s);
    expect(s.fetchFavorites).toHaveBeenCalledTimes(1);
    expect(s.fetchRecent).toHaveBeenCalledTimes(1);
  });
});
