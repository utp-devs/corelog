import { signal } from '@angular/core';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  AuthStore,
  AvatarApiService,
  LOCATION_ASSIGN,
  ThemeService,
  type User,
} from '@corelog/core';
import { map, of, timer } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { UserMenu } from './user-menu';

const user: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
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

describe('UserMenu avatar', () => {
  let getAvatar: ReturnType<typeof vi.fn>;
  let current: ReturnType<typeof signal<User | null>>;

  beforeEach(() => {
    window.localStorage.clear();
    URL.createObjectURL = vi.fn(() => 'blob:ana-avatar');
    URL.revokeObjectURL = vi.fn();
    // Asynchronous like the HTTP client: the avatar cache registers its entry after subscribing.
    getAvatar = vi.fn(() => timer(0).pipe(map(() => new Blob(['png'], { type: 'image/png' }))));
    current = signal<User | null>(user);
    TestBed.configureTestingModule({
      imports: [UserMenu],
      providers: [
        provideRouter([]),
        { provide: AvatarApiService, useValue: { getAvatar } },
        { provide: AuthStore, useValue: { user: current, logout: vi.fn() } },
      ],
    });
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('shows the initials of the current user when there is no photo', () => {
    const fixture: ComponentFixture<UserMenu> = TestBed.createComponent(UserMenu);
    fixture.detectChanges();
    const avatar = fixture.nativeElement.querySelector('app-avatar') as HTMLElement;
    expect(avatar.textContent).toContain('AP');
    expect(getAvatar).not.toHaveBeenCalled();
  });

  it('loads the photo with the session when the user has an avatar', async () => {
    current.set({ ...user, avatarKey: 'k1' });
    const fixture = TestBed.createComponent(UserMenu);
    fixture.detectChanges();
    await new Promise((resolve) => setTimeout(resolve, 10));
    fixture.detectChanges();
    expect(document.querySelector('img[src="blob:ana-avatar"]')).not.toBeNull();
    expect(getAvatar).toHaveBeenCalledWith('u1');
  });
});

describe('UserMenu panel', () => {
  let current: ReturnType<typeof signal<User | null>>;
  let logout: ReturnType<typeof vi.fn>;
  let assign: ReturnType<typeof vi.fn>;

  function open(): { fixture: ComponentFixture<UserMenu>; menu: HTMLElement } {
    const fixture = TestBed.createComponent(UserMenu);
    fixture.detectChanges();
    const trigger = fixture.nativeElement.querySelector(
      'button[aria-haspopup="menu"]',
    ) as HTMLButtonElement;
    trigger.click();
    fixture.detectChanges();
    return { fixture, menu: fixture.nativeElement.querySelector('[role="menu"]') as HTMLElement };
  }

  const row = (menu: HTMLElement, text: string): HTMLElement =>
    Array.from(
      menu.querySelectorAll<HTMLElement>(
        '[role="menuitem"], [role="menuitemradio"], [role="menuitemcheckbox"]',
      ),
    ).find((item) => (item.textContent ?? '').includes(text)) as HTMLElement;

  beforeEach(() => {
    window.localStorage.clear();
    delete document.documentElement.dataset['density'];
    current = signal<User | null>(user);
    logout = vi.fn();
    assign = vi.fn();
    TestBed.configureTestingModule({
      imports: [UserMenu],
      providers: [
        provideRouter([]),
        { provide: AvatarApiService, useValue: { getAvatar: () => of(null) } },
        { provide: AuthStore, useValue: { user: current, logout } },
        { provide: LOCATION_ASSIGN, useValue: assign },
      ],
    });
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('shows the name, email and role of the user in a header that is not disabled', () => {
    const { menu } = open();
    expect(menu.textContent).toContain('Ana Pérez');
    expect(menu.textContent).toContain('ana@example.com');
    expect(menu.textContent).toContain('Usuario');
    expect(menu.querySelector('.user-menu__identity [aria-disabled="true"]')).toBeNull();
  });

  it('switches between light, dark and system and shows the active one', () => {
    const { fixture, menu } = open();
    const theme = TestBed.inject(ThemeService);
    expect(row(menu, 'Claro').getAttribute('aria-checked')).toBe('true');
    row(menu, 'Oscuro').click();
    fixture.detectChanges();
    expect(theme.mode()).toBe('dark');
    expect(row(menu, 'Oscuro').getAttribute('aria-checked')).toBe('true');
    expect(row(menu, 'Claro').getAttribute('aria-checked')).toBe('false');
    row(menu, 'Sistema').click();
    fixture.detectChanges();
    expect(theme.mode()).toBe('system');
    expect(fixture.nativeElement.querySelector('[role="menu"]')).not.toBeNull();
  });

  it('exposes the compact density as a checked switch', () => {
    const { fixture, menu } = open();
    const density = row(menu, 'Densidad compacta');
    expect(density.getAttribute('aria-checked')).toBe('false');
    density.click();
    fixture.detectChanges();
    expect(density.getAttribute('aria-checked')).toBe('true');
    expect(density.querySelector('.user-menu__switch--on')).not.toBeNull();
  });

  it('hints the shortcuts and opens them', () => {
    const fixture = TestBed.createComponent(UserMenu);
    const opened = vi.fn();
    fixture.componentInstance.openShortcuts.subscribe(opened);
    fixture.detectChanges();
    (
      fixture.nativeElement.querySelector('button[aria-haspopup="menu"]') as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    const menu = fixture.nativeElement.querySelector('[role="menu"]') as HTMLElement;
    expect(row(menu, 'Atajos de teclado').textContent).toContain('?');
    row(menu, 'Atajos de teclado').click();
    expect(opened).toHaveBeenCalledTimes(1);
  });

  it('signs out from a separated, danger-styled row', () => {
    const { menu } = open();
    const signOut = row(menu, 'Cerrar sesión');
    expect(signOut.classList.contains('user-menu__item--danger')).toBe(true);
    expect(signOut.previousElementSibling?.getAttribute('role')).toBe('separator');
    signOut.click();
    expect(logout).toHaveBeenCalledTimes(1);
    expect(assign).toHaveBeenCalledWith('/login');
  });
});
