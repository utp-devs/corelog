import { APP_BASE_HREF, PlatformLocation } from '@angular/common';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { PALETTE_SEARCH_DEBOUNCE_MS, SearchApiService, type SearchHit } from '@corelog/core';
import { CommandRegistry } from '@corelog/ui/command-palette';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { GlobalSearch, HEADER_SEARCH_DEBOUNCE_MS } from './global-search';

const hits: SearchHit[] = [
  { kind: 'issue', id: 'i-1', title: 'Exportar datos', subtitle: 'CORE', link: '/browse/i-1' },
];

describe('GlobalSearch', () => {
  let fixture: ComponentFixture<GlobalSearch>;
  let el: HTMLElement;
  let search: ReturnType<typeof vi.fn>;
  let navigate: ReturnType<typeof vi.spyOn>;

  function setup(results: SearchHit[] = hits, compact = false): void {
    search = vi.fn(() => of(results));
    TestBed.configureTestingModule({
      imports: [GlobalSearch],
      providers: [provideRouter([]), { provide: SearchApiService, useValue: { search } }],
    });
    navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
    fixture = TestBed.createComponent(GlobalSearch);
    fixture.componentRef.setInput('compact', compact);
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  function type(value: string): void {
    const input = el.querySelector<HTMLInputElement>('input[type="search"]');
    if (!input) throw new Error('search field not rendered');
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
  }

  function pressWindowKey(init: KeyboardEventInit): void {
    window.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, ...init }));
    fixture.detectChanges();
  }

  const paletteDialog = (): HTMLElement | null =>
    document.querySelector<HTMLElement>('[role="dialog"][aria-label="Paleta de comandos"]');

  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    TestBed.inject(CommandRegistry).clear();
    TestBed.resetTestingModule();
  });

  it('does not query the API for a single character', () => {
    setup();
    type('e');
    vi.advanceTimersByTime(HEADER_SEARCH_DEBOUNCE_MS + 50);
    expect(search).not.toHaveBeenCalled();
  });

  it('searches after the user pauses and shows the hits', () => {
    setup();
    type('ex');
    vi.advanceTimersByTime(HEADER_SEARCH_DEBOUNCE_MS);
    fixture.detectChanges();
    expect(el.textContent).toContain('Exportar datos');
    expect(search).toHaveBeenCalledWith('ex', 10);
  });

  it('reports when nothing matches', () => {
    setup([]);
    type('zz');
    vi.advanceTimersByTime(HEADER_SEARCH_DEBOUNCE_MS);
    fixture.detectChanges();
    expect(el.querySelector('[role="status"]')?.textContent).toContain('Sin resultados');
  });

  it('opens the command palette with Ctrl+K and closes it with Escape', () => {
    setup();
    expect(paletteDialog()).toBeNull();
    pressWindowKey({ key: 'k', ctrlKey: true });
    expect(paletteDialog()).not.toBeNull();
    paletteDialog()?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();
    expect(paletteDialog()).toBeNull();
  });

  it('opens the command palette with "/" and keeps the header field out of focus', () => {
    setup();
    const field = el.querySelector<HTMLInputElement>('input[type="search"]');
    pressWindowKey({ key: '/' });
    expect(paletteDialog()).not.toBeNull();
    expect(field).not.toBe(document.activeElement);
  });

  it('navigates to a registered command when it is chosen', () => {
    setup();
    pressWindowKey({ key: 'k', ctrlKey: true });
    const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(
      (item) => /Ir a Incidencias/.test(item.textContent ?? ''),
    );
    option?.click();
    fixture.detectChanges();
    expect(navigate).toHaveBeenCalledWith('/issues');
    expect(paletteDialog()).toBeNull();
  });

  it('navigates to the hit link when a remote result is chosen with Enter', async () => {
    setup();
    pressWindowKey({ key: 'k', ctrlKey: true });
    const input = document.querySelector<HTMLInputElement>('input[role="combobox"]');
    if (!input) throw new Error('palette input not rendered');
    input.value = 'exportar';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    vi.advanceTimersByTime(PALETTE_SEARCH_DEBOUNCE_MS);
    // The search result arrives through a promise: let the pending microtasks run.
    for (let tick = 0; tick < 5; tick++) {
      await Promise.resolve();
    }
    fixture.detectChanges();
    const hit = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find((item) =>
      /Exportar datos/.test(item.textContent ?? ''),
    );
    hit?.dispatchEvent(new MouseEvent('mouseenter'));
    paletteDialog()?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    fixture.detectChanges();
    expect(navigate).toHaveBeenCalledWith('/browse/i-1');
  });

  it('shows only the icon button on narrow layouts and opens the palette from it', () => {
    setup([], true);
    expect(el.querySelector('input[type="search"]')).toBeNull();
    el.querySelector<HTMLButtonElement>('button[aria-label="Buscar"]')?.click();
    fixture.detectChanges();
    expect(paletteDialog()).not.toBeNull();
  });
});

describe('GlobalSearch under the workspace base href', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('opens a search hit under the workspace base without repeating it', async () => {
    const search = vi.fn(() => of(hits));
    TestBed.configureTestingModule({
      imports: [GlobalSearch],
      providers: [
        provideRouter([{ path: 'browse/:id', children: [] }]),
        { provide: APP_BASE_HREF, useValue: '/workspace/' },
        { provide: SearchApiService, useValue: { search } },
      ],
    });
    const fixture = TestBed.createComponent(GlobalSearch);
    fixture.detectChanges();
    const router = TestBed.inject(Router);
    await router.navigateByUrl(hits[0].link);
    expect(router.url).toBe('/browse/i-1');
    expect(TestBed.inject(PlatformLocation).pathname).toBe('/workspace/browse/i-1');
  });
});
