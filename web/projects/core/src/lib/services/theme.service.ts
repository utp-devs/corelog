import { DOCUMENT, Injectable, computed, inject, signal } from '@angular/core';

import { StorageService } from './storage.service';

export type ThemeMode = 'light' | 'dark';

/** Storage key of the theme choice, stored as a plain string. */
export const THEME_STORAGE_KEY = 'corelog.theme';

function parseMode(value: string | null): ThemeMode {
  return value === 'dark' ? 'dark' : 'light';
}

/** Light or dark theme. Applies `data-theme` on the html element, which drives the --cl-* tokens. */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly document = inject(DOCUMENT);
  private readonly storage = inject(StorageService);

  private readonly modeState = signal<ThemeMode>(
    parseMode(this.storage.getItem(THEME_STORAGE_KEY)),
  );

  readonly mode = this.modeState.asReadonly();
  readonly isDark = computed(() => this.modeState() === 'dark');

  constructor() {
    this.apply(this.modeState());
  }

  setMode(mode: ThemeMode): void {
    this.modeState.set(mode);
    this.storage.setItem(THEME_STORAGE_KEY, mode);
    this.apply(mode);
  }

  toggle(): void {
    this.setMode(this.isDark() ? 'light' : 'dark');
  }

  private apply(mode: ThemeMode): void {
    const root = this.document.documentElement;
    root.dataset['theme'] = mode;
    root.style.colorScheme = mode;
  }
}
