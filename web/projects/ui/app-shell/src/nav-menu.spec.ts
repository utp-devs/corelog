import { signal } from '@angular/core';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { AuthStore, ProjectStore, TeamStore, pushRecent, type User } from '@corelog/core';
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

const platformAdmin = { id: 'u1', role: 'admin' } as User;
const plainUser = { id: 'u2', role: 'user' } as User;

interface Setup {
  fixture: ComponentFixture<TopMenu>;
  el: HTMLElement;
  fetchProjects: ReturnType<typeof vi.fn>;
  fetchTeams: ReturnType<typeof vi.fn>;
}

function setup(menu: NavMenu, options: { user?: User; currentPath?: string } = {}): Setup {
  const fetchProjects = vi.fn();
  const fetchTeams = vi.fn();
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      {
        provide: AuthStore,
        useValue: { user: signal(options.user ?? plainUser), isAuthenticated: signal(true) },
      },
      { provide: ProjectStore, useValue: { projects: signal([]), fetchProjects } },
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
  return { fixture, el: fixture.nativeElement as HTMLElement, fetchProjects, fetchTeams };
}

const trigger = (el: HTMLElement): HTMLButtonElement =>
  el.querySelector<HTMLButtonElement>('button[aria-haspopup="menu"]') as HTMLButtonElement;

function open(s: Setup): HTMLElement {
  trigger(s.el).click();
  s.fixture.detectChanges();
  return s.el.querySelector('[role="menu"]') as HTMLElement;
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
    expect(s.el.querySelector('[role="menu"]')).toBeNull();
    expect(trigger(s.el).getAttribute('aria-expanded')).toBe('false');
    const menu = open(s);
    expect(menu.getAttribute('aria-label')).toBe('Proyectos');
    expect(trigger(s.el).getAttribute('aria-expanded')).toBe('true');
    expect(trigger(s.el).getAttribute('aria-controls')).toBe(menu.id);
  });

  it('lists the recently visited projects with their key', () => {
    pushRecent({
      kind: 'project',
      id: 'p1',
      title: 'Proyecto demo',
      subtitle: 'DEM',
      link: '/projects/p1/board',
    });
    pushRecent({ kind: 'issue', id: 'i1', title: 'Falla', subtitle: 'DEM-1', link: '/issues/i1' });
    const menu = open(setup(projectsMenu));
    expect(menu.textContent).toContain('Recientes');
    expect(menu.textContent).toContain('Proyecto demo');
    expect(menu.textContent).toContain('DEM');
    expect(menu.textContent).not.toContain('Falla');
    expect(menu.textContent).toContain('Ver todos los proyectos');
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

  it('marks the menu of the current area as active', () => {
    const s = setup(projectsMenu, { currentPath: '/projects/p1/board' });
    expect(trigger(s.el).classList.contains('nav-menu__trigger--active')).toBe(true);
  });

  it('opens with the keyboard on the first row and moves with the arrows, Home and End', () => {
    const s = setup(teamsMenu);
    trigger(s.el).click();
    s.fixture.detectChanges();
    TestBed.tick();
    const menu = s.el.querySelector('[role="menu"]') as HTMLElement;
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
    const menu = s.el.querySelector('[role="menu"]') as HTMLElement;
    press(menu, 'Escape');
    s.fixture.detectChanges();
    expect(s.el.querySelector('[role="menu"]')).toBeNull();
    expect(document.activeElement).toBe(trigger(s.el));
  });

  it('closes when a click lands outside', () => {
    const s = setup(teamsMenu);
    open(s);
    document.body.click();
    s.fixture.detectChanges();
    expect(s.el.querySelector('[role="menu"]')).toBeNull();
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
    expect(s.el.querySelector('[role="menu"]')).toBeNull();
  });

  it('loads the projects when the menu opens and none are known', () => {
    const s = setup(projectsMenu);
    open(s);
    expect(s.fetchProjects).toHaveBeenCalledTimes(1);
  });
});
