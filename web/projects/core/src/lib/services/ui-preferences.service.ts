import { DOCUMENT, Injectable, inject, signal } from '@angular/core';

import { StorageService } from './storage.service';

/** Same key and JSON format as the data table density choice, so both stay in step. */
export const DENSITY_STORAGE_KEY = 'corelog.density';
/** Sidebar state: 'collapsed' or 'expanded'. Absent means "follow the viewport width". */
export const SIDEBAR_STORAGE_KEY = 'corelog.sidebar';
/** Window event dispatched on every density change so open tables follow the choice at once. */
export const DENSITY_EVENT = 'corelog:density';

export type UiDensity = 'comfortable' | 'compact';

function isDensity(value: unknown): value is UiDensity {
  return value === 'comfortable' || value === 'compact';
}

/** Density from the stored JSON string. Missing or unknown values fall back to comfortable. */
export function parseStoredDensity(raw: string | null): UiDensity {
  try {
    const parsed: unknown = JSON.parse(raw ?? 'null');
    return isDensity(parsed) ? parsed : 'comfortable';
  } catch {
    return 'comfortable';
  }
}

/** Sidebar choice from the stored string: true for collapsed, false for expanded, null when not chosen. */
export function parseStoredSidebar(raw: string | null): boolean | null {
  if (raw === 'collapsed') return true;
  if (raw === 'expanded') return false;
  return null;
}

/**
 * Interface preferences kept in localStorage: table density and sidebar state. The density is also
 * applied as `data-density` on the html element and announced with a window event.
 */
@Injectable({ providedIn: 'root' })
export class UiPreferencesService {
  private readonly storage = inject(StorageService);
  private readonly document = inject(DOCUMENT);

  private readonly densityState = signal<UiDensity>(
    parseStoredDensity(this.storage.getItem(DENSITY_STORAGE_KEY)),
  );
  private readonly sidebarState = signal<boolean | null>(
    parseStoredSidebar(this.storage.getItem(SIDEBAR_STORAGE_KEY)),
  );

  readonly density = this.densityState.asReadonly();
  /** Stored sidebar choice; null until the user chooses, so the layout follows the viewport width. */
  readonly sidebarCollapsed = this.sidebarState.asReadonly();

  constructor() {
    this.applyDensity(this.densityState());
  }

  setDensity(density: UiDensity): void {
    this.storage.setItem(DENSITY_STORAGE_KEY, JSON.stringify(density));
    this.densityState.set(density);
    this.applyDensity(density);
    this.document.defaultView?.dispatchEvent(
      new CustomEvent<UiDensity>(DENSITY_EVENT, { detail: density }),
    );
  }

  setSidebarCollapsed(collapsed: boolean): void {
    this.storage.setItem(SIDEBAR_STORAGE_KEY, collapsed ? 'collapsed' : 'expanded');
    this.sidebarState.set(collapsed);
  }

  private applyDensity(density: UiDensity): void {
    this.document.documentElement.dataset['density'] = density;
  }
}
