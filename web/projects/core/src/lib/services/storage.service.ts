import { DOCUMENT, Injectable, inject } from '@angular/core';

/**
 * Thin wrapper over localStorage. Every call is guarded: browsers may throw on access
 * (privacy modes, disabled storage) and the app must keep working without persistence.
 */
@Injectable({ providedIn: 'root' })
export class StorageService {
  private readonly document = inject(DOCUMENT);

  getItem(key: string): string | null {
    return this.safely(() => this.storage()?.getItem(key) ?? null) ?? null;
  }

  setItem(key: string, value: string): void {
    this.safely(() => this.storage()?.setItem(key, value));
  }

  removeItem(key: string): void {
    this.safely(() => this.storage()?.removeItem(key));
  }

  private storage(): Storage | null {
    return this.document.defaultView?.localStorage ?? null;
  }

  private safely<R>(operation: () => R): R | undefined {
    try {
      return operation();
    } catch {
      return undefined;
    }
  }
}
