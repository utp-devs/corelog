import { TestBed } from '@angular/core/testing';
import { THEME_STORAGE_KEY, ThemeService } from '@corelog/core';

import { ThemeToggleFab } from './theme-toggle-fab';

function setup(stored?: string) {
  localStorage.clear();
  if (stored) localStorage.setItem(THEME_STORAGE_KEY, stored);
  const fixture = TestBed.createComponent(ThemeToggleFab);
  fixture.detectChanges();
  const button = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
  return { fixture, button, theme: TestBed.inject(ThemeService) };
}

describe('ThemeToggleFab', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
    localStorage.clear();
  });

  it('names the action and carries the test id', () => {
    const { button } = setup('light');
    expect(button.getAttribute('data-testid')).toBe('theme-toggle');
    expect(button.getAttribute('aria-label')).toBe('Cambiar a tema oscuro');
    expect(button.getAttribute('title')).toBe('Cambiar a tema oscuro');
    expect(button.querySelector('.pi-moon')).not.toBeNull();
  });

  it('switches the shared theme, persists it and flips label and icon', () => {
    const { fixture, button, theme } = setup('light');
    button.click();
    fixture.detectChanges();
    expect(theme.resolved()).toBe('dark');
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.dataset['theme']).toBe('dark');
    expect(button.getAttribute('aria-label')).toBe('Cambiar a tema claro');
    expect(button.querySelector('.pi-sun')).not.toBeNull();
    button.click();
    fixture.detectChanges();
    expect(theme.resolved()).toBe('light');
    expect(button.getAttribute('aria-label')).toBe('Cambiar a tema oscuro');
  });

  it('leaves the system mode when toggled', () => {
    const { button, theme } = setup('system');
    button.click();
    expect(theme.mode()).not.toBe('system');
  });
});
