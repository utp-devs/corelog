import { Location } from '@angular/common';
import { Injectable, InjectionToken, inject, signal } from '@angular/core';
import type { NavigationError } from '@angular/router';

/** Session key holding the time of the last automatic reload caused by a missing chunk. */
export const CHUNK_RELOAD_KEY = 'corelog.chunk-reload-at';
/** A second chunk failure inside this window is not retried: the notice is shown instead. */
export const CHUNK_RELOAD_WINDOW_MS = 60_000;

const CHUNK_ERROR_PATTERNS: readonly RegExp[] = [
  /failed to fetch dynamically imported module/i,
  /error loading dynamically imported module/i,
  /importing a module script failed/i,
  /loading chunk [\w-]+ failed/i,
  /loading css chunk/i,
];

/** True when an error comes from a lazy chunk that could not be downloaded (usually after a new deploy). */
export function isChunkLoadError(error: unknown): boolean {
  if (error === null || error === undefined) {
    return false;
  }
  const name = (error as { name?: unknown }).name;
  if (name === 'ChunkLoadError') {
    return true;
  }
  const message =
    typeof error === 'string' ? error : String((error as { message?: unknown }).message ?? '');
  return CHUNK_ERROR_PATTERNS.some((pattern) => pattern.test(message));
}

/** Reload allowed when none happened in the last window (or the stored value is unusable). */
export function mayReload(stored: string | null, now: number): boolean {
  const at = Number(stored);
  if (!stored || !Number.isFinite(at) || at > now) {
    return true;
  }
  return now - at > CHUNK_RELOAD_WINDOW_MS;
}

/** Browser effects of the recovery, replaceable in tests. */
export interface ChunkRecoveryEnv {
  readonly now: () => number;
  readonly read: () => string | null;
  readonly write: (value: string) => void;
  readonly reload: (target: string) => void;
}

export const CHUNK_RECOVERY_ENV = new InjectionToken<ChunkRecoveryEnv>('CHUNK_RECOVERY_ENV', {
  providedIn: 'root',
  factory: () => ({
    now: () => Date.now(),
    read: () => {
      try {
        return sessionStorage.getItem(CHUNK_RELOAD_KEY);
      } catch {
        return null;
      }
    },
    write: (value) => {
      try {
        sessionStorage.setItem(CHUNK_RELOAD_KEY, value);
      } catch {
        /* storage unavailable: the notice is the fallback */
      }
    },
    reload: (target) => window.location.assign(target),
  }),
});

/**
 * Recovers from a lazy route whose chunk no longer exists on the server (the page was loaded before a new
 * build). The first failure reloads the page once on the target URL; a second failure within a minute
 * shows a notice with a reload button instead of doing nothing.
 */
@Injectable({ providedIn: 'root' })
export class ChunkRecovery {
  private readonly env = inject(CHUNK_RECOVERY_ENV);
  private readonly location = inject(Location);

  private readonly staleSignal = signal(false);
  /** True when the app is out of date and the automatic reload already happened. */
  readonly stale = this.staleSignal.asReadonly();

  /** Handles a failed navigation. Returns true when it was a chunk failure (reload or notice). */
  handle(error: NavigationError): boolean {
    if (!isChunkLoadError(error.error)) {
      return false;
    }
    const now = this.env.now();
    if (mayReload(this.env.read(), now)) {
      this.env.write(String(now));
      this.env.reload(this.location.prepareExternalUrl(error.url));
    } else {
      this.staleSignal.set(true);
    }
    return true;
  }

  /** Reload button of the notice: hard reload of the current page. */
  reloadNow(): void {
    this.env.reload(window.location.href);
  }
}
