import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { THEME_STORAGE_KEY } from '@corelog/core';

import { AUTH_LOGO } from './auth-brand';
import type { AuthHeroVariant } from './auth-hero';
import { AuthLayout } from './auth-layout';

@Component({
  imports: [AuthLayout],
  template: `<app-auth-layout [variant]="variant()" title="Iniciar sesión"
    ><p>Formulario</p></app-auth-layout
  >`,
})
class Host {
  readonly variant = input<AuthHeroVariant>('login');
}

function stubPhone(matches: boolean): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: matches && query.includes('max-width: 720px'),
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }),
  });
}

function render(variant: AuthHeroVariant = 'login'): HTMLElement {
  TestBed.configureTestingModule({ providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(Host);
  fixture.componentRef.setInput('variant', variant);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('AuthLayout', () => {
  beforeEach(() => localStorage.clear());

  afterEach(() => {
    TestBed.resetTestingModule();
    localStorage.clear();
    Reflect.deleteProperty(window, 'matchMedia');
  });

  it('shows the login carousel with four photographs and a two-line headline with one accent word', () => {
    const el = render('login');
    expect(el.querySelectorAll('.hero-carousel__image')).toHaveLength(4);
    expect(el.querySelectorAll('.auth-layout__headline-line')).toHaveLength(2);
    expect(el.querySelector('.auth-layout__accent')?.textContent).toBe('claro');
  });

  it('uses the matching single photograph for register and invite, without carousel controls', () => {
    for (const [variant, file] of [
      ['register', 'auth-register-hero'],
      ['invite', 'auth-invite-hero'],
    ] as const) {
      TestBed.resetTestingModule();
      const el = render(variant);
      const images = el.querySelectorAll('.hero-carousel__image');
      expect(images).toHaveLength(1);
      expect(images[0].getAttribute('src')).toContain(file);
      expect(el.querySelector('.hero-carousel__controls')).toBeNull();
    }
  });

  it('renders the logo from the single brand constant on a light pill', () => {
    const el = render();
    const logo = el.querySelector('.auth-layout__brand img');
    expect(logo?.getAttribute('src')).toBe(AUTH_LOGO.src);
    expect(logo?.getAttribute('alt')).toBe('CoreLog');
    expect(el.querySelector('.auth-layout__logo-plate')).not.toBeNull();
  });

  it('keeps the form in a labelled section next to the hero', () => {
    const el = render();
    const section = el.querySelector('section.auth-layout__panel');
    const heading = section?.querySelector('h1');
    expect(section?.getAttribute('aria-labelledby')).toBe(heading?.id);
    expect(section?.textContent).toContain('Formulario');
  });

  it('puts the same photographs, decorative and blurred by css, behind the container', () => {
    const el = render('login');
    const backdrop = el.querySelector('.auth-layout__backdrop');
    expect(backdrop?.getAttribute('aria-hidden')).toBe('true');
    const images = Array.from(backdrop?.querySelectorAll('img') ?? []);
    const carousel = Array.from(el.querySelectorAll('.hero-carousel__image')).map((i) =>
      i.getAttribute('src'),
    );
    expect(images.map((i) => i.getAttribute('src'))).toEqual(carousel);
    expect(images.every((i) => i.getAttribute('alt') === '')).toBe(true);
    expect(images[0].getAttribute('loading')).toBe('eager');
    expect(images[1].getAttribute('loading')).toBe('lazy');
  });

  it('cross-fades the backdrop to the slide the carousel shows', () => {
    const el = render('login');
    const active = (): number =>
      Array.from(el.querySelectorAll('.auth-layout__backdrop-image')).findIndex((i) =>
        i.classList.contains('auth-layout__backdrop-image--active'),
      );
    expect(active()).toBe(0);
    el.querySelector<HTMLElement>('.hero-carousel__button[aria-label="Imagen siguiente"]')?.click();
    TestBed.tick();
    expect(active()).toBe(1);
  });

  it('is dark when the visitor has no stored preference', () => {
    const el = render();
    expect(el.querySelector('app-auth-layout')?.getAttribute('data-auth-theme')).toBe('dark');
  });

  it('is dark when dark is stored and light when light is stored', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark');
    expect(render().querySelector('app-auth-layout')?.getAttribute('data-auth-theme')).toBe('dark');
    TestBed.resetTestingModule();
    localStorage.setItem(THEME_STORAGE_KEY, 'light');
    expect(render().querySelector('app-auth-layout')?.getAttribute('data-auth-theme')).toBe(
      'light',
    );
  });

  it('mounts the floating theme button and switches the variant with it', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark');
    const el = render();
    const button = el.querySelector<HTMLButtonElement>('[data-testid="theme-toggle"]');
    expect(button?.getAttribute('aria-label')).toBe('Cambiar a tema claro');
    button?.click();
    TestBed.tick();
    expect(el.querySelector('app-auth-layout')?.getAttribute('data-auth-theme')).toBe('light');
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
  });

  it('drops the hero on phones and shows the full-bleed background instead', () => {
    stubPhone(true);
    const el = render();
    expect(el.querySelector('.auth-layout__hero')).toBeNull();
    expect(el.querySelector('app-hero-carousel')).toBeNull();
    const background = el.querySelector('.auth-layout__background');
    expect(background?.getAttribute('src')).toBe('/images/auth/auth-mobile-bg.webp');
    expect(background?.getAttribute('alt')).toBe('');
  });

  it('does not load the phone background on larger screens', () => {
    const el = render();
    expect(el.querySelector('.auth-layout__background')).toBeNull();
  });
});
