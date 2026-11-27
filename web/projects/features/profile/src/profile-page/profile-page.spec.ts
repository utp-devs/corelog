import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  AccountSecurityApiService,
  ApiError,
  AuthStore,
  AvatarApiService,
  NotificationApiService,
  PeopleApiService,
  ThemeService,
  UNSAVED_CHANGES_PROMPT,
  UNSAVED_TITLE,
  unsavedChangesGuard,
  User,
} from '@corelog/core';
import {
  installViewport,
  LocationRecorder,
  provideCorelogTesting,
  recordLocation,
} from '@corelog/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Observable, of, throwError } from 'rxjs';

import { ProfilePage } from './profile-page';

const user: User = {
  id: 'user-1',
  name: 'Ana Torres',
  email: 'ana@corelog.test',
  role: 'user',
  teamId: null,
  displayName: '',
  jobTitle: 'Analista',
  department: 'Operaciones',
  avatarKey: null,
  timezone: 'America/Bogota',
  createdAt: '2026-01-10T00:00:00Z',
  lastLoginAt: null,
  emailVerifiedAt: null,
};

const sessionsFixture = [
  {
    id: 'current-1',
    userAgent: 'Chrome en Windows',
    createdAt: '2026-11-22T10:00:00Z',
    lastSeenAt: '2026-11-22T11:00:00Z',
    current: true,
  },
  {
    id: 'other-1',
    userAgent: 'Movil de prueba',
    createdAt: '2026-11-21T08:00:00Z',
    lastSeenAt: '2026-11-21T09:00:00Z',
    current: false,
  },
];

interface Mocks {
  readonly location: LocationRecorder;
  readonly prompt: ReturnType<typeof vi.fn>;
  readonly auth: {
    user: ReturnType<typeof signal<User | null>>;
    changePassword: ReturnType<typeof vi.fn>;
    logout: ReturnType<typeof vi.fn>;
  };
  readonly people: { getMe: ReturnType<typeof vi.fn>; updateMe: ReturnType<typeof vi.fn> };
  readonly security: Record<
    'listSessions' | 'revokeSession' | 'revokeOtherSessions' | 'deactivateAccount',
    ReturnType<typeof vi.fn>
  >;
  readonly notifications: {
    getPreferences: ReturnType<typeof vi.fn>;
    updatePreferences: ReturnType<typeof vi.fn>;
  };
  readonly theme: {
    mode: ReturnType<typeof signal<'light' | 'dark'>>;
    setMode: ReturnType<typeof vi.fn>;
  };
}

function setup(): Mocks {
  const mocks: Mocks = {
    location: recordLocation(),
    prompt: vi.fn(async () => false),
    auth: {
      user: signal<User | null>(user),
      changePassword: vi.fn(async () => undefined),
      logout: vi.fn(),
    },
    people: {
      getMe: vi.fn(() => of({ ...user, avatarKey: 'k-1' })),
      updateMe: vi.fn((payload: Record<string, unknown>) => of({ ...user, ...payload })),
    },
    security: {
      listSessions: vi.fn(() => of(sessionsFixture)),
      revokeSession: vi.fn((): Observable<void> => of(undefined)),
      revokeOtherSessions: vi.fn((): Observable<void> => of(undefined)),
      deactivateAccount: vi.fn((): Observable<void> => of(undefined)),
    },
    notifications: {
      getPreferences: vi.fn(() => of({ inApp: true })),
      updatePreferences: vi.fn((inApp: boolean) => of({ inApp })),
    },
    theme: { mode: signal<'light' | 'dark'>('dark'), setMode: vi.fn() },
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter([
        {
          path: 'profile',
          children: [
            {
              path: '',
              title: 'Mi perfil',
              canDeactivate: [unsavedChangesGuard],
              component: ProfilePage,
            },
            {
              path: ':section',
              title: 'Mi perfil',
              canDeactivate: [unsavedChangesGuard],
              component: ProfilePage,
            },
          ],
        },
      ]),
      ...provideCorelogTesting(mocks.location),
      { provide: UNSAVED_CHANGES_PROMPT, useValue: mocks.prompt },
      { provide: AuthStore, useValue: mocks.auth },
      {
        provide: PeopleApiService,
        useValue: { getMe: mocks.people.getMe, updateMe: mocks.people.updateMe },
      },
      {
        provide: AvatarApiService,
        useValue: {
          getAvatar: vi.fn(() => of(null)),
          uploadMyAvatar: vi.fn(() => of('/users/user-1/avatar')),
          deleteMyAvatar: vi.fn(() => of(undefined)),
        },
      },
      { provide: AccountSecurityApiService, useValue: mocks.security },
      { provide: NotificationApiService, useValue: mocks.notifications },
      { provide: ThemeService, useValue: mocks.theme },
    ],
  });
  return mocks;
}

