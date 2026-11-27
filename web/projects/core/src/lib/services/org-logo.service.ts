import { Injectable, OnDestroy, Signal, WritableSignal, inject, signal } from '@angular/core';

import { OrganizationApiService } from './api/organization-api.service';

interface CacheEntry {
  readonly url: WritableSignal<string | null>;
  refs: number;
  objectUrl: string | null;
}

/** A reader of one organization logo. Call `release` when the view goes away. */
export interface OrgLogoWatch {
  /** Object URL once loaded; null while loading, without a logo, or when the fetch failed (show initials). */
  readonly url: Signal<string | null>;
  release(): void;
}

const NO_LOGO: Signal<string | null> = signal<string | null>(null).asReadonly();

/**
 * Object URLs of organization logos. The logo endpoint needs the bearer token and <img> cannot send it,
 * so the image is fetched as a blob and shown through an object URL. Readers of the same organization and
 * revision share one request and one URL; the URL is revoked when the last reader releases it. A new
 * `revision` (bumped after an upload or removal) loads the image again.
 */
@Injectable({ providedIn: 'root' })
export class OrgLogoService implements OnDestroy {
  private readonly api = inject(OrganizationApiService);
  private readonly entries = new Map<string, CacheEntry>();

  /** An organization without a logo gets no request and no URL. */
  watch(orgId: string, hasLogo: boolean, revision = 0): OrgLogoWatch {
    if (!hasLogo) {
      return { url: NO_LOGO, release: () => undefined };
    }
    const key = `${orgId}|${revision}`;
    let entry = this.entries.get(key);
    if (!entry) {
      entry = this.load(key, orgId);
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

  private load(key: string, orgId: string): CacheEntry {
    const entry: CacheEntry = { url: signal<string | null>(null), refs: 0, objectUrl: null };
    this.api.fetchLogoBlob(orgId).subscribe({
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
