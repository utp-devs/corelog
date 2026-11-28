import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  ConsentStore,
  LegalStore,
  PhoneApiService,
  type ApiError,
  type ChangePasswordPayload,
  type ConsentStatus,
  type LegalDocumentSummary,
  type User,
} from '@corelog/core';
import { of, throwError } from 'rxjs';

import { PhoneVerifyDialog } from './phone-verify-dialog';
import { ProfilePasswordForm } from './password-form';
import { ProfilePrivacySection } from './privacy-section';

function apiError(code: string): ApiError {
  return { status: 422, code, message: code, details: [] };
}

async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function button(el: HTMLElement, text: string): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
    b.textContent?.trim().startsWith(text),
  );
}

describe('ProfilePasswordForm', () => {
  afterEach(() => TestBed.resetTestingModule());

  function setup() {
    const save = vi.fn<(payload: ChangePasswordPayload) => Promise<void>>().mockResolvedValue();
    TestBed.configureTestingModule({ imports: [ProfilePasswordForm] });
    const fixture = TestBed.createComponent(ProfilePasswordForm);
    fixture.componentRef.setInput('save', save);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const type = (label: string, value: string) => {
      const found = Array.from(el.querySelectorAll('label')).find((l) =>
        l.textContent?.trim().startsWith(label),
      );
      const input = el.querySelector<HTMLInputElement>(`#${found?.htmlFor}`) as HTMLInputElement;
      input.value = value;
      input.dispatchEvent(new Event('input'));
      input.dispatchEvent(new Event('blur'));
    };
    return { fixture, el, save, type };
  }

  it('shows a live mismatch message on the confirmation and re-evaluates it', async () => {
    const { fixture, el, type } = setup();
    type('Nueva contraseña', 'Secreta123');
    type('Confirmar nueva contraseña', 'Secreta12');
    await settle(fixture);
    expect(el.textContent).toContain('Las contraseñas no coinciden');
    type('Nueva contraseña', 'Secreta12');
    await settle(fixture);
    expect(el.textContent).not.toContain('Las contraseñas no coinciden');
  });

  it('does not submit while the confirmation differs', async () => {
    const { fixture, el, save, type } = setup();
    type('Contraseña actual', 'vieja-clave');
    type('Nueva contraseña', 'Secreta123');
    type('Confirmar nueva contraseña', 'otra-cosa');
    el.querySelector<HTMLFormElement>('form')?.dispatchEvent(new Event('submit'));
    await settle(fixture);
    expect(save).not.toHaveBeenCalled();
  });

  it('sends the confirmation along with the passwords', async () => {
    const { fixture, el, save, type } = setup();
    type('Contraseña actual', 'vieja-clave');
    type('Nueva contraseña', 'Secreta123');
    type('Confirmar nueva contraseña', 'Secreta123');
    el.querySelector<HTMLFormElement>('form')?.dispatchEvent(new Event('submit'));
    await settle(fixture);
    expect(save).toHaveBeenCalledWith({
      currentPassword: 'vieja-clave',
      newPassword: 'Secreta123',
      newPasswordConfirmation: 'Secreta123',
    });
  });
});

describe('PhoneVerifyDialog', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
    vi.useRealTimers();
  });

  function setup(api: Partial<Record<'requestVerification' | 'verify', ReturnType<typeof vi.fn>>>) {
    const verified: User[] = [];
    TestBed.configureTestingModule({
      imports: [PhoneVerifyDialog],
      providers: [{ provide: PhoneApiService, useValue: api }],
    });
    const fixture = TestBed.createComponent(PhoneVerifyDialog);
    fixture.componentInstance.verified.subscribe((u) => verified.push(u));
    fixture.componentRef.setInput('open', true);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, verified };
  }

  const sent = { destination: '+51******678', expiresAt: '2026-01-01T00:10:00Z' };

  it('asks for a code on open and shows the masked destination', async () => {
    const requestVerification = vi.fn(() => of(sent));
    const { fixture, el } = setup({ requestVerification });
    await settle(fixture);
    expect(requestVerification).toHaveBeenCalledTimes(1);
    expect(el.textContent).toContain('+51******678');
    expect(el.textContent).toContain('código de 6 dígitos');
    expect(el.textContent).toMatch(/Podrás pedir otro código en \d+ s/);
  });

  it('keeps only six digits and enables "Verificar" when complete', async () => {
    const { fixture, el } = setup({ requestVerification: vi.fn(() => of(sent)) });
    await settle(fixture);
    const input = el.querySelector<HTMLInputElement>('#phone-verify-code') as HTMLInputElement;
    input.value = '12ab34567';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(input.value).toBe('123456');
    expect(button(el, 'Verificar')?.disabled).toBe(false);
    input.value = '123';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(button(el, 'Verificar')?.disabled).toBe(true);
  });

  it('verifies the code, shows the success state and emits the account', async () => {
    const account = { id: 'u', phone: { verified: true } } as unknown as User;
    const verify = vi.fn(() => of(account));
    const { fixture, el, verified } = setup({ requestVerification: vi.fn(() => of(sent)), verify });
    await settle(fixture);
    const input = el.querySelector<HTMLInputElement>('#phone-verify-code') as HTMLInputElement;
    input.value = '123456';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button(el, 'Verificar')?.click();
    await settle(fixture);
    expect(verify).toHaveBeenCalledWith('123456');
    expect(el.textContent).toContain('Teléfono verificado');
    expect(verified).toEqual([account]);
  });

  it.each([
    ['user.phone_code_invalid', 'El código no es correcto'],
    ['user.phone_code_expired', 'El código venció'],
    ['user.phone_code_attempts', 'Agotaste los intentos'],
  ])('explains %s without leaving the code step', async (code, text) => {
    const { fixture, el } = setup({
      requestVerification: vi.fn(() => of(sent)),
      verify: vi.fn(() => throwError(() => apiError(code))),
    });
    await settle(fixture);
    const input = el.querySelector<HTMLInputElement>('#phone-verify-code') as HTMLInputElement;
    input.value = '000000';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button(el, 'Verificar')?.click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(text);
    expect(el.querySelector('[data-step="code"]')).not.toBeNull();
  });

  it('shows the send limit and offers to retry', async () => {
    const requestVerification = vi
      .fn()
      .mockReturnValueOnce(throwError(() => apiError('user.phone_send_limit')))
      .mockReturnValue(of(sent));
    const { fixture, el } = setup({ requestVerification });
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('demasiados códigos');
    button(el, 'Reintentar')?.click();
    await settle(fixture);
    expect(el.textContent).toContain('+51******678');
  });

  it('counts down before another code can be requested', async () => {
    vi.useFakeTimers();
    const { fixture, el } = setup({ requestVerification: vi.fn(() => of(sent)) });
    await vi.advanceTimersByTimeAsync(0);
    fixture.detectChanges();
    expect(button(el, 'Enviar otro código')).toBeUndefined();
    await vi.advanceTimersByTimeAsync(30_000);
    fixture.detectChanges();
    expect(button(el, 'Enviar otro código')).toBeDefined();
  });
});