function render(path: string): Promise<RouterTestingHarness> {
  return RouterTestingHarness.create(path);
}

function root(harness: RouterTestingHarness): HTMLElement {
  return harness.fixture.nativeElement as HTMLElement;
}

/** Polls until the text shows up; the page loads its data asynchronously. */
async function expectText(harness: RouterTestingHarness, text: string): Promise<void> {
  await vi.waitFor(() => {
    harness.detectChanges();
    expect(root(harness).textContent).toContain(text);
  });
}

function buttonNamed(harness: RouterTestingHarness, name: string): HTMLButtonElement | undefined {
  const buttons = Array.from(
    root(harness).querySelectorAll('button') as NodeListOf<HTMLButtonElement>,
  );
  return buttons.find((button) => (button.textContent ?? '').replace(/\s+/g, ' ').trim() === name);
}

/** The input that a labelled field row points at, searched inside the given scope. */
function fieldLabelled(scope: ParentNode, label: RegExp): HTMLInputElement {
  const found = Array.from(scope.querySelectorAll('label')).find((element) =>
    label.test((element.textContent ?? '').replace(/\s+/g, ' ').trim()),
  );
  const control = found?.htmlFor ? document.getElementById(found.htmlFor) : null;
  if (!(control instanceof HTMLInputElement)) throw new Error(`no field for ${label}`);
  return control;
}

function typeInto(harness: RouterTestingHarness, label: RegExp, value: string): void {
  const input = fieldLabelled(root(harness), label);
  input.value = value;
  input.dispatchEvent(new Event('input'));
  harness.detectChanges();
}

function switchNamed(harness: RouterTestingHarness, name: string): HTMLElement | null {
  return root(harness).querySelector(`[role="switch"][aria-label="${name}"]`);
}

function navLink(harness: RouterTestingHarness, href: string): HTMLAnchorElement {
  const link = root(harness).querySelector(`nav a[href="${href}"]`);
  if (!(link instanceof HTMLAnchorElement)) throw new Error(`no link to ${href}`);
  return link;
}

