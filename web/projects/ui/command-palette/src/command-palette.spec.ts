import { TestBed, type ComponentFixture } from '@angular/core/testing';
import type { SearchHit } from '@corelog/core';
import { RECENT_STORAGE_KEY } from '@corelog/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CommandPalette, type PaletteSearch } from './command-palette';
import { CommandRegistry } from './command-registry';

const PALETTE_DEBOUNCE_MS = 150;

const hits: SearchHit[] = [
  { kind: 'project', id: 'p-1', title: 'Proyecto Core', subtitle: 'CORE', link: '/projects/p-1' },
  { kind: 'issue', id: 'i-1', title: 'Exportar datos', subtitle: 'CORE-12', link: '/browse/i-1' },
];

interface SetupOptions {
  search?: PaletteSearch;
  seed?: string;
}

function setup(options: SetupOptions = {}) {
  const search = options.search ?? vi.fn<PaletteSearch>().mockResolvedValue(hits);
  const fixture = TestBed.createComponent(CommandPalette);
  fixture.componentRef.setInput('search', search);
  if (options.seed !== undefined) {
    fixture.componentRef.setInput('seed', options.seed);
  }
  fixture.componentRef.setInput('open', true);
  fixture.detectChanges();
  const openHit = vi.fn();
  fixture.componentInstance.openHit.subscribe(openHit);
  const el = fixture.nativeElement as HTMLElement;
  const input = el.querySelector<HTMLInputElement>('input[role="combobox"]');
  if (!input) {
    throw new Error('combobox not rendered');
  }
  return { fixture, el, input, search, openHit };
}

function dialogOf(el: HTMLElement): HTMLElement {
  const dialog = el.querySelector<HTMLElement>('[role="dialog"]');
  if (!dialog) {
    throw new Error('dialog not rendered');
  }
  return dialog;
}

function optionsOf(el: HTMLElement): HTMLElement[] {
  return Array.from(el.querySelectorAll<HTMLElement>('[role="option"]'));
}

function findOption(el: HTMLElement, name: RegExp): HTMLElement {
  const option = optionsOf(el).find((item) => name.test(item.textContent ?? ''));
  if (!option) {
    throw new Error(`option ${String(name)} not found`);
  }
  return option;
}

function keyDown(
  fixture: ComponentFixture<CommandPalette>,
  target: HTMLElement,
  key: string,
  shiftKey = false,
) {
  target.dispatchEvent(
    new KeyboardEvent('keydown', { key, shiftKey, bubbles: true, cancelable: true }),
  );
  fixture.detectChanges();
}

