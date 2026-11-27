import { DOCUMENT, Injectable, computed, inject, signal } from '@angular/core';

import { StorageService } from './storage.service';

/** The theme the user chose. `system` follows the operating system preference. */
export type ThemeMode = 'light' | 'dark' | 'system';

/** The theme that is applied to the page. */
export type ResolvedTheme = 'light' | 'dark';

/** Storage key of the theme choice, stored as a plain string. */
export const THEME_STORAGE_KEY = 'corelog.theme';

const DARK_SCHEME_QUERY = '(prefers-color-scheme: dark)';

function parseMode(value: string | null): ThemeMode {
  return value === 'dark' || value === 'system' ? value : 'light';
}

/**
 * Light, dark or system theme. Applies `data-theme` on the html element, which drives the --cl-*
 * tokens. In `system` mode the applied theme follows `prefers-color-scheme` and updates when the
 * operating system changes it.
 */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly document = inject(DOCUMENT);
  private readonly storage = inject(StorageService);
  private readonly systemQuery = this.document.defaultView?.matchMedia?.(DARK_SCHEME_QUERY) ?? null;

  private readonly modeState = signal<ThemeMode>(
    parseMode(this.storage.getItem(THEME_STORAGE_KEY)),
  );
  private readonly preferenceState = signal(this.storage.getItem(THEME_STORAGE_KEY) !== null);
  private readonly systemDark = signal(this.systemQuery?.matches ?? false);

  /** True once the person (or an earlier visit) chose a theme; screens may pick their own default before that. */
  readonly hasPreference = this.preferenceState.asReadonly();
  /** The chosen mode: light, dark or system. */
  readonly mode = this.modeState.asReadonly();
  /** The theme in effect. Equals the mode unless the mode is `system`. */
  readonly resolved = computed<ResolvedTheme>(() => {
    const mode = this.modeState();
    if (mode === 'system') return this.systemDark() ? 'dark' : 'light';
    return mode;
  });
  readonly isDark = computed(() => this.resolved() === 'dark');

  constructor() {
    this.systemQuery?.addEventListener?.('change', (event) => {
      this.systemDark.set(event.matches);
      this.apply();
    });
    this.apply();
  }

  setMode(mode: ThemeMode): void {
    this.modeState.set(mode);
    this.preferenceState.set(true);
    this.storage.setItem(THEME_STORAGE_KEY, mode);
    this.apply();
  }

  /** Switches between light and dark, leaving the system mode. */
  toggle(): void {
    this.setMode(this.isDark() ? 'light' : 'dark');
  }

  private apply(): void {
    const root = this.document.documentElement;
    const theme = this.resolved();
    root.dataset['theme'] = theme;
    root.style.colorScheme = theme;
  }
}
