import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import { ThemeService } from '@corelog/core';
import { HeroCarousel } from '@corelog/ui/hero-carousel';
import { ThemeToggleFab } from '@corelog/ui/theme-toggle-fab';

import { AUTH_LOGO } from './auth-brand';
import { AUTH_HERO, type AuthHeroVariant } from './auth-hero';

let nextLayoutId = 0;

/** Id of the main landmark; the skip link points here. */
export const AUTH_MAIN_ID = 'auth-main';

/** Phones: no hero photograph, a full-bleed background behind the form. */
const PHONE_QUERY = '(max-width: 720px)';

/** Background of the phone layout. */
export const AUTH_MOBILE_BACKGROUND = '/images/auth/auth-mobile-bg.webp';

/**
 * Shared template for the access screens: dark by default and light when the visitor chose it (a floating button
 * switches the app theme), a large rounded container with a photo hero
 * (carousel, headline and subtitle) beside the form. On tablets the hero becomes a strip above the form; on
 * phones the hero is dropped and a background image goes behind the form instead. The optional switch link
 * ("already have an account?") is rendered under the form.
 */
@Component({
  selector: 'app-auth-layout',
  imports: [RouterLink, HeroCarousel, ThemeToggleFab],
  host: { '[attr.data-auth-theme]': 'authTheme()' },
  templateUrl: './auth-layout.html',
  styleUrl: './auth-layout.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthLayout {
  readonly title = input.required<string>();
  readonly subtitle = input<string | null>(null);
  /** Photographs and copy of the hero. */
  readonly variant = input<AuthHeroVariant>('login');
  /** Text before the switch link, for example "¿No tienes cuenta?". */
  readonly switchText = input<string | null>(null);
  readonly switchLabel = input<string | null>(null);
  readonly switchPath = input<string | null>(null);
  readonly switchQuery = input<Readonly<Record<string, string>>>({});

  protected readonly mainId = AUTH_MAIN_ID;
  protected readonly titleId = `auth-title-${nextLayoutId++}`;
  protected readonly logo = AUTH_LOGO;
  protected readonly mobileBackground = AUTH_MOBILE_BACKGROUND;
  protected readonly hero = computed(() => AUTH_HERO[this.variant()]);
  private readonly theme = inject(ThemeService);
  /** Dark unless the visitor chose the light theme (a stored preference wins; none means dark). */
  protected readonly authTheme = computed(() =>
    this.theme.hasPreference() && this.theme.resolved() === 'light' ? 'light' : 'dark',
  );
  protected readonly phone = signal(false);
  /** Slide shown by the carousel; the blurred backdrop shows the same photograph. */
  protected readonly activeSlide = signal(0);

  constructor() {
    const query = inject(DOCUMENT).defaultView?.matchMedia?.(PHONE_QUERY);
    if (query) {
      this.phone.set(query.matches);
      const listener = (event: MediaQueryListEvent): void => this.phone.set(event.matches);
      query.addEventListener?.('change', listener);
      inject(DestroyRef).onDestroy(() => query.removeEventListener?.('change', listener));
    }
  }
}
