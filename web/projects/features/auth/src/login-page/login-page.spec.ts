import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { ApiError, AuthStore, HOME_URL, LOCATION_ASSIGN, LoginPayload } from '@corelog/core';
import { vi } from 'vitest';

import { LoginPage } from './login-page';

function apiError(patch: Partial<ApiError> = {}): ApiError {
  return {
    status: 401,
    code: 'auth.invalid_credentials',
    message: 'invalid email or password',
    details: [],
    ...patch,
  };
}

interface Setup {
  fixture: ComponentFixture<LoginPage>;
  el: HTMLElement;
  login: ReturnType<typeof vi.fn<(payload: LoginPayload) => Promise<void>>>;
  assign: ReturnType<typeof vi.fn<(url: string) => void>>;
  router: Router;
}

function setup(query: Record<string, string> = {}): Setup {
  const login = vi.fn<(payload: LoginPayload) => Promise<void>>().mockResolvedValue(undefined);
  const assign = vi.fn<(url: string) => void>();
  TestBed.configureTestingModule({
    imports: [LoginPage],
    providers: [
      provideRouter([]),
      {
        provide: ActivatedRoute,
        useValue: { snapshot: { queryParamMap: convertToParamMap(query) } },
      },
      { provide: AuthStore, useValue: { login, twoFactorChallenge: signal<string | null>(null) } },
      { provide: LOCATION_ASSIGN, useValue: assign },
      { provide: HOME_URL, useValue: '/workspace/' },
    ],
  });
  const router = TestBed.inject(Router);
  vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
  const fixture = TestBed.createComponent(LoginPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, login, assign, router };
}

/** The control a visible label points to. */
function controlOf(el: HTMLElement, label: string): HTMLInputElement | null {
  const found = Array.from(el.querySelectorAll('label')).find((l) =>
    l.textContent?.trim().startsWith(label),
  );
  return found ? el.querySelector<HTMLInputElement>(`#${found.htmlFor}`) : null;
}

function fill(el: HTMLElement, email: string, password: string): void {
  const emailInput = controlOf(el, 'Correo electrónico');
  const passwordInput = controlOf(el, 'Contraseña');
  if (!emailInput || !passwordInput) throw new Error('login fields not rendered');
  emailInput.value = email;
  emailInput.dispatchEvent(new Event('input'));
  passwordInput.value = password;
  passwordInput.dispatchEvent(new Event('input'));
}

function submit(el: HTMLElement): void {
  el.querySelector<HTMLButtonElement>('button[type="submit"]')?.click();
}

