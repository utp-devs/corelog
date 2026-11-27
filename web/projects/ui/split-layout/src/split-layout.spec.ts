import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { SplitLayout, SplitSideWidth } from './split-layout';

const STACKED_QUERY = '(max-width: 991.98px)';

// Stubs matchMedia: the stacked query matches when `narrow` is true.
function mockViewport(narrow: boolean): () => void {
  const original = window.matchMedia;
  window.matchMedia = ((query: string) => ({
    matches: narrow && query === STACKED_QUERY,
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
  imports: [SplitLayout],
  template: `
    <app-split-layout [sideWidth]="width()" [sideLabel]="label()">
      <p appSplitMain>Incidencia ABC-1</p>
      <p appSplitSide>Detalles de la incidencia</p>
    </app-split-layout>
  `,
})
class Host {
  readonly width = signal<SplitSideWidth>(320);
  readonly label = signal('Panel lateral');
}

describe('SplitLayout', () => {
  let restore: () => void = () => undefined;

  beforeEach(() => {
    restore = mockViewport(false);
    TestBed.configureTestingModule({ imports: [Host] });
  });

  afterEach(() => {
    restore();
  });

  it('renders main content and the side panel in a named aside landmark', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Incidencia ABC-1');
    const aside = el.querySelector('aside');
    expect(aside?.getAttribute('aria-label')).toBe('Panel lateral');
    expect(aside?.textContent).toContain('Detalles de la incidencia');
  });

  it('uses sideLabel as the landmark name', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.label.set('Detalles');
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('aside')?.getAttribute('aria-label')).toBe(
      'Detalles',
    );
  });

  it('defaults to a 320 px side panel and accepts 360 and 400', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const layout = fixture.nativeElement.querySelector('.layout') as HTMLElement;
    expect(layout.getAttribute('data-side-width')).toBe('320');
    fixture.componentInstance.width.set(400);
    fixture.detectChanges();
    expect(layout.getAttribute('data-side-width')).toBe('400');
  });

  it('is not stacked on wide viewports', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.layout').getAttribute('data-stacked')).toBe(
      'false',
    );
  });

  it('stacks the side panel below the main content under 992 px', () => {
    restore();
    restore = mockViewport(true);
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ imports: [Host] });
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.layout').getAttribute('data-stacked')).toBe(
      'true',
    );
  });
});
