import { ComponentFixture, TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import {
  ApiError,
  AuthStore,
  HOME_URL,
  InvitationPreview,
  LOCATION_ASSIGN,
  NotificationService,
  OrgLogoService,
  OrganizationMembership,
  OrganizationStore,
  User,
} from '@corelog/core';
import { Subject, of, throwError } from 'rxjs';
import { vi } from 'vitest';

import { InviteAcceptPage } from './invite-accept-page';

const TOKEN = 'tok-abcdefghijklmnop';

const USER: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
  role: 'user',
  teamId: null,
  displayName: 'Ana',
  jobTitle: '',
  department: '',
  avatarKey: null,
  timezone: 'UTC',
  createdAt: '2026-01-01T00:00:00Z',
  lastLoginAt: null,
  emailVerifiedAt: null,
} as User;

function preview(patch: Partial<InvitationPreview> = {}): InvitationPreview {
  return {
    organizationId: 'org-1',
    organizationName: 'Acme',
    hasLogo: false,
    invitedBy: null,
    role: 'member',
    email: 'ana@example.com',
    status: 'pending',
    expiresAt: '2026-12-31T00:00:00Z',
    ...patch,
  };
}

function apiError(patch: Partial<ApiError>): ApiError {
  return { status: 500, code: 'unknown', message: 'boom', details: [], ...patch };
}

interface Fakes {
  previewInvitation: ReturnType<typeof vi.fn>;
  acceptInvitation: ReturnType<typeof vi.fn>;
  fetchOrgs: ReturnType<typeof vi.fn>;
  setCurrentOrg: ReturnType<typeof vi.fn>;
  success: ReturnType<typeof vi.fn>;
  assign: ReturnType<typeof vi.fn>;
  logout: ReturnType<typeof vi.fn>;
  watchLogo: ReturnType<typeof vi.fn>;
}

function setup(options: { user?: User | null; previewResult?: unknown } = {}): {
  fixture: ComponentFixture<InviteAcceptPage>;
  el: HTMLElement;
  fakes: Fakes;
  router: Router;
} {
  const fakes: Fakes = {
    previewInvitation: vi.fn(() =>
      options.previewResult !== undefined ? options.previewResult : of(preview()),
    ),
    acceptInvitation: vi.fn(() =>
      of<OrganizationMembership>({
        organizationId: 'org-acme',
        userId: 'u1',
        role: 'member',
        joinedAt: '2026-01-02T00:00:00Z',
      }),
    ),
    fetchOrgs: vi.fn(),
    setCurrentOrg: vi.fn(),
    success: vi.fn(),
    assign: vi.fn(),
    logout: vi.fn(),
    watchLogo: vi.fn((_orgId: string, hasLogo: boolean) => ({
      url: signal<string | null>(hasLogo ? 'blob:logo' : null).asReadonly(),
      release: vi.fn(),
    })),
  };
  const user = options.user === undefined ? USER : options.user;

  TestBed.configureTestingModule({
    imports: [InviteAcceptPage],
    providers: [
      provideRouter([]),
      { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap({ token: TOKEN })) } },
      {
        provide: AuthStore,
        useValue: {
          sessionChecked: () => true,
          user: () => user,
          logout: fakes.logout,
        },
      },
      {
        provide: OrganizationStore,
        useValue: {
          previewInvitation: fakes.previewInvitation,
          acceptInvitation: fakes.acceptInvitation,
          fetchOrgs: fakes.fetchOrgs,
          setCurrentOrg: fakes.setCurrentOrg,
        },
      },
      { provide: OrgLogoService, useValue: { watch: fakes.watchLogo } },
      { provide: NotificationService, useValue: { success: fakes.success } },
      { provide: LOCATION_ASSIGN, useValue: fakes.assign },
      { provide: HOME_URL, useValue: '/workspace/' },
    ],
  });
  const router = TestBed.inject(Router);
  vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
  const fixture = TestBed.createComponent(InviteAcceptPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, fakes, router };
}

function button(el: HTMLElement, label: string): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find(
    (b) => b.textContent?.trim() === label,
  );
}