async function settle(fixture: ComponentFixture<LoginPage>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('LoginPage', () => {
  it('shows the Spanish validation messages when the form is submitted empty', async () => {
    const { fixture, el, login } = setup();
    submit(el);
    await settle(fixture);
    expect(el.textContent).toContain('Introduce un correo electrónico válido');
    expect(el.textContent).toContain('La contraseña es obligatoria');
    expect(login).not.toHaveBeenCalled();
  });

  it('submits the credentials when they are valid', async () => {
    const { fixture, el, login } = setup();
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    expect(login).toHaveBeenCalledWith({ email: 'ana@corelog.test', password: 'Secreta123' });
  });

  it('offers a link to create an account', () => {
    const { el } = setup();
    const link = Array.from(el.querySelectorAll('a')).find(
      (a) => a.textContent?.trim() === 'Crear una',
    );
    expect(link?.getAttribute('href')).toBe('/register');
  });
});

describe('LoginPage server field errors', () => {
  it('shows the server detail under the field it refers to', async () => {
    const { fixture, el, login } = setup();
    login.mockRejectedValue(
      apiError({
        status: 422,
        code: 'validation.failed',
        message: 'invalid request body',
        details: [{ field: 'email', message: 'debe ser un correo válido' }],
      }),
    );
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    const email = controlOf(el, 'Correo electrónico');
    const described = document.getElementById(email?.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toContain('debe ser un correo válido');
  });

  it('shows the Spanish message in an alert when there are no field details', async () => {
    const { fixture, el, login } = setup();
    login.mockRejectedValue(apiError());
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('no son correctos');
  });
});

describe('LoginPage landmarks and labels', () => {
  it('exposes a skip link to the main landmark and a level-1 heading, with no brand aside', () => {
    const { el } = setup();
    const skip = Array.from(el.querySelectorAll('a')).find(
      (a) => a.textContent?.trim() === 'Saltar al formulario',
    );
    expect(skip?.getAttribute('href')).toBe('#auth-main');
    expect(el.querySelector('main#auth-main')).not.toBeNull();
    expect(el.querySelector('h1')?.textContent?.trim()).toBe('Iniciar sesión');
    expect(el.querySelector('aside')).toBeNull();
  });

  it('marks the page that holds the brand and form panels as the first child of main', () => {
    const { el } = setup();
    const page = el.querySelector('main#auth-main')?.firstElementChild;
    expect(page?.getAttribute('data-page')).toBe('auth');
    expect(page?.querySelector('.auth-layout__brand')).not.toBeNull();
    expect(page?.querySelector('.auth-layout__panel')).not.toBeNull();
  });

  it('renders the brand banner in the brand panel beside the form panel', () => {
    const { el } = setup();
    const brand = el.querySelector('main#auth-main .auth-layout__brand');
    const form = el.querySelector('main#auth-main section.auth-layout__panel');
    expect(brand?.querySelector('img[alt="CoreLog"]')).not.toBeNull();
    expect(form?.querySelector('form')).not.toBeNull();
  });

  it('uses the access layout with the carousel hero beside the form', () => {
    const { el } = setup();
    expect(el.querySelector('app-hero-carousel')).not.toBeNull();
    expect(el.querySelector('.hero-carousel__dot')).not.toBeNull();
  });

  it('associates each input with its visible label and marks it required', () => {
    const { el } = setup();
    const email = controlOf(el, 'Correo electrónico');
    expect(email?.getAttribute('aria-required')).toBe('true');
    expect(email?.getAttribute('autocomplete')).toBe('email');
    expect(email?.getAttribute('type')).toBe('email');
  });
});

describe('LoginPage form behaviour', () => {
  it('disables the submit button while the request is pending', async () => {
    const { fixture, el, login } = setup();
    let release: () => void = () => undefined;
    login.mockReturnValue(new Promise<void>((resolve) => (release = resolve)));
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    fixture.detectChanges();
    expect(el.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true);
    release();
    await settle(fixture);
  });

  it('does not offer a password recovery link or a remember-me option', () => {
    const { el } = setup();
    expect(el.textContent).not.toMatch(/olvid|recordarme/i);
  });

  it('moves focus to the first invalid field after an empty submit', async () => {
    const { fixture, el } = setup();
    submit(el);
    await settle(fixture);
    await fixture.whenStable();
    expect(document.activeElement).toBe(controlOf(el, 'Correo electrónico'));
  });

  it('uses current-password autocomplete on the password field', () => {
    const { el } = setup();
    const password = controlOf(el, 'Contraseña');
    expect(password?.getAttribute('autocomplete')).toBe('current-password');
    expect(password?.type).toBe('password');
  });

  it('toggles password visibility from a keyboard-reachable button', async () => {
    const { fixture, el } = setup();
    const toggle = el.querySelector<HTMLButtonElement>('button[aria-label="Mostrar contraseña"]');
    const password = controlOf(el, 'Contraseña');
    expect(toggle?.getAttribute('aria-pressed')).toBe('false');
    toggle?.click();
    fixture.detectChanges();
    expect(password?.type).toBe('text');
    expect(toggle?.getAttribute('aria-pressed')).toBe('true');
    toggle?.click();
    fixture.detectChanges();
    expect(password?.type).toBe('password');
  });

  it('never shows the raw English API message', async () => {
    const { fixture, el, login } = setup();
    login.mockRejectedValue(apiError());
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    const text = el.querySelector('[role="alert"]')?.textContent ?? '';
    expect(text).toContain('no son correctos');
    expect(text).not.toContain('invalid');
  });

  it('maps server details to the email field and marks it invalid', async () => {
    const { fixture, el, login } = setup();
    login.mockRejectedValue(
      apiError({
        status: 422,
        code: 'validation.failed',
        message: 'invalid request body',
        details: [{ field: 'email', message: 'debe ser un correo válido' }],
      }),
    );
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    const email = controlOf(el, 'Correo electrónico');
    expect(email?.getAttribute('aria-invalid')).toBe('true');
    const described = document.getElementById(email?.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toContain('debe ser un correo válido');
  });

  it('keeps the return target when linking to registration and redirects to it after sign-in', async () => {
    const { fixture, el, login, assign, router } = setup({ returnUrl: '/workspace/inicio' });
    const link = Array.from(el.querySelectorAll('a')).find(
      (a) => a.textContent?.trim() === 'Crear una',
    );
    expect(link?.getAttribute('href')).toContain('returnUrl=%2Fworkspace%2Finicio');
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    expect(login).toHaveBeenCalled();
    expect(assign).toHaveBeenCalledWith('/workspace/inicio');
    expect(router.navigateByUrl).not.toHaveBeenCalled();
  });

  it('ignores an external return target and lands on the default page', async () => {
    const { fixture, el, assign } = setup({ returnUrl: 'https://evil.example' });
    fill(el, 'ana@corelog.test', 'Secreta123');
    submit(el);
    await settle(fixture);
    expect(assign).toHaveBeenCalledWith('/workspace/');
  });
});
