import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { HelpMenu } from './help-menu';

function open(inputs: Record<string, string | null> = {}): {
  fixture: ComponentFixture<HelpMenu>;
  menu: HTMLElement;
} {
  const fixture = TestBed.createComponent(HelpMenu);
  for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
  fixture.detectChanges();
  (fixture.nativeElement.querySelector('button[aria-label="Ayuda"]') as HTMLButtonElement).click();
  fixture.detectChanges();
  return { fixture, menu: fixture.nativeElement.querySelector('[role="menu"]') as HTMLElement };
}

describe('HelpMenu', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('lists only the keyboard shortcuts when no address is configured', () => {
    const { menu } = open();
    const labels = Array.from(menu.querySelectorAll('[role="menuitem"]')).map((row) =>
      (row.textContent ?? '').replace(/\s+/g, ' ').trim(),
    );
    expect(labels).toEqual(['Atajos de teclado?']);
  });

  it('adds documentation, news and support as external links when they are configured', () => {
    const { menu } = open({
      docsUrl: 'https://docs.example.com',
      whatsNewUrl: 'https://example.com/novedades',
      supportUrl: 'mailto:soporte@example.com',
    });
    const links = Array.from(menu.querySelectorAll<HTMLAnchorElement>('a[role="menuitem"]'));
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      'https://docs.example.com',
      'https://example.com/novedades',
      'mailto:soporte@example.com',
    ]);
    expect(links.every((link) => link.target === '_blank')).toBe(true);
    expect(menu.textContent).toContain('Documentación');
    expect(menu.textContent).toContain('Novedades');
    expect(menu.textContent).toContain('Soporte');
  });

  it('opens the shortcuts from the first row', () => {
    const fixture = TestBed.createComponent(HelpMenu);
    const opened = vi.fn();
    fixture.componentInstance.openShortcuts.subscribe(opened);
    fixture.detectChanges();
    (
      fixture.nativeElement.querySelector('button[aria-label="Ayuda"]') as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    (fixture.nativeElement.querySelector('[role="menuitem"]') as HTMLElement).click();
    expect(opened).toHaveBeenCalledTimes(1);
  });
});
