import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { HeroCarousel, type HeroSlide } from './hero-carousel';

const SLIDES: readonly HeroSlide[] = [
  { src: '/images/a.webp', caption: 'Primera', position: '70% 40%' },
  { src: '/images/b.webp', caption: 'Segunda' },
  { src: '/images/c.webp', caption: 'Tercera' },
];

@Component({
  imports: [HeroCarousel],
  template: `<app-hero-carousel [slides]="slides" label="Fotos"
    ><h2>Titular</h2></app-hero-carousel
  >`,
})
class Host {
  slides = SLIDES;
}

function stubReducedMotion(matches: boolean): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: matches && query.includes('prefers-reduced-motion'),
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }),
  });
}

function setup(): { fixture: ComponentFixture<Host>; el: HTMLElement } {
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

function activeIndex(el: HTMLElement): number {
  return Array.from(el.querySelectorAll('.hero-carousel__image')).findIndex((img) =>
    img.classList.contains('hero-carousel__image--active'),
  );
}

function click(el: HTMLElement, selector: string, position = 0): void {
  (el.querySelectorAll(selector)[position] as HTMLElement).click();
}

describe('HeroCarousel', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    stubReducedMotion(false);
  });

  afterEach(() => {
    vi.useRealTimers();
    Reflect.deleteProperty(window, 'matchMedia');
  });

  it('renders decorative images, the first eager with high priority and the rest lazy', () => {
    const { el } = setup();
    const images = Array.from(el.querySelectorAll('img'));
    expect(images).toHaveLength(3);
    expect(images.every((img) => img.getAttribute('alt') === '')).toBe(true);
    expect(images[0].getAttribute('loading')).toBe('eager');
    expect(images[0].getAttribute('fetchpriority')).toBe('high');
    expect(images[1].getAttribute('loading')).toBe('lazy');
    expect(images[1].hasAttribute('fetchpriority')).toBe(false);
    expect(images[0].getAttribute('width')).toBe('1536');
    expect(images[0].style.getPropertyValue('--hero-position')).toBe('70% 40%');
  });

  it('projects content and names the region', () => {
    const { el } = setup();
    expect(el.querySelector('h2')?.textContent).toBe('Titular');
    const region = el.querySelector('[role="region"]');
    expect(region?.getAttribute('aria-roledescription')).toBe('carrusel');
    expect(region?.getAttribute('aria-label')).toBe('Fotos');
  });

  it('advances every 7 seconds and wraps around', () => {
    const { fixture, el } = setup();
    expect(activeIndex(el)).toBe(0);
    vi.advanceTimersByTime(7000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(1);
    vi.advanceTimersByTime(14000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(0);
  });

  it('pauses while the pointer is over it and resumes afterwards', () => {
    const { fixture, el } = setup();
    const host = el.querySelector('app-hero-carousel') as HTMLElement;
    host.dispatchEvent(new Event('mouseenter'));
    fixture.detectChanges();
    vi.advanceTimersByTime(21000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(0);
    host.dispatchEvent(new Event('mouseleave'));
    fixture.detectChanges();
    vi.advanceTimersByTime(7000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(1);
  });

  it('pauses while focus is inside', () => {
    const { fixture, el } = setup();
    const host = el.querySelector('app-hero-carousel') as HTMLElement;
    host.dispatchEvent(new Event('focusin'));
    fixture.detectChanges();
    vi.advanceTimersByTime(14000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(0);
    host.dispatchEvent(new Event('focusout'));
    fixture.detectChanges();
    vi.advanceTimersByTime(7000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(1);
  });

  it('moves with the previous and next buttons', () => {
    const { fixture, el } = setup();
    click(el, 'button[aria-label="Imagen siguiente"]');
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(1);
    click(el, 'button[aria-label="Imagen anterior"]');
    click(el, 'button[aria-label="Imagen anterior"]');
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(2);
  });

  it('has one named dot per slide and marks the current one', () => {
    const { fixture, el } = setup();
    const dots = Array.from(el.querySelectorAll('.hero-carousel__dot'));
    expect(dots.map((d) => d.getAttribute('aria-label'))).toEqual([
      'Ir a la imagen 1 de 3',
      'Ir a la imagen 2 de 3',
      'Ir a la imagen 3 de 3',
    ]);
    click(el, '.hero-carousel__dot', 2);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(2);
    expect(dots[2].getAttribute('aria-current')).toBe('true');
    expect(dots[0].hasAttribute('aria-current')).toBe(false);
  });

  it('supports the arrow keys', () => {
    const { fixture, el } = setup();
    const host = el.querySelector('app-hero-carousel') as HTMLElement;
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(1);
    host.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(0);
  });

  it('stops advancing after the pause button and the live region becomes polite', () => {
    const { fixture, el } = setup();
    const caption = el.querySelector('.hero-carousel__caption') as HTMLElement;
    expect(caption.getAttribute('aria-live')).toBe('off');
    click(el, 'button[aria-label="Pausar la rotación"]');
    fixture.detectChanges();
    vi.advanceTimersByTime(21000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(0);
    expect(caption.getAttribute('aria-live')).toBe('polite');
    expect(el.querySelector('button[aria-label="Reanudar la rotación"]')).not.toBeNull();
  });

  it('announces the caption of the current slide', () => {
    const { fixture, el } = setup();
    click(el, 'button[aria-label="Imagen siguiente"]');
    fixture.detectChanges();
    const caption = el.querySelector('.hero-carousel__caption') as HTMLElement;
    expect(caption.textContent).toContain('2 de 3: Segunda');
  });

  it('never advances and hides the pause button with reduced motion', () => {
    stubReducedMotion(true);
    const { fixture, el } = setup();
    vi.advanceTimersByTime(30000);
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(0);
    expect(el.querySelector('app-hero-carousel')?.getAttribute('data-reduced-motion')).toBe('true');
    expect(el.querySelector('button[aria-label="Pausar la rotación"]')).toBeNull();
    click(el, 'button[aria-label="Imagen siguiente"]');
    fixture.detectChanges();
    expect(activeIndex(el)).toBe(1);
  });

  it('shows no controls for a single slide', () => {
    const fixture = TestBed.createComponent(HeroCarousel);
    fixture.componentRef.setInput('slides', [SLIDES[0]]);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.hero-carousel__controls')).toBeNull();
  });

  it('emits the active slide index, starting with the first one', () => {
    const fixture = TestBed.createComponent(HeroCarousel);
    fixture.componentRef.setInput('slides', SLIDES);
    const seen: number[] = [];
    fixture.componentInstance.slideChange.subscribe((i) => seen.push(i));
    fixture.detectChanges();
    (fixture.nativeElement as HTMLElement)
      .querySelector<HTMLElement>('button[aria-label="Imagen siguiente"]')
      ?.click();
    fixture.detectChanges();
    expect(seen).toEqual([0, 1]);
  });
});
