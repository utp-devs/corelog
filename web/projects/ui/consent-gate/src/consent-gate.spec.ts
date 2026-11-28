import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  AuthStore,
  ConsentStore,
  LegalStore,
  type ConsentRef,
  type ConsentSource,
  type PendingConsent,
} from '@corelog/core';

import { ConsentGate } from './consent-gate';

const PENDING: PendingConsent[] = [
  { slug: 'privacy_policy', title: 'Política de privacidad', version: 2 },
  { slug: 'terms_of_use', title: 'Términos de uso', version: 4 },
];

interface Setup {
  fixture: ComponentFixture<ConsentGate>;
  el: HTMLElement;
  pending: ReturnType<typeof signal<readonly PendingConsent[]>>;
  accept: ReturnType<
    typeof vi.fn<(refs: readonly ConsentRef[], source: ConsentSource) => Promise<void>>
  >;
  logout: ReturnType<typeof vi.fn>;
}

function setup(initial: readonly PendingConsent[] = PENDING): Setup {
  const pending = signal<readonly PendingConsent[]>(initial);
  const accept = vi
    .fn<(refs: readonly ConsentRef[], source: ConsentSource) => Promise<void>>()
    .mockImplementation(async () => {
      pending.set([]);
    });
  const logout = vi.fn();
  TestBed.configureTestingModule({
    imports: [ConsentGate],
    providers: [
      provideRouter([]),
      {
        provide: ConsentStore,
        useValue: {
          pending,
          hasPending: () => pending().length > 0,
          loading: signal(false),
          accept,
        },
      },
      { provide: AuthStore, useValue: { logout } },
      {
        provide: LegalStore,
        useValue: {
          read: (slug: string) =>
            Promise.resolve({
              slug,
              title: slug,
              version: 2,
              summary: '',
              required: true,
              effectiveFrom: '',
              bodyMarkdown: 'Texto',
              publishedAt: null,
            }),
        },
      },
    ],
  });
  const fixture = TestBed.createComponent(ConsentGate);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, pending, accept, logout };
}

async function settle(fixture: ComponentFixture<ConsentGate>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function button(el: HTMLElement, text: string): HTMLElement | undefined {
  return Array.from(el.querySelectorAll<HTMLElement>('button, [role="button"]')).find((b) =>
    b.textContent?.trim().startsWith(text),
  );
}

function tickAll(el: HTMLElement): void {
  el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((box) => {
    box.checked = true;
    box.dispatchEvent(new Event('change'));
  });
}

describe('ConsentGate', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('stays hidden while nothing is pending', () => {
    const { el } = setup([]);
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });

  it('lists every changed document with its version and a way to read it', () => {
    const { el } = setup();
    const dialog = el.querySelector('[role="dialog"]');
    expect(dialog?.textContent).toContain('Política de privacidad');
    expect(dialog?.textContent).toContain('versión 2');
    expect(dialog?.textContent).toContain('Términos de uso');
    expect(dialog?.textContent).toContain('versión 4');
  });

  it('cannot be dismissed with Escape, a click outside or a close button', async () => {
    const { fixture, el } = setup();
    el.querySelector('[role="dialog"]')?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }),
    );
    el.querySelector('.dialog')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    await settle(fixture);
    expect(el.querySelector('[role="dialog"]')).not.toBeNull();
    expect(el.querySelector('button[aria-label="Cerrar"]')).toBeNull();
  });

  it('enables "Aceptar y continuar" only once every document is ticked', async () => {
    const { fixture, el } = setup();
    expect((button(el, 'Aceptar y continuar') as HTMLButtonElement | undefined)?.disabled).toBe(
      true,
    );
    const first = el.querySelector<HTMLInputElement>('input[type="checkbox"]') as HTMLInputElement;
    first.checked = true;
    first.dispatchEvent(new Event('change'));
    await settle(fixture);
    expect((button(el, 'Aceptar y continuar') as HTMLButtonElement | undefined)?.disabled).toBe(
      true,
    );
    tickAll(el);
    await settle(fixture);
    expect((button(el, 'Aceptar y continuar') as HTMLButtonElement | undefined)?.disabled).toBe(
      false,
    );
  });

  it('accepts the shown versions with the login_reprompt source and closes', async () => {
    const { fixture, el, accept } = setup();
    tickAll(el);
    await settle(fixture);
    button(el, 'Aceptar y continuar')?.click();
    await settle(fixture);
    expect(accept).toHaveBeenCalledWith(
      [
        { slug: 'privacy_policy', version: 2 },
        { slug: 'terms_of_use', version: 4 },
      ],
      'login_reprompt',
    );
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });

  it('keeps the gate open and explains the failure when the acceptance is rejected', async () => {
    const { fixture, el, accept } = setup();
    accept.mockRejectedValueOnce({
      status: 422,
      code: 'legal.version_invalid',
      message: 'Versión no vigente',
      details: [],
    });
    tickAll(el);
    await settle(fixture);
    button(el, 'Aceptar y continuar')?.click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Versión no vigente');
    expect(el.querySelector('[role="dialog"]')).not.toBeNull();
  });

  it('signs the person out from the secondary action', () => {
    const { el, logout } = setup();
    button(el, 'Cerrar sesión')?.click();
    expect(logout).toHaveBeenCalled();
  });

  it('opens the document from its link and ticks it when accepted there', async () => {
    const { fixture, el } = setup();
    button(el, 'Política de privacidad')?.click();
    await settle(fixture);
    const dialogs = el.querySelectorAll('[role="dialog"]');
    expect(dialogs.length).toBe(2);
    const accept = Array.from(dialogs[1].querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      b.textContent?.includes('Aceptar y cerrar'),
    );
    accept?.click();
    await settle(fixture);
    expect(el.querySelectorAll('[role="dialog"]').length).toBe(1);
    expect(el.querySelector<HTMLInputElement>('input[type="checkbox"]')?.checked).toBe(true);
  });
});