describe('ProfilePrivacySection', () => {
  afterEach(() => TestBed.resetTestingModule());

  const DOCS: LegalDocumentSummary[] = [
    ['privacy_policy', 'Política de privacidad', true, 2],
    ['cookies', 'Política de cookies', false, 1],
    ['communications', 'Comunicaciones', false, 1],
  ].map(([slug, title, required, version]) => ({
    slug: slug as string,
    title: title as string,
    version: version as number,
    summary: 'Resumen',
    required: required as boolean,
    effectiveFrom: '',
  }));

  function setup(accepted: ConsentStatus['accepted']) {
    const status = signal<ConsentStatus | null>({ accepted, pending: [] });
    const accept = vi.fn().mockResolvedValue(undefined);
    const revoke = vi.fn().mockResolvedValue(undefined);
    TestBed.configureTestingModule({
      imports: [ProfilePrivacySection],
      providers: [
        provideRouter([]),
        {
          provide: LegalStore,
          useValue: {
            documents: signal(DOCS),
            communications: signal(DOCS[2]),
            find: (slug: string) => DOCS.find((d) => d.slug === slug),
            load: () => Promise.resolve(),
            read: () => Promise.resolve({ ...DOCS[0], bodyMarkdown: 'x', publishedAt: null }),
          },
        },
        {
          provide: ConsentStore,
          useValue: { status, loading: signal(false), load: vi.fn(), accept, revoke },
        },
      ],
    });
    const fixture = TestBed.createComponent(ProfilePrivacySection);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, accept, revoke };
  }

  it('lists the documents with the accepted version and date, and a way to read each', () => {
    const { el } = setup([
      { slug: 'privacy_policy', version: 1, acceptedAt: '2026-03-04T12:00:00Z' },
    ]);
    const list = el.querySelector('[data-testid="consent-documents"]');
    expect(list?.textContent).toContain('Aceptaste la versión 1 el 04/03/2026');
    expect(list?.textContent).toContain('Hay una versión más reciente (2)');
    expect(list?.textContent).toContain('Obligatorio');
    expect(list?.textContent).toContain('Documento informativo');
    expect(button(el, 'Leer')).toBeDefined();
    expect(el.textContent).toContain('no se pueden revocar');
  });

  it('opens a document in the read only dialog', async () => {
    const { fixture, el } = setup([]);
    button(el, 'Leer')?.click();
    await settle(fixture);
    expect(el.querySelector('[role="dialog"]')).not.toBeNull();
    expect(button(el, 'Aceptar y cerrar')).toBeUndefined();
  });

  it('grants the communications authorization from the switch', async () => {
    const { fixture, el, accept } = setup([]);
    const toggle = el.querySelector<HTMLElement>('[role="switch"]');
    toggle?.click();
    await settle(fixture);
    expect(accept).toHaveBeenCalledWith([{ slug: 'communications', version: 1 }], 'profile');
  });

  it('revokes the communications authorization when it was granted', async () => {
    const { fixture, el, revoke } = setup([
      { slug: 'communications', version: 1, acceptedAt: '2026-03-04T12:00:00Z' },
    ]);
    await settle(fixture);
    el.querySelector<HTMLElement>('[role="switch"]')?.click();
    await settle(fixture);
    expect(revoke).toHaveBeenCalledWith('communications');
  });
});
