import { TestBed } from '@angular/core/testing';

import { THEME_STORAGE_KEY, ThemeService } from './theme.service';

describe('ThemeService', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    document.documentElement.style.colorScheme = '';
  });

  it('starts in light mode when nothing is stored', () => {
    const theme = TestBed.inject(ThemeService);
    expect(theme.mode()).toBe('light');
    expect(theme.isDark()).toBe(false);
    expect(document.documentElement.dataset['theme']).toBe('light');
  });

  it('restores the dark theme from storage', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark');
    const theme = TestBed.inject(ThemeService);
    expect(theme.isDark()).toBe(true);
    expect(document.documentElement.dataset['theme']).toBe('dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });

  it('persists the mode as a plain string and toggles it', () => {
    const theme = TestBed.inject(ThemeService);
    theme.toggle();
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.dataset['theme']).toBe('dark');
    theme.toggle();
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
    expect(document.documentElement.dataset['theme']).toBe('light');
  });

  it('ignores invalid stored values and falls back to light', () => {
    localStorage.setItem(THEME_STORAGE_KEY, '{"mode":"auto"}');
    expect(TestBed.inject(ThemeService).mode()).toBe('light');
  });

  describe('system mode', () => {
    let listeners: Array<(event: { matches: boolean }) => void>;
    let prefersDark: boolean;
    const original = window.matchMedia;

    beforeEach(() => {
      listeners = [];
      prefersDark = true;
      window.matchMedia = ((query: string) => ({
        matches: query.includes('prefers-color-scheme: dark') ? prefersDark : false,
        addEventListener: (_: string, listener: (event: { matches: boolean }) => void) =>
          listeners.push(listener),
        removeEventListener: () => undefined,
      })) as unknown as typeof window.matchMedia;
    });

    afterEach(() => {
      window.matchMedia = original;
    });

    it('applies the operating system preference and keeps the mode as system', () => {
      localStorage.setItem(THEME_STORAGE_KEY, 'system');
      const theme = TestBed.inject(ThemeService);
      expect(theme.mode()).toBe('system');
      expect(theme.resolved()).toBe('dark');
      expect(theme.isDark()).toBe(true);
      expect(document.documentElement.dataset['theme']).toBe('dark');
    });

    it('follows a change of the operating system preference', () => {
      const theme = TestBed.inject(ThemeService);
      theme.setMode('system');
      expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('system');
      listeners.forEach((listener) => listener({ matches: false }));
      expect(theme.resolved()).toBe('light');
      expect(document.documentElement.dataset['theme']).toBe('light');
    });

    it('toggle leaves the system mode with the opposite of the applied theme', () => {
      const theme = TestBed.inject(ThemeService);
      theme.setMode('system');
      theme.toggle();
      expect(theme.mode()).toBe('light');
    });
  });
});
