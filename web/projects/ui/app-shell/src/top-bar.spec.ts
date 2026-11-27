import { Component, signal } from '@angular/core';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  AuthStore,
  AvatarApiService,
  NotificationStore,
  SearchApiService,
  type User,
} from '@corelog/core';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DEFAULT_SHELL_NAV_CONFIG } from './navigation';
import { TopBar } from './top-bar';

const user: User = {
  id: 'u1',
  name: 'Luis Ruiz',
  email: 'luis@example.com',
  role: 'user',
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

@Component({
  imports: [TopBar],
  template: `
    <ng-template #switcher><span>Switcher real</span></ng-template>
    <app-top-bar
      [menus]="menus"
      [canCreate]="canCreate"
      [orgName]="orgName"
      [orgSwitcher]="withSwitcher ? switcher : null"
      [showMenuButton]="showMenu"
    />
  `,
})
class TopBarHost {
  readonly menus = DEFAULT_SHELL_NAV_CONFIG.menus;
  canCreate = true;
  orgName: string | null = null;
  withSwitcher = false;
  showMenu = false;
}

describe('TopBar', () => {
  let host: TopBarHost;
  let fixture: ComponentFixture<TopBarHost>;
  let el: HTMLElement;

  function render(options: Partial<TopBarHost> = {}, width = 1440): void {
    setViewport(width);
    TestBed.configureTestingModule({
      imports: [TopBarHost],
      providers: [
        provideRouter([]),
        { provide: AuthStore, useValue: { user: signal<User | null>(user), logout: vi.fn() } },
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
    fixture = TestBed.createComponent(TopBarHost);
    host = fixture.componentInstance;
    Object.assign(host, options);
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  const button = (name: string): HTMLButtonElement | null =>
    el.querySelector<HTMLButtonElement>(`button[aria-label="${name}"]`);

  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    delete (window as Partial<Window>).matchMedia;
  });

  it('shows the primary menus and Equipos to every user', () => {
    render();
    const nav = el.querySelector('nav[aria-label="Principal"]') as HTMLElement;
    expect(nav.textContent).toContain('Proyectos');
    expect(nav.textContent).toContain('Filtros');
    expect(nav.textContent).toContain('Paneles');
    expect(nav.textContent).toContain('Equipos');
  });

  it('keeps Equipos in the menus for every role', () => {
    render({ canCreate: false });
    const nav = el.querySelector('nav[aria-label="Principal"]') as HTMLElement;
    expect(nav.textContent).toContain('Equipos');
  });

  it('hides the Crear button when the shell does not allow creating (no signed-in user)', () => {
    render({ canCreate: false });
    expect(el.querySelector('button.create')).toBeNull();
    expect(el.querySelector('nav[aria-label="Principal"]')?.textContent).toContain('Proyectos');
  });

  it('opens the placeholder create dialog from the Crear button', () => {
    render();
    el.querySelector<HTMLButtonElement>('button.create')?.click();
    fixture.detectChanges();
    const dialog = document.querySelector('[role="dialog"]');
    expect(dialog?.textContent).toContain('Crear incidencia');
  });

  it('renders the notes button disabled', () => {
    render();
    expect(button('Notas (próximamente)')?.disabled).toBe(true);
  });

  it('renders the real org switcher slot instead of the organization name', () => {
    render({ withSwitcher: true, orgName: 'Acme' });
    expect(el.textContent).toContain('Switcher real');
    expect(el.textContent).not.toContain('Acme');
  });

  it('shows the menu button only when the shell asks for it', () => {
    render();
    expect(button('Abrir menú de navegación')).toBeNull();
    fixture.destroy();
    TestBed.resetTestingModule();
    render({ showMenu: true });
    expect(button('Abrir menú de navegación')).not.toBeNull();
  });
});

describe('TopBar responsive layout', () => {
  let host: TopBarHost;
  let fixture: ComponentFixture<TopBarHost>;
  let el: HTMLElement;

  function render(options: Partial<TopBarHost>, width: number): void {
    setViewport(width);
    TestBed.configureTestingModule({
      imports: [TopBarHost],
      providers: [
        provideRouter([]),
        { provide: AuthStore, useValue: { user: signal<User | null>(user), logout: vi.fn() } },
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
    fixture = TestBed.createComponent(TopBarHost);
    host = fixture.componentInstance;
    Object.assign(host, options);
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  const button = (name: string): HTMLButtonElement | null =>
    el.querySelector<HTMLButtonElement>(`button[aria-label="${name}"]`);

  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    delete (window as Partial<Window>).matchMedia;
  });

  it('at 1280 px shows the full bar: menus, text Create, search field and notes', () => {
    render({ showMenu: true, orgName: 'Acme', withSwitcher: true }, 1280);
    expect(el.querySelector('nav[aria-label="Principal"]')?.textContent).toContain('Equipos');
    expect(el.querySelector('button.create')?.textContent).toContain('Crear');
    expect(el.querySelector('input[aria-label="Buscar proyectos e incidencias"]')).not.toBeNull();
    expect(button('Notas (próximamente)')).not.toBeNull();
    expect(el.textContent).toContain('Switcher real');
    expect(button('Buscar')).toBeNull();
  });

  it('at 1100 px keeps the menus but turns the search field into an icon', () => {
    render({}, 1100);
    expect(el.querySelector('nav[aria-label="Principal"]')).not.toBeNull();
    expect(el.querySelector('input[aria-label="Buscar proyectos e incidencias"]')).toBeNull();
    expect(button('Buscar')).not.toBeNull();
  });

  it('at 820 px collapses the menus into the Menú button and keeps Create, switcher, search icon and avatar', () => {
    render({ showMenu: true, orgName: 'Acme', withSwitcher: true }, 820);
    expect(el.querySelector('nav[aria-label="Principal"]')).toBeNull();
    expect(button('Abrir menú de navegación')?.textContent).toContain('Menú');
    expect(el.querySelector('button.create')?.textContent).toContain('Crear');
    expect(el.textContent).toContain('Switcher real');
    expect(button('Buscar')).not.toBeNull();
    expect(button('Menú de Luis Ruiz')).not.toBeNull();
  });

  it('at 390 px shows icon-only controls, the logo icon and no org switcher or notes in the bar', () => {
    render({ showMenu: true, orgName: 'Acme', withSwitcher: true }, 390);
    const create = button('Crear');
    expect(create?.textContent?.trim()).toBe('');
    expect(create?.querySelector('app-shell-icon')).not.toBeNull();
    expect(button('Abrir menú de navegación')?.textContent?.trim()).toBe('');
    expect(button('Buscar')).not.toBeNull();
    expect(button('Menú de Luis Ruiz')).not.toBeNull();
    expect(el.textContent).not.toContain('Switcher real');
    expect(button('Notas (próximamente)')).toBeNull();
    const brand = el.querySelector('a[aria-label="CoreLog, ir a incidencias"] img');
    expect(brand?.getAttribute('src')).toBe('/brand/corelog-icon-64.png');
  });

  it('opens the command palette from the icon search button', () => {
    render({}, 820);
    button('Buscar')?.click();
    fixture.detectChanges();
    expect(
      document.querySelector('[role="dialog"][aria-label="Paleta de comandos"]'),
    ).not.toBeNull();
  });
});
