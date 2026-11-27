import { DestroyRef, DOCUMENT, Injectable, Signal, computed, inject, signal } from '@angular/core';

/** Breakpoints in px, compared against the viewport width (min-width semantics). */
export const BREAKPOINTS = {
  xs: 576,
  md: 768,
  lg: 992,
  xl: 1280,
  xxl: 1680,
} as const;

export type Breakpoint = keyof typeof BREAKPOINTS;

/** Media query that matches a viewport at or above the breakpoint. */
export function breakpointQuery(breakpoint: Breakpoint): string {
  return `(min-width: ${BREAKPOINTS[breakpoint]}px)`;
}

/**
 * Signal-based media queries. Each query has one MediaQueryList for the lifetime of the service;
 * the signal follows its `change` events. Without matchMedia (SSR, some test hosts) it is false.
 */
@Injectable({ providedIn: 'root' })
export class BreakpointService {
  private readonly document = inject(DOCUMENT);
  private readonly queries = new Map<string, Signal<boolean>>();
  private readonly disposers: Array<() => void> = [];

  constructor() {
    inject(DestroyRef).onDestroy(() => this.disposers.forEach((dispose) => dispose()));
  }

  /** True while the media query matches. Updates when the viewport changes. */
  matches(query: string): Signal<boolean> {
    const cached = this.queries.get(query);
    if (cached) return cached;
    const created = this.createQuery(query);
    this.queries.set(query, created);
    return created;
  }

  /** Viewport at or above each breakpoint. */
  readonly breakpoints = computed<Record<Breakpoint, boolean>>(() => ({
    xs: this.matches(breakpointQuery('xs'))(),
    md: this.matches(breakpointQuery('md'))(),
    lg: this.matches(breakpointQuery('lg'))(),
    xl: this.matches(breakpointQuery('xl'))(),
    xxl: this.matches(breakpointQuery('xxl'))(),
  }));

  private createQuery(query: string): Signal<boolean> {
    const win = this.document.defaultView;
    if (!win || typeof win.matchMedia !== 'function') return signal(false).asReadonly();

    const list = win.matchMedia(query);
    const value = signal(list.matches);
    const onChange = () => value.set(list.matches);
    list.addEventListener('change', onChange);
    this.disposers.push(() => list.removeEventListener('change', onChange));
    return value.asReadonly();
  }
}
