import { Injectable, OnDestroy, Signal, WritableSignal, inject, signal } from '@angular/core';

import { AvatarApiService } from './api/avatar-api.service';

interface CacheEntry {
  readonly url: WritableSignal<string | null>;
  refs: number;
  objectUrl: string | null;
}

/** A reader of one avatar. Call `release` when the view goes away. */
export interface AvatarUrlWatch {
  /** Object URL once loaded; null while loading, without avatar, or when the fetch failed (show initials). */
  readonly url: Signal<string | null>;
  release(): void;
}

const NO_AVATAR: Signal<string | null> = signal<string | null>(null).asReadonly();

/**
 * Object URLs of user avatars. The avatar endpoint needs the bearer token and <img> cannot send it, so
 * the image is fetched as a blob and shown through an object URL. Readers of the same user and version
 * share one request and one URL; the URL is revoked when the last reader releases it. A new `version`
 * (for example the avatar key) loads the image again.
 */
@Injectable({ providedIn: 'root' })
export class AvatarUrlService implements OnDestroy {
  private readonly avatars = inject(AvatarApiService);
  private readonly entries = new Map<string, CacheEntry>();

  watch(userId: string, hasAvatar: boolean, version = ''): AvatarUrlWatch {
    if (!hasAvatar) {
      return { url: NO_AVATAR, release: () => undefined };
    }
    const key = `${userId}|${version}`;
    let entry = this.entries.get(key);
    if (!entry) {
      entry = this.load(key, userId);
      this.entries.set(key, entry);
    }
    const current = entry;
    current.refs += 1;
    let released = false;
    return {
      url: current.url.asReadonly(),
      release: () => {
        if (released) return;
        released = true;
        this.release(key, current);
      },
    };
  }

  ngOnDestroy(): void {
    for (const entry of this.entries.values()) {
      this.revoke(entry);
    }
    this.entries.clear();
  }

  private load(key: string, userId: string): CacheEntry {
    const entry: CacheEntry = { url: signal<string | null>(null), refs: 0, objectUrl: null };
    this.avatars.getAvatar(userId).subscribe({
      next: (blob) => {
        // Released before the blob arrived: nobody will show it, so no URL is created.
        if (blob === null || this.entries.get(key) !== entry) return;
        entry.objectUrl = URL.createObjectURL(blob);
        entry.url.set(entry.objectUrl);
      },
      error: () => undefined,
    });
    return entry;
  }

  private release(key: string, entry: CacheEntry): void {
    entry.refs -= 1;
    if (entry.refs > 0) return;
    if (this.entries.get(key) === entry) this.entries.delete(key);
    this.revoke(entry);
  }

  private revoke(entry: CacheEntry): void {
    if (entry.objectUrl) {
      URL.revokeObjectURL(entry.objectUrl);
      entry.objectUrl = null;
    }
    entry.url.set(null);
  }
}
