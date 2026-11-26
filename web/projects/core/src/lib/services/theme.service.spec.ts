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

  it('restores the dark theme stored by the React app', () => {
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
});