describe('ProfilePage', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('general section', () => {
    it('shows the profile data with the email read-only and the save button disabled until a change', async () => {
      setup();
      const restoreViewport = installViewport(1280);
      try {
        const harness = await render('/profile/general');
        await expectText(harness, 'Ana Torres');

        expect(fieldLabelled(root(harness), /^Nombre\b(?! para)/).value).toBe('Ana Torres');
        expect(fieldLabelled(root(harness), /^Cargo/).value).toBe('Analista');
        expect(fieldLabelled(root(harness), /^Correo electrónico/).disabled).toBe(true);
        expect(root(harness).textContent).toContain('America/Bogota');
        expect(buttonNamed(harness, 'Guardar cambios')?.disabled).toBe(true);
      } finally {
        restoreViewport();
      }
    });

    it('saves the changed fields and updates the signed-in user', async () => {
      const mocks = setup();
      const harness = await render('/profile/general');
      await expectText(harness, 'Ana Torres');

      typeInto(harness, /^Nombre para mostrar/, 'Ana T.');
      buttonNamed(harness, 'Guardar cambios')?.click();

      await vi.waitFor(() =>
        expect(mocks.people.updateMe).toHaveBeenCalledWith(
          expect.objectContaining({
            name: 'Ana Torres',
            displayName: 'Ana T.',
            jobTitle: 'Analista',
            timezone: 'America/Bogota',
          }),
        ),
      );
      await expectText(harness, 'Ana T.');
    });

    it('maps server field errors to the matching field', async () => {
      const mocks = setup();
      mocks.people.updateMe.mockReturnValueOnce(
        throwError((): ApiError => ({
          status: 422,
          code: 'validation.failed',
          message: 'Revisa los datos',
          details: [{ field: 'job_title', message: 'Cargo demasiado largo' }],
        })),
      );
      const harness = await render('/profile/general');
      await expectText(harness, 'Ana Torres');

      typeInto(harness, /^Cargo/, 'Otro cargo');
      buttonNamed(harness, 'Guardar cambios')?.click();

      await expectText(harness, 'Cargo demasiado largo');
    });

    it('maps the timezone error code to the timezone field', async () => {
      const mocks = setup();
      mocks.people.updateMe.mockReturnValueOnce(
        throwError((): ApiError => ({
          status: 422,
          code: 'user.timezone_invalid',
          message: 'Zona horaria no válida',
          details: [],
        })),
      );
      const harness = await render('/profile/general');
      await expectText(harness, 'Ana Torres');

      typeInto(harness, /^Cargo/, 'Otro cargo');
      buttonNamed(harness, 'Guardar cambios')?.click();

      await expectText(harness, 'Zona horaria no válida.');
    });

    it('asks before leaving the section with unsaved changes and stays when the user chooses to keep editing', async () => {
      const mocks = setup();
      mocks.prompt.mockResolvedValueOnce(false);
      const harness = await render('/profile/general');
      await expectText(harness, 'Ana Torres');

      typeInto(harness, /^Nombre para mostrar/, 'Ana T.');
      navLink(harness, '/profile/notifications').click();

      await vi.waitFor(() =>
        expect(mocks.prompt).toHaveBeenCalledWith(
          expect.objectContaining({ title: UNSAVED_TITLE }),
        ),
      );
      harness.detectChanges();
      expect(fieldLabelled(root(harness), /^Nombre para mostrar/).value).toBe('Ana T.');
      expect(switchNamed(harness, 'Avisos en la aplicación')).toBeNull();
    });

    it('leaves the section after confirming the exit', async () => {
      const mocks = setup();
      mocks.prompt.mockResolvedValueOnce(true);
      const harness = await render('/profile/general');
      await expectText(harness, 'Ana Torres');

      typeInto(harness, /^Nombre para mostrar/, 'Ana T.');
      navLink(harness, '/profile/notifications').click();

      await vi.waitFor(() => {
        harness.detectChanges();
        expect(switchNamed(harness, 'Avisos en la aplicación')).not.toBeNull();
      });
    });
  });

  describe('other sections', () => {
    it('lists the sessions and closes one that is not the current session', async () => {
      const mocks = setup();
      const harness = await render('/profile/security');
      await expectText(harness, 'Movil de prueba');
      expect(root(harness).textContent).toContain('Esta sesión');

      buttonNamed(harness, 'Cerrar')?.click();

      await vi.waitFor(() => expect(mocks.security.revokeSession).toHaveBeenCalledWith('other-1'));
    });

    it('requires the password before deactivating the account', async () => {
      const mocks = setup();
      const harness = await render('/profile/security');
      await expectText(harness, 'Movil de prueba');

      buttonNamed(harness, 'Eliminar cuenta')?.click();
      await expectText(harness, 'Se cerrarán todas tus sesiones');

      const dialog = root(harness).querySelector('.p-dialog') as HTMLElement;
      const confirm = Array.from(
        dialog.querySelectorAll('button') as NodeListOf<HTMLButtonElement>,
      ).find((button) => button.textContent?.trim() === 'Eliminar cuenta');
      expect(confirm?.disabled).toBe(true);

      const password = fieldLabelled(dialog, /^Contraseña actual/);
      password.value = 'clave-prueba';
      password.dispatchEvent(new Event('input'));
      harness.detectChanges();
      expect(confirm?.disabled).toBe(false);
      confirm?.click();

      await vi.waitFor(() =>
        expect(mocks.security.deactivateAccount).toHaveBeenCalledWith('clave-prueba'),
      );
      expect(mocks.auth.logout).toHaveBeenCalled();
      await vi.waitFor(() => expect(mocks.location.urls).toContain('/login'));
    });

    it('saves the in-app notification preference when the switch changes', async () => {
      const mocks = setup();
      const harness = await render('/profile/notifications');
      await vi.waitFor(() => {
        harness.detectChanges();
        expect(switchNamed(harness, 'Avisos en la aplicación')).not.toBeNull();
      });

      (switchNamed(harness, 'Avisos en la aplicación') as HTMLElement).click();

      await vi.waitFor(() =>
        expect(mocks.notifications.updatePreferences).toHaveBeenCalledWith(false),
      );
    });

    it('changes the theme from the appearance section', async () => {
      const mocks = setup();
      const harness = await render('/profile/appearance');
      await expectText(harness, 'Apariencia');

      buttonNamed(harness, 'Claro')?.click();

      expect(mocks.theme.setMode).toHaveBeenCalledWith('light');
    });

    it('shows the placeholder for tokens', async () => {
      setup();
      const harness = await render('/profile/tokens');
      await expectText(harness, 'Próximamente');

      const heading = Array.from(root(harness).querySelectorAll('h2')).find(
        (h) => h.textContent?.trim() === 'Tokens',
      );
      expect(heading).toBeDefined();
    });
  });

  it('shows the account summary with the role label', async () => {
    setup();
    const harness = await render('/profile');
    await expectText(harness, 'Usuario');

    expect(root(harness).querySelector('[aria-label="Resumen de la cuenta"]')).not.toBeNull();
  });
});
