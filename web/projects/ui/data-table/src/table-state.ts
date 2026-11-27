import { DestroyRef, Signal, computed, effect, inject, signal } from '@angular/core';

/** Same key and event as the layout preferences: density changes reach every open table. */
export const DENSITY_STORAGE_KEY = 'corelog.density';
const DENSITY_EVENT = 'corelog:density';

/** Reads a JSON value from localStorage, falling back when it is missing or unreadable. */
export function readStoredValue<T>(storageKey: string, fallback: T): T {
  try {
    const raw = window.localStorage.getItem(storageKey);
    return raw === null ? fallback : (JSON.parse(raw) as T);
  } catch {
    return fallback;
  }
}

/** Writes a JSON value to localStorage. Density writes are broadcast to the other tables. */
export function writeStoredValue<T>(storageKey: string, value: T): void {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(value));
  } catch {
    // Storage unavailable (private mode or quota): the value stays in memory only.
  }
  if (storageKey === DENSITY_STORAGE_KEY) {
    window.dispatchEvent(new CustomEvent<T>(DENSITY_EVENT, { detail: value }));
  }
}

/** Value persisted as JSON. Must be created in an injection context. */
export interface PersistentState<T> {
  readonly value: Signal<T>;
  set(next: T): void;
}

/**
 * Signal persisted in localStorage. The density key also follows changes made by other
 * tables or by the layout preferences, without a reload.
 */
export function persistentState<T>(storageKey: string, initial: T): PersistentState<T> {
  const state = signal<T>(readStoredValue(storageKey, initial));
  if (storageKey === DENSITY_STORAGE_KEY) {
    const onDensity = (event: Event) => state.set((event as CustomEvent<T>).detail);
    window.addEventListener(DENSITY_EVENT, onDensity);
    inject(DestroyRef).onDestroy(() => window.removeEventListener(DENSITY_EVENT, onDensity));
  }
  return {
    value: state.asReadonly(),
    set(next: T) {
      state.set(next);
      writeStoredValue(storageKey, next);
    },
  };
}

/** Tracks a CSS media query. Without matchMedia support the query never matches. */
export function mediaQuery(query: string): Signal<boolean> {
  const supported = typeof window !== 'undefined' && typeof window.matchMedia === 'function';
  if (!supported) {
    return signal(false).asReadonly();
  }
  const list = window.matchMedia(query);
  const matches = signal(list.matches);
  const onChange = () => matches.set(list.matches);
  list.addEventListener('change', onChange);
  inject(DestroyRef).onDestroy(() => list.removeEventListener('change', onChange));
  return matches.asReadonly();
}

/** True only while `active` stays true for `delayMs`. Avoids flicker on fast loads. */
export function delayedFlag(active: Signal<boolean>, delayMs: number): Signal<boolean> {
  const elapsed = signal(false);
  effect((onCleanup) => {
    if (!active()) {
      elapsed.set(false);
      return;
    }
    const timer = window.setTimeout(() => elapsed.set(true), delayMs);
    onCleanup(() => window.clearTimeout(timer));
  });
  return computed(() => active() && elapsed());
}
