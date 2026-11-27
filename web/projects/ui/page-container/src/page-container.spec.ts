import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PageContainer, PageSidebar, PageVariant } from './page-container';

const STACKED_QUERY = '(max-width: 991.98px)';

// Stubs matchMedia so the stacked-layout query matches when `narrow` is true.
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
  imports: [PageContainer, PageSidebar],
  template: `
    <app-page-container
      [variant]="variant()"
      [ready]="ready()"
      [pageName]="pageName()"
      [fill]="fill()"
    >
      @if (fillChild()) {
        <section>x</section>
      } @else {
        <p>Contenido de la página</p>
      }
      @if (withSidebar()) {
        <p appPageSidebar>Detalle</p>
      }
    </app-page-container>
  `,
})
class Host {
  readonly variant = signal<PageVariant>('wide');
  readonly ready = signal(false);
  readonly pageName = signal<string | undefined>(undefined);
  readonly fill = signal(false);
  readonly fillChild = signal(false);
  readonly withSidebar = signal(false);
}

function create(): { fixture: ComponentFixture<Host>; host: Host; el: HTMLElement } {
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

describe('PageContainer', () => {
  let restore: () => void = () => undefined;

  beforeEach(() => {
    restore = mockViewport(false);
    TestBed.configureTestingModule({ imports: [Host] });
  });

  afterEach(() => {
    restore();
  });

  it('renders the children inside the container', () => {
    const { el } = create();
    expect(el.textContent).toContain('Contenido de la página');
  });

  it.each(['reading', 'wide', 'fluid'] as const)(
    'exposes the %s variant as data-variant',
    (variant) => {
      const { fixture, host, el } = create();
      host.variant.set(variant);
      fixture.detectChanges();
      expect(el.querySelector('.root')?.getAttribute('data-variant')).toBe(variant);
    },
  );

  it('sets data-ready to "true" when ready is true and "false" otherwise', () => {
    const { fixture, host, el } = create();
    const root = el.querySelector('.root');
    host.ready.set(true);
    fixture.detectChanges();
    expect(root?.getAttribute('data-ready')).toBe('true');
    host.ready.set(false);
    fixture.detectChanges();
    expect(root?.getAttribute('data-ready')).toBe('false');
  });

  it('sets data-page from pageName', () => {
    const { fixture, host, el } = create();
    host.variant.set('fluid');
    host.pageName.set('board');
    fixture.detectChanges();
    expect(el.querySelector('.root')?.getAttribute('data-page')).toBe('board');
  });

  it('does not render an aside landmark without a sidebar', () => {
    const { el } = create();
    expect(el.querySelector('aside')).toBeNull();
    expect(el.querySelector('[role="complementary"]')).toBeNull();
  });

  it('renders the sidebar in an aside landmark, beside the content on wide viewports', () => {
    const { fixture, host, el } = create();
    host.withSidebar.set(true);
    fixture.detectChanges();
    const aside = el.querySelector('aside');
    expect(aside?.getAttribute('aria-label')).toBe('Panel lateral');
    expect(aside?.textContent).toContain('Detalle');
    const root = el.querySelector('.root');
    expect(root?.getAttribute('data-has-sidebar')).toBe('true');
    expect(root?.getAttribute('data-stacked')).toBe('false');
  });

  it('moves the sidebar below the content under 992 px', () => {
    restore();
    restore = mockViewport(true);
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ imports: [Host] });
    const { fixture, host, el } = create();
    host.withSidebar.set(true);
    fixture.detectChanges();
    expect(el.querySelector('.root')?.getAttribute('data-stacked')).toBe('true');
  });

  it('is not filled by default', () => {
    const { el } = create();
    expect(el.querySelector('.root')?.getAttribute('data-fill')).toBe('false');
  });

  it('marks the page as filled with fill, so the content column grows to the viewport', () => {
    const { fixture, host, el } = create();
    host.fill.set(true);
    host.fillChild.set(true);
    fixture.detectChanges();
    expect(el.querySelector('.root')?.getAttribute('data-fill')).toBe('true');
    expect(el.querySelector('section')?.parentElement?.className).toContain('content');
  });
});
