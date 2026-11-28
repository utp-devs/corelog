import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter, Router } from '@angular/router';
import {
  ApiError,
  AuthStore,
  CountryStore,
  HOME_URL,
  LOCATION_ASSIGN,
  LegalStore,
  RegisterPayload,
  type Country,
  type LegalDocument,
  type LegalDocumentSummary,
} from '@corelog/core';
import { vi } from 'vitest';

import { RegisterPage } from './register-page';

const COUNTRIES: Country[] = [
  { code: 'PE', name: 'Perú', dialCode: '+51', example: '912345678', areas: [] },
  { code: 'CO', name: 'Colombia', dialCode: '+57', example: '3211234567', areas: [] },
];

const DOCUMENTS: LegalDocumentSummary[] = [
  ['privacy_policy', 'Política de privacidad', true],
  ['data_processing', 'Tratamiento de datos', true],
  ['terms_of_use', 'Términos de uso', true],
  ['communications', 'Comunicaciones', false],
].map(([slug, title, required]) => ({
  slug: slug as string,
  title: title as string,
  version: 1,
  summary: '',
  required: required as boolean,
  effectiveFrom: '2026-01-01T00:00:00Z',
}));

interface Setup {
  fixture: ComponentFixture<RegisterPage>;
  el: HTMLElement;
  register: ReturnType<typeof vi.fn<(payload: RegisterPayload) => Promise<void>>>;
  assign: ReturnType<typeof vi.fn<(url: string) => void>>;
}

async function setup(): Promise<Setup> {
  const register = vi
    .fn<(payload: RegisterPayload) => Promise<void>>()
    .mockResolvedValue(undefined);
  const assign = vi.fn<(url: string) => void>();
  const documents = signal<readonly LegalDocumentSummary[]>(DOCUMENTS);
  const legal = {
    documents,
    status: signal('ready'),
    communications: signal(DOCUMENTS[3]),
    find: (slug: string) => DOCUMENTS.find((d) => d.slug === slug),
    load: () => Promise.resolve(),
    read: (slug: string): Promise<LegalDocument> =>
      Promise.resolve({
        ...(DOCUMENTS.find((d) => d.slug === slug) as LegalDocumentSummary),
        bodyMarkdown: '# Texto del documento',
        publishedAt: null,
      }),
  };
  const countries = signal<readonly Country[]>(COUNTRIES);
  const countryStore = {
    countries,
    status: signal('ready'),
    find: (code: string) => COUNTRIES.find((c) => c.code === code),
    defaultCountry: () => COUNTRIES[0],
    load: () => Promise.resolve(),
  };
  TestBed.configureTestingModule({
    imports: [RegisterPage],
    providers: [
      provideRouter([]),
      { provide: ActivatedRoute, useValue: { snapshot: { queryParamMap: convertToParamMap({}) } } },
      { provide: AuthStore, useValue: { register } },
      { provide: LegalStore, useValue: legal },
      { provide: CountryStore, useValue: countryStore },
      { provide: LOCATION_ASSIGN, useValue: assign },
      { provide: HOME_URL, useValue: '/workspace/' },
    ],
  });
  vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
  const fixture = TestBed.createComponent(RegisterPage);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, register, assign };
}

/** The control a visible label points to. */
function controlOf(el: HTMLElement, label: string): HTMLInputElement | null {
  const found = Array.from(el.querySelectorAll('label')).find((l) =>
    l.textContent?.trim().startsWith(label),
  );
  return found ? el.querySelector<HTMLInputElement>(`#${found.htmlFor}`) : null;
}

