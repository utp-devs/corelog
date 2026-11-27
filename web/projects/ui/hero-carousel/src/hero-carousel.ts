import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';

/** One photograph of the carousel. The image is decorative: the caption carries the meaning. */
export interface HeroSlide {
  readonly src: string;
  /** Short text announced and shown when the slide is active. */
  readonly caption: string;
  /** CSS object-position of the crop, for example `70% 40%`. Defaults to the center. */
  readonly position?: string;
  readonly width?: number;
  readonly height?: number;
}

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

/**
 * Photo carousel for hero areas. Content projected into it (headline, text) sits above
 * the photographs. It advances by itself every `intervalMs`, pauses while the pointer or focus is inside and can be
 * paused with the pause button; with reduced motion it never advances alone and has no transitions.
 * The first image loads eagerly with high priority, the others lazily.
 */
@Component({
  selector: 'app-hero-carousel',
  templateUrl: './hero-carousel.html',
  styleUrl: './hero-carousel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(mouseenter)': 'hovered.set(true)',
    '(mouseleave)': 'hovered.set(false)',
    '(focusin)': 'focused.set(true)',
    '(focusout)': 'focused.set(false)',
    '(keydown)': 'onKeydown($event)',
    '[attr.data-reduced-motion]': 'reducedMotion() ? "true" : null',
  },
})
export class HeroCarousel {
  readonly slides = input.required<readonly HeroSlide[]>();
  /** Accessible name of the carousel region. */
  readonly label = input('Imágenes destacadas');
  readonly intervalMs = input(7000);
  /** Emits the index of the active slide, also the first one, so a backdrop can follow the carousel. */
  readonly slideChange = output<number>();

  private readonly document = inject(DOCUMENT);

  protected readonly index = signal(0);
  protected readonly hovered = signal(false);
  protected readonly focused = signal(false);
  /** The person pressed pause; kept until they press play. */
  protected readonly userPaused = signal(false);
  protected readonly reducedMotion = signal(false);

  protected readonly count = computed(() => this.slides().length);
  protected readonly current = computed<HeroSlide | undefined>(() => this.slides()[this.index()]);
  protected readonly multiple = computed(() => this.count() > 1);
  /** True while the carousel advances by itself. */
  protected readonly playing = computed(
    () =>
      this.multiple() &&
      !this.reducedMotion() &&
      !this.userPaused() &&
      !this.hovered() &&
      !this.focused(),
  );
  protected readonly status = computed(() => {
    const slide = this.current();
    return slide ? `${this.index() + 1} de ${this.count()}: ${slide.caption}` : '';
  });

  constructor() {
    const query = this.document.defaultView?.matchMedia?.(REDUCED_MOTION_QUERY);
    if (query) {
      this.reducedMotion.set(query.matches);
      const listener = (event: MediaQueryListEvent): void => this.reducedMotion.set(event.matches);
      query.addEventListener?.('change', listener);
      inject(DestroyRef).onDestroy(() => query.removeEventListener?.('change', listener));
    }

    effect(() => this.slideChange.emit(this.index()));

    effect((onCleanup) => {
      if (!this.playing()) {
        return;
      }
      const timer = setInterval(() => this.next(), this.intervalMs());
      onCleanup(() => clearInterval(timer));
    });
  }

  protected next(): void {
    const count = this.count();
    if (count > 0) {
      this.index.update((i) => (i + 1) % count);
    }
  }

  protected previous(): void {
    const count = this.count();
    if (count > 0) {
      this.index.update((i) => (i - 1 + count) % count);
    }
  }

  protected goTo(position: number): void {
    this.index.set(position);
  }

  protected togglePlayback(): void {
    this.userPaused.update((paused) => !paused);
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'ArrowRight') {
      event.preventDefault();
      this.next();
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault();
      this.previous();
    }
  }
}
