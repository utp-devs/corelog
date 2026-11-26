import { DOCUMENT, Injectable, computed, inject, signal } from '@angular/core';

import { LOCATION_ASSIGN, LOGIN_URL } from '../config/navigation';
import { StorageService } from './storage.service';

/** Same key as the React app (ui/src/adapters/out/storage/token-storage.ts), so both share the session. */
export const AUTH_TOKEN_KEY = 'corelog.auth.token';

/** Owns the bearer token. The token lives in localStorage, shared by every CoreLog app on the origin. */
@Injectable({ providedIn: 'root' })
export class SessionService {
  private readonly document = inject(DOCUMENT);
  private readonly storage = inject(StorageService);
  private readonly navigate = inject(LOCATION_ASSIGN);
  private readonly loginUrl = inject(LOGIN_URL);

  private readonly tokenState = signal<string | null>(this.storage.getItem(AUTH_TOKEN_KEY));

  readonly isAuthenticated = computed(() => this.tokenState() !== null);

  constructor() {
    // Another tab (or the React app) may sign in or out while this app is open.
    this.document.defaultView?.addEventListener('storage', (event: StorageEvent) => {
      if (event.key === AUTH_TOKEN_KEY || event.key === null) {
        this.tokenState.set(this.storage.getItem(AUTH_TOKEN_KEY));
      }
    });
  }

  token(): string | null {
    return this.tokenState();
  }

  startSession(token: string): void {
    this.storage.setItem(AUTH_TOKEN_KEY, token);
    this.tokenState.set(token);
  }

  clear(): void {
    this.storage.removeItem(AUTH_TOKEN_KEY);
    this.tokenState.set(null);
  }

  /** Ends the session and sends the user to the login page, returning to the current page. */
  expire(): void {
    this.clear();
    this.redirectToLogin();
  }

  redirectToLogin(): void {
    const location = this.document.defaultView?.location;
    const returnUrl = location ? `${location.pathname}${location.search}` : '/';
    this.navigate(`${this.loginUrl}?returnUrl=${encodeURIComponent(returnUrl)}`);
  }
}