function type(el: HTMLElement, label: string, value: string): void {
  const input = controlOf(el, label);
  if (!input) throw new Error(`missing ${label}`);
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

function submitButton(el: HTMLElement): HTMLButtonElement {
  return el.querySelector<HTMLButtonElement>('button[type="submit"]') as HTMLButtonElement;
}

function submit(el: HTMLElement): void {
  submitButton(el).click();
}

async function settle(fixture: ComponentFixture<RegisterPage>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function tick(el: HTMLElement, slug: string): void {
  const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
  const order = ['privacy_policy', 'data_processing', 'terms_of_use', 'communications'];
  const box = boxes[order.indexOf(slug)];
  box.checked = true;
  box.dispatchEvent(new Event('change'));
}

async function fillValid(s: Setup): Promise<void> {
  type(s.el, 'Nombres', 'Ana María');
  type(s.el, 'Apellidos', 'Torres');
  type(s.el, 'Teléfono', '912345678');
  type(s.el, 'Correo electrónico', 'ana@corelog.test');
  type(s.el, 'Contraseña', 'Secreta123');
  type(s.el, 'Confirmar contraseña', 'Secreta123');
  tick(s.el, 'privacy_policy');
  tick(s.el, 'data_processing');
  tick(s.el, 'terms_of_use');
  await settle(s.fixture);
}

describe('RegisterPage', () => {
  it('explains the rules of the names, the phone and the password in Spanish', async () => {
    const s = await setup();
    type(s.el, 'Nombres', 'Ana1');
    type(s.el, 'Apellidos', '   ');
    type(s.el, 'Contraseña', 'corta');
    type(s.el, 'Teléfono', '12');
    await settle(s.fixture);
    expect(s.el.textContent).toContain('Usa solo letras, espacios, guiones y apóstrofes');
    expect(s.el.textContent).toContain('Escribe tus apellidos');
    expect(s.el.textContent).toContain('La contraseña debe tener al menos 8 caracteres');
    expect(s.el.textContent).toContain('El teléfono no es válido para el país elegido');
    expect(s.register).not.toHaveBeenCalled();
  });

  it('shows a live mismatch message and re-evaluates it when the password changes', async () => {
    const s = await setup();
    type(s.el, 'Contraseña', 'Secreta123');
    type(s.el, 'Confirmar contraseña', 'Secreta12');
    await settle(s.fixture);
    expect(s.el.textContent).toContain('Las contraseñas no coinciden');

    type(s.el, 'Contraseña', 'Secreta12');
    await settle(s.fixture);
    expect(s.el.textContent).not.toContain('Las contraseñas no coinciden');

    type(s.el, 'Contraseña', 'Otra-clave-9');
    await settle(s.fixture);
    expect(s.el.textContent).toContain('Las contraseñas no coinciden');
  });

  it('keeps submit disabled until the required documents are ticked', async () => {
    const s = await setup();
    type(s.el, 'Nombres', 'Ana');
    type(s.el, 'Apellidos', 'Torres');
    type(s.el, 'Teléfono', '912345678');
    type(s.el, 'Correo electrónico', 'ana@corelog.test');
    type(s.el, 'Contraseña', 'Secreta123');
    type(s.el, 'Confirmar contraseña', 'Secreta123');
    await settle(s.fixture);
    expect(submitButton(s.el).disabled).toBe(true);

    tick(s.el, 'privacy_policy');
    tick(s.el, 'data_processing');
    await settle(s.fixture);
    expect(submitButton(s.el).disabled).toBe(true);

    tick(s.el, 'terms_of_use');
    await settle(s.fixture);
    expect(submitButton(s.el).disabled).toBe(false);
  });

  it('creates the account with names, phone, confirmation and the consented versions', async () => {
    const s = await setup();
    await fillValid(s);
    tick(s.el, 'communications');
    await settle(s.fixture);
    submit(s.el);
    await settle(s.fixture);
    expect(s.register).toHaveBeenCalledWith({
      firstName: 'Ana María',
      lastName: 'Torres',
      phone: { country: 'PE', number: '912345678' },
      email: 'ana@corelog.test',
      password: 'Secreta123',
      passwordConfirmation: 'Secreta123',
      locale: 'es',
      consents: [
        { slug: 'privacy_policy', version: 1 },
        { slug: 'data_processing', version: 1 },
        { slug: 'terms_of_use', version: 1 },
        { slug: 'communications', version: 1 },
      ],
    });
  });

  it('does not tick the optional communications box by itself', async () => {
    const s = await setup();
    await fillValid(s);
    submit(s.el);
    await settle(s.fixture);
    const sent = s.register.mock.calls[0][0];
    expect(sent.consents.map((c) => c.slug)).not.toContain('communications');
  });
});

describe('RegisterPage legal documents', () => {
  it('opens the document in a modal and ticks the box when it is accepted there', async () => {
    const s = await setup();
    const link = Array.from(s.el.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim() === 'Política de privacidad',
    );
    link?.click();
    await settle(s.fixture);
    expect(s.el.querySelector('[role="dialog"]')).not.toBeNull();
    expect(s.el.textContent).toContain('Texto del documento');

    const accept = Array.from(
      s.el.querySelectorAll<HTMLButtonElement>('[role="dialog"] button'),
    ).find((b) => b.textContent?.includes('Aceptar y cerrar'));
    accept?.click();
    await settle(s.fixture);
    expect(s.el.querySelector('[role="dialog"]')).toBeNull();
    const box = s.el.querySelector<HTMLInputElement>('input[type="checkbox"]');
    expect(box?.checked).toBe(true);
  });

  it('closes the modal without ticking when it is only closed', async () => {
    const s = await setup();
    const link = Array.from(s.el.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim() === 'Términos de uso',
    );
    link?.click();
    await settle(s.fixture);
    const close = Array.from(
      s.el.querySelectorAll<HTMLButtonElement>('[role="dialog"] button'),
    ).find((b) => b.textContent?.trim() === 'Cerrar');
    close?.click();
    await settle(s.fixture);
    expect(s.el.querySelector('[role="dialog"]')).toBeNull();
    const boxes = Array.from(s.el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    expect(boxes.some((b) => b.checked)).toBe(false);
  });

  it('offers the legal documents in the footer without ticking anything', async () => {
    const s = await setup();
    const footer = s.el.querySelector('nav[aria-label="Documentos legales"]');
    expect(
      Array.from(footer?.querySelectorAll('button') ?? []).map((b) => b.textContent?.trim()),
    ).toEqual(['Privacidad', 'Datos personales', 'Términos', 'Cookies']);
  });
});

describe('RegisterPage form behaviour', () => {
  it('uses the autocomplete tokens for names, email and new password', async () => {
    const { el } = await setup();
    expect(controlOf(el, 'Nombres')?.getAttribute('autocomplete')).toBe('given-name');
    expect(controlOf(el, 'Apellidos')?.getAttribute('autocomplete')).toBe('family-name');
    expect(controlOf(el, 'Correo electrónico')?.getAttribute('autocomplete')).toBe('email');
    expect(controlOf(el, 'Contraseña')?.getAttribute('autocomplete')).toBe('new-password');
    expect(controlOf(el, 'Confirmar contraseña')?.getAttribute('autocomplete')).toBe(
      'new-password',
    );
  });

  it('announces the password strength as the user types', async () => {
    const { fixture, el } = await setup();
    type(el, 'Contraseña', 'Secreta123!456');
    fixture.detectChanges();
    expect(el.textContent).toContain('Seguridad de la contraseña: Fuerte');
  });

  it('shows the password helper until the field has an error', async () => {
    const s = await setup();
    expect(s.el.textContent).toContain('Mínimo 8 caracteres.');
    type(s.el, 'Contraseña', 'corta');
    await settle(s.fixture);
    expect(s.el.textContent).not.toContain('Mínimo 8 caracteres.');
  });
});

describe('RegisterPage errors and navigation', () => {
  it('shows a Spanish alert when the email is already registered', async () => {
    const s = await setup();
    const taken: ApiError = {
      status: 409,
      code: 'auth.email_taken',
      message: 'email is already registered',
      details: [],
    };
    s.register.mockRejectedValue(taken);
    await fillValid(s);
    submit(s.el);
    await settle(s.fixture);
    expect(s.el.textContent).toContain('Ya existe una cuenta con este correo');
    expect(s.el.textContent).not.toContain('already registered');
  });

  it('puts a phone rejected by the server on the phone field', async () => {
    const s = await setup();
    const rejected: ApiError = {
      status: 422,
      code: 'user.phone_taken',
      message: 'taken',
      details: [],
    };
    s.register.mockRejectedValue(rejected);
    await fillValid(s);
    submit(s.el);
    await settle(s.fixture);
    expect(s.el.textContent).toContain('Este teléfono ya está en uso en otra cuenta.');
    expect(controlOf(s.el, 'Teléfono')?.getAttribute('aria-invalid')).toBe('true');
  });

  it('maps a missing consent reported by the server to the checkboxes', async () => {
    const s = await setup();
    const missing: ApiError = {
      status: 422,
      code: 'legal.consent_required',
      message: 'Debe aceptar',
      details: [{ field: 'terms_of_use', message: 'Falta' }],
    };
    s.register.mockRejectedValue(missing);
    await fillValid(s);
    submit(s.el);
    await settle(s.fixture);
    expect(s.el.querySelector('[role="alert"]')?.textContent).toContain(
      'Acepta los documentos obligatorios',
    );
  });

  it('links back to the sign-in page', async () => {
    const { el } = await setup();
    const link = Array.from(el.querySelectorAll('a')).find(
      (a) => a.textContent?.trim() === 'Iniciar sesión',
    );
    expect(link?.getAttribute('href')).toBe('/login');
  });

  it('disables the submit button while the request is pending', async () => {
    const s = await setup();
    let release: () => void = () => undefined;
    s.register.mockReturnValue(new Promise<void>((resolve) => (release = resolve)));
    await fillValid(s);
    submit(s.el);
    s.fixture.detectChanges();
    expect(submitButton(s.el).disabled).toBe(true);
    release();
    await settle(s.fixture);
  });
});