function type(fixture: ComponentFixture<CommandPalette>, input: HTMLInputElement, value: string) {
  input.value = value;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

/** Lets the debounce timer fire and the pending promises settle, then renders. */
async function settle(fixture: ComponentFixture<CommandPalette>, ms = 0) {
  await vi.advanceTimersByTimeAsync(ms);
  fixture.detectChanges();
}

describe('CommandPalette', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.localStorage.clear();
    TestBed.configureTestingModule({});
    const registry = TestBed.inject(CommandRegistry);
    registry.register({
      id: 'nav-issues',
      title: 'Ir a Incidencias',
      group: 'Navegación',
      run: vi.fn(),
    });
    registry.register({
      id: 'act-theme',
      title: 'Cambiar tema',
      keywords: ['oscuro'],
      group: 'Acciones',
      run: vi.fn(),
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('renders nothing while closed', () => {
    const fixture = TestBed.createComponent(CommandPalette);
    fixture.componentRef.setInput('search', vi.fn());
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="dialog"]')).toBeNull();
  });

  it('lists commands grouped by section and marks the first option active', () => {
    const { el } = setup();
    const listbox = el.querySelector<HTMLElement>('[role="listbox"]');
    expect(listbox).not.toBeNull();
    const acciones = el.querySelector<HTMLElement>('[role="group"][aria-label="Acciones"]');
    const navegacion = el.querySelector<HTMLElement>('[role="group"][aria-label="Navegación"]');
    expect(acciones?.textContent).toContain('Cambiar tema');
    expect(navegacion?.textContent).toContain('Ir a Incidencias');
    const options = optionsOf(el);
    expect(options[0].getAttribute('aria-selected')).toBe('true');
  });

  it('moves the active option with Up and Down and points aria-activedescendant at it', () => {
    const { fixture, el, input } = setup();
    const options = optionsOf(el);
    keyDown(fixture, dialogOf(el), 'ArrowDown');
    expect(options[1].getAttribute('aria-selected')).toBe('true');
    expect(input.getAttribute('aria-activedescendant')).toBe(options[1].id);
    keyDown(fixture, dialogOf(el), 'ArrowUp');
    keyDown(fixture, dialogOf(el), 'ArrowUp');
    expect(options[1].getAttribute('aria-selected')).toBe('true');
  });

  it('runs the active command on Enter and closes first', () => {
    const registry = TestBed.inject(CommandRegistry);
    registry.clear();
    const run = vi.fn();
    registry.register({ id: 'only', title: 'Solo comando', group: 'Acciones', run });
    const { fixture, el } = setup();
    keyDown(fixture, dialogOf(el), 'Enter');
    expect(fixture.componentInstance.open()).toBe(false);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it('closes on Escape', () => {
    const { fixture, el } = setup();
    keyDown(fixture, dialogOf(el), 'Escape');
    expect(fixture.componentInstance.open()).toBe(false);
  });

  it('filters commands by title and keywords', () => {
    const { fixture, el, input } = setup();
    type(fixture, input, 'oscuro');
    expect(findOption(el, /Cambiar tema/)).toBeTruthy();
    expect(optionsOf(el).some((item) => /Ir a Incidencias/.test(item.textContent ?? ''))).toBe(
      false,
    );
  });

  it('shows the hint for a single character and does not search', async () => {
    const { fixture, el, input, search } = setup();
    type(fixture, input, 'e');
    await settle(fixture, PALETTE_DEBOUNCE_MS * 2);
    expect(search).not.toHaveBeenCalled();
    expect(el.textContent).toContain('Escribe al menos 2 caracteres');
  });

  it('debounces the remote search by 150 ms and shows grouped hits', async () => {
    const { fixture, el, input, search } = setup();
    type(fixture, input, 'ex');
    type(fixture, input, 'exp');
    type(fixture, input, 'expo');
    expect(search).not.toHaveBeenCalled();
    await settle(fixture, PALETTE_DEBOUNCE_MS - 1);
    expect(search).not.toHaveBeenCalled();
    await settle(fixture, 1);
    expect(search).toHaveBeenCalledTimes(1);
    expect(search).toHaveBeenCalledWith('expo');
    await settle(fixture);
    expect(el.querySelector('[role="group"][aria-label="Proyectos"]')?.textContent).toContain(
      'Proyecto Core',
    );
    expect(el.querySelector('[role="group"][aria-label="Incidencias"]')?.textContent).toContain(
      'Exportar datos',
    );
  });

  it('shows the loading state while the search is pending', async () => {
    const { fixture, el, input } = setup({
      search: vi.fn(() => new Promise<SearchHit[]>(() => undefined)),
    });
    type(fixture, input, 'ex');
    await settle(fixture, PALETTE_DEBOUNCE_MS);
    expect(el.querySelector('[role="status"]')?.textContent).toContain('Buscando…');
  });

  it('shows the empty state when nothing matches', async () => {
    const { fixture, el, input } = setup({ search: vi.fn<PaletteSearch>().mockResolvedValue([]) });
    type(fixture, input, 'zzz');
    await settle(fixture, PALETTE_DEBOUNCE_MS);
    expect(el.textContent).toContain('Sin resultados');
  });

  it('shows the error state with a retry that searches again', async () => {
    const search = vi
      .fn<PaletteSearch>()
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(hits);
    const { fixture, el, input } = setup({ search });
    type(fixture, input, 'ex');
    await settle(fixture, PALETTE_DEBOUNCE_MS);
    expect(el.textContent).toContain('No se pudo buscar');
    const retry = Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Reintentar',
    );
    retry?.click();
    fixture.detectChanges();
    await settle(fixture, PALETTE_DEBOUNCE_MS);
    expect(el.textContent).toContain('Exportar datos');
    expect(search).toHaveBeenCalledTimes(2);
  });

  it('lists recent items from localStorage and opens them with Enter', () => {
    window.localStorage.setItem(
      RECENT_STORAGE_KEY,
      JSON.stringify([
        { kind: 'issue', id: 'r-1', title: 'Reciente X', subtitle: 'CORE-3', link: '/browse/r-1' },
      ]),
    );
    const { fixture, el, openHit } = setup();
    expect(el.querySelector('[role="group"][aria-label="Recientes"]')?.textContent).toContain(
      'Reciente X',
    );
    const recentOption = findOption(el, /Reciente X/);
    recentOption.dispatchEvent(new MouseEvent('mouseenter'));
    keyDown(fixture, dialogOf(el), 'Enter');
    expect(openHit).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'r-1', link: '/browse/r-1' }),
    );
  });

  it('moves focus into the input on open and restores the previous element on close', async () => {
    const trigger = document.createElement('button');
    trigger.textContent = 'Abrir';
    document.body.appendChild(trigger);
    trigger.focus();

    const fixture = TestBed.createComponent(CommandPalette);
    fixture.componentRef.setInput('search', vi.fn<PaletteSearch>().mockResolvedValue([]));
    fixture.componentRef.setInput('open', true);
    fixture.detectChanges();
    await settle(fixture);
    const input = fixture.nativeElement.querySelector('input[role="combobox"]') as HTMLInputElement;
    expect(document.activeElement).toBe(input);

    fixture.componentRef.setInput('open', false);
    fixture.detectChanges();
    expect(document.activeElement).toBe(trigger);
    trigger.remove();
  });

  it('keeps Tab inside the dialog', async () => {
    const { fixture, el, input } = setup({
      search: vi.fn<PaletteSearch>().mockRejectedValue(new Error('x')),
    });
    type(fixture, input, 'ex');
    await settle(fixture, PALETTE_DEBOUNCE_MS);
    const retry = Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Reintentar',
    );
    if (!retry) {
      throw new Error('retry button not rendered');
    }
    retry.focus();
    keyDown(fixture, dialogOf(el), 'Tab');
    expect(document.activeElement).toBe(input);
  });

  it('closes when the backdrop is clicked', () => {
    const { fixture, el } = setup();
    const overlay = dialogOf(el).parentElement as HTMLElement;
    overlay.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    fixture.detectChanges();
    expect(fixture.componentInstance.open()).toBe(false);
  });

  it('pulls focus back when something outside the dialog takes it', () => {
    const outside = document.createElement('input');
    document.body.appendChild(outside);
    const { fixture, input } = setup();
    outside.focus();
    fixture.detectChanges();
    expect(document.activeElement).toBe(input);
    outside.remove();
  });

  it('highlights the typed words and shows the key of a result as a chip', async () => {
    vi.useFakeTimers();
    const { fixture, el, input } = setup();
    type(fixture, input, 'export');
    await settle(fixture, PALETTE_DEBOUNCE_MS);

    const marks = Array.from(el.querySelectorAll('.command-palette__mark')).map(
      (mark) => mark.textContent,
    );
    expect(marks).toContain('Export');
    expect(
      findOption(el, /Exportar datos/).querySelector('.command-palette__option-key')?.textContent,
    ).toContain('CORE-12');
  });

  it('lists the keyboard hints in a footer', () => {
    const { el } = setup();
    const footer = el.querySelector('.command-palette__footer');
    expect(footer?.textContent).toContain('Navegar');
    expect(footer?.textContent).toContain('Abrir');
    expect(footer?.textContent).toContain('Cerrar');
  });
});