async function settle(fixture: ComponentFixture<InviteAcceptPage>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('InviteAcceptPage', () => {
  it('offers login and register to a visitor and returns to the invitation after auth', async () => {
    const { fixture, el, router } = setup({ user: null });
    await settle(fixture);
    expect(el.querySelector('h1')?.textContent).toContain('Invitación a Acme');
    expect(button(el, 'Aceptar invitación')).toBeUndefined();
    button(el, 'Iniciar sesión')?.click();
    expect(router.navigateByUrl).toHaveBeenCalledWith(
      `/login?returnUrl=${encodeURIComponent(`/invite/${TOKEN}`)}`,
    );
  });

  it('shows the organization, role and invited email for a valid invitation', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    const details = el.querySelector('.invite-accept-page__card');
    expect(details?.textContent).toContain('Acme');
    expect(details?.textContent).toContain('Rol');
    expect(details?.textContent).toContain('Miembro');
    expect(details?.textContent).toContain('ana@example.com');
    expect(button(el, 'Aceptar invitación')?.disabled).toBe(false);
  });

  it('names the inviter in the invitation sentence when the invitation has one', async () => {
    const { fixture, el } = setup({
      previewResult: of(preview({ invitedBy: 'Lucía Gómez' })),
    });
    await settle(fixture);
    expect(el.textContent).toContain('Lucía Gómez te invita a unirte a Acme como Miembro.');
  });

  it('omits the inviter from the sentence when the invitation has none', async () => {
    const { fixture, el } = setup();
    await settle(fixture);
    expect(el.textContent).toContain('Se te invita a unirte a Acme como Miembro.');
    expect(el.textContent).not.toMatch(/null|undefined/);
  });

  it('shows the organization logo when the organization has one', async () => {
    const { fixture, el, fakes } = setup({ previewResult: of(preview({ hasLogo: true })) });
    await settle(fixture);
    expect(fakes.watchLogo).toHaveBeenCalledWith('org-1', true, 0);
    expect(el.querySelector('[data-testid="org-logo"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="org-logo-fallback"]')).toBeNull();
  });

  it('shows the initials tile when the organization has no logo', async () => {
    const { fixture, el, fakes } = setup();
    await settle(fixture);
    expect(fakes.watchLogo).toHaveBeenCalledWith('org-1', false, 0);
    expect(el.querySelector('[data-testid="org-logo"]')).toBeNull();
    expect(el.querySelector('[data-testid="org-logo-fallback"]')).not.toBeNull();
  });

  it('accepts, refreshes the organizations, selects the new one and opens the workspace', async () => {
    const { fixture, el, fakes } = setup();
    await settle(fixture);
    button(el, 'Aceptar invitación')?.click();
    await settle(fixture);
    expect(fakes.acceptInvitation).toHaveBeenCalledWith(TOKEN);
    expect(fakes.fetchOrgs).toHaveBeenCalled();
    expect(fakes.setCurrentOrg).toHaveBeenCalledWith('org-acme');
    expect(fakes.success).toHaveBeenCalledWith('Te has unido a Acme');
    expect(fakes.assign).toHaveBeenCalledWith('/workspace/');
  });

  it('explains an email mismatch returned by the server and offers to switch accounts', async () => {
    const { fixture, el, fakes } = setup({
      previewResult: of(preview({ email: 'otra@example.com' })),
    });
    fakes.acceptInvitation.mockReturnValue(
      throwError(() => apiError({ status: 403, code: 'org.email_mismatch', message: 'mismatch' })),
    );
    await settle(fixture);
    button(el, 'Aceptar invitación')?.click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Esta invitación es para otra@example.com',
    );
    expect(button(el, 'Cambiar de cuenta')).toBeDefined();
  });

  it('warns before accepting when the signed-in account is not the invited one', async () => {
    const { fixture, el } = setup({ previewResult: of(preview({ email: 'otra@example.com' })) });
    await settle(fixture);
    expect(el.textContent).toMatch(/has iniciado sesión como ana@example.com/);
  });

  it.each([
    ['accepted', /ya se ha utilizado/],
    ['revoked', /fue revocada/],
    ['expired', /ha caducado/],
  ] as const)('shows the %s state without an accept button', async (status, copy) => {
    const { fixture, el } = setup({ previewResult: of(preview({ status })) });
    await settle(fixture);
    expect(el.textContent).toMatch(copy);
    expect(button(el, 'Aceptar invitación')).toBeUndefined();
  });

  it('shows the invalid state for an unknown token', async () => {
    const { fixture, el } = setup({
      previewResult: throwError(() =>
        apiError({ status: 404, code: 'org.invitation_not_found', message: 'no' }),
      ),
    });
    await settle(fixture);
    expect(el.textContent).toContain('Este enlace no es válido.');
    expect(button(el, 'Aceptar invitación')).toBeUndefined();
  });

  it('says the user is already a member and offers the workspace', async () => {
    const { fixture, el, fakes } = setup();
    fakes.acceptInvitation.mockReturnValue(
      throwError(() => apiError({ status: 409, code: 'org.already_member', message: 'member' })),
    );
    await settle(fixture);
    button(el, 'Aceptar invitación')?.click();
    await settle(fixture);
    expect(el.textContent).toContain('Ya eres miembro de Acme');
    button(el, 'Ir al espacio de trabajo')?.click();
    expect(fakes.assign).toHaveBeenCalledWith('/workspace/');
  });

  it('offers a way home and to retry when the invitation could not be loaded', async () => {
    const { fixture, el, router } = setup({
      previewResult: throwError(() => apiError({ status: 500, message: 'boom' })),
    });
    await settle(fixture);
    expect(button(el, 'Reintentar')).toBeDefined();
    button(el, 'Ir al inicio')?.click();
    expect(router.navigateByUrl).toHaveBeenCalledWith('/');
  });

  it('shows a loading status while the preview is pending', () => {
    const pending = new Subject<InvitationPreview>();
    const { el } = setup({ previewResult: pending.asObservable() });
    expect(el.querySelector('[role="status"]')).not.toBeNull();
  });
});
