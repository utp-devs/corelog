import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Toolbar, ToolbarLeft, ToolbarRight, ToolbarSecondary } from './toolbar';

const OVERFLOW_QUERY = '(max-width: 1279.98px)';

// Stubs matchMedia: the overflow query matches when `narrow` is true.
function mockViewport(narrow: boolean): () => void {
  const original = window.matchMedia;
  window.matchMedia = ((query: string) => ({
    matches: narrow && query === OVERFLOW_QUERY,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  return () => {
    window.matchMedia = original;
  };
}

@Component({
  imports: [Toolbar, ToolbarLeft, ToolbarRight, ToolbarSecondary],
  template: `
    <app-toolbar [sticky]="sticky()">
      @if (showLeft()) {
        <div appToolbarLeft>
          <input aria-label="Buscar" />
        </div>
      }
      <div appToolbarRight>
        <button type="button">Crear</button>
      </div>
      @if (showSecondary()) {
        <button appToolbarSecondary type="button">Exportar</button>
      }
    </app-toolbar>
  `,
})
class Host {
  readonly sticky = signal(false);
  readonly showLeft = signal(true);
  readonly showSecondary = signal(true);
}

function create(): { fixture: ComponentFixture<Host>; el: HTMLElement } {
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

/** Searches the whole document: the overflow menu lives in the overlay container. */
function buttonByText(_el: HTMLElement, text: string): HTMLButtonElement | null {
  return (
    Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === text) ??
    null
  );
}

describe('Toolbar', () => {
  let restore: () => void = () => undefined;

  beforeEach(() => {
    restore = mockViewport(false);
    TestBed.configureTestingModule({ imports: [Host] });
  });

  afterEach(() => {
    restore();
  });

  it('is exposed as a named toolbar and renders left and right slots', () => {
    const { el } = create();
    const toolbar = el.querySelector('[role="toolbar"]');
    expect(toolbar?.getAttribute('aria-label')).toBe('Barra de acciones');
    expect(toolbar?.querySelector('input[aria-label="Buscar"]')).not.toBeNull();
    expect(buttonByText(el, 'Crear')).not.toBeNull();
  });

  it('shows secondary actions inline at 1280 px and wider, with no "Más" menu', () => {
    const { el } = create();
    expect(buttonByText(el, 'Exportar')).not.toBeNull();
    expect(buttonByText(el, 'Más')).toBeNull();
  });

  it('moves secondary actions into a "Más" menu below 1280 px', () => {
    restore();
    restore = mockViewport(true);
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ imports: [Host] });
    const { fixture, el } = create();

    expect(buttonByText(el, 'Exportar')).toBeNull();
    const more = buttonByText(el, 'Más');
    expect(more).not.toBeNull();
    expect(more?.getAttribute('aria-haspopup')).toBe('true');

    more?.click();
    fixture.detectChanges();
    expect(buttonByText(el, 'Exportar')).not.toBeNull();
    expect(buttonByText(el, 'Exportar')?.closest('.cdk-overlay-container')).not.toBeNull();
    expect(more?.getAttribute('aria-expanded')).toBe('true');
    expect(document.getElementById(more?.getAttribute('aria-controls') ?? '')).not.toBeNull();
  });

  it('keeps primary actions visible when the overflow menu is used', () => {
    restore();
    restore = mockViewport(true);
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ imports: [Host] });
    const { fixture, el } = create();
    buttonByText(el, 'Más')?.click();
    fixture.detectChanges();
    expect(buttonByText(el, 'Exportar')).not.toBeNull();
    expect(buttonByText(el, 'Crear')).not.toBeNull();
  });

  it('closes the overflow menu on Escape and on a click outside the toolbar', () => {
    restore();
    restore = mockViewport(true);
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ imports: [Host] });
    const { fixture, el } = create();

    buttonByText(el, 'Más')?.click();
    fixture.detectChanges();
    expect(buttonByText(el, 'Exportar')).not.toBeNull();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    fixture.detectChanges();
    expect(buttonByText(el, 'Exportar')).toBeNull();

    buttonByText(el, 'Más')?.click();
    fixture.detectChanges();
    document.body.click();
    fixture.detectChanges();
    expect(buttonByText(el, 'Exportar')).toBeNull();
  });

  it('does not render a "Más" button when there are no secondary actions', () => {
    restore();
    restore = mockViewport(true);
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ imports: [Host] });
    const { fixture, el } = create();
    fixture.componentInstance.showSecondary.set(false);
    fixture.detectChanges();
    expect(buttonByText(el, 'Más')).toBeNull();
  });

  it('marks the toolbar as sticky only when requested', () => {
    const { fixture, el } = create();
    const toolbar = el.querySelector('[role="toolbar"]');
    expect(toolbar?.getAttribute('data-sticky')).toBe('false');
    fixture.componentInstance.sticky.set(true);
    fixture.detectChanges();
    expect(toolbar?.getAttribute('data-sticky')).toBe('true');
  });
});
