import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { ShortcutsDialog, type ShortcutEntry } from './shortcuts-dialog';

const hotkeys: ShortcutEntry[] = [
  { keys: 'c', description: 'Crear incidencia', group: 'General' },
  { keys: 'g b', description: 'Ir al tablero del proyecto', group: 'Navegación' },
  { keys: 'n', description: 'Notas rápidas', group: 'General', comingSoon: true },
];

function create(options: { open: boolean; hotkeys?: readonly ShortcutEntry[] }) {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(ShortcutsDialog);
  fixture.componentRef.setInput('open', options.open);
  if (options.hotkeys) {
    fixture.componentRef.setInput('hotkeys', options.hotkeys);
  }
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('ShortcutsDialog', () => {
  it('lists registered shortcuts grouped with their keys', () => {
    const { el } = create({ open: true, hotkeys });
    const dialog = el.querySelector<HTMLElement>('[role="dialog"]');
    expect(dialog?.getAttribute('aria-labelledby')).toBeTruthy();
    expect(dialog?.textContent).toContain('Atajos de teclado');
    const headings = Array.from(el.querySelectorAll('h3')).map((heading) =>
      heading.textContent?.trim(),
    );
    expect(headings).toEqual(['General', 'Navegación']);
    expect(el.textContent).toContain('Crear incidencia');
    expect(el.textContent).toContain('luego');
    const soon = Array.from(el.querySelectorAll('li')).find((li) =>
      li.textContent?.includes('Notas rápidas'),
    );
    expect(soon?.textContent).toContain('Próximamente');
  });

  it('shows an empty message when nothing is registered', () => {
    const { el } = create({ open: true });
    expect(el.textContent).toContain('No hay atajos disponibles en esta pantalla.');
  });

  it('calls onClose from the dialog close button', () => {
    const { fixture, el } = create({ open: true, hotkeys });
    el.querySelector<HTMLButtonElement>('button[aria-label="Close"]')?.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.open()).toBe(false);
  });

  it('stays closed when open is false', () => {
    const { el } = create({ open: false });
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });
});
